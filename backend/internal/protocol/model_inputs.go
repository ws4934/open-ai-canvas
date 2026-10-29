package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// InputSchema owns wire types and destinations. Templates are editable model
// presets, not a list of models allowed to use this protocol.
type InputSchema struct {
	Fields     []InputField    `json:"fields"`
	MediaRoles []InputRole     `json:"mediaRoles"`
	Templates  []InputTemplate `json:"templates,omitempty"`
}

type InputField struct {
	ImageParameter
	Operations      []string     `json:"operations,omitempty"`
	MaxItems        int          `json:"maxItems,omitempty"`
	Fields          []InputField `json:"fields,omitempty"`
	ReadOnly        bool         `json:"readOnly,omitempty"`
	Hidden          bool         `json:"hidden,omitempty"`
	MinExclusive    bool         `json:"minExclusive,omitempty"`
	UpperBoundField string       `json:"upperBoundField,omitempty"`
}

type InputRole struct {
	Role              string       `json:"role"`
	Label             string       `json:"label"`
	Kind              string       `json:"kind"`
	Single            bool         `json:"single,omitempty"`
	MaxCount          int          `json:"maxCount"`
	MinSeconds        float64      `json:"minSeconds,omitempty"`
	MaxSeconds        float64      `json:"maxSeconds,omitempty"`
	Fields            []InputField `json:"fields,omitempty"`
	UniqueField       string       `json:"uniqueField,omitempty"`
	Grid              float64      `json:"grid,omitempty"`
	CollectionBinding string       `json:"collectionBinding,omitempty"`
}

type InputCondition struct {
	Parameter string `json:"parameter,omitempty"`
	MediaRole string `json:"mediaRole,omitempty"`
	Values    []any  `json:"values,omitempty"`
}

type InputRule struct {
	Message     string           `json:"message"`
	Match       string           `json:"match,omitempty"`
	When        []InputCondition `json:"when"`
	Limits      []InputField     `json:"limits,omitempty"`
	ForbidRoles []string         `json:"forbidRoles,omitempty"`
	MediaLimits []InputRole      `json:"mediaLimits,omitempty"`
}

type ModelInputConfig struct {
	Operations []string     `json:"operations,omitempty"`
	Fields     []InputField `json:"fields"`
	MediaRoles []InputRole  `json:"mediaRoles"`
	Rules      []InputRule  `json:"rules,omitempty"`
}

type InputTemplate struct {
	ID     string           `json:"id"`
	Label  string           `json:"label"`
	Models []string         `json:"models,omitempty"`
	Config ModelInputConfig `json:"config"`
}

func cloneInputConfig(value *ModelInputConfig) *ModelInputConfig {
	if value == nil {
		return nil
	}
	encoded, _ := json.Marshal(value)
	var result ModelInputConfig
	_ = json.Unmarshal(encoded, &result)
	return &result
}

func (s *InputSchema) SuggestedConfig(model string) *ModelInputConfig {
	if s == nil {
		return nil
	}
	for _, template := range s.Templates {
		for _, name := range template.Models {
			if name == model {
				return cloneInputConfig(&template.Config)
			}
		}
	}
	return nil
}

