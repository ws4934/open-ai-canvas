package app

import (
	"encoding/json"
	"testing"

	"yingce/backend/internal/model"
)

func TestDramaSegmentPreviewModelErrorsKeepReferenceArray(t *testing.T) {
	svc, fake := newDramaTestService(t)
	for _, test := range []struct {
		name   string
		choice DramaModelChoice
	}{
		{"unselected", DramaModelChoice{}},
		{"removed", DramaModelChoice{ChannelID: "removed", ModelKey: "removed"}},
		{"missing_route", DramaModelChoice{LogicalModelID: "removed-route"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := DramaSettings{Models: DramaModels{Video: test.choice}}
			request, refs, err := svc.prepareDramaSegment(settings, dramaSegmentDraft{}, model.DramaSegment{}, nil, nil)
			if err == nil || refs != nil || request.Operation != "" || request.Seconds != 0 || request.MaxVideoSeconds != 0 {
				t.Fatalf("invalid model must remain blocked, without inventing a request: %+v %v", request, err)
			}
			assertDramaReferenceArray(t, request)
		})
	}
	if len(fake.requests) != 0 {
		t.Fatal("preview must not submit generation tasks")
	}
}

func TestDramaSegmentPreviewKeepsResolutionAndPromptBudget(t *testing.T) {
	svc, fake := newDramaTestService(t)
	video := dramaVideoFixture()
	video.Resolutions, video.DefaultResolution = []string{"480p", "720p"}, "720p"
	video.References.PromptMaxChars = 1800
	setDramaVideoCapability(t, svc, fake, video)
	settings := DramaSettings{VideoResolution: "480p", Models: fakeDramaModels}
	settings.AspectRatio = "9:16"
	segment := model.DramaSegment{ID: "segment", SegmentIndex: 1}
	request, _, err := svc.prepareDramaSegment(settings, dramaSegmentDraft{}, segment, nil, nil)
	if err != nil || request.Resolution != "480p" || request.PromptMaxChars != 1800 {
		t.Fatalf("preview must use selected resolution and actual prompt limit: %+v, %v", request, err)
	}
	assertDramaReferenceArray(t, request)
	profile, err := svc.dramaVideoProfile(settings.Models.Video)
	if err != nil {
		t.Fatal(err)
	}
	run := dramaRun{s: svc, settings: settings}
	budget, err := run.segmentPromptBudget(profile, dramaSegmentDraft{}, &segment, nil, nil)
	if err != nil || budget != 1798 {
		t.Fatalf("prompt budget: got=%d, err=%v", budget, err)
	}
	if len(fake.requests) != 0 {
		t.Fatal("read-only preview must not submit generation tasks")
	}
}

func TestDramaEpisodeRemainsReadableAfterVideoModelRemoved(t *testing.T) {
	svc, fake := newDramaTestService(t)
	project := createTestDramaWithMedia(t, svc, DramaReviewEvery, DramaMediaManual).Project.ID
	runDramaTicks(t, svc, project, 20)
	summary, err := svc.DramaSummary("usr-drama", project)
	if err != nil || len(summary.Episodes) == 0 {
		t.Fatalf("missing episode fixture: %+v %v", summary, err)
	}
	episodeID := summary.Episodes[0].ID
	before, err := svc.DramaEpisodeDetail("usr-drama", project, episodeID)
	if err != nil || len(before.Segments) == 0 || len(before.Shots) == 0 {
		t.Fatalf("missing storyboard fixture: %+v %v", before, err)
	}
	if err := fake.db.Delete(&model.ChannelModel{}, "id = ?", "drama-video").Error; err != nil {
		t.Fatal(err)
	}
	requestCount := len(fake.requests)
	detail, err := svc.DramaEpisodeDetail("usr-drama", project, episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Segments) != len(before.Segments) || len(detail.Shots) != len(before.Shots) || detail.Episode.ID != before.Episode.ID || len(fake.requests) != requestCount {
		t.Fatal("read-only preview lost episode content or submitted new work")
	}
	for _, segment := range detail.Segments {
		if segment.Request.Warning == "" {
			t.Fatal("missing model must retain a visible warning")
		}
		assertDramaReferenceArray(t, segment.Request)
	}
	if _, err := svc.DramaEpisodeDetail("other-user", project, episodeID); err == nil {
		t.Fatal("preview bypassed project ownership")
	}
	if _, err := svc.GenerateDramaSegments("usr-drama", project, episodeID, GenerateDramaSegmentsRequest{Segments: []int{detail.Segments[0].Index}}); err == nil {
		t.Fatal("missing model must still reject generation")
	}
	if len(fake.requests) != requestCount {
		t.Fatal("missing model submitted paid generation work")
	}
}

func assertDramaReferenceArray(t *testing.T, request DramaSegmentRequest) {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire["references"]) != "[]" {
		t.Fatalf("empty references must be an array, got %s; the project UI calls references.map", wire["references"])
	}
}
