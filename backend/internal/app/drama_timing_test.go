package app

import (
	"encoding/json"
	"strings"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/production"
	"yingce/backend/internal/protocol"
)

func TestDramaSegmentDurationUsesSelectedContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration VideoDurationConfig
		planned  int64
		seconds  int
		maximum  int
		wantErr  bool
	}{
		{"range_ignores_old_fixed_values", VideoDurationConfig{Selection: "range", Min: 1, Max: 15, Step: 1, Values: []int{6}, Default: 6}, 12200, 13, 15, false},
		{"fixed_ignores_old_range", VideoDurationConfig{Selection: "enum", Min: 1, Max: 15, Step: 1, Values: []int{6}, Default: 6}, 5200, 6, 6, false},
		{"fixed_cannot_shorten_story", VideoDurationConfig{Selection: "enum", Min: 1, Max: 15, Step: 1, Values: []int{6}, Default: 6}, 12200, 0, 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, fake := newDramaTestService(t)
			video := dramaVideoFixture()
			video.Duration = tc.duration
			setDramaVideoCapability(t, svc, fake, video)
			draft := dramaSegmentDraft{view: DramaSegmentView{DurationMs: tc.planned}}
			settings := DramaSettings{Models: fakeDramaModels}
			settings.AspectRatio = "9:16"
			request, _, err := svc.prepareDramaSegment(settings, draft, model.DramaSegment{}, nil, nil)
			if (err != nil) != tc.wantErr || request.MaxVideoSeconds != tc.maximum || (!tc.wantErr && request.Seconds != tc.seconds) {
				t.Fatalf("request=%+v err=%v; want seconds=%d maximum=%d error=%v", request, err, tc.seconds, tc.maximum, tc.wantErr)
			}
			if len(fake.requests) != 0 {
				t.Fatal("duration preview submitted a paid request")
			}
		})
	}
}

func TestModelInputDurationProjectionClearsInactiveConstraints(t *testing.T) {
	minimum, maximum, step := 1.0, 15.0, 1.0
	video := dramaVideoFixture()
	video.Duration.Values = []int{6}
	input := &protocol.ModelInputConfig{Fields: []protocol.InputField{{ImageParameter: protocol.ImageParameter{Name: "videoSeconds", Binding: "duration", Min: &minimum, Max: &maximum, Step: &step, Default: 6}}}}
	projectVideoModelInputs(&video, input)
	if video.Duration.Selection != "range" || len(video.Duration.Values) != 0 || video.Duration.Max != 15 {
		t.Fatalf("range retained old enum: %+v", video.Duration)
	}
	input.Fields[0].Values = []any{6}
	projectVideoModelInputs(&video, input)
	if video.Duration.Selection != "enum" || video.Duration.Min != 0 || video.Duration.Max != 0 || video.Duration.Step != 0 || len(video.Duration.Values) != 1 {
		t.Fatalf("enum retained old range: %+v", video.Duration)
	}
}

func TestDramaPromptDurationChangeAllowsExplicitManualRewrite(t *testing.T) {
	svc, fake := newDramaTestService(t)
	video := dramaVideoFixture()
	video.Duration = VideoDurationConfig{Selection: "range", Min: 1, Max: 15, Step: 1, Default: 6}
	setDramaVideoCapability(t, svc, fake, video)
	settings := DramaSettings{Models: fakeDramaModels}
	settings.AspectRatio = "9:16"
	draft := dramaSegmentDraft{view: DramaSegmentView{DurationMs: 12200}}
	segment := model.DramaSegment{PromptSeconds: 6, VideoPrompt: "【00—06秒｜镜头一：拉远】"}
	if _, _, err := svc.prepareDramaSegment(settings, draft, segment, nil, nil); err == nil {
		t.Fatal("old generated prompt must be rewritten for the new duration")
	}
	segment.VideoPromptOverride = "【00—13秒｜镜头一：拉远】"
	if request, _, err := svc.prepareDramaSegment(settings, draft, segment, nil, nil); err != nil || request.Seconds != 13 {
		t.Fatalf("explicit valid manual rewrite must not depend on the old generated prompt: %+v %v", request, err)
	}
}

