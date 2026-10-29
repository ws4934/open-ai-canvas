package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestTaskHistoryCleanupHTTP(t *testing.T) {
	if !storageReadOnlyExempt("POST", "/api/tasks/clear-history") {
		t.Fatal("storage read-only mode must still allow clearing history")
	}
	router, db, cookie := profileHandlerFixture(t)
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	RegisterTaskRoutes(router.Group("/api"), svc)
	for _, task := range []model.Task{
		{ID: "ended", UserID: "profile-user", Status: model.TaskStatusFailed},
		{ID: "active", UserID: "profile-user", Status: model.TaskStatusRunning},
		{ID: "other", UserID: "other-user", Status: model.TaskStatusSucceeded},
	} {
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`{}`, `{"ids":[]}`, `{"ids":[""]}`, `{"ids":["` + strings.Repeat("x", 37) + `"]}`} {
		if rec := profileRequest(router, "POST", "/api/tasks/clear-history", body, cookie); rec.Code != 400 {
			t.Fatalf("invalid clear accepted: %d %s", rec.Code, rec.Body)
		}
	}
	if rec := profileRequest(router, "POST", "/api/tasks/clear-history", `{"ids":["ended"]}`, ""); rec.Code != 401 {
		t.Fatalf("anonymous clear accepted: %d", rec.Code)
	}
	rec := profileRequest(router, "POST", "/api/tasks/clear-history", `{"ids":["ended","active","other","missing"]}`, cookie)
	var response struct {
		Code int                            `json:"code"`
		Data service.TaskHistoryClearResult `json:"data"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &response) != nil || response.Code != 0 || len(response.Data.ClearedIDs) != 1 || len(response.Data.Skipped) != 3 {
		t.Fatalf("bad clear response: %d %s", rec.Code, rec.Body)
	}
	if rec := profileRequest(router, "GET", "/api/tasks", "", cookie); rec.Code != 200 || strings.Contains(rec.Body.String(), `"id":"ended"`) || !strings.Contains(rec.Body.String(), `"id":"active"`) {
		t.Fatalf("history filter wrong: %d %s", rec.Code, rec.Body)
	}
	if rec := profileRequest(router, "GET", "/api/tasks/ended", "", cookie); rec.Code != 200 {
		t.Fatalf("cleanup broke task references: %d %s", rec.Code, rec.Body)
	}
}
