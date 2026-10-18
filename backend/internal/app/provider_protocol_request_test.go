package app

import (
	"context"
	"encoding/json"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/protocol"
)

func TestProtocolRequestFromInputUsesConfiguredBase64ForOpenAIImage(t *testing.T) {
	request := protocolRequestFromInput(canvasGenerationInput{
		Mode: "image",
		Config: providerConfig{
			InterfaceType: string(model.ChannelInterfaceOpenAIImage),
			ImageParameters: map[string]any{
				"size": "1024x1024",
			},
		},
		ImageCapability: &ImageCapabilityConfig{
			ResponseFormat: ParameterSupport{Supported: true, Default: "b64_json"},
		},
	})

	if got := request.ProviderOptions[string(model.ChannelInterfaceOpenAIImage)]["response_format"]; got != "b64_json" {
		t.Fatalf("response_format = %#v, want b64_json", got)
	}
}

func TestConfiguredImageResponseFormatReachesOfficialPlugin(t *testing.T) {
	for _, interfaceType := range []string{string(model.ChannelInterfaceOpenAIImage), string(model.ChannelInterfaceGrokImage)} {
		for _, tc := range []struct {
			name    string
			profile *ImageCapabilityConfig
			want    string
		}{
			{name: "auto", profile: &ImageCapabilityConfig{ResponseFormat: ParameterSupport{Supported: true, Default: "auto"}}},
			{name: "unset", profile: &ImageCapabilityConfig{ResponseFormat: ParameterSupport{Supported: true}}},
			{name: "unsupported", profile: &ImageCapabilityConfig{ResponseFormat: ParameterSupport{Supported: false, Default: "b64_json"}}},
			{name: "missing_profile"},
			{name: "b64_json", profile: &ImageCapabilityConfig{ResponseFormat: ParameterSupport{Supported: true, Default: "b64_json"}}, want: "b64_json"},
			{name: "url", profile: &ImageCapabilityConfig{ResponseFormat: ParameterSupport{Supported: true, Default: "url"}}, want: "url"},
		} {
			t.Run(interfaceType+"/"+tc.name, func(t *testing.T) {
				ctx := ensureOfficialProtocolAdapter(context.Background(), interfaceType)
				adapter, ok := declarativeProtocolAdapterForContext(ctx, interfaceType)
				if !ok {
					t.Fatal("official image adapter is missing")
				}
				input := canvasGenerationInput{
					Mode: "image", Prompt: "a landscape",
					Config:          providerConfig{InterfaceType: interfaceType, Model: "image-model"},
					ImageCapability: tc.profile,
					Metadata:        map[string]any{"providerOptions": map[string]any{interfaceType: map[string]any{"response_format": "stale"}}},
				}
				spec, err := adapter.BuildCreate(ctx, protocol.RequestContext{Request: protocolRequestFromInput(input)})
				if err != nil {
					t.Fatal(err)
				}
				body, ok := spec.Body.(map[string]any)
				if !ok {
					t.Fatalf("request body = %#v", spec.Body)
				}
				bodies := map[string]map[string]any{"official_plugin": body}
				if interfaceType == string(model.ChannelInterfaceGrokImage) {
					legacyBody, _, err := grokImageRequestBody(input)
					if err != nil {
						t.Fatal(err)
					}
					encoded, err := json.Marshal(legacyBody)
					if err != nil {
						t.Fatal(err)
					}
					var decoded map[string]any
					if err := json.Unmarshal(encoded, &decoded); err != nil {
						t.Fatal(err)
					}
					bodies["handwritten"] = decoded
				}
				for route, body := range bodies {
					got, exists := body["response_format"]
					if tc.want == "" {
						if exists {
							t.Fatalf("%s must omit automatic/unsupported format from the upstream body, got %#v", route, got)
						}
					} else if got != tc.want {
						t.Fatalf("%s response_format = %#v, want %q", route, got, tc.want)
					}
				}
			})
		}
	}
}

func TestProtocolRequestFromInputOmitsAutomaticOpenAIImageFormat(t *testing.T) {
	request := protocolRequestFromInput(canvasGenerationInput{
		Mode: "image",
		Config: providerConfig{
			InterfaceType:   string(model.ChannelInterfaceOpenAIImage),
			ImageParameters: map[string]any{"size": "1024x1024"},
		},
		ImageCapability: &ImageCapabilityConfig{
			ResponseFormat: ParameterSupport{Supported: true, Default: "auto"},
		},
	})

	if _, ok := request.ProviderOptions[string(model.ChannelInterfaceOpenAIImage)]["response_format"]; ok {
		t.Fatal("response_format should be omitted when the model default is auto")
	}
}

func TestProtocolRequestFromInputOmitsUnsupportedOpenAIImageFormat(t *testing.T) {
	request := protocolRequestFromInput(canvasGenerationInput{
		Mode:   "image",
		Config: providerConfig{InterfaceType: string(model.ChannelInterfaceOpenAIImage)},
		ImageCapability: &ImageCapabilityConfig{
			ResponseFormat: ParameterSupport{Supported: false},
		},
	})

	if _, ok := request.ProviderOptions[string(model.ChannelInterfaceOpenAIImage)]["response_format"]; ok {
		t.Fatal("response_format should be omitted when the capability is unsupported")
	}
}
