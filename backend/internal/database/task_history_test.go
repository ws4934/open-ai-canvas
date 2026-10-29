package database

import (
	"fmt"
	"os"
	"testing"
	"time"

	"yingce/backend/internal/model"
)

func TestTaskHistoryVisibilityMigrationPreservesTasks(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE tasks (id TEXT PRIMARY KEY, status TEXT, error TEXT, result_json TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tasks VALUES ('existing', 'failed', 'original', '{"url":"/api/resources/media/file"}')`).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(schemaMigrations[len(schemaMigrations)-1].apply); err != nil {
			t.Fatal(err)
		}
	}
	var task model.Task
	if err := db.First(&task, "id = ?", "existing").Error; err != nil {
		t.Fatal(err)
	}
	if task.HistoryHiddenAt != nil || task.Status != model.TaskStatusFailed || task.Error != "original" || task.ResultJSON != `{"url":"/api/resources/media/file"}` {
		t.Fatalf("visibility migration changed task: %+v", task)
	}
}

func TestPostgresTaskHistoryVisibilityUpgrade(t *testing.T) {
	dsn := os.Getenv("CANVAS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	base, err := Open(Config{Driver: "postgres", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	baseSQL, _ := base.DB()
	t.Cleanup(func() { _ = baseSQL.Close() })
	schema := fmt.Sprintf("history_upgrade_%d", time.Now().UnixNano())
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	testDSN, err := postgresDSNWithSearchPath(dsn, schema)
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(Config{Driver: "postgres", DSN: testDSN})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Task{ID: "retained", UserID: "owner", Status: model.TaskStatusFailed, Attempts: 3, ResultJSON: `{"media":"retained"}`, Error: "original"}).Error; err != nil {
		t.Fatal(err)
	}
	// 将当前基线恢复到 v77 的实际列和账本边界，再走正式启动迁移。
	if err := db.Migrator().DropColumn(&model.Task{}, "HistoryHiddenAt"); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&schemaMigration{}, "version = ?", 78).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	var task model.Task
	if err := db.First(&task, "id = ?", "retained").Error; err != nil {
		t.Fatal(err)
	}
	if task.HistoryHiddenAt != nil || task.Status != model.TaskStatusFailed || task.Attempts != 3 || task.ResultJSON != `{"media":"retained"}` || task.Error != "original" {
		t.Fatalf("upgrade changed execution: %+v", task)
	}
	if err := RequireSchemaVersion(db); err != nil {
		t.Fatal(err)
	}
}