func TestDramaFixedVideoModelSplitsLongDialogueInPersistedStoryboard(t *testing.T) {
	svc, fake := newDramaTestService(t)
	video := dramaVideoFixture()
	video.Duration = VideoDurationConfig{Selection: "enum", Values: []int{6}, Default: 6}
	setDramaVideoCapability(t, svc, fake, video)
	created := createTestDramaWithMedia(t, svc, DramaReviewAuto, DramaMediaManual)
	runDramaTicks(t, svc, created.Project.ID, 20)
	summary, err := svc.DramaSummary("usr-drama", created.Project.ID)
	if err != nil || len(summary.Episodes) == 0 {
		t.Fatalf("episode fixture: %v", err)
	}
	episode, err := svc.repo.DramaEpisode(created.Project.ID, summary.Episodes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	run := dramaRun{s: svc, project: &summary.Project, settings: summary.Production.Settings}
	line := "第一句话要完整保留，第二句话也不能丢，最后一句接着说。"
	requests := len(fake.requests)
	if err := run.applyScriptShots(episode, []production.ScriptShot{{Scene: "萧家大厅", Props: []string{"退婚书"}, Speaker: "萧炎", Dialogue: line, Action: "少年走向厅门", DurationSeconds: 16.2}}, nil, production.ScriptEpisode{}, true); err != nil {
		t.Fatal(err)
	}
	shots, err := svc.repo.DramaShots(episode.ID)
	if err != nil {
		t.Fatal(err)
	}
	var dialogue strings.Builder
	for i, shot := range shots {
		if shot.Position != i || shot.DurationMs > 6000 || shot.DurationMs < run.settings.Storyboard.SpokenMs(shot.Dialogue) || len(decodeStringList(shot.CastIDsJSON)) == 0 || len(decodeStringList(shot.PropIDsJSON)) == 0 {
			t.Fatalf("invalid split shot: %+v", shot)
		}
		dialogue.WriteString(shot.Dialogue)
	}
	if dialogue.String() != line || len(shots) < 3 {
		t.Fatalf("dialogue was clipped: %q shots=%d", dialogue.String(), len(shots))
	}
	for _, draft := range buildDramaSegmentDrafts(run.settings, dramaBibleMeta{}, shots, nil, nil) {
		request, _, err := svc.prepareDramaSegment(run.settings, draft, model.DramaSegment{}, nil, nil)
		if err != nil || request.Seconds != 6 || draft.view.DurationMs > 6000 {
			t.Fatalf("fixed model cannot generate split segment: %+v err=%v", request, err)
		}
	}
	if len(fake.requests) != requests {
		t.Fatal("storyboard adjustment submitted model requests")
	}
}

func TestDramaRejectsReversedPromptBeforeReadyOrVideoSubmission(t *testing.T) {
	svc, fake := newDramaTestService(t)
	created := createTestDramaWithMedia(t, svc, DramaReviewAuto, DramaMediaManual)
	runDramaTicks(t, svc, created.Project.ID, 20)
	summary, _ := svc.DramaSummary("usr-drama", created.Project.ID)
	rows, err := svc.repo.DramaSegments(summary.Episodes[0].ID)
	if err != nil || len(rows) == 0 {
		t.Fatalf("segment fixture: %v", err)
	}
	segment := rows[0]
	prompt := "第1段视频。人物按原定站位说话，保持参考图的服装、朝向、光线与表情。\n【08.61—06.00秒｜镜头四：中景远拉】\n镜头从人物正面向后拉开，保持动作和环境音连续。"
	payload, _ := json.Marshal(dramaRevisionPayload{Revision: segment.Revision, Seconds: 6})
	result, _ := json.Marshal(map[string]any{"text": prompt})
	run := dramaRun{s: svc, project: &summary.Project, settings: summary.Production.Settings}
	if err := run.applySegmentPrompt(&model.DramaJob{ShotID: segment.ID, PayloadJSON: string(payload)}, &model.Task{ResultJSON: string(result)}); err == nil || !strings.Contains(err.Error(), "时间码") {
		t.Fatalf("invalid writer result accepted: %v", err)
	}
	stored, err := svc.repo.DramaSegment(created.Project.ID, segment.ID)
	if err != nil || stored.VideoPrompt != segment.VideoPrompt {
		t.Fatalf("invalid prompt replaced the current version: %v", err)
	}
	segment.VideoPromptOverride = prompt
	shots, _ := svc.repo.DramaShots(segment.EpisodeID)
	drafts := buildDramaSegmentDrafts(run.settings, dramaBibleMeta{}, shots, nil, rows)
	if _, _, err := svc.prepareDramaSegment(run.settings, drafts[0], segment, nil, nil); err == nil || !strings.Contains(err.Error(), "时间码") {
		t.Fatalf("invalid saved prompt accepted for video generation: %v", err)
	}
	if len(dramaVideoRequests(fake)) != 0 {
		t.Fatal("invalid prompt submitted a video request")
	}
}

func TestDramaFractionalPlanKeepsWriterVideoAndFinalDurationConsistent(t *testing.T) {
	svc, fake := newDramaTestService(t)
	video := dramaVideoFixture()
	video.Duration = VideoDurationConfig{Selection: "range", Min: 1, Max: 15, Step: 1, Values: []int{6}, Default: 6}
	setDramaVideoCapability(t, svc, fake, video)
	svc.dramaTaskHook = func(userID string, req CreateTaskRequest, timeline *renderProject) (*model.Task, error) {
		before := fake.byKind["script"]
		task, err := fake.submit(userID, req, timeline)
		if err == nil && fake.byKind["script"] > before {
			shots := []production.ScriptShot{}
			for _, seconds := range []float64{2.5, 2.72, 3.39, 3.59} {
				shots = append(shots, production.ScriptShot{Scene: "萧家大厅", Characters: []string{"萧炎"}, Action: "少年向前走，抬头看向门外", DurationSeconds: seconds})
			}
			script, _ := json.Marshal(map[string]any{"shots": shots})
			result, _ := json.Marshal(map[string]any{"mode": "text", "text": string(script)})
			task.ResultJSON = string(result)
			err = fake.db.Save(task).Error
		}
		return task, err
	}
	settings := DramaSettings{ReviewMode: DramaReviewAuto, MediaMode: DramaMediaAuto, Concurrency: 16, Models: fakeDramaModels, EpisodeMode: DramaEpisodeModeChapter}
	settings.EpisodeSeconds = 60
	created, err := svc.CreateDrama("usr-drama", CreateDramaRequest{Name: "时长一致", Start: true, Settings: settings, Chapters: []DramaChapterInput{{Title: "第一章", Text: "少年走向门外。"}}})
	if err != nil {
		t.Fatal(err)
	}
	runDramaTicks(t, svc, created.Project.ID, 24)
	writers := 0
	for _, req := range fake.requests {
		if req.Type == "canvas_text" && strings.Contains(req.Prompt, "【本段素材稿】") {
			writers++
			if !strings.Contains(req.Prompt, "本段 13 秒") || !strings.Contains(req.Prompt, "08.61—13.00秒") {
				t.Fatal("writer received a shorter or reversed timeline")
			}
		}
	}
	videos := dramaVideoRequests(fake)
	if writers != 1 || len(videos) != 1 || videos[0].Input["config"].(map[string]any)["videoSeconds"] != "13" || !strings.Contains(videos[0].Prompt, "08.61—13.00秒") {
		t.Fatalf("video and writer duration differ: writers=%d videos=%+v", writers, videos)
	}
	if len(fake.renders) != 1 || fake.renders[0].DurationMs != 13000 {
		t.Fatalf("final cut used a different duration: %+v", fake.renders)
	}
	summary, err := svc.DramaSummary("usr-drama", created.Project.ID)
	if err != nil || len(summary.Episodes) != 1 {
		t.Fatalf("episode summary: %+v %v", summary, err)
	}
	detail, err := svc.DramaEpisodeDetail("usr-drama", created.Project.ID, summary.Episodes[0].ID)
	if err != nil || len(detail.Segments) != 1 {
		t.Fatalf("segment preview: %+v %v", detail, err)
	}
	segment := detail.Segments[0]
	if segment.DurationMs != 12200 || segment.VideoSeconds != 13 || segment.Request.Seconds != 13 || segment.Request.Warning != "" {
		t.Fatalf("preview must separate plan and generated duration: %+v", segment)
	}
}
