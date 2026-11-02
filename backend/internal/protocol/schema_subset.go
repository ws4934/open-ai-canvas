package protocol

import (
	"encoding/json"
	"strings"
)

// A plugin declares its upstream schema subset; the host does not infer one
// from a model or provider name. The source schema is never modified.
func manifestSchemaSubset(value any, rules map[string]any) any {
	switch input := value.(type) {
	case []any:
		out := make([]any, len(input))
		for i, item := range input {
			out[i] = manifestSchemaSubset(item, rules)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(input))
		omitted := map[string]bool{}
		for _, field := range manifestArray(rules["omitKeywords"]) {
			omitted[manifestString(field)] = true
		}
		for key, item := range input {
			if !omitted[key] && !(key == "const" && rules["constToEnum"] == true) {
				if key == "properties" || key == "$defs" || key == "definitions" || key == "patternProperties" || key == "dependentSchemas" {
					if entries, ok := item.(map[string]any); ok {
						projected := make(map[string]any, len(entries))
						for name, schema := range entries {
							projected[name] = manifestSchemaSubset(schema, rules)
						}
						out[key] = projected
						continue
					}
				}
				switch key {
				case "items", "additionalItems", "additionalProperties", "unevaluatedItems", "unevaluatedProperties", "contains", "propertyNames", "not", "if", "then", "else", "allOf", "anyOf", "oneOf":
					out[key] = manifestSchemaSubset(item, rules)
				default:
					out[key] = item
				}
			}
		}
		if rules["constToEnum"] == true {
			if constant, ok := input["const"]; ok {
				out["enum"] = []any{constant}
			}
		}
		if rules["stringEnumOnly"] == true {
			if choices, ok := out["enum"].([]any); ok {
				allStrings := true
				for _, item := range choices {
					if _, ok := item.(string); !ok {
						allStrings = false
						break
					}
				}
				if !allStrings {
					delete(out, "enum")
					if prefix, ok := rules["enumHintPrefix"].(string); ok && prefix != "" {
						hint, _ := json.Marshal(choices)
						out["description"] = strings.TrimSpace(manifestString(out["description"]) + prefix + string(hint))
					}
				} else if rules["inferStringEnumType"] == true && out["type"] == nil {
					out["type"] = "string"
				}
			}
		}
		return out
	default:
		return value
	}
}
