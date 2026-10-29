package protocol

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func xaiContractAdapters(t *testing.T) map[string]Adapter {
	t.Helper()
	root := filepath.Join("..", "..", "..", "plugin-packages")
	source, err := os.ReadFile(filepath.Join(root, "xai-video", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	installed, err := LoadInstalledPlugin(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile(filepath.Join(root, "xai-video.yingce-plugin"))
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ParsePluginPackage(archive)
	if err != nil {
		t.Fatal(err)
	}
	packaged, err := LoadInstalledPlugin(pkg.Files["manifest.json"], nil)
	if err != nil {
		t.Fatal(err)
	}
	builtin, _ := Builtins().Get("xai-video")
	return map[string]Adapter{"builtin": builtin, "manifest": installed, "package": packaged}
}

func xaiOptions(values map[string]any) map[string]map[string]any {
	return map[string]map[string]any{"xai-video": values}
}

func TestXAIVideo15PinsVoicesAndNative1080(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		t.Run(name, func(t *testing.T) {
			request := GenerationRequest{Model: "grok-imagine-video-1.5", Duration: 15, Resolution: "720p", Images: []MediaReference{xaiFrame("first_frame", 0), xaiFrame("last_frame", 1)}, ProviderOptions: xaiOptions(map[string]any{
				"reference_audios": []any{map[string]any{"voice_id": "EVE"}, map[string]any{"voice_id": "leo"}, map[string]any{"voice_id": "ara"}},
				"keyframes":        []any{map[string]any{"image": map[string]any{"url": "https://media.example/key.png"}, "timestamp_s": 2.0}, map[string]any{"image": map[string]any{"file_id": "file-key"}, "timestamp_s": 4.0}},
			})}
			spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
			if err != nil {
				t.Fatal(err)
			}
			body := spec.Body.(map[string]any)
			voices := body["reference_audios"].([]any)
			if body["last_frame"] == nil || len(body["keyframes"].([]any)) != 2 || len(voices) != 3 || voices[0].(map[string]any)["voice_id"] != "eve" {
				t.Fatalf("advanced inputs=%+v", body)
			}
			if body["generate_audio"] != nil || body["watermark"] != nil {
				t.Fatal("invented unsupported xAI switches")
			}
			request = GenerationRequest{Model: "grok-imagine-video-1.5", Resolution: "1080p", Images: []MediaReference{xaiFrame("first_frame", 0)}}
			spec, err = adapter.BuildCreate(context.Background(), RequestContext{Request: request})
			if err != nil {
				t.Fatal(err)
			}
			body = spec.Body.(map[string]any)
			if manifestInt(body["duration"]) != 8 || body["resolution"] != "1080p" || body["image"] == nil || body["aspect_ratio"] != nil {
				t.Fatalf("native image-to-video=%+v", body)
			}
		})
	}
}

func TestXAIVideoLegacySourceOperations(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		for _, operation := range []string{"inpaint", "extend"} {
			t.Run(name+"/"+operation, func(t *testing.T) {
				request := GenerationRequest{Model: "grok-imagine-video", Prompt: "continue the action", Operation: operation, AspectRatio: "9:16", Resolution: "1080p", Videos: []MediaReference{{URL: "https://media.example/source.mp4", Metadata: map[string]any{"durationMs": 8000}}}}
				spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
				if err != nil {
					t.Fatal(err)
				}
				body := spec.Body.(map[string]any)
				if body["video"] == nil || body["aspect_ratio"] != nil || body["resolution"] != nil {
					t.Fatalf("source operation=%+v", body)
				}
				if operation == "extend" {
					if spec.Path != "/v1/videos/extensions" || manifestInt(body["duration"]) != 6 {
						t.Fatalf("extension=%+v", spec)
					}
				} else if spec.Path != "/v1/videos/edits" || body["duration"] != nil {
					t.Fatalf("edit=%+v", spec)
				}
			})
		}
	}
}

func TestXAIVideoAdvancedLimitsAndKeyframeSlots(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		for _, test := range []struct {
			name    string
			options map[string]any
			want    string
		}{
			{"keyframe_end", map[string]any{"keyframes": []any{map[string]any{"image": map[string]any{"url": "https://media.example/key.png"}, "timestamp_s": 8.0}}}, "小于"},
			{"keyframe_duplicate", map[string]any{"keyframes": []any{map[string]any{"image": map[string]any{"url": "https://media.example/a.png"}, "timestamp_s": 0.2}, map[string]any{"image": map[string]any{"url": "https://media.example/b.png"}, "timestamp_s": 0.4}}}, "1/3"},
			{"audio_file", map[string]any{"reference_audios": []any{map[string]any{"url": "https://media.example/audio.wav"}}}, "voice_id"},
			{"profile_mismatch", map[string]any{"modelProfile": "video"}, "不匹配"},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				_, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Duration: 8, ProviderOptions: xaiOptions(test.options)}})
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error=%v, want %s", err, test.want)
				}
			})
		}
		// 两个时间戳不足 1/3 秒但落在不同网格，不能凭间隔误拒绝。
		_, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{Model: "grok-imagine-video-1.5", Duration: 8, ProviderOptions: xaiOptions(map[string]any{"keyframes": []any{map[string]any{"image": map[string]any{"url": "https://media.example/a.png"}, "timestamp_s": 0.1}, map[string]any{"image": map[string]any{"url": "https://media.example/b.png"}, "timestamp_s": 0.4}}})}})
		if err != nil {
			t.Fatalf("%s rejected distinct keyframe slots: %v", name, err)
		}
	}
}

