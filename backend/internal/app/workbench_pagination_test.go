package app

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func seedWorkbenchPaginationTasks(t *testing.T, db *gorm.DB) []model.Task {
	t.Helper()
	tasks := make([]model.Task, 123)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := range tasks {
		status := []model.TaskStatus{model.TaskStatusSucceeded, model.TaskStatusFailed, model.TaskStatusCancelled}[index%3]
		tasks[index] = model.Task{ID: fmt.Sprintf("history-%03d", len(tasks)-index), UserID: "user", Type: "canvas_image", WorkbenchKind: "image", Status: status, Prompt: fmt.Sprintf("图片 %d", index), InputJSON: `{}`, CreatedAt: base.Add(time.Duration(index/3) * time.Minute)}
	}
	tasks[0].Prompt = `Old NEEDLE 50%_\done`
	// 自定义渠道也可通过标准任务入口带工作台 metadata，不能漏掉这类现有任务。
	tasks[0].WorkbenchKind, tasks[0].InputJSON = "", `{"metadata":{"source":"workbench","workbenchKind":"image"}}`
	tasks[122].ResultJSON = `{"images":[{"url":"/api/resources/first/file"},{"url":"/api/resources/second/file"}]}`
	tasks[122].Error = "上游请求超时，请稍后重试。"
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	others := []model.Task{
		{ID: "active-image", UserID: "user", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusRunning, InputJSON: `{}`},
		{ID: "queued-image", UserID: "user", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusQueued, InputJSON: `{}`},
		{ID: "other-user", UserID: "other", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusSucceeded, Prompt: "NEEDLE", InputJSON: `{}`},
		{ID: "other-kind", UserID: "user", Type: "canvas_video", WorkbenchKind: "video", Status: model.TaskStatusSucceeded, Prompt: "NEEDLE", InputJSON: `{}`},
		{ID: "ordinary-canvas", UserID: "user", Type: "canvas_image", Status: model.TaskStatusSucceeded, Prompt: "NEEDLE", InputJSON: `{}`},
	}
	if err := db.Create(&others).Error; err != nil {
		t.Fatal(err)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].ID > tasks[j].ID
		}
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks
}

func checkWorkbenchPagination(t *testing.T, db *gorm.DB) {
	t.Helper()
	want := seedWorkbenchPaginationTasks(t, db)
	repo := repository.New(db)
	var seen []string
	for page := 1; page <= 7; page++ {
		result, err := repo.WorkbenchTasks("user", "image", page, 20, false, "")
		if err != nil {
			t.Fatal(err)
		}
		count := 20
		if page == 7 {
			count = 3
		}
		if result.Total != 123 || result.Page != page || result.PageSize != 20 || len(result.Tasks) != count || result.HasMore != (page < 7) {
			t.Fatalf("incorrect page %d: total=%d page=%d size=%d rows=%d more=%t", page, result.Total, result.Page, result.PageSize, len(result.Tasks), result.HasMore)
		}
		for _, task := range result.Tasks {
			seen = append(seen, task.ID)
		}
	}
	for index, task := range want {
		if seen[index] != task.ID {
			t.Fatalf("unstable order or missing task at %d: %s != %s", index, seen[index], task.ID)
		}
	}
	for _, search := range []string{"needle", "%_", `\done`} {
		result, err := repo.WorkbenchTasks("user", "image", 1, 20, false, search)
		if err != nil || result.Total != 1 || len(result.Tasks) != 1 || result.Tasks[0].ID != "history-123" {
			t.Fatalf("full-history literal search %q failed: %+v %v", search, result, err)
		}
	}
	active, err := repo.WorkbenchTasks("user", "image", 1, 100, true, "")
	if err != nil || active.Total != 2 || len(active.Tasks) != 2 {
		t.Fatalf("active query mixed history or accounts: %+v %v", active, err)
	}
	last, err := repo.WorkbenchTasks("user", "image", 1000000, 20, false, "")
	if err != nil || last.Page != 7 || len(last.Tasks) != 3 {
		t.Fatalf("out-of-range page not clamped: %+v %v", last, err)
	}
	empty, err := repo.WorkbenchTasks("user", "image", 4, 20, false, "不存在")
	if err != nil || empty.Total != 0 || empty.Page != 1 || len(empty.Tasks) != 0 {
		t.Fatalf("empty query retained an invalid page: %+v %v", empty, err)
	}
}

func TestWorkbenchHistoryPaginationSQLite(t *testing.T) {
	svc, db, _, _ := creationTestService(t)
	checkWorkbenchPagination(t, db)
	admin := &model.User{ID: "pagination-admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateFeatureAvailability(admin, FeatureAvailability{ImageWorkbenchEnabled: true}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.WorkbenchTasks("user", "image", WorkbenchTaskQuery{})
	if err != nil || result.Total != 123 || result.PageSize != 20 || len(result.Tasks) != 20 {
		t.Fatalf("service page lost metadata: %+v %v", result, err)
	}
	for _, task := range result.Tasks {
		if task.ID == "history-001" && (len(task.PreviewImages) != 2 || task.Error == "") {
			t.Fatal("pagination lost previews or failure reasons")
		}
	}
	if _, err := svc.WorkbenchTasks("user", "video", WorkbenchTaskQuery{}); err == nil {
		t.Fatal("pagination bypassed feature availability")
	}
	if _, err := svc.WorkbenchTasks("user", "audio", WorkbenchTaskQuery{}); err == nil {
		t.Fatal("invalid workbench kind accepted")
	}
}

func TestWorkbenchHistoryPaginationPostgres(t *testing.T) {
	if os.Getenv("CANVAS_TEST_POSTGRES_DSN") == "" {
		t.Skip("CANVAS_TEST_POSTGRES_DSN is not configured")
	}
	db := agentPermissionsDB(t)
	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatal(err)
	}
	checkWorkbenchPagination(t, db)
}
