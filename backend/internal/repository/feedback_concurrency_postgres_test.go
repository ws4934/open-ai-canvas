package repository

import (
	"errors"
	"testing"
	"time"

	"yingce/backend/internal/database"
	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestFeedbackResourceDeletionConcurrencyPostgres(t *testing.T) {
	for _, deletion := range []string{"cleanup", "asset_purge"} {
		for _, first := range []string{"feedback", "delete"} {
			t.Run(deletion+"_"+first+"_first", func(t *testing.T) {
				repo, db, application := newBillingConcurrencyRepository(t)
				if err := database.MigrateSchema(db); err != nil {
					t.Fatal(err)
				}
				resource := model.Resource{ID: "resource", UserID: "owner", Kind: "image", Status: model.ResourceStatusReady}
				if err := db.Create(&resource).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&model.Asset{ID: "asset", UserID: "owner", Kind: "image"}).Error; err != nil {
					t.Fatal(err)
				}
				gate := db.Begin()
				if gate.Error != nil {
					t.Fatal(gate.Error)
				}
				defer gate.Rollback()
				if err := gate.Exec("SELECT id FROM resources WHERE id = 'resource' FOR UPDATE").Error; err != nil {
					t.Fatal(err)
				}
				type result struct {
					action string
					err    error
				}
				results := make(chan result, 2)
				actions := []string{first, "delete"}
				if first == "delete" {
					actions[1] = "feedback"
				}
				for i, action := range actions {
					go func(action string) {
						var err error
						if action == "feedback" {
							err = repo.CreateFeedback(&model.UserFeedback{ID: "feedback", UserID: "owner", Attachments: `[{"id":"resource"}]`})
						} else if deletion == "cleanup" {
							err = repo.DeleteDetachedResources([]model.Resource{resource}, nil)
						} else {
							err = repo.DeleteAssetsAndResources("owner", []string{"asset"}, []string{resource.ID}, nil, true)
						}
						results <- result{action, err}
					}(action)
					deadline := time.Now().Add(5 * time.Second)
					for {
						var blocked int64
						if err := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND wait_event_type = 'Lock'", application).Scan(&blocked).Error; err != nil {
							t.Fatal(err)
						}
						if blocked == int64(i+1) {
							break
						}
						if time.Now().After(deadline) {
							t.Fatalf("operation did not reach lock barrier: %d", blocked)
						}
						time.Sleep(10 * time.Millisecond)
					}
				}
				if err := gate.Commit().Error; err != nil {
					t.Fatal(err)
				}
				outcomes := map[string]error{}
				for range 2 {
					outcome := <-results
					outcomes[outcome.action] = outcome.err
				}
				if first == "feedback" {
					if outcomes["feedback"] != nil || !errors.Is(outcomes["delete"], ErrResourceCleanupStillReferenced) {
						t.Fatalf("published attachment was not preserved: %+v", outcomes)
					}
					if _, err := repo.ResourceForUser("owner", resource.ID); err != nil {
						t.Fatal(err)
					}
				} else {
					if outcomes["delete"] != nil || !errors.Is(outcomes["feedback"], gorm.ErrRecordNotFound) {
						t.Fatalf("deleted resource was accepted as attachment: %+v", outcomes)
					}
					if _, err := repo.GetFeedback("feedback"); !errors.Is(err, gorm.ErrRecordNotFound) {
						t.Fatalf("invalid feedback persisted: %v", err)
					}
				}
			})
		}
	}
}