func TestXAIVideoTaskIDAndExpiredStatus(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		created, err := adapter.ParseCreate(context.Background(), []byte(`{"request_id":"xai-task"}`))
		if err != nil || created.TaskID != "xai-task" {
			t.Fatalf("%s create=%+v err=%v", name, created, err)
		}
		polled, err := adapter.ParsePoll(context.Background(), PollContext{TaskID: created.TaskID}, []byte(`{"status":"expired"}`))
		if err != nil || polled.Status != StatusFailed || !strings.Contains(polled.Message, "过期") {
			t.Fatalf("%s expired=%+v err=%v", name, polled, err)
		}
	}
}

func TestXAIVideoCountAndSourceBoundaries(t *testing.T) {
	refs := func(count int) []MediaReference {
		images := make([]MediaReference, count)
		for i := range images {
			images[i] = xaiFrame("reference_image", i)
		}
		return images
	}
	voices := func(count int) map[string]map[string]any {
		items := make([]any, count)
		for i := range items {
			items[i] = map[string]any{"voice_id": "eve"}
		}
		return xaiOptions(map[string]any{"reference_audios": items})
	}
	frames := func(count int) map[string]map[string]any {
		items := make([]any, count)
		for i := range items {
			items[i] = map[string]any{"image": map[string]any{"file_id": "file-key"}, "timestamp_s": i + 1}
		}
		return xaiOptions(map[string]any{"keyframes": items})
	}
	source := func(operation string, milliseconds, addedSeconds int) GenerationRequest {
		return GenerationRequest{Model: "grok-imagine-video", Prompt: "continue", Operation: operation, Duration: addedSeconds, Videos: []MediaReference{{URL: "https://media.example/source.mp4", Metadata: map[string]any{"durationMs": milliseconds}}}}
	}
	for name, adapter := range xaiContractAdapters(t) {
		for _, test := range []struct {
			name    string
			request GenerationRequest
			invalid bool
		}{
			{"legacy_7_references", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Images: refs(7), Duration: 10}, false},
			{"legacy_8_references", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Images: refs(8)}, true},
			{"full_15_references", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Images: refs(15)}, true},
			{"full_4_voices", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", ProviderOptions: voices(4)}, true},
			{"full_4_keyframes", GenerationRequest{Model: "grok-imagine-video-1.5", ProviderOptions: frames(4)}, false},
			{"full_5_keyframes", GenerationRequest{Model: "grok-imagine-video-1.5", ProviderOptions: frames(5)}, true},
			{"negative_duration", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Duration: -1}, true},
			{"edit_8700ms", source("inpaint", 8700, 0), false},
			{"edit_8701ms", source("inpaint", 8701, 0), true},
			{"extend_2000ms_add_2s", source("extend", 2000, 2), false},
			{"extend_15000ms_add_10s", source("extend", 15000, 10), false},
			{"extend_1999ms", source("extend", 1999, 2), true},
			{"extend_15001ms", source("extend", 15001, 2), true},
			{"extend_add_1s", source("extend", 8000, 1), true},
			{"extend_add_11s", source("extend", 8000, 11), true},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				_, err := adapter.BuildCreate(context.Background(), RequestContext{Request: test.request})
				if (err != nil) != test.invalid {
					t.Fatalf("invalid=%v, error=%v", test.invalid, err)
				}
			})
		}
	}
}

func TestXAIVideoEmbeddedAndPackagedContractsAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugin-packages")
	source, err := os.ReadFile(filepath.Join(root, "xai-video", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var standalone, embedded map[string]any
	if err = json.Unmarshal(source, &standalone); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(xaiVideoManifest, &embedded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(standalone["contributes"], embedded["contributes"]) {
		t.Fatal("regenerate the xAI built-in contract together with its plugin")
	}
}

func xaiFrame(role string, order int) MediaReference {
	return MediaReference{URL: "https://media.example/image.png", Role: role, Order: order}
}

func TestXAIVideo15SupportsFrameAndAllReferences(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		t.Run(name, func(t *testing.T) {
			images := []MediaReference{xaiFrame("first_frame", 0)}
			for i := range 14 {
				images = append(images, xaiFrame("reference_image", i+1))
			}
			spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "characters speak", Duration: 15, Resolution: "720p", Images: images}})
			if err != nil {
				t.Fatal(err)
			}
			body := spec.Body.(map[string]any)
			if body["image"] == nil || len(body["reference_images"].([]any)) != 14 {
				t.Fatalf("lost inputs: %+v", body)
			}
		})
	}
}

func TestXAIVideoModelLimits(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		for _, test := range []struct {
			name      string
			request   GenerationRequest
			wantError string
		}{
			{"base_1080", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Resolution: "1080p"}, "1080"},
			{"base_mixed", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Images: []MediaReference{xaiFrame("first_frame", 0), xaiFrame("reference_image", 1)}}, "混用"},
			{"base_reference_duration", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Duration: 11, Images: []MediaReference{xaiFrame("reference_image", 0)}}, "10"},
			{"15_reference_1080", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Resolution: "1080p", Images: []MediaReference{xaiFrame("reference_image", 0)}}, "720"},
			{"15_duration", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Duration: 16}, "15"},
			{"base_last", GenerationRequest{Model: "grok-imagine-video", Prompt: "a scene", Images: []MediaReference{xaiFrame("last_frame", 0)}}, "1.5"},
			{"15_edit", GenerationRequest{Model: "grok-imagine-video-1.5", Prompt: "a scene", Operation: "inpaint", Videos: []MediaReference{{URL: "https://media.example/input.mp4"}}}, "旧版"},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				_, err := adapter.BuildCreate(context.Background(), RequestContext{Request: test.request})
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error=%v, want %q", err, test.wantError)
				}
			})
		}
	}
}
