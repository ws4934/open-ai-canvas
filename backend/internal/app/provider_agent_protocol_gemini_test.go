package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCanonicalAgentGeminiBodyUsesSupportedSchemaSubset(t *testing.T) {
	tools := cloudAgentAllToolSchemas()
	before, _ := json.Marshal(tools)
	body := canonicalAgentGeminiBody(&canonicalAgentRequest{Tools: tools, ToolChoice: "auto"})
	raw, _ := json.Marshal(body)
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, value interface{})
	walk = func(path string, value interface{}) {
		switch node := value.(type) {
		case []interface{}:
			for _, item := range node {
				walk(path, item)
			}
		case map[string]interface{}:
			for _, key := range []string{"const", "exclusiveMinimum", "exclusiveMaximum", "uniqueItems"} {
				if _, ok := node[key]; ok {
					t.Errorf("%s keeps unsupported %s", path, key)
				}
			}
			if values, ok := node["enum"].([]interface{}); ok {
				for _, item := range values {
					if _, ok := item.(string); !ok {
						t.Errorf("%s keeps non-string enum %v", path, values)
					}
				}
			}
			for key, child := range node {
				walk(path+"."+key, child)
			}
		}
	}
	walk("tools", decoded.(map[string]interface{})["tools"])
	if after, _ := json.Marshal(tools); !bytes.Equal(before, after) {
		t.Fatal("gemini conversion mutated shared tool definitions")
	}
}
