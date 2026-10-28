package app

import (
	"encoding/json"
	"strings"
	"testing"

	"yingce/backend/internal/model"
)

func TestTaskForOutputRedactsRoutingAndSecrets(t *testing.T) {
	task := model.Task{
		InputJSON:              `{"mode":"image","metadata":{"source":"create-page"},"config":{"apiKey":"secret"},"resourceId":"resource-1"}`,
		LogicalModelRevisionID: "revision-1",
		RouteID:                "route-1",
		ChannelModelID:         "channel-model-1",
	}

	output := taskForOutput(task)
	if output.LogicalModelRevisionID != "" || output.RouteID != "" || output.ChannelModelID != "" {
		t.Fatalf("internal routing fields leaked: %+v", output)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(output.InputJSON), &input); err != nil {
		t.Fatalf("public input is not valid JSON: %v", err)
	}
	if _, exists := input["config"]; exists {
		t.Fatal("provider config must not be exposed")
	}
	if input["resourceId"] != "resource-1" {
		t.Fatalf("resource identity was not preserved: %#v", input)
	}
}

func TestTaskForOutputRedactsHistoricalFailure(t *testing.T) {
	task := model.Task{Error: `模型服务暂时不可用（HTTP 503）；上游：{"error":{"code":"model_not_found","message":"request id abc at https://private.example/task_abc123"}}`, ProviderCancelError: "取消失败 task_abc123"}
	output := taskForOutput(task)
	if strings.Contains(output.Error, "private.example") || strings.Contains(output.Error, "request id") || strings.Contains(output.Error, "task_abc123") || !strings.Contains(output.Error, "更换模型") {
		t.Fatalf("unsafe task error: %q", output.Error)
	}
	if strings.Contains(output.ProviderCancelError, "task_abc123") {
		t.Fatalf("unsafe cancel error: %q", output.ProviderCancelError)
	}
}

func TestTaskSummaryCarriesSafeFailureReasonsForAllTaskKinds(t *testing.T) {
	for _, taskType := range []string{"canvas_image", "canvas_video", "canvas_audio", "canvas_text", model.TaskTypeTimelineRender, model.TaskTypeTimelineTranscription} {
		t.Run(taskType, func(t *testing.T) {
			task := model.Task{ID: "failed-task", Type: taskType, Status: model.TaskStatusFailed, Stage: "任务失败", Error: "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。"}
			encoded, err := json.Marshal(taskSummaryForOutput(task))
			if err != nil {
				t.Fatal(err)
			}
			var summary map[string]any
			if err := json.Unmarshal(encoded, &summary); err != nil {
				t.Fatal(err)
			}
			if summary["error"] != taskErrorModerationMessage || summary["errorCode"] != contentModerationErrorCode {
				t.Fatalf("failure reason missing from task summary: %s", encoded)
			}
			if got := taskForOutput(task).Error; got != summary["error"] {
				t.Fatalf("list/detail failure reason differs: %q / %q", summary["error"], got)
			}
		})
	}
}

func TestTaskSummaryKeepsEveryGeneratedImagePreview(t *testing.T) {
	task := model.Task{ID: "multi-image", Type: "canvas_image", Status: model.TaskStatusSucceeded, ResultJSON: `{"mode":"image","images":[{"dataUrl":"/api/resources/first/file","url":"/api/resources/first/file"},{"dataUrl":"/api/resources/second/file","url":"/api/resources/second/file"}]}`}
	encoded, err := json.Marshal(taskSummaryForOutput(task))
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		PreviewURL    string   `json:"previewUrl"`
		PreviewImages []string `json:"previewImages"`
	}
	if err := json.Unmarshal(encoded, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.PreviewURL != "/api/resources/first/file" || len(summary.PreviewImages) != 2 || summary.PreviewImages[1] != "/api/resources/second/file" {
		t.Fatalf("multiple generated images collapsed in summary: %s", encoded)
	}
}

