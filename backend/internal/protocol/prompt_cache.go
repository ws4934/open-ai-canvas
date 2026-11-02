package protocol

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ManifestPromptCache struct {
	Fields          []string          `json:"fields"`
	Model           any               `json:"model"`
	MinimumTokens   any               `json:"minimumTokens"`
	Create          ManifestOperation `json:"create"`
	Delete          ManifestOperation `json:"delete"`
	Resource        any               `json:"resource"`
	ExpireTime      any               `json:"expireTime"`
	ResourcePattern string            `json:"resourcePattern"`
	ApplyField      string            `json:"applyField"`
	Missing         any               `json:"missing"`
}

type PromptCacheSpec struct {
	Definition    ManifestPromptCache `json:"definition"`
	Model         string              `json:"model"`
	MinimumTokens int                 `json:"minimumTokens"`
	Auth          ManifestAuth        `json:"auth"`
}

func validatePromptCache(cache ManifestPromptCache) error {
	if len(cache.Fields) == 0 || cache.Model == nil || cache.MinimumTokens == nil || cache.Resource == nil || cache.ExpireTime == nil || cache.Missing == nil || strings.TrimSpace(cache.ApplyField) == "" {
		return fmt.Errorf("promptCache requires fields, model, minimumTokens, resource, expireTime, applyField and missing")
	}
	for _, field := range append(append([]string{}, cache.Fields...), cache.ApplyField) {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`).MatchString(field) {
			return fmt.Errorf("promptCache field %q must be a top-level identifier", field)
		}
	}
	if !strings.HasPrefix(cache.ResourcePattern, "^") || !strings.HasSuffix(cache.ResourcePattern, "$") {
		return fmt.Errorf("promptCache resourcePattern must match the complete resource name")
	}
	if _, err := regexp.Compile(cache.ResourcePattern); err != nil {
		return fmt.Errorf("promptCache resourcePattern: %w", err)
	}
	for _, operation := range []ManifestOperation{cache.Create, cache.Delete} {
		if operation.Stream != nil || operation.PromptCache != nil || operation.Input != "" {
			return fmt.Errorf("promptCache requests cannot nest stream, promptCache or conversation input")
		}
		if err := validateManifestOperation(operation); err != nil {
			return fmt.Errorf("promptCache request: %w", err)
		}
	}
	return nil
}

func (c *PromptCacheSpec) ValidateResource(value string) error {
	pattern, err := regexp.Compile(c.Definition.ResourcePattern)
	if err != nil {
		return err
	}
	if value == "" || !pattern.MatchString(value) {
		return fmt.Errorf("插件 Prompt Cache 返回了无效资源名")
	}
	return nil
}

func (c *PromptCacheSpec) Prefix(body map[string]any) map[string]any {
	if value, _ := body[c.Definition.ApplyField].(string); strings.TrimSpace(value) != "" {
		return nil
	}
	prefix := map[string]any{}
	for _, key := range c.Definition.Fields {
		if value := body[key]; value != nil {
			prefix[key] = value
		}
	}
	return prefix
}

func (c *PromptCacheSpec) Apply(spec RequestSpec, resource string) RequestSpec {
	apply := func(body any) {
		if object, ok := body.(map[string]any); ok {
			for _, key := range c.Definition.Fields {
				delete(object, key)
			}
			object[c.Definition.ApplyField] = resource
		}
	}
	apply(spec.Body)
	if spec.Stream != nil {
		apply(spec.Stream.Request.Body)
	}
	return spec
}

func (c *PromptCacheSpec) BuildCreate(prefix map[string]any) (RequestSpec, error) {
	return buildManifestOperationWithEnv(c.Definition.Create, c.Auth, GenerationRequest{Model: c.Model}, "", map[string]any{"cache": map[string]any{"prefix": prefix, "model": c.Model}})
}

func (c *PromptCacheSpec) BuildDelete(resource string) (RequestSpec, error) {
	if err := c.ValidateResource(resource); err != nil {
		return RequestSpec{}, err
	}
	return buildManifestOperationWithEnv(c.Definition.Delete, c.Auth, GenerationRequest{Model: c.Model}, "", map[string]any{"cache": map[string]any{"resourceName": resource, "model": c.Model}})
}

func (c *PromptCacheSpec) ParseCreate(body []byte) (string, string, error) {
	payload, err := decodeObject(body)
	if err != nil {
		return "", "", err
	}
	env := map[string]any{"response": payload}
	resource, err := evaluateManifestValue(c.Definition.Resource, env)
	if err != nil {
		return "", "", err
	}
	expiry, err := evaluateManifestValue(c.Definition.ExpireTime, env)
	if err != nil {
		return "", "", err
	}
	name := manifestString(resource)
	return name, manifestString(expiry), c.ValidateResource(name)
}

func (c *PromptCacheSpec) IsMissing(status int, body, resource string) bool {
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(body), &payload)
	value, err := evaluateManifestValue(c.Definition.Missing, map[string]any{"response": payload, "error": map[string]any{"status": status, "body": body, "resourceName": resource}})
	return err == nil && manifestTruthy(value)
}
