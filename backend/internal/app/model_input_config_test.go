package app

import (
	"encoding/json"
	"strings"
	"testing"

	"yingce/backend/internal/protocol"
)

func TestFutureModelInputAdmissionCountsUnhydratedResources(t *testing.T) {
	adapter, _ := protocol.Builtins().Get("xai-video")
	profile := DefaultModelCapabilityConfigForModel("xai-video", "grok-future-test")
	profile.InputConfig = adapter.Metadata().InputSchema.SuggestedConfig("grok-imagine-video-1.5")
	max := 30.0
	for i := range profile.InputConfig.Fields {
		if profile.InputConfig.Fields[i].Binding == "duration" {
			profile.InputConfig.Fields[i].Max = &max
		}
	}
	for i := range profile.InputConfig.MediaRoles {
		if profile.InputConfig.MediaRoles[i].Role == "reference_image" {
			profile.InputConfig.MediaRoles[i].MaxCount = 24
		}
	}
	normalized, err := NormalizeModelCapabilityConfigForModel("video", "xai-video", "grok-future-test", profile)
	if err != nil {
		t.Fatal(err)
	}
	refs := make([]any, 24)
	for i := range refs {
		refs[i] = map[string]any{"id": "resource", "storageKey": "resources/opaque", "role": "reference_image"}
	}
	input := map[string]any{"mode": "video", "prompt": "a scene", "config": map[string]any{"model": "grok-future-test", "interfaceType": "xai-video", "videoSeconds": "30", "size": "16:9", "vquality": "720p", "capabilityConfig": normalized}, "referenceImages": refs}
	if err := (&Service{}).ValidateTaskCapability(input); err != nil {
		t.Fatal(err)
	}
	config := input["config"].(map[string]any)
	if config["videoSeconds"] != "30" || config["inputParameters"].(map[string]any)["videoSeconds"] != 30 {
		t.Fatalf("canonical parameters lost: %+v", config)
	}
	input["referenceImages"] = append(refs, refs[0])
	if err := (&Service{}).ValidateTaskCapability(input); err == nil || !strings.Contains(err.Error(), "24") {
		t.Fatalf("missing pre-charge resource count validation: %v", err)
	}
}

func TestModelInputAssignmentsPreserveKeyframeThroughTaskAndProvider(t *testing.T) {
	adapter, _ := protocol.Builtins().Get("xai-video")
	profile := DefaultModelCapabilityConfigForModel("xai-video", "grok-imagine-video-1.5")
	profile.InputConfig = adapter.Metadata().InputSchema.SuggestedConfig("grok-imagine-video-1.5")
	input := map[string]any{"mode": "video", "prompt": "a scene", "config": map[string]any{"model": "grok-imagine-video-1.5", "interfaceType": "xai-video", "videoSeconds": "8", "vquality": "720p", "inputMediaRoles": `{"pin":{"role":"keyframe","inputFields":{"timestamp_s":2}}}`, "inputParameters": map[string]any{"reference_audios": []any{map[string]any{"voice_id": "eve"}}}}, "referenceImages": []any{map[string]any{"id": "pin", "url": "https://media.example/pin.png", "width": 800}}}
	if err := applyTaskModelInputs(input, profile); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(input)
	var task canvasGenerationInput
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatal(err)
	}
	body := officialVideoCreateBody(t, task)
	if len(body["keyframes"].([]any)) != 1 || len(body["reference_audios"].([]any)) != 1 {
		t.Fatalf("extension inputs lost: %+v", body)
	}
	task.ReferenceImages[0].InputFields["width"] = 1
	if metadata := protocolMediaReference(task.ReferenceImages[0], "image", 0).Metadata; metadata["width"] != 800 {
		t.Fatal("input fields overwrote authoritative media metadata")
	}
}

func TestModelInputPricingUsesDeclaredScalarFields(t *testing.T) {
	adapter, _ := protocol.Builtins().Get("xai-video")
	config := adapter.Metadata().InputSchema.SuggestedConfig("grok-imagine-video-1.5")
	selector, resolution, seconds, err := normalizeModelInputPriceSelector(config, ChannelModelPriceTierRequest{Selector: map[string]string{"videoSeconds": "12", "vquality": "1080p"}})
	if err != nil || seconds != 12 || resolution != "1080p" || selector["videoSeconds"] != "12" {
		t.Fatalf("selector=%v, resolution=%s, seconds=%d, err=%v", selector, resolution, seconds, err)
	}
	for _, selector := range []map[string]string{{"reference_audios": "eve"}, {"undeclared": "1"}, {"videoSeconds": "16"}, {"operation": "extend"}} {
		if err := validateModelInputPriceSelector(config, selector); err == nil {
			t.Fatalf("accepted invalid selector: %v", selector)
		}
	}
}

func TestModelInputLogicalCatalogKeepsOnlyCommonContracts(t *testing.T) {
	adapter, _ := protocol.Builtins().Get("xai-video")
	config := adapter.Metadata().InputSchema.SuggestedConfig("grok-imagine-video-1.5")
	product := CapabilitySpec{Version: 1, Capability: "video"}
	public := capabilitySpecWithRoutePresets(product, []CapabilitySpec{{InputConfig: config}})
	if public.InputConfig == nil {
		t.Fatal("single-route input contract lost in public catalog")
	}
	public = capabilitySpecWithRoutePresets(product, []CapabilitySpec{{InputConfig: config}, {}})
	if public.InputConfig != nil {
		t.Fatal("different provider contracts were mixed into one form")
	}
}

func TestCloudAgentModelInputsRoundTripThroughGenerationContract(t *testing.T) {
	args := cloudAgentMediaArgs{Mode: "video", Prompt: "a scene", ChannelID: "qa-channel", ChannelModelKey: "grok-future-test", InputParameters: `{"reference_audios":[{"voice_id":"eve"}]}`, InputMediaRoles: `{"pin":{"role":"keyframe","inputFields":{"timestamp_s":2}}}`, ReferenceNodeIDs: []string{"pin"}}
	spec, err := cloudAgentGenerationSpec(args, map[string]any{"referenceImages": []any{map[string]any{"id": "pin", "storageKey": "resource:frame"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	config := spec.Options.TaskConfig()
	if config["inputParameters"].(map[string]any)["reference_audios"] == nil || config["inputMediaRoles"] != args.InputMediaRoles {
		t.Fatalf("lost Agent input options: %+v", config)
	}
	copy := cloudAgentMediaArgs{Mode: "video"}
	applyCloudAgentMediaDraftOptions(&copy, spec.Options)
	if copy.InputParameters != args.InputParameters || copy.InputMediaRoles != args.InputMediaRoles {
		t.Fatalf("lost draft input parameters: %+v", copy)
	}
}