func TestTaskImagePreviewsSkipInlineDataAndKeepOrder(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      int
	}{
		{"all images", `{"images":[{"url":"/api/resources/first/file"},{"dataUrl":"https://cdn.example.test/second.png"},"/api/resources/third/file"]}`, 3},
		{"no duplicate url fields", `{"images":[{"url":"/api/resources/first/file","dataUrl":"/api/resources/first/file"}]}`, 1},
		{"unusable data", `{"images":[{"dataUrl":"data:image/png;base64,AAAA"},{"url":"file:///tmp/private.png"},{"url":"/api/resources/last/file"}]}`, 1},
		{"video poster is not an image output", `{"video":{"url":"https://cdn.example.test/video.mp4","posterUrl":"https://cdn.example.test/poster.png"}}`, 0},
		{"malformed", `{`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskImagePreviews(tc.raw); len(got) != tc.want {
				t.Fatalf("image previews = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestWorkbenchTaskListKeepsImagePreviewsAfterDatabaseRead(t *testing.T) {
	s, db, _, _ := creationTestService(t)
	admin := &model.User{ID: "multi-image-admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateFeatureAvailability(admin, FeatureAvailability{ImageWorkbenchEnabled: true}); err != nil {
		t.Fatal(err)
	}
	task := model.Task{ID: "saved-multi-image", UserID: "user", Type: "canvas_image", WorkbenchKind: "image", Status: model.TaskStatusSucceeded, ResultJSON: `{"images":[{"url":"/api/resources/first/file"},{"url":"/api/resources/second/file"}]}`}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		tasks, err := s.WorkbenchTasks("user", "image", false, 100)
		if err != nil || len(tasks) != 1 {
			t.Fatalf("workbench list: tasks=%v err=%v", tasks, err)
		}
		if len(tasks[0].PreviewImages) != 2 || tasks[0].PreviewImages[1] != "/api/resources/second/file" {
			t.Fatalf("reloaded list lost the second image: %+v", tasks[0])
		}
	}
}

func TestTaskSummaryFailureProjectionMatchesDetail(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"network", `Post "https://private.example/v1/images": dial tcp 10.0.0.1:443: connect: connection refused`, taskErrorNetworkMessage},
		{"rate limit", "HTTP 429", "服务当前繁忙，请稍后重试。"},
		{"gateway", "HTTP 502 Bad Gateway", taskErrorUnavailableMessage},
		{"parameter", "图片尺寸不支持，请选择 1024x1024。", "图片尺寸不支持，请选择 1024x1024。"},
		{"image size", "模型服务拒绝请求；上游：image file too large 25MB", "模型服务拒绝请求；参考图片超过模型支持的大小限制，请压缩图片后重试。"},
		{"structured", `接口请求失败：{"error":{"message":"图片尺寸不支持"},"headers":{"Authorization":"private-token"},"url":"https://private.example"}`, "图片尺寸不支持"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := model.Task{Status: model.TaskStatusFailed, Error: tc.raw}
			summary := taskSummaryForOutput(task)
			if summary.Error != tc.want || summary.Error != taskForOutput(task).Error {
				t.Fatalf("list/detail failed to preserve safe reason: %q / %q", summary.Error, taskForOutput(task).Error)
			}
			if task.Error != tc.raw {
				t.Fatal("error projection mutated the stored failure")
			}
		})
	}
}

func TestTaskErrorProjectionKeepsAbsentErrorsEmpty(t *testing.T) {
	for _, status := range []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusRunning, model.TaskStatusSucceeded, model.TaskStatusFailed, model.TaskStatusCancelled} {
		task := model.Task{Status: status}
		if got := taskSummaryForOutput(task).Error; got != "" {
			t.Fatalf("%s invented failure reason %q", status, got)
		}
		if got := taskForOutput(task).Error; got != "" {
			t.Fatalf("%s detail invented failure reason %q", status, got)
		}
	}
}

func TestDramaJobFailureViewsKeepSafeReasons(t *testing.T) {
	raw := "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。"
	job := model.DramaJob{Status: model.DramaJobFailed, Error: raw}
	if got := dramaJobView(job).Error; got != taskErrorModerationMessage {
		t.Fatalf("drama job lost failure reason: %q", got)
	}
	task := &model.Task{Type: "canvas_video", Status: model.TaskStatusFailed, Error: raw}
	if got := dramaJobTaskView(task, nil).Error; got != taskForOutput(*task).Error {
		t.Fatalf("drama task/detail reason differs: %q", got)
	}
	if job.Error != raw || task.Error != raw {
		t.Fatal("public drama projection mutated stored errors")
	}
}

