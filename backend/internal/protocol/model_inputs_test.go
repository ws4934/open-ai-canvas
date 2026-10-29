package protocol

import (
	"context"
	"strings"
	"testing"
)

// This deliberately invented model/config tests extensibility, not xAI's
// advertised limits. It must work without adding its name to any plugin list.
func TestXAIFutureModelUsesItsOwnInputConfig(t *testing.T) {
	for name, adapter := range xaiContractAdapters(t) {
		t.Run(name, func(t *testing.T) {
			schema := adapter.Metadata().InputSchema
			if schema.SuggestedConfig("grok-future-test") != nil {
				t.Fatal("unknown model was assigned a legacy preset")
			}
			config := schema.SuggestedConfig("grok-imagine-video-1.5")
			max := 30.0
			for i := range config.Fields {
				field := &config.Fields[i]
				if field.Binding == "duration" {
					field.Max = &max
				}
				if field.Binding == "resolution" {
					field.Values = append(field.Values, "1440p")
				}
			}
			for i := range config.MediaRoles {
				if config.MediaRoles[i].Role == "reference_image" {
					config.MediaRoles[i].MaxCount = 24
				}
			}
			config.Rules[0].Limits[0].Values = append(config.Rules[0].Limits[0].Values, "1440p")
			refs := make([]MediaReference, 24)
			for i := range refs {
				refs[i] = xaiFrame("reference_image", i)
			}
			request := GenerationRequest{Capability: CapabilityVideo, Model: "grok-future-test", Prompt: "a scene", Duration: 30, Resolution: "1440p", ModelInputConfig: config, Images: refs}
			spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
			if err != nil {
				t.Fatal(err)
			}
			body := spec.Body.(map[string]any)
			if body["model"] != "grok-future-test" || manifestInt(body["duration"]) != 30 || body["resolution"] != "1440p" || len(body["reference_images"].([]any)) != 24 {
				t.Fatalf("future model was capped by an older preset: %+v", body)
			}
			request.Images = append(request.Images, xaiFrame("reference_image", 25))
			if _, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
				t.Fatal("model's own reference limit was ignored")
			}
			request.Images = refs
			request.Duration = 31
			if _, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
				t.Fatal("model's own duration limit was ignored")
			}
		})
	}
}

func TestModelInputsPreserveFalseZeroAndPluginDestinations(t *testing.T) {
	zero, max := 0.0, 100.0
	schema := &InputSchema{Fields: []InputField{
		{ImageParameter: ImageParameter{Name: "seed", Label: "种子", Type: "integer", Binding: "providerOptions.example.seed", Min: &zero, Max: &max, Default: 10}},
		{ImageParameter: ImageParameter{Name: "raw", Label: "原始输出", Type: "boolean", Binding: "providerOptions.example.raw", Default: true}},
	}}
	if err := ValidateInputSchema(schema, "example"); err != nil {
		t.Fatal(err)
	}
	config, err := ResolveModelInputs(schema, &ModelInputConfig{Fields: []InputField{
		{ImageParameter: ImageParameter{Name: "seed", Type: "string", Binding: "providerOptions.other.key", Default: 0, Min: &zero, Max: &max}},
		{ImageParameter: ImageParameter{Name: "raw", Default: false}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	request, err := ApplyModelInputs(GenerationRequest{}, config)
	if err != nil {
		t.Fatal(err)
	}
	if manifestInt(request.ProviderOptions["example"]["seed"]) != 0 || request.ProviderOptions["example"]["raw"] != false || request.ProviderOptions["other"] != nil {
		t.Fatalf("invalid bindings/defaults: %+v", request)
	}
	_, err = ApplyModelInputs(GenerationRequest{InputParameters: map[string]any{"undeclared": true}}, config)
	if err == nil || !strings.Contains(err.Error(), "未声明") {
		t.Fatalf("unknown field: %v", err)
	}
}

func TestModelInputsValidateRulesAndMediaAssignments(t *testing.T) {
	adapter, _ := Builtins().Get("xai-video")
	schema := adapter.Metadata().InputSchema
	invalid := schema.SuggestedConfig("grok-imagine-video-1.5")
	invalid.Rules[0].MediaLimits = []InputRole{{Role: "undeclared", MaxSeconds: 8}}
	if _, err := ResolveModelInputs(schema, invalid); err == nil {
		t.Fatal("unknown role in rule was accepted")
	}
	config := schema.SuggestedConfig("grok-imagine-video-1.5")
	request := GenerationRequest{Capability: CapabilityVideo, Duration: 8, Resolution: "720p", Images: []MediaReference{{Kind: "image", Role: "keyframe", Metadata: map[string]any{"timestamp_s": 2.0}}}}
	if _, err := ApplyModelInputs(request, config); err != nil {
		t.Fatal(err)
	}
	request.Images[0].Metadata["timestamp_s"] = 8.0
	if _, err := ApplyModelInputs(request, config); err == nil {
		t.Fatal("end timestamp was accepted")
	}
	request.Images = nil
	request.Videos = []MediaReference{{Kind: "video", Role: "first_frame"}}
	if _, err := ApplyModelInputs(request, config); err == nil {
		t.Fatal("image role on video accepted")
	}
}

func TestModelInputsDisabledFieldsCannotLeakThroughProviderOptions(t *testing.T) {
	adapter, _ := Builtins().Get("xai-video")
	config := adapter.Metadata().InputSchema.SuggestedConfig("grok-imagine-video")
	request := GenerationRequest{Capability: CapabilityVideo, Prompt: "a scene", ProviderOptions: xaiOptions(map[string]any{"reference_audios": []any{map[string]any{"voice_id": "eve"}}})}
	if _, err := ApplyModelInputs(request, config); err == nil || !strings.Contains(err.Error(), "未开放") {
		t.Fatalf("disabled field bypass: %v", err)
	}
	request.ProviderOptions = nil
	request.Operation, request.Resolution, request.AspectRatio = "edit", "1080p", "9:16"
	request.Videos = []MediaReference{{Kind: "video", Role: "reference_video"}}
	resolved, err := ApplyModelInputs(request, config)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Resolution != "" || resolved.AspectRatio != "" || resolved.Duration != 0 {
		t.Fatalf("irrelevant generation options leaked to editing: %+v", resolved)
	}
}
