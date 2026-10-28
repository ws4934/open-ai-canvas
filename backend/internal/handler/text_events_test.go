package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTaskTextEventCursorPrefersQueryAndSupportsLastEventID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/api/tasks/task-1/text-events?after=7", nil)
	request.Header.Set("Last-Event-ID", "3")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	after, err := taskTextEventCursor(context)
	if err != nil || after != 7 {
		t.Fatalf("cursor = %d, err = %v", after, err)
	}

	request = httptest.NewRequest("GET", "/api/tasks/task-1/text-events", nil)
	request.Header.Set("Last-Event-ID", "3")
	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	after, err = taskTextEventCursor(context)
	if err != nil || after != 3 {
		t.Fatalf("Last-Event-ID cursor = %d, err = %v", after, err)
	}
}

func TestTaskTextEventCursorRejectsInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"after=abc", "after=-1"} {
		t.Run(query, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/tasks/task-1/text-events?"+query, nil)
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = request
			if after, err := taskTextEventCursor(context); err == nil || after != 0 {
				t.Fatalf("cursor = %d, err = %v; want a rejected cursor", after, err)
			}
		})
	}
}

func TestStreamTaskTextEventsCachedReplayReachesTerminalWithoutDuplicateDelta(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.TaskTextDelta{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Task{ID: "task", UserID: "user", Status: model.TaskStatusRunning}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TaskTextDelta{ID: "delta", UserID: "user", TaskID: "task", Sequence: 8, Content: "text", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	initial, err := svc.CachedTaskTextReplay(ctx, "user", "task", 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Task{}).Where("id = ?", "task").Updates(map[string]any{"status": model.TaskStatusSucceeded, "progress": 100}).Error; err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("GET", "/api/tasks/task/text-events", nil).WithContext(ctx)
	streamTaskTextEvents(c, svc, "user", "task", 7, initial)
	body := response.Body.String()
	if strings.Count(body, "id: 8\nevent: delta") != 1 || !strings.Contains(body, "event: terminal") || !strings.Contains(body, `"progress":100`) {
		t.Fatalf("unexpected cached SSE stream: %q", body)
	}
}

func TestStreamTaskTextEventsWritesSequenceAndTerminalEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest("GET", "/api/tasks/task-1/text-events", nil)
	replay := &service.TextReplayResult{
		Deltas:    []model.TaskTextDelta{{Sequence: 8, Content: "增量"}},
		Complete:  true,
		FinalText: "增量",
		Status:    model.TaskStatusSucceeded,
		Stage:     "已完成",
		Progress:  100,
	}
	streamTaskTextEvents(context, nil, "user-1", "task-1", 7, replay)
	if response.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	body := response.Body.String()
	if !strings.Contains(body, "event: progress") || !strings.Contains(body, `"progress":100`) || !strings.Contains(body, "id: 8\nevent: delta") || !strings.Contains(body, "event: terminal") {
		t.Fatalf("unexpected SSE body: %q", body)
	}
}

func TestTaskTextFailureReplayAndSSEPreserveSafeReason(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.TaskTextDelta{}); err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	for id, raw := range map[string]string{
		"moderation": "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。",
		"network":    `Post "https://private.example/v1/chat": dial tcp 10.0.0.1:443: connect: connection refused`,
	} {
		task := model.Task{ID: id, UserID: "user", Type: "canvas_text", Status: model.TaskStatusFailed, Stage: "任务失败", Error: raw}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		replay, err := svc.TaskTextReplay("user", id, 0)
		if err != nil {
			t.Fatal(err)
		}
		detail, err := svc.Task("user", id)
		if err != nil {
			t.Fatal(err)
		}
		if replay.Error == "" || replay.Error != detail.Error {
			t.Fatalf("replay/detail failure reason differs: %q / %q", replay.Error, detail.Error)
		}
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest("GET", "/api/tasks/"+id+"/text-events", nil)
		streamTaskTextEvents(c, svc, "user", id, 0, replay)
		body := response.Body.String()
		if !strings.Contains(body, "event: terminal") || !strings.Contains(body, replay.Error) || strings.Contains(body, "private.example") || strings.Contains(body, "10.0.0.1") || strings.Contains(body, "；上游：") {
			t.Fatalf("SSE lost safe failure reason: %s", body)
		}
		if _, err := svc.TaskTextReplay("other-user", id, 0); err == nil {
			t.Fatal("another user read a task failure")
		}
	}
}