func ValidateInputSchema(schema *InputSchema, providerID string) error {
	if schema == nil {
		return nil
	}
	fields := map[string]bool{}
	bindings := map[string]bool{}
	for _, field := range schema.Fields {
		if fields[field.Name] {
			return fmt.Errorf("duplicate input field %s", field.Name)
		}
		fields[field.Name] = true
		if err := validateInputField(field); err != nil {
			return err
		}
		if bindings[field.Binding] {
			return fmt.Errorf("duplicate input binding %s", field.Binding)
		}
		bindings[field.Binding] = true
		switch field.Binding {
		case "duration", "resolution", "aspectRatio", "generateAudio", "watermark", "operation", "quality", "imageCount":
			expected := "string"
			if field.Binding == "duration" || field.Binding == "imageCount" {
				expected = "integer"
			}
			if field.Binding == "generateAudio" || field.Binding == "watermark" {
				expected = "boolean"
			}
			if field.Type != expected {
				return fmt.Errorf("input %s binding requires %s", field.Name, expected)
			}
		default:
			parts := strings.Split(field.Binding, ".")
			if len(parts) != 3 || parts[0] != "providerOptions" || parts[1] != providerID || !imageParameterName.MatchString(parts[2]) {
				return fmt.Errorf("input %s requires a declared binding in its own provider namespace", field.Name)
			}
		}
	}
	roles := map[string]bool{}
	for _, role := range schema.MediaRoles {
		if !imageParameterName.MatchString(role.Role) || roles[role.Role] || role.Label == "" || role.Kind != "image" && role.Kind != "video" && role.Kind != "audio" {
			return fmt.Errorf("invalid media role %s", role.Role)
		}
		roles[role.Role] = true
		if err := validateInputRole(role); err != nil {
			return err
		}
		if role.CollectionBinding != "" {
			parts := strings.Split(role.CollectionBinding, ".")
			if len(parts) != 3 || parts[0] != "providerOptions" || parts[1] != providerID || !imageParameterName.MatchString(parts[2]) {
				return fmt.Errorf("invalid media collection binding %s", role.CollectionBinding)
			}
		}
		properties := map[string]bool{}
		for _, field := range role.Fields {
			if field.Binding != "" || properties[field.Name] || field.UpperBoundField != "" && !fields[field.UpperBoundField] {
				return fmt.Errorf("invalid media field %s", field.Name)
			}
			properties[field.Name] = true
			if err := validateInputField(field); err != nil {
				return err
			}
		}
		if role.UniqueField != "" && !properties[role.UniqueField] {
			return fmt.Errorf("unknown media unique field %s", role.UniqueField)
		}
	}
	seen := map[string]bool{}
	for _, template := range schema.Templates {
		if template.ID == "" || template.Label == "" || seen[template.ID] {
			return fmt.Errorf("invalid input template %s", template.ID)
		}
		seen[template.ID] = true
		if _, err := ResolveModelInputs(schema, &template.Config); err != nil {
			return fmt.Errorf("input template %s: %w", template.ID, err)
		}
	}
	return nil
}

func validateInputField(field InputField) error {
	if field.Type == "string[]" || field.Type == "object[]" {
		if !imageParameterName.MatchString(field.Name) || field.Label == "" || field.MaxItems < 0 {
			return fmt.Errorf("invalid list field %s", field.Name)
		}
		for _, value := range field.Values {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s requires string choices", field.Name)
			}
		}
		seen := map[string]bool{}
		for _, property := range field.Fields {
			if seen[property.Name] || property.Binding != "" || property.Type == "string[]" || property.Type == "object[]" {
				return fmt.Errorf("invalid list property %s", property.Name)
			}
			seen[property.Name] = true
			if err := validateInputField(property); err != nil {
				return err
			}
		}
		if field.Default != nil && !field.accepts(field.Default) {
			return fmt.Errorf("%s has an invalid default", field.Label)
		}
		return nil
	}
	base := field.ImageParameter
	base.Binding = ""
	return ValidateImageParameters([]ImageParameter{base})
}

func validateInputRole(role InputRole) error {
	if role.MaxCount < 0 || role.Single && role.MaxCount > 1 || !finite(role.Grid) || role.Grid < 0 || !finite(role.MinSeconds) || !finite(role.MaxSeconds) || role.MinSeconds < 0 || role.MaxSeconds < 0 || role.MaxSeconds > 0 && role.MinSeconds > role.MaxSeconds {
		return fmt.Errorf("素材角色 %s 的数量、时间网格或时长无效", role.Role)
	}
	return nil
}

