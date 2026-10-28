package repository

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yingce/backend/internal/model"
)

func TestTasksWithWorkbenchKindRequiresMatchingTaskType(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatal(err)
	}

	legacyImage := `{"metadata":{"source":"workbench","workbenchKind":"image"}}`
	legacyVideo := `{"metadata":{"source":"workbench","workbenchKind":"video"}}`
	createdAt := time.Now()
	tasks := []model.Task{
		{ID: "image-column", UserID: "user", Type: "canvas_image", WorkbenchKind: "image", InputJSON: `{}`, CreatedAt: createdAt},
		{ID: "image-legacy", UserID: "user", Type: "canvas_image", InputJSON: legacyImage, CreatedAt: createdAt.Add(time.Second)},
		{ID: "video-column", UserID: "user", Type: "canvas_video", WorkbenchKind: "video", InputJSON: `{}`, CreatedAt: createdAt.Add(2 * time.Second)},
		{ID: "video-legacy", UserID: "user", Type: "canvas_video", InputJSON: legacyVideo, CreatedAt: createdAt.Add(3 * time.Second)},
		{ID: "wrong-column-kind", UserID: "user", Type: "canvas_video", WorkbenchKind: "image", InputJSON: `{}`, CreatedAt: createdAt.Add(4 * time.Second)},
		{ID: "wrong-legacy-type", UserID: "user", Type: "canvas_video", InputJSON: legacyImage, CreatedAt: createdAt.Add(5 * time.Second)},
		{ID: "wrong-source", UserID: "user", Type: "canvas_image", InputJSON: `{"metadata":{"source":"canvas","workbenchKind":"image"}}`, CreatedAt: createdAt.Add(6 * time.Second)},
		{ID: "other-user", UserID: "other", Type: "canvas_image", InputJSON: legacyImage, CreatedAt: createdAt.Add(7 * time.Second)},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	repo := New(db)
	for _, tc := range []struct {
		kind string
		want map[string]bool
	}{
		{kind: "image", want: map[string]bool{"image-column": true, "image-legacy": true}},
		{kind: "video", want: map[string]bool{"video-column": true, "video-legacy": true}},
	} {
		got, err := repo.TasksWithWorkbenchKind("user", 50, "", false, tc.kind)
		if err != nil {
			t.Fatalf("list %s tasks: %v", tc.kind, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s task IDs = %#v, want %d tasks", tc.kind, got, len(tc.want))
		}
		for _, task := range got {
			if !tc.want[task.ID] {
				t.Errorf("%s list included unexpected task %q (type=%q kind=%q)", tc.kind, task.ID, task.Type, task.WorkbenchKind)
			}
			delete(tc.want, task.ID)
		}
		if len(tc.want) != 0 {
			t.Errorf("%s list omitted expected task IDs: %#v", tc.kind, tc.want)
		}
	}
}

func TestTaskListsSelectFailureReasons(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatal(err)
	}
	for _, taskType := range []string{"canvas_image", "canvas_video", "canvas_audio", "canvas_text", model.TaskTypeTimelineRender, model.TaskTypeTimelineTranscription} {
		kind := ""
		if taskType == "canvas_image" {
			kind = "image"
		} else if taskType == "canvas_video" {
			kind = "video"
		}
		task := model.Task{ID: taskType, UserID: "owner", Type: taskType, WorkbenchKind: kind, Status: model.TaskStatusFailed, Error: "失败原因：" + taskType, InputJSON: "{}"}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Task{ID: "foreign", UserID: "other", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusFailed, Error: "private-other-user", InputJSON: "{}"}).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	for _, kind := range []string{"", "image", "video"} {
		tasks, err := repo.TasksWithWorkbenchKind("owner", 100, "", false, kind)
		if err != nil {
			t.Fatal(err)
		}
		want := 6
		if kind != "" {
			want = 1
		}
		if len(tasks) != want {
			t.Fatalf("%s list returned %d tasks, want %d", kind, len(tasks), want)
		}
		for _, task := range tasks {
			if task.ID == "foreign" || task.Error != "失败原因："+task.Type {
				t.Fatalf("%s list lost failure reason or user boundary: %+v", kind, task)
			}
		}
	}
}