func TestTaskListsAndWorkbenchCarryFailureReasonsFromDatabase(t *testing.T) {
	svc, db, _, _ := creationTestService(t)
	admin := &model.User{ID: "admin-fixture", Username: "admin-fixture", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateFeatureAvailability(admin, FeatureAvailability{TaskCenterEnabled: true, ImageWorkbenchEnabled: true, VideoWorkbenchEnabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"image", "video", "audio", "text"} {
		task := model.Task{ID: "failed-" + kind, UserID: "user", Type: "canvas_" + kind, WorkbenchKind: kind, Status: model.TaskStatusFailed, Stage: "任务失败", InputJSON: "{}", Error: "模型服务拒绝了请求，请检查模型和参数；上游：内容未通过上游安全审核，请修改提示词或参考素材后重试。"}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"", "image", "video"} {
		var tasks []TaskSummary
		var err error
		if kind == "" {
			tasks, err = svc.Tasks("user", 100)
		} else {
			tasks, err = svc.WorkbenchTasks("user", kind, false, 100)
		}
		if err != nil {
			t.Fatal(err)
		}
		want := 4
		if kind != "" {
			want = 1
		}
		if len(tasks) != want {
			t.Fatalf("%s list returned %d tasks, want %d", kind, len(tasks), want)
		}
		for _, task := range tasks {
			if task.Error != taskErrorModerationMessage || task.ErrorCode != contentModerationErrorCode {
				t.Fatalf("%s list lost persisted failure: %+v", kind, task)
			}
		}
	}
}

func TestTaskOutputIncludesOnlySafeWorkbenchRestoreData(t *testing.T) {
	task := model.Task{InputJSON: `{"mode":"video","metadata":{"source":"workbench","workbenchKind":"video","workbenchToolId":"tool-1","workbenchPresetId":"preset-1","workbench":{"kind":"video","parameters":{"seconds":8,"ratio":"16:9","resolution":"1080p","generateAudio":true},"references":[{"resourceId":"resource-1","kind":"image","role":"first_frame","slotKey":"subject"}]}},"config":{"apiKey":"private-key","videoSeconds":"8","size":"16:9","vquality":"1080p"},"referenceImages":[{"resourceId":"resource-1","storageKey":"resource:resource-1","role":"first_frame","slotKey":"subject"}]}`}
	publicJSON := publicTaskInputJSON(task.InputJSON)
	var output map[string]any
	if err := json.Unmarshal([]byte(publicJSON), &output); err != nil {
		t.Fatal(err)
	}
	if _, exists := output["config"]; exists {
		t.Fatal("provider config leaked from workbench restore data")
	}
	workbench, ok := output["workbench"].(map[string]any)
	if !ok || workbench["kind"] != "video" {
		t.Fatalf("workbench restore data missing: %#v", output)
	}
	parameters, ok := workbench["parameters"].(map[string]any)
	if !ok || parameters["seconds"] != float64(8) || parameters["ratio"] != "16:9" || parameters["generateAudio"] != true {
		t.Fatalf("workbench parameters not preserved safely: %#v", workbench)
	}
	references, ok := workbench["references"].([]any)
	if !ok || len(references) != 1 {
		t.Fatalf("workbench references missing: %#v", workbench)
	}
	reference := references[0].(map[string]any)
	if reference["resourceId"] != "resource-1" || reference["kind"] != "image" || reference["role"] != "first_frame" || reference["slotKey"] != "subject" {
		t.Fatalf("safe reference fields not preserved: %#v", reference)
	}
}

func TestTaskOutputRestoresWorkbenchImageResolution(t *testing.T) {
	task := model.Task{InputJSON: `{"mode":"image","metadata":{"source":"workbench","workbenchKind":"image","workbench":{"kind":"image","parameters":{"size":"1824x1024","imageResolution":"2k","quality":"auto","count":1}}},"config":{"size":"1824x1024","imageResolution":"2k","quality":"auto","count":"1"}}`}
	var output map[string]any
	if err := json.Unmarshal([]byte(publicTaskInputJSON(task.InputJSON)), &output); err != nil {
		t.Fatal(err)
	}
	workbench, ok := output["workbench"].(map[string]any)
	if !ok {
		t.Fatalf("workbench restore data missing: %#v", output)
	}
	parameters, ok := workbench["parameters"].(map[string]any)
	if !ok || parameters["imageResolution"] != "2k" {
		t.Fatalf("image resolution not preserved for restore: %#v", workbench)
	}
}

func TestTaskMediaPreviewUsesSafeMediaURLs(t *testing.T) {
	previewURL, previewKind := taskMediaPreview(`{"images":["data:image/png;base64,AAAA","/api/resources/resource-1/file"],"video":"https://cdn.example.com/output.mp4"}`, "video")
	if previewURL != "/api/resources/resource-1/file" || previewKind != "image" {
		t.Fatalf("unexpected preview: url=%q kind=%q", previewURL, previewKind)
	}
	if previewURL, _ := taskMediaPreview(`{"url":"file:///tmp/output.mp4"}`, "video"); previewURL != "" {
		t.Fatalf("unsafe local URL was exposed: %q", previewURL)
	}
}

func TestTaskMediaPreviewSeparatesVideoPoster(t *testing.T) {
	previewURL, previewKind, posterURL := taskMediaPreviewWithPoster(`{"video":{"url":"https://cdn.example.com/output.mp4","posterUrl":"https://cdn.example.com/poster.webp"}}`, "canvas_video")
	if previewURL != "https://cdn.example.com/output.mp4" || previewKind != "video" || posterURL != "https://cdn.example.com/poster.webp" {
		t.Fatalf("unexpected video preview: url=%q kind=%q poster=%q", previewURL, previewKind, posterURL)
	}
	if _, _, posterURL := taskMediaPreviewWithPoster(`{"video":{"url":"https://cdn.example.com/output.mp4","posterUrl":"file:///tmp/poster.png"}}`, "canvas_video"); posterURL != "" {
		t.Fatalf("unsafe local poster was exposed: %q", posterURL)
	}
}

func TestTaskClientContextRequiresCreatePageMetadata(t *testing.T) {
	valid := taskClientContext(`{"metadata":{"source":"create-page","conversationId":"conversation-1","messageId":"message-1","batchIndex":2,"batchCount":4}}`)
	if valid == nil || valid.ConversationID != "conversation-1" || valid.BatchIndex != 2 {
		t.Fatalf("valid client context was not decoded: %+v", valid)
	}
	if context := taskClientContext(`{"metadata":{"source":"other","conversationId":"conversation-1","messageId":"message-1"}}`); context != nil {
		t.Fatalf("unexpected context for non-create-page task: %+v", context)
	}
}

func TestTaskClientContextPreservesCanvasNodeID(t *testing.T) {
	context := taskClientContext(`{"mode":"image","metadata":{"nodeId":"canvas-node-1","source":"canvas"}}`)
	if context == nil || context.NodeID != "canvas-node-1" {
		t.Fatalf("canvas node context was not preserved: %+v", context)
	}
}

func TestTaskSummaryPreservesCanvasNodeWithoutExposingInput(t *testing.T) {
	summary := taskSummaryForOutput(model.Task{
		ID: "task-1", ProjectID: "project-1", Type: "canvas_image",
		InputJSON: `{"metadata":{"nodeId":"node-1"},"config":{"apiKey":"private-test-value"}}`,
	})
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	context, ok := output["clientContext"].(map[string]any)
	if !ok || context["nodeId"] != "node-1" || len(context) != 1 {
		t.Fatalf("summary lost safe canvas association: %#v", output)
	}
	for _, key := range []string{"inputJson", "config", "apiKey"} {
		if _, exists := output[key]; exists {
			t.Fatalf("summary leaked %s", key)
		}
	}
}

func TestTaskClientContextProjectsChapterOperations(t *testing.T) {
	characters := taskClientContext(`{"metadata":{"domainProjectId":"project-1","chapterId":"chapter-1","operation":"chapter_character_breakdown"}}`)
	if characters == nil || characters.DomainProjectID != "project-1" || characters.ChapterID != "chapter-1" || characters.ChapterOperation != "characters" {
		t.Fatalf("character task context was not decoded: %+v", characters)
	}
	storyboard := taskClientContext(`{"metadata":{"source":"short-drama-chapter-storyboard","domainProjectId":"project-1","chapterId":"chapter-2"}}`)
	if storyboard == nil || storyboard.ChapterID != "chapter-2" || storyboard.ChapterOperation != "storyboard" {
		t.Fatalf("storyboard task context was not decoded: %+v", storyboard)
	}
	if context := taskClientContext(`{"metadata":{"domainProjectId":"project-1","chapterId":"chapter-1","operation":"unrelated"}}`); context != nil {
		t.Fatalf("unexpected context for unrelated project task: %+v", context)
	}
}

func TestTaskClientContextProjectsShotWorkflow(t *testing.T) {
	context := taskClientContext(`{"metadata":{"domainProjectId":"project-1","shotId":"shot-1","workflowStepId":"step-1","artifactType":"video"}}`)
	if context == nil || context.DomainProjectID != "project-1" || context.ShotID != "shot-1" || context.WorkflowStepID != "step-1" || context.ArtifactType != "video" {
		t.Fatalf("shot task context was not decoded: %+v", context)
	}
}

func TestTaskOutputResourceReadsPersistedMedia(t *testing.T) {
	id, mediaType := taskOutputResource(`{"mode":"video","video":{"resourceId":"resource-1","storageKey":"resource:resource-1"}}`, "canvas_video")
	if id != "resource-1" || mediaType != "video" {
		t.Fatalf("unexpected task output resource: id=%q mediaType=%q", id, mediaType)
	}
}