// Only constraints/defaults are model-editable. Types, labels and destinations
// always come from the installed plugin; a new model may widen template bounds.
func ResolveModelInputs(schema *InputSchema, config *ModelInputConfig) (*ModelInputConfig, error) {
	if config == nil {
		return nil, nil
	}
	if schema == nil {
		return nil, fmt.Errorf("当前协议未声明可扩展模型参数")
	}
	result := cloneInputConfig(config)
	definitions := map[string]InputField{}
	for _, field := range schema.Fields {
		definitions[field.Name] = field
	}
	seen := map[string]bool{}
	for index, field := range result.Fields {
		definition, ok := definitions[field.Name]
		if !ok || seen[field.Name] {
			return nil, fmt.Errorf("协议未声明参数 %s，或参数重复", field.Name)
		}
		seen[field.Name] = true
		definition.Values, definition.Min, definition.Max, definition.Step = field.Values, field.Min, field.Max, field.Step
		if !definition.ReadOnly {
			definition.Default = field.Default
		}
		definition.Enabled, definition.MaxItems = field.Enabled, field.MaxItems
		if definition.Required && !definition.IsEnabled() {
			return nil, fmt.Errorf("%s是必填参数", definition.Label)
		}
		if err := validateInputField(definition); err != nil {
			return nil, err
		}
		result.Fields[index] = definition
	}
	for _, field := range schema.Fields {
		if !seen[field.Name] {
			result.Fields = append(result.Fields, field)
		}
	}
	roles := map[string]InputRole{}
	for _, role := range schema.MediaRoles {
		roles[role.Role] = role
	}
	seenRoles := map[string]bool{}
	for index, role := range result.MediaRoles {
		definition, ok := roles[role.Role]
		if !ok || seenRoles[role.Role] || role.MaxCount < 0 || definition.Single && role.MaxCount > 1 {
			return nil, fmt.Errorf("素材角色 %s 配置无效", role.Role)
		}
		seenRoles[role.Role] = true
		definition.MaxCount = role.MaxCount
		definition.Grid, definition.MinSeconds, definition.MaxSeconds = role.Grid, role.MinSeconds, role.MaxSeconds
		if err := validateInputRole(definition); err != nil {
			return nil, err
		}
		result.MediaRoles[index] = definition
	}
	for _, role := range schema.MediaRoles {
		if !seenRoles[role.Role] {
			result.MediaRoles = append(result.MediaRoles, role)
		}
	}
	for _, rule := range result.Rules {
		if rule.Message == "" || len(rule.When) == 0 || rule.Match != "" && rule.Match != "all" && rule.Match != "any" {
			return nil, fmt.Errorf("模型组合规则无效")
		}
		for _, condition := range rule.When {
			if (condition.Parameter == "") == (condition.MediaRole == "") || condition.Parameter != "" && definitions[condition.Parameter].Name == "" || condition.MediaRole != "" && roles[condition.MediaRole].Role == "" {
				return nil, fmt.Errorf("组合规则引用了未声明参数或素材角色")
			}
		}
		for _, limit := range rule.Limits {
			if definitions[limit.Name].Name == "" {
				return nil, fmt.Errorf("组合规则引用了未声明参数 %s", limit.Name)
			}
			for _, field := range result.Fields {
				if field.Name == limit.Name {
					if err := validateInputField(inputWithLimit(field, limit)); err != nil {
						return nil, err
					}
				}
			}
		}
		for _, limit := range rule.MediaLimits {
			role, ok := roles[limit.Role]
			if !ok {
				return nil, fmt.Errorf("组合规则引用了未声明素材角色 %s", limit.Role)
			}
			role.MinSeconds, role.MaxSeconds = limit.MinSeconds, limit.MaxSeconds
			if err := validateInputRole(role); err != nil {
				return nil, err
			}
		}
		for _, role := range rule.ForbidRoles {
			if roles[role].Role == "" {
				return nil, fmt.Errorf("组合规则引用了未声明素材角色 %s", role)
			}
		}
	}
	return result, nil
}

func (field InputField) accepts(value any) bool {
	if value == nil || value == "" {
		return !field.Required
	}
	if field.MinExclusive && field.Min != nil && manifestFloat(value) <= *field.Min {
		return false
	}
	if field.Type != "string[]" && field.Type != "object[]" {
		return field.ImageParameter.Accepts(value)
	}
	items := inputItems(value)
	if items == nil || field.MaxItems > 0 && len(items) > field.MaxItems {
		return false
	}
	for _, item := range items {
		if field.Type == "object[]" {
			object, ok := item.(map[string]any)
			if !ok {
				return false
			}
			known := map[string]bool{}
			for _, property := range field.Fields {
				known[property.Name] = true
				if !property.accepts(object[property.Name]) {
					return false
				}
			}
			for name := range object {
				if !known[name] {
					return false
				}
			}
			continue
		}
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" || len(field.Values) > 0 && !containsInputValue(field.Values, item) {
			return false
		}
	}
	return true
}

