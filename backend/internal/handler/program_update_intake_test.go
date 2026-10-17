package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestProgramUpdateIntakeRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/tasks", "/api/workbench/tasks", "/api/timeline/renders", "/api/timeline/transcriptions",
		"/api/tasks/task/retry", "/api/tasks/task/recover-media", "/api/agent/runs", "/api/agent/runs/run/messages",
		"/api/creation-runs/run/execute", "/api/ai/custom", "/api/ai/system/channel/v1/messages",
	} {
		if !programUpdateStartsWork(http.MethodPost, path) {
			t.Errorf("new work not fenced: %s", path)
		}
	}
	for _, path := range []string{
		"/api/tasks/task/cancel", "/api/tasks/task/text-deltas", "/api/tasks/task/text-replay-complete",
		"/api/agent/runs/run/approvals/approval/decision", "/api/agent/runs/run/cancel", "/api/agent/bridge/model",
		"/api/admin/system-update/start", "/api/admin/system-update/rollback", "/api/creation-runs/run/submissions/approve",
	} {
		if programUpdateStartsWork(http.MethodPost, path) {
			t.Errorf("continuation fenced: %s", path)
		}
	}
	if programUpdateStartsWork(http.MethodGet, "/api/ai/custom") {
		t.Fatal("polling fenced")
	}
}

func TestProgramUpdateIntakeMiddlewareMaintainsContinuations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.Task{}, &model.CloudAgentExecution{}); err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { svc.Close() })
	if err := svc.DrainForProgramUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ProgramUpdateIntakeMiddleware(svc))
	router.POST("/api/tasks", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/api/tasks/:id/cancel", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		path string
		code int
	}{{"/api/tasks", 503}, {"/api/tasks/task/cancel", 204}, {"/api/channel/v1/messages", 503}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, nil))
		if response.Code != test.code {
			t.Fatalf("%s got %d %s", test.path, response.Code, response.Body.String())
		}
	}
}
