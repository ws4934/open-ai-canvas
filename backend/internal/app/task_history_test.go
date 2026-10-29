package app

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func checkTaskHistoryCleanup(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.Task{}, &model.TaskLog{}, &model.Result{}, &model.Resource{}, &model.Asset{}, &model.RouteAttempt{}, &model.BillingOrder{}); err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, task := range []model.Task{
		{ID: "done", UserID: "owner", Status: model.TaskStatusSucceeded, BillingOrderID: "order", ResultJSON: `{"url":"/api/resources/media/file"}`},
		{ID: "failed", UserID: "owner", Status: model.TaskStatusFailed},
		{ID: "cancelled", UserID: "owner", Status: model.TaskStatusCancelled},
		{ID: "active", UserID: "owner", Status: model.TaskStatusRunning},
		{ID: "queued", UserID: "owner", Status: model.TaskStatusQueued},
		{ID: "foreign", UserID: "other", Status: model.TaskStatusFailed},
	} {
		task.Type, task.WorkbenchKind, task.InputJSON, task.UpdatedAt = "canvas_image", "image", `{}`, before
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []any{
		&model.TaskLog{ID: "log", TaskID: "done", UserID: "owner", Message: "retained"},
		&model.Result{ID: "result", TaskID: "done", UserID: "owner"},
		&model.Resource{ID: "media", UserID: "owner"},
		&model.Asset{ID: "asset", UserID: "owner"},
		&model.RouteAttempt{ID: "attempt", TaskID: "done", AttemptNumber: 1},
		&model.BillingOrder{ID: "order", TaskID: "done", UserID: "owner", Status: "settled"},
	} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &Service{repo: repository.New(db)}
	ids := []string{"done", "failed", "cancelled", "active", "queued", "foreign", "missing", " done "}
	result, err := svc.ClearTaskHistory("owner", TaskHistoryClearRequest{IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.ClearedIDs, []string{"done", "failed", "cancelled"}) || len(result.Skipped) != 4 || result.Skipped[0].Reason != "active" || result.Skipped[2].Reason != "not_found" || result.Skipped[3].Reason != "not_found" {
		t.Fatalf("wrong clear result: %+v", result)
	}
	for _, id := range result.ClearedIDs {
		task, err := svc.repo.TaskForUser("owner", id)
		if err != nil || task.HistoryHiddenAt == nil || !task.UpdatedAt.Equal(before) {
			t.Fatalf("cleanup changed execution: %+v %v", task, err)
		}
		encoded, _ := json.Marshal(task)
		if strings.Contains(string(encoded), "historyHiddenAt") || strings.Contains(string(encoded), "HistoryHiddenAt") {
			t.Fatal("internal history flag leaked")
		}
	}
	list, err := svc.repo.Tasks("owner", 50, "", false)
	if err != nil || len(list) != 2 {
		t.Fatalf("hidden tasks in general history: %d %v", len(list), err)
	}
	page, err := svc.repo.WorkbenchTasks("owner", "image", 7, 20, false, "")
	if err != nil || page.Total != 0 || page.Page != 1 {
		t.Fatalf("hidden tasks in workbench count/search: %+v %v", page, err)
	}
	again, err := svc.ClearTaskHistory("owner", TaskHistoryClearRequest{IDs: []string{"done"}})
	if err != nil || len(again.ClearedIDs) != 1 {
		t.Fatalf("cleanup not idempotent: %+v %v", again, err)
	}
	for _, row := range []any{&model.TaskLog{}, &model.Result{}, &model.Resource{}, &model.Asset{}, &model.RouteAttempt{}, &model.BillingOrder{}} {
		var count int64
		if err := db.Model(row).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("cleanup deleted associated %T: %d %v", row, count, err)
		}
	}
	order, err := svc.repo.BillingOrder("order")
	if err != nil || order.Status != "settled" {
		t.Fatalf("cleanup changed billing: %+v %v", order, err)
	}
	foreign, _ := svc.repo.Task("foreign")
	if foreign.HistoryHiddenAt != nil {
		t.Fatal("cleanup crossed account scope")
	}
}

func TestTaskHistoryCleanupSQLite(t *testing.T) {
	_, db, _, _ := creationTestService(t)
	checkTaskHistoryCleanup(t, db)
}

func TestTaskHistoryCleanupPostgres(t *testing.T) {
	if os.Getenv("CANVAS_TEST_POSTGRES_DSN") == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	checkTaskHistoryCleanup(t, agentPermissionsDB(t))
}

func TestTaskHistoryCleanupValidatesInput(t *testing.T) {
	svc, _, _, _ := creationTestService(t)
	for _, ids := range [][]string{nil, {""}, {"  "}, {strings.Repeat("x", 37)}, make([]string, 201)} {
		if _, err := svc.ClearTaskHistory("user", TaskHistoryClearRequest{IDs: ids}); err == nil {
			t.Fatal("invalid history write accepted")
		}
	}
	if _, err := svc.ClearTaskHistory("", TaskHistoryClearRequest{IDs: []string{"task"}}); err == nil {
		t.Fatal("anonymous history write accepted")
	}
}

func TestTaskMediaRecoveryRestoresHistoryWithoutNewSubmission(t *testing.T) {
	svc, db, _, _ := creationTestService(t)
	now := time.Now()
	task := model.Task{ID: "recover-hidden", UserID: "user", Status: model.TaskStatusFailed, HistoryHiddenAt: &now, Attempts: 4, RouteRun: 2, ProviderRequestID: "job", BillingOrderID: "same-order", MediaRecoveryJSON: "checkpoint"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.RequeueTaskMediaRecovery(&task, "checkpoint"); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.repo.TaskForUser("user", task.ID)
	if err != nil || stored.HistoryHiddenAt != nil || stored.Attempts != 4 || stored.RouteRun != 2 || stored.ProviderRequestID != "job" || stored.BillingOrderID != "same-order" {
		t.Fatalf("save recovery changed generation: %+v %v", stored, err)
	}
}