func inputItems(value any) []any {
	switch list := value.(type) {
	case []any:
		return list
	case []string:
		items := make([]any, len(list))
		for i, value := range list {
			items[i] = value
		}
		return items
	default:
		return nil
	}
}

func containsInputValue(values []any, value any) bool {
	for _, candidate := range values {
		if ImageParameterValuesEqual(candidate, value) {
			return true
		}
	}
	return false
}

func inputBindingValue(request GenerationRequest, field InputField) any {
	switch field.Binding {
	case "duration":
		if request.Duration != 0 {
			return request.Duration
		}
	case "resolution":
		if request.Resolution != "" {
			return request.Resolution
		}
	case "aspectRatio":
		if request.AspectRatio != "" {
			return request.AspectRatio
		}
	case "operation":
		if request.Operation != "" {
			return request.Operation
		}
	case "generateAudio":
		return request.GenerateAudio
	case "watermark":
		return request.Watermark
	case "quality":
		if request.Quality != "" {
			return request.Quality
		}
	case "imageCount":
		if request.ImageCount != 0 {
			return request.ImageCount
		}
	default:
		parts := strings.Split(field.Binding, ".")
		if len(parts) == 3 {
			return request.ProviderOptions[parts[1]][parts[2]]
		}
	}
	return nil
}

