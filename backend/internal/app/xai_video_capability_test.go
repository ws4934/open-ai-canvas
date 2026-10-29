package app

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestXAIVideoDefaultCapabilitiesMatchModelProfiles(t *testing.T) {
	for _, test := range []struct {
		model         string
		images        int
		res1080, edit bool
	}{
		{"grok-imagine-video", 7, false, true},
		{"grok-imagine-video-1.5", 16, true, false},
		{"grok-imagine-video-1.5-lite", 1, true, false},
	} {
		t.Run(test.model, func(t *testing.T) {
			video := DefaultModelCapabilityConfigForModel("xai-video", test.model).Video
			if video.Duration.Default != 8 || video.Duration.Max != 15 || len(video.Ratios) != 9 || video.References.MaxImages != test.images {
				t.Fatalf("capability=%+v", video)
			}
			if slices.Contains(video.Resolutions, "1080p") != test.res1080 || slices.Contains(video.Operations, "extend") != test.edit || !video.GenerateAudio.Supported || !video.GenerateAudio.Default || !video.GenerateAudio.Fixed {
				t.Fatalf("wrong model profile: %+v", video)
			}
			if slices.Contains(video.Resolutions, "1440p") || video.References.MaxAudios != 0 {
				t.Fatal("xAI profile advertised unsupported input or output")
			}
		})
	}
}

func TestXAIVideoNativeAudioSurvivesCapabilityStorage(t *testing.T) {
	config := DefaultModelCapabilityConfigForModel("xai-video", "grok-imagine-video-1.5")
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeModelCapabilityConfig(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeModelCapabilityConfigForModel("video", "xai-video", "grok-imagine-video-1.5", decoded)
	if err != nil || normalized == nil || !normalized.Video.GenerateAudio.Fixed || !normalized.Video.GenerateAudio.Default {
		t.Fatalf("native audio hint lost: %+v, error=%v", normalized, err)
	}
}

func TestXAIVideoPlatformInputPreservesFrameReferencesAndOptions(t *testing.T) {
	images := []providerMedia{{ID: "start", DataURL: testReferenceImageDataURL}, {ID: "end", DataURL: testReferenceImageDataURL}}
	for i := 0; i < 14; i++ {
		images = append(images, providerMedia{DataURL: testReferenceImageDataURL})
	}
	body := officialVideoCreateBody(t, canvasGenerationInput{
		Mode: "video", Prompt: "角色说话", Config: providerConfig{Model: "grok-imagine-video-1.5", InterfaceType: "xai-video", VideoSeconds: "15", VQuality: "720p"}, ReferenceImages: images,
		Metadata: map[string]any{"videoEditOperation": "image_to_video", "videoStartFrameNodeId": "start", "videoEndFrameNodeId": "end", "providerOptions": map[string]any{"xai-video": map[string]any{"reference_audios": []any{map[string]any{"voice_id": "eve"}}, "keyframes": []any{map[string]any{"image": map[string]any{"file_id": "file-clip-anchor"}, "timestamp_s": 2.0}}}}},
	})
	if body["image"] == nil || body["last_frame"] == nil || len(body["reference_images"].([]any)) != 14 || len(body["reference_audios"].([]any)) != 1 || len(body["keyframes"].([]any)) != 1 {
		t.Fatalf("platform input lost xAI media: %+v", body)
	}
}
