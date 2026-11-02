package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Provider-neutral facts for declarative Agent templates. Tool batches retain
// their pairing; naming, media encoding and upstream field names belong to plugins.
func canonicalAgentConversation(source *canonicalAgentRequest) (map[string]any, error) {
	var instructions []string
	turns := []any{}
	callNames := map[string]string{}
	previousKind := ""
	for _, message := range source.Messages {
		role, kind := stringField(message, "role"), ""
		parts := []any{}
		if role == "system" {
			if text := canonicalAgentText(message["content"]); text != "" {
				instructions = append(instructions, text)
			}
			continue
		}
		addCall := func(id, name string, args any, signature string) error {
			var object any = args
			if raw, ok := args.(string); ok {
				if err := json.Unmarshal([]byte(raw), &object); err != nil {
					return fmt.Errorf("Agent 工具参数 JSON 无效：%w", err)
				}
			}
			callNames[id] = name
			parts = append(parts, map[string]any{"kind": "tool_call", "id": id, "name": name, "arguments": object, "signature": signature})
			return nil
		}
		if stringField(message, "type") == "function_call" {
			role, kind = "assistant", "calls"
			id, err := canonicalAgentFunctionCallID(message, "Agent 工具调用")
			if err != nil {
				return nil, err
			}
			if err := addCall(id, stringField(message, "name"), message["arguments"], stringField(message, "thoughtSignature")); err != nil {
				return nil, err
			}
		} else if role == "tool" {
			kind = "results"
			id := stringField(message, "tool_call_id")
			name := callNames[id]
			if name == "" {
				return nil, fmt.Errorf("Agent 工具结果 %s 缺少调用名称", id)
			}
			parts = append(parts, map[string]any{"kind": "tool_result", "id": id, "name": name, "result": canonicalAgentJSONValue(message["content"])})
		} else {
			if items, ok := message["content"].([]any); ok {
				for _, value := range items {
					item, _ := value.(map[string]any)
					typeName := stringField(item, "type")
					if typeName == "text" {
						parts = append(parts, map[string]any{"kind": "text", "text": item["text"]})
						continue
					}
					media, _ := item[typeName].(map[string]any)
					url := stringField(media, "url")
					mime, data, inline := canonicalAgentDataURL(url)
					if !inline {
						mime = stringField(media, "mimeType")
						if mime == "" && typeName == "image_url" {
							mime = "image/png"
						}
					}
					parts = append(parts, map[string]any{"kind": "media", "url": url, "mimeType": mime, "data": data, "inline": inline})
				}
			} else if text := canonicalAgentText(message["content"]); text != "" {
				parts = append(parts, map[string]any{"kind": "text", "text": text})
			}
			for _, call := range canonicalAgentToolCalls(message["tool_calls"]) {
				function, _ := call["function"].(map[string]any)
				id, err := canonicalAgentToolCallID(call, "Agent 工具调用")
				if err != nil {
					return nil, err
				}
				if err := addCall(id, stringField(function, "name"), function["arguments"], stringField(call, "thoughtSignature")); err != nil {
					return nil, err
				}
			}
		}
		if len(parts) == 0 {
			continue
		}
		if kind != "" && kind == previousKind && len(turns) > 0 {
			previous := turns[len(turns)-1].(map[string]any)
			previous["parts"] = append(interfaceSlice(previous["parts"]), parts...)
		} else {
			turns = append(turns, map[string]any{"role": role, "parts": parts})
		}
		previousKind = kind
	}
	tools := []any{}
	for _, tool := range source.Tools {
		tools = append(tools, tool["function"])
	}
	return map[string]any{"instructions": strings.Join(instructions, "\n\n"), "messages": turns, "tools": tools, "toolChoice": source.ToolChoice}, nil
}
