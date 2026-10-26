package app

import (
	"errors"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestDramaUpdateDrainPreservesQueuedWork(t *testing.T) {
	for _, race := range []bool{false, true} {
		name := "already fenced"
		if race {
			name = "fenced during submission"
		}
		t.Run(name, func(t *testing.T) {
			s, fake := newDramaTestService(t)
			project := createTestDrama(t, s, DramaReviewAuto).Project.ID
			s.updateIntakeFenced = !race
			if race {
				s.dramaTaskHook = func(userID string, req CreateTaskRequest, _ *renderProject) (*model.Task, error) {
					s.updateIntakeFenced = true
					return s.CreateTask(userID, req)
				}
			}
			runDramaTicks(t, s, project, 1)
			jobs, err := s.repo.DramaJobsByStatus(project, model.DramaJobQueued, model.DramaJobFailed)
			if err != nil || len(jobs) != 1 || jobs[0].Status != model.DramaJobQueued || jobs[0].Error != "" || len(fake.requests) != 0 {
				t.Fatalf("update fence damaged queued work: %#v, %v", jobs, err)
			}
			s.ResumeAfterProgramUpdate()
			s.dramaTaskHook = fake.submit
			runDramaTicks(t, s, project, 24)
			production, err := s.repo.DramaProduction(project)
			if err != nil || production.Status != model.DramaStatusCompleted {
				t.Fatalf("queued work did not resume: %#v, %v", production, err)
			}
		})
	}
}

func TestDramaRealTaskAdmissionEnforcesReservedBudgetAndUpdateFence(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	for _, record := range []any{
		&model.Project{ID: "drama-project", UserID: "user", Name: "Budget test", Type: "short-drama", Status: model.ProjectStatusActive},
		&model.DramaProduction{ProjectID: "drama-project", UserID: "user", Status: model.DramaStatusRunning, BudgetMicrocredits: 150},
	} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	req := creationTextRequest()
	req.ProjectID = "drama-project"
	req.Input["metadata"] = map[string]any{"source": "drama_production", "domainProjectId": req.ProjectID}
	first, err := s.submitDramaTask("user", req)
	if err != nil {
		t.Fatal(err)
	}
	order, err := s.repo.BillingOrder(first.BillingOrderID)
	if err != nil || !order.ChargeLimitSet || order.ChargeLimitMicrocredits != 100 {
		t.Fatalf("quote was not capped: %#v, %v", order, err)
	}
	if _, err := s.submitDramaTask("user", req); !errors.Is(err, repository.ErrDramaBudgetExceeded) {
		t.Fatalf("second in-flight task exceeded remaining budget: %v", err)
	}
	var tasks int64
	if err := db.Model(&model.Task{}).Where("project_id = ?", req.ProjectID).Count(&tasks).Error; err != nil || tasks != 1 {
		t.Fatalf("rejected admission persisted a task: %d, %v", tasks, err)
	}
	if err := s.repo.RefundBillingOrder(order.ID, "isolated test refund"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.submitDramaTask("user", req); err != nil {
		t.Fatalf("refunded reservation still occupied budget: %v", err)
	}
	s.updateIntakeFenced = true
	if _, err := s.submitDramaTask("user", req); !isAppErrorStatus(err, 503) {
		t.Fatalf("drama task bypassed program-update intake: %v", err)
	}
}

func TestDramaResourceWritesRejectForeignMissingAndUnreadyResources(t *testing.T) {
	s, fake := newDramaTestService(t)
	project := createTestDrama(t, s, DramaReviewAuto).Project.ID
	for _, record := range []any{
		&model.DramaCast{ID: "cast", ProjectID: project, ImageStatus: model.DramaMediaRunning},
		&model.DramaEpisode{ID: "episode", ProjectID: project, Number: 1, Revision: 1},
		&model.DramaSegment{ID: "segment", ProjectID: project, EpisodeID: "episode", Revision: 1},
	} {
		if err := fake.db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	foreign := fake.resource("other", "video", "video/mp4")
	unready := fake.resource("usr-drama", "video", "video/mp4")
	if err := fake.db.Model(&model.Resource{}).Where("id = ?", unready).Update("status", "pending").Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{foreign, "missing", unready} {
		if err := s.repo.CreateDramaTake(&model.DramaTake{ID: "take-" + id, ProjectID: project, OwnerID: "segment", ResourceID: id}); err == nil {
			t.Fatalf("invalid take resource accepted: %s", id)
		}
		for name, update := range map[string]func() (bool, error){
			"cast": func() (bool, error) {
				return s.repo.UpdateDramaCastIf("cast", []string{model.DramaMediaRunning}, map[string]any{"image_resource_id": id})
			},
			"video": func() (bool, error) {
				return s.repo.UpdateDramaSegmentRevision("segment", 1, map[string]any{"video_resource_id": id})
			},
			"tail": func() (bool, error) {
				return s.repo.UpdateDramaSegmentRevision("segment", 1, map[string]any{"tail_frame_resource_id": id})
			},
			"final": func() (bool, error) {
				return s.repo.UpdateDramaEpisodeIfRevision("episode", 1, map[string]any{"final_resource_id": id})
			},
		} {
			if applied, err := update(); err == nil || applied {
				t.Fatalf("%s accepted invalid resource %s: applied=%t err=%v", name, id, applied, err)
			}
		}
	}
}
