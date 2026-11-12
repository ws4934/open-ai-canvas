package protocol

import (
	"context"
	"testing"
)

func TestXAIVideoPinnedFramesAndReferences(t *testing.T) {
	for _, adapter := range []Adapter{officialPackageAdapter(t, "xai-video.yingce-plugin", "xai-video"), xAIVideosAdapter()} {
		request := GenerationRequest{Capability: CapabilityVideo, Model: "grok-imagine-video-1.5", Prompt: "keep the character", Images: []MediaReference{
			{URL: "https://cdn.example/first.png", Role: "first_frame"},
			{URL: "https://cdn.example/last.png", Role: "last_frame"},
			{URL: "https://cdn.example/character.png", Role: "reference_image"},
		}}
		spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
		if err != nil {
			t.Fatal(err)
		}
		body := manifestTestBody(t, spec)
		if body["image"].(map[string]any)["url"] != request.Images[0].URL || body["last_frame"].(map[string]any)["url"] != request.Images[1].URL {
			t.Fatalf("pinned frames were lost: %#v", body)
		}
		refs := body["reference_images"].([]any)
		if len(refs) != 1 || refs[0].(map[string]any)["url"] != request.Images[2].URL {
			t.Fatalf("references were lost: %#v", body)
		}
		request.Images = request.Images[1:2]
		spec, err = adapter.BuildCreate(context.Background(), RequestContext{Request: request})
		if err != nil {
			t.Fatal(err)
		}
		body = manifestTestBody(t, spec)
		if body["image"] != nil || body["last_frame"] == nil {
			t.Fatalf("last frame changed to first frame: %#v", body)
		}
		for _, role := range []string{"first_frame", "last_frame"} {
			request.Images = []MediaReference{{URL: "https://cdn.example/a.png", Role: role}, {URL: "https://cdn.example/b.png", Role: role}}
			if _, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
				t.Fatalf("duplicate %s silently discarded", role)
			}
		}
	}
}

func TestOfficialXAIVideoRequestIDAndResult(t *testing.T) {
	adapter := officialPackageAdapter(t, "xai-video.yingce-plugin", "xai-video")
	created, err := adapter.ParseCreate(context.Background(), []byte(`{"request_id":"xai-request"}`))
	if err != nil || created.TaskID != "xai-request" {
		t.Fatalf("create: %+v %v", created, err)
	}
	poll, err := adapter.BuildPoll(context.Background(), PollContext{TaskID: created.TaskID})
	if err != nil || poll.Path != "/v1/videos/xai-request" {
		t.Fatalf("poll: %+v %v", poll, err)
	}
	result, err := adapter.ParsePoll(context.Background(), PollContext{TaskID: created.TaskID}, []byte(`{"status":"done","video":{"url":"https://cdn.example/result.mp4"}}`))
	if err != nil || result.Status != StatusSucceeded || result.Result == nil || len(result.Result.Videos) != 1 || result.Result.Videos[0].URL != "https://cdn.example/result.mp4" {
		t.Fatalf("result: %+v %v", result, err)
	}
}
