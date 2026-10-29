package handler

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestWorkbenchHistoryPageHTTP(t *testing.T) {
	router, db, cookie := profileHandlerFixture(t)
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	RegisterWorkbenchRoutes(router.Group("/api"), svc)
	admin := &model.User{ID: "page-admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateFeatureAvailability(admin, service.FeatureAvailability{ImageWorkbenchEnabled: true}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := range 43 {
		task := model.Task{ID: fmt.Sprintf("http-history-%03d", index), UserID: "profile-user", WorkbenchKind: "image", Type: "canvas_image", Status: model.TaskStatusSucceeded, Prompt: fmt.Sprintf("任务 %d", index), InputJSON: `{}`, CreatedAt: base.Add(time.Duration(index) * time.Minute)}
		if index == 0 {
			task.Prompt = "旧作品需要完整搜索"
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		query       string
		page, count int
		total       int64
	}{
		{"kind=image", 1, 20, 43},
		{"kind=image&page=2&pageSize=20", 2, 20, 43},
		{"kind=image&page=3&pageSize=20", 3, 3, 43},
		{"kind=image&q=旧作品", 1, 1, 1},
		{"kind=image&activeOnly=true", 1, 0, 0},
	} {
		rec := profileRequest(router, "GET", "/api/workbench/tasks?"+test.query, "", cookie)
		var response struct {
			Code int                       `json:"code"`
			Data service.WorkbenchTaskPage `json:"data"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &response) != nil || response.Code != 0 {
			t.Fatalf("page query %s: %d %s", test.query, rec.Code, rec.Body.String())
		}
		if response.Data.Page != test.page || len(response.Data.Tasks) != test.count || response.Data.Total != test.total || response.Data.PageSize != 20 {
			t.Fatalf("incorrect page response for %s: %+v", test.query, response.Data)
		}
	}
	for _, query := range []string{"kind=image&page=0", "kind=image&page=abc", "kind=image&pageSize=-1", "kind=audio"} {
		if rec := profileRequest(router, "GET", "/api/workbench/tasks?"+query, "", cookie); rec.Code != 400 {
			t.Fatalf("invalid query %s accepted: %d %s", query, rec.Code, rec.Body.String())
		}
	}
	if rec := profileRequest(router, "GET", "/api/workbench/tasks?kind=image", "", ""); rec.Code != 401 {
		t.Fatalf("anonymous history read accepted: %d", rec.Code)
	}
}