func ApplyModelInputs(request GenerationRequest, config *ModelInputConfig) (GenerationRequest, error) {
	if config == nil {
		return request, nil
	}
	values := map[string]any{}
	supplied := map[string]bool{}
	roles := append([]InputRole{}, config.MediaRoles...)
	request = normalizeInputRoles(request)
	if request.Operation == "" && request.Capability == CapabilityVideo {
		request.Operation = "text_to_video"
		for _, media := range request.Images {
			request.Operation = "image_to_video"
			if media.Role == "reference_image" {
				request.Operation = "reference_to_video"
				break
			}
		}
		if len(request.Videos) > 0 {
			request.Operation = "edit"
		}
		if len(request.Audios) > 0 {
			request.Operation = "audio_to_video"
		}
	}
	counts := map[string]int{}
	if len(config.Operations) > 0 && request.Operation != "" && !containsInputValue(stringInputValues(config.Operations), request.Operation) {
		return request, fmt.Errorf("当前模型不支持操作 %s", request.Operation)
	}
	for _, media := range append(append(append([]MediaReference{}, request.Images...), request.Videos...), request.Audios...) {
		counts[media.Role]++
	}
	for _, role := range roles {
		parts := strings.Split(role.CollectionBinding, ".")
		if len(parts) == 3 {
			counts[role.Role] += len(inputItems(request.ProviderOptions[parts[1]][parts[2]]))
		}
	}
	for _, media := range append(append(append([]MediaReference{}, request.Images...), request.Videos...), request.Audios...) {
		known := false
		for _, role := range roles {
			if role.Role == media.Role && (media.Kind == "" || role.Kind == media.Kind) {
				known = true
				break
			}
		}
		if !known {
			return request, fmt.Errorf("当前模型不支持素材角色 %s", media.Role)
		}
	}
	for _, field := range config.Fields {
		if !inputFieldApplies(field, request.Operation) {
			if err := setInputBinding(&request, field, nil); err != nil {
				return request, err
			}
			continue
		}
		value, explicit := request.InputParameters[field.Name]
		if field.Hidden && explicit && !ImageParameterValuesEqual(value, inputBindingValue(request, field)) {
			return request, fmt.Errorf("%s由任务上下文决定", field.Label)
		}
		if !explicit || field.Hidden {
			value = inputBindingValue(request, field)
		}
		supplied[field.Name] = value != nil && value != ""
		if field.ReadOnly || value == nil || value == "" {
			value = field.Default
		}
		if !field.IsEnabled() {
			if !manifestEmpty(value) && (explicit || strings.HasPrefix(field.Binding, "providerOptions.")) {
				return request, fmt.Errorf("%s未开放", field.Label)
			}
			if err := setInputBinding(&request, field, nil); err != nil {
				return request, err
			}
			continue
		}
		if value != nil {
			values[field.Name] = value
		}
	}
	for _, rule := range config.Rules {
		matched := rule.Match != "any"
		for _, condition := range rule.When {
			hit := counts[condition.MediaRole] > 0
			if condition.Parameter != "" {
				hit = !manifestEmpty(values[condition.Parameter])
				if len(condition.Values) > 0 {
					hit = containsInputValue(condition.Values, values[condition.Parameter])
				}
			}
			if rule.Match == "any" {
				matched = matched || hit
			} else {
				matched = matched && hit
			}
		}
		if !matched {
			continue
		}
		for _, limit := range rule.MediaLimits {
			for index := range roles {
				if roles[index].Role == limit.Role {
					roles[index].MinSeconds, roles[index].MaxSeconds = limit.MinSeconds, limit.MaxSeconds
				}
			}
		}
		for _, role := range rule.ForbidRoles {
			if counts[role] > 0 {
				return request, fmt.Errorf("%s", rule.Message)
			}
		}
		for _, limit := range rule.Limits {
			if limit.Default != nil && !supplied[limit.Name] {
				values[limit.Name] = limit.Default
			}
			for _, field := range config.Fields {
				if field.Name != limit.Name {
					continue
				}
				field = inputWithLimit(field, limit)
				if !field.accepts(values[field.Name]) {
					return request, fmt.Errorf("%s", rule.Message)
				}
			}
		}
	}
	allowed := map[string]bool{}
	for _, field := range config.Fields {
		allowed[field.Name] = true
		if !inputFieldApplies(field, request.Operation) {
			continue
		}
		value := values[field.Name]
		if !field.IsEnabled() || value == nil && !field.Required {
			continue
		}
		if !field.accepts(value) {
			if field.Min != nil && field.Max != nil {
				return request, fmt.Errorf("%s需在 %g–%g 范围内", field.Label, *field.Min, *field.Max)
			}
			return request, fmt.Errorf("%s超出当前模型支持范围", field.Label)
		}
		if err := setInputBinding(&request, field, value); err != nil {
			return request, err
		}
	}
	for name := range request.InputParameters {
		if !allowed[name] {
			return request, fmt.Errorf("协议未声明参数 %s", name)
		}
	}
	for _, role := range roles {
		if counts[role.Role] > role.MaxCount {
			return request, fmt.Errorf("%s最多允许 %d 项", role.Label, role.MaxCount)
		}
		slots := map[int]bool{}
		mediaItems := append(append(append([]MediaReference{}, request.Images...), request.Videos...), request.Audios...)
		parts := strings.Split(role.CollectionBinding, ".")
		if len(parts) == 3 {
			for _, item := range inputItems(request.ProviderOptions[parts[1]][parts[2]]) {
				if fields, ok := item.(map[string]any); ok {
					mediaItems = append(mediaItems, MediaReference{Role: role.Role, Metadata: fields})
				}
			}
		}
		for _, media := range mediaItems {
			if media.Role != role.Role {
				continue
			}
			if seconds := manifestFloat(media.Metadata["durationMs"]) / 1000; seconds > 0 && (seconds < role.MinSeconds || role.MaxSeconds > 0 && seconds > role.MaxSeconds) {
				return request, fmt.Errorf("%s时长需在 %g–%g 秒之间", role.Label, role.MinSeconds, role.MaxSeconds)
			}
			for _, field := range role.Fields {
				value := media.Metadata[field.Name]
				if !field.accepts(value) {
					return request, fmt.Errorf("%s：%s无效", role.Label, field.Label)
				}
				if field.UpperBoundField != "" && manifestFloat(value) >= manifestFloat(values[field.UpperBoundField]) {
					return request, fmt.Errorf("%s必须小于%s", field.Label, field.UpperBoundField)
				}
			}
			if role.UniqueField != "" && role.Grid > 0 {
				slot := int(math.Round(manifestFloat(media.Metadata[role.UniqueField]) / role.Grid))
				if slots[slot] {
					return request, fmt.Errorf("%s不能落在同一个时间网格", role.Label)
				}
				slots[slot] = true
			}
		}
	}
	request.InputParameters = values
	request.Output.Duration, request.Output.Resolution, request.Output.AspectRatio = request.Duration, request.Resolution, request.AspectRatio
	request.Output.GenerateAudio, request.Output.Watermark, request.Output.Quality, request.Output.Count = request.GenerateAudio, request.Watermark, request.Quality, request.ImageCount
	request.Inputs = append(append(append([]MediaReference{}, request.Images...), request.Videos...), request.Audios...)
	return request, nil
}

