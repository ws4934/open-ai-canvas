package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"yingce/backend/internal/protocol"
)

func normalizeModelInputs(name string, input *ModelCapabilityConfig, registries []*protocol.Registry) (*protocol.ModelInputConfig, error) {
	if input == nil || input.InputConfig == nil {
		return nil, nil
	}
	registry := protocol.Builtins()
	if len(registries) > 0 && registries[0] != nil {
		registry = registries[0]
	}
	adapter, ok := registry.Resolve(name)
	if !ok {
		return nil, BadAuthRequest("模型参数所属协议未安装")
	}
	resolved, err := protocol.ResolveModelInputs(adapter.Metadata().InputSchema, input.InputConfig)
	if err != nil {
		return nil, BadAuthRequest(err.Error())
	}
	return resolved, nil
}

func validateModelInputPriceSelector(config *protocol.ModelInputConfig, selector map[string]string) error {
	for name, value := range selector {
		if name == "operation" {
			if value != "" && value != "*" && !containsString(config.Operations, value) {
				return BadAuthRequest("价格档生成方式不在模型支持范围内")
			}
			continue
		}
		var definition *protocol.InputField
		for i := range config.Fields {
			field := &config.Fields[i]
			if field.Name == name && field.IsEnabled() && field.Pricing && field.Type != "object[]" && field.Type != "string[]" {
				definition = field
				break
			}
		}
		if definition == nil {
			return BadAuthRequest("模型未声明此计价参数：" + name)
		}
		if value != "" && value != "*" && !definition.ImageParameter.Accepts(imageParameterValue(definition.ImageParameter, value)) {
			return BadAuthRequest("价格档参数不在支持范围内：" + definition.Label)
		}
	}
	return nil
}

func normalizeModelInputPriceSelector(config *protocol.ModelInputConfig, input ChannelModelPriceTierRequest) (map[string]string, string, int, error) {
	selector := map[string]string{}
	for name, value := range input.Selector {
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name != "" && value != "" && value != "*" {
			selector[name] = value
		}
	}
	if input.Resolution != "" && input.Resolution != "*" && selector["vquality"] == "" {
		selector["vquality"] = normalizeChannelModelTierResolution(input.Resolution)
	}
	if input.VideoSeconds > 0 && selector["videoSeconds"] == "" {
		selector["videoSeconds"] = strconv.Itoa(input.VideoSeconds)
	}
	if err := validateModelInputPriceSelector(config, selector); err != nil {
		return nil, "", 0, err
	}
	resolution := "*"
	if value := selector["vquality"]; value != "" {
		resolution = normalizeChannelModelTierResolution(value)
	}
	seconds, _ := strconv.Atoi(selector["videoSeconds"])
	return selector, resolution, seconds, nil
}

func projectVideoModelInputs(video *VideoCapabilityConfig, input *protocol.ModelInputConfig) {
	if video == nil || input == nil {
		return
	}
	stringsOf := func(values []any) []string {
		result := make([]string, len(values))
		for i, value := range values {
			result[i] = fmt.Sprint(value)
		}
		return result
	}
	for _, field := range input.Fields {
		switch field.Binding {
		case "duration":
			if field.Min != nil {
				video.Duration.Min = int(*field.Min)
			}
			if field.Max != nil {
				video.Duration.Max = int(*field.Max)
			}
			if field.Step != nil {
				video.Duration.Step = int(*field.Step)
			}
			if len(field.Values) > 0 {
				video.Duration.Selection = "enum"
				video.Duration.Values = nil
				for _, value := range field.Values {
					number, _ := strconv.Atoi(fmt.Sprint(value))
					video.Duration.Values = append(video.Duration.Values, number)
				}
			} else {
				video.Duration.Selection = "range"
			}
			if field.Default != nil {
				video.Duration.Default, _ = strconv.Atoi(fmt.Sprint(field.Default))
			}
		case "resolution":
			video.Resolutions = stringsOf(field.Values)
			if field.Default != nil {
				video.DefaultResolution = fmt.Sprint(field.Default)
			}
		case "aspectRatio":
			video.Ratios = stringsOf(field.Values)
			if field.Default != nil {
				video.DefaultRatio = fmt.Sprint(field.Default)
			}
		case "generateAudio":
			video.GenerateAudio = VideoBooleanConfig{Supported: field.IsEnabled(), Default: field.Default == true, Fixed: field.ReadOnly}
		case "watermark":
			video.Watermark = VideoBooleanConfig{Supported: field.IsEnabled(), Default: field.Default == true, Fixed: field.ReadOnly}
		}
	}
	if input.Operations != nil {
		video.Operations = append([]string{}, input.Operations...)
		if len(video.Operations) > 0 {
			video.DefaultOperation = video.Operations[0]
		}
	}
	video.References.MaxImages, video.References.MaxVideos, video.References.MaxAudios = 0, 0, 0
	// Duration limits are now role/operation-specific; a legacy shared maximum
	// must not reject a newly configured model before its own rules run.
	video.References.MaxVideoDuration, video.References.MaxAudioDuration = 0, 0
	for _, role := range input.MediaRoles {
		switch role.Kind {
		case "image":
			video.References.MaxImages += role.MaxCount
		case "video":
			video.References.MaxVideos += role.MaxCount
		case "audio":
			video.References.MaxAudios += role.MaxCount
		}
	}
}

