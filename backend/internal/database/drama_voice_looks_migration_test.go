package database

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"yingce/backend/internal/model"

	"gorm.io/gorm"
)

func TestDramaVoiceLooksUpgradePreservesForkHistoryMigration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			config := Config{Driver: driver, DSN: filepath.Join(t.TempDir(), "upgrade.db")}
			if driver == "postgres" {
				dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
				}
				base, err := Open(Config{Driver: driver, DSN: dsn})
				if err != nil {
					t.Fatal(err)
				}
				baseSQL, _ := base.DB()
				t.Cleanup(func() { _ = baseSQL.Close() })
				schema := fmt.Sprintf("drama_voice_upgrade_%d", time.Now().UnixNano())
				if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
						t.Error(err)
					}
				})
				config.DSN, err = postgresDSNWithSearchPath(dsn, schema)
				if err != nil {
					t.Fatal(err)
				}
			}
			db, err := Open(config)
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := MigrateSchema(db); err != nil {
				t.Fatal(err)
			}
			hidden := time.Now().UTC().Truncate(time.Second)
			for _, row := range []any{
				&model.Task{ID: "hidden-task", UserID: "owner", Status: model.TaskStatusFailed, HistoryHiddenAt: &hidden, Error: "retained"},
				&model.DramaCast{ID: "cast", ProjectID: "drama", Kind: "character", Name: "角色", Appearance: "保留形象"},
				&model.DramaShot{ID: "shot", ProjectID: "drama", EpisodeID: "episode", CastIDsJSON: `["cast"]`, Dialogue: "保留台词", DurationMs: 3200},
			} {
				if err := db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			var before schemaMigration
			if err := db.First(&before, "version = ?", 78).Error; err != nil {
				t.Fatal(err)
			}
			if before.Name != "task_history_visibility" || before.Checksum != "sha256:task-history-visibility-v78-20261028" {
				t.Fatalf("published fork migration was changed: %+v", before)
			}
			if err := db.Exec("DROP INDEX idx_drama_casts_parent_id").Error; err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"VoiceProfileID", "VoiceSampleResourceID", "ParentID", "LookName", "LookFromChapter", "LookToChapter"} {
				if err := db.Migrator().DropColumn(&model.DramaCast{}, field); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Migrator().DropColumn(&model.DramaShot{}, "LookCastIDsJSON"); err != nil {
				t.Fatal(err)
			}
			if err := db.Delete(&schemaMigration{}, "version = ?", 79).Error; err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := MigrateSchema(db); err != nil {
					t.Fatal(err)
				}
			}
			var after schemaMigration
			if err := db.First(&after, "version = ?", 78).Error; err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("history migration ledger changed: before=%+v after=%+v err=%v", before, after, err)
			}
			assertDramaVoiceUpgradeRows(t, db, hidden)
			if err := RequireSchemaVersion(db); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertDramaVoiceUpgradeRows(t *testing.T, db *gorm.DB, hidden time.Time) {
	t.Helper()
	var task model.Task
	var cast model.DramaCast
	var shot model.DramaShot
	for _, row := range []any{&task, &cast, &shot} {
		if err := db.First(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if task.HistoryHiddenAt == nil || !task.HistoryHiddenAt.Equal(hidden) || task.Error != "retained" {
		t.Fatalf("task history was changed: %+v", task)
	}
	if cast.Name != "角色" || cast.Appearance != "保留形象" || cast.VoiceProfileID != "" || cast.VoiceSampleResourceID != "" || cast.ParentID != "" || cast.LookName != "" || cast.LookFromChapter != 0 || cast.LookToChapter != 0 {
		t.Fatalf("cast was not preserved with empty new fields: %+v", cast)
	}
	if shot.Dialogue != "保留台词" || shot.DurationMs != 3200 || shot.CastIDsJSON != `["cast"]` || shot.LookCastIDsJSON != "" {
		t.Fatalf("shot was changed: %+v", shot)
	}
	if !db.Migrator().HasIndex(&model.DramaCast{}, "ParentID") {
		t.Fatal("look parent index was not created")
	}
}