func setInputBinding(request *GenerationRequest, field InputField, value any) error {
	text := ""
	if value != nil {
		text = fmt.Sprint(value)
	}
	switch field.Binding {
	case "duration":
		request.Duration = manifestInt(value)
	case "resolution":
		request.Resolution = text
	case "aspectRatio":
		request.AspectRatio = text
	case "operation":
		request.Operation = text
	case "generateAudio":
		request.GenerateAudio, _ = value.(bool)
	case "watermark":
		request.Watermark, _ = value.(bool)
	case "quality":
		request.Quality = text
	case "imageCount":
		request.ImageCount = manifestInt(value)
	default:
		parts := strings.Split(field.Binding, ".")
		if len(parts) != 3 {
			return fmt.Errorf("%s的协议映射无效", field.Label)
		}
		if value == nil {
			delete(request.ProviderOptions[parts[1]], parts[2])
			return nil
		}
		if request.ProviderOptions == nil {
			request.ProviderOptions = map[string]map[string]any{}
		}
		if request.ProviderOptions[parts[1]] == nil {
			request.ProviderOptions[parts[1]] = map[string]any{}
		}
		request.ProviderOptions[parts[1]][parts[2]] = value
	}
	return nil
}

func normalizeInputRoles(request GenerationRequest) GenerationRequest {
	request.Images = append([]MediaReference{}, request.Images...)
	explicitFirst := false
	unmarked := 0
	for _, image := range request.Images {
		explicitFirst = explicitFirst || image.Role == "first_frame"
		if image.Role == "" {
			unmarked++
		}
	}
	for index := range request.Images {
		role := request.Images[index].Role
		if role == "" {
			if !explicitFirst && request.Operation != "reference_to_video" && (request.Operation == "image_to_video" || unmarked == 1) {
				role = "first_frame"
				explicitFirst = true
			} else {
				role = "reference_image"
			}
		}
		if role == "reference" || role == "subject_reference" || role == "style_reference" {
			role = "reference_image"
		}
		request.Images[index].Role = role
	}
	request.Videos = append([]MediaReference{}, request.Videos...)
	for index := range request.Videos {
		if request.Videos[index].Role == "" {
			request.Videos[index].Role = "reference_video"
		}
	}
	request.Audios = append([]MediaReference{}, request.Audios...)
	for index := range request.Audios {
		if request.Audios[index].Role == "" {
			request.Audios[index].Role = "reference_audio"
		}
	}
	return request
}

func stringInputValues(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func inputFieldApplies(field InputField, operation string) bool {
	return len(field.Operations) == 0 || operation == "" || containsInputValue(stringInputValues(field.Operations), operation)
}

func inputWithLimit(field, limit InputField) InputField {
	if limit.Values != nil {
		field.Values = limit.Values
	}
	if limit.Min != nil {
		field.Min = limit.Min
	}
	if limit.Max != nil {
		field.Max = limit.Max
	}
	if limit.Step != nil {
		field.Step = limit.Step
	}
	if limit.MaxItems > 0 {
		field.MaxItems = limit.MaxItems
	}
	// A model's normal default may be outside a conditional range. The rule can
	// supply its own default; otherwise runtime validates the selected value.
	field.Default = limit.Default
	return field
}
