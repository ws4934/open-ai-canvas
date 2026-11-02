package protocol

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestDeclarativeStreamRulesPreserveWhitespaceAndUsage(t *testing.T) {
	rules := ManifestStream{
		Text: map[string]any{"$ref": "response.delta"}, Reasoning: map[string]any{"$ref": "response.thought"},
		Append:   map[string]any{"calls": map[string]any{"$ref": "response.calls"}},
		Merge:    map[string]any{"usage": map[string]any{"$ref": "response.usage"}},
		Complete: map[string]any{"$eq": []any{map[string]any{"$ref": "event"}, "finished"}},
		Error:    map[string]any{"$ref": "response.failure"}, Response: map[string]any{"$ref": "stream"},
	}
	decoder := NewStreamDecoder(rules)
	for _, event := range []map[string]any{
		{"delta": "hello", "thought": "think ", "usage": map[string]any{"input": 12}},
		{"delta": " ", "calls": []any{map[string]any{"name": "read"}}},
		{"delta": "world", "usage": map[string]any{"output": 4}},
	} {
		if _, _, err := decoder.Consume("delta", event); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := decoder.Finish(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("missing completion: %v", err)
	}
	if _, _, err := decoder.Consume("finished", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	data, err := decoder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["text"] != "hello world" || result["reasoning"] != "think " {
		t.Fatalf("whitespace lost: %s", data)
	}
	usage := result["merge"].(map[string]any)["usage"].(map[string]any)
	if usage["input"] != float64(12) || usage["output"] != float64(4) {
		t.Fatalf("usage lost: %s", data)
	}
	if len(result["append"].(map[string]any)["calls"].([]any)) != 1 {
		t.Fatalf("tool calls lost: %s", data)
	}
	_, _, err = decoder.Consume("delta", map[string]any{"failure": map[string]any{"code": "blocked", "message": "policy refused"}})
	var failure *ResponseError
	if !errors.As(err, &failure) || failure.Code != "blocked" {
		t.Fatalf("plugin error lost: %v", err)
	}
	if _, err := decoder.Finish(); !errors.As(err, &failure) {
		t.Fatalf("failure must invalidate completion: %v", err)
	}
}

func TestSchemaSubsetPreservesPropertyNamesAndLiteralValues(t *testing.T) {
	source := map[string]any{"type": "object", "properties": map[string]any{
		"const":       map[string]any{"type": "string", "const": "choice"},
		"selection":   map[string]any{"const": "hello"},
		"uniqueItems": map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"type": "number", "enum": []any{1, 2}}},
	}, "default": map[string]any{"const": "literal", "uniqueItems": "literal"}}
	before, _ := json.Marshal(source)
	rules := map[string]any{"constToEnum": true, "stringEnumOnly": true, "omitKeywords": []any{"uniqueItems"}}
	value, err := evaluateManifestValue(map[string]any{"$schemaSubset": map[string]any{"from": map[string]any{"$ref": "schema"}, "constToEnum": true, "stringEnumOnly": true, "inferStringEnumType": true, "enumHintPrefix": " choices:", "omitKeywords": rules["omitKeywords"]}}, map[string]any{"schema": source})
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	properties := result["properties"].(map[string]any)
	if !reflect.DeepEqual(properties["const"].(map[string]any)["enum"], []any{"choice"}) || properties["uniqueItems"] == nil || !reflect.DeepEqual(result["default"], source["default"]) {
		t.Fatalf("projection changed data: %#v", result)
	}
	array := properties["uniqueItems"].(map[string]any)
	if properties["selection"].(map[string]any)["type"] != "string" || array["items"].(map[string]any)["description"] != "choices:[1,2]" {
		t.Fatalf("constraint hints lost: %#v", properties)
	}
	if array["uniqueItems"] != nil || array["items"].(map[string]any)["enum"] != nil {
		t.Fatalf("unsupported constraints remain: %#v", array)
	}
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("source schema mutated")
	}
}

func TestPromptCacheDeclarationRejectsInvalidContracts(t *testing.T) {
	valid := ManifestPromptCache{Fields: []string{"systemInstruction"}, Model: "model", MinimumTokens: 1, Create: ManifestOperation{Method: "POST", Path: "/caches"}, Delete: ManifestOperation{Method: "DELETE", Path: "/caches/item"}, Resource: "resource", ExpireTime: "expiry", ResourcePattern: "^caches/[a-z]+$", ApplyField: "cachedContent", Missing: false}
	if err := validatePromptCache(valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ManifestPromptCache){
		"partial regex":    func(c *ManifestPromptCache) { c.ResourcePattern = "caches/" },
		"invalid regex":    func(c *ManifestPromptCache) { c.ResourcePattern = "^[a$" },
		"nested":           func(c *ManifestPromptCache) { c.Create.PromptCache = &ManifestPromptCache{} },
		"field path":       func(c *ManifestPromptCache) { c.ApplyField = "cached.resource" },
		"missing response": func(c *ManifestPromptCache) { c.Resource = nil },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := valid
			mutate(&invalid)
			if validatePromptCache(invalid) == nil {
				t.Fatal("invalid cache contract accepted")
			}
		})
	}
}