// Write canonical values before routing/pricing; execution reuses the same
// model-owned definitions instead of trusting client-supplied wire bindings.
func applyTaskModelInputs(input map[string]any, profile *ModelCapabilityConfig) error {
	if profile == nil || profile.InputConfig == nil {
		return nil
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return BadAuthRequest("任务输入格式无效")
	}
	var task canvasGenerationInput
	if err := json.Unmarshal(encoded, &task); err != nil {
		return BadAuthRequest("任务参数格式无效")
	}
	if task.Config.InputMediaRoles != "" {
		var assignments map[string]struct {
			Role        string         `json:"role"`
			InputFields map[string]any `json:"inputFields,omitempty"`
		}
		decoder := json.NewDecoder(bytes.NewBufferString(task.Config.InputMediaRoles))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&assignments); err != nil {
			return BadAuthRequest("素材角色配置无效")
		}
		for index := range task.ReferenceImages {
			if assigned, exists := assignments[task.ReferenceImages[index].ID]; exists {
				task.ReferenceImages[index].Role, task.ReferenceImages[index].InputFields = assigned.Role, assigned.InputFields
			}
		}
		refs, _ := json.Marshal(task.ReferenceImages)
		var updated []any
		_ = json.Unmarshal(refs, &updated)
		input["referenceImages"] = updated
	}
	task.Config.CapabilityConfig = profile
	request := protocolRequestFromInput(task)
	request.Images = nil
	fallbackRole := ""
	if metadataString(task.Metadata, "videoStartFrameNodeId") != "" || metadataString(task.Metadata, "videoEndFrameNodeId") != "" {
		fallbackRole = "reference_image"
	}
	for index, media := range task.ReferenceImages {
		item := protocolMediaReference(media, "image", index)
		item.Role = videoImageRoleOrDefault(task, media, fallbackRole)
		request.Images = append(request.Images, item)
	}
	request.Videos = nil
	for index, media := range task.ReferenceVideos {
		item := protocolMediaReference(media, "video", index)
		item.Role = "reference_video"
		request.Videos = append(request.Videos, item)
	}
	request.Audios = nil
	for index, media := range task.ReferenceAudios {
		item := protocolMediaReference(media, "audio", index)
		item.Role = firstNonEmpty(media.Role, "reference_audio")
		request.Audios = append(request.Audios, item)
	}
	request, err = protocol.ApplyModelInputs(request, profile.InputConfig)
	if err != nil {
		return BadAuthRequest(err.Error())
	}
	config, ok := input["config"].(map[string]any)
	if !ok {
		return BadAuthRequest("任务缺少配置")
	}
	config["inputParameters"] = request.InputParameters
	config["capabilityConfig"] = profile
	for _, field := range profile.InputConfig.Fields {
		value, exists := request.InputParameters[field.Name]
		if !exists {
			continue
		}
		key := map[string]string{"duration": "videoSeconds", "resolution": "vquality", "aspectRatio": "size", "generateAudio": "videoGenerateAudio", "watermark": "videoWatermark", "quality": "quality", "imageCount": "count"}[field.Binding]
		if key != "" {
			config[key] = providerConfigOptionValue(value)
		}
	}
	return nil
}
