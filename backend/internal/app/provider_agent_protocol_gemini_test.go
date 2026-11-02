package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestGeminiRuntimeToolRoundTripRetainsCallsNamesAndSignatures(t *testing.T) {
	call := cloudAgentCall{ID: "lookup", ThoughtSignature: "signed-thought"}
	call.Function.Name, call.Function.Arguments = "canvas_read_storyboard", `{"nodeId":"story"}`
	part := runtimeToolCalls([]cloudAgentCall{call})[0]
	part["type"] = "toolCall"
	runtimeMessages := []map[string]any{
		{"role": "user", "content": "继续制作"},
		{"role": "assistant", "content": []any{part}},
		{"role": "toolResult", "toolCallId": call.ID, "content": []any{map[string]any{"type": "text", "text": `{"snapshotHash":"node-version"}`}}},
	}
	canonical, err := canonicalFromRuntimeMessages(runtimeMessages, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(canonical)
	body := geminiAgentBodyForTest(&canonical)
	messages := interfaceSlice(body["contents"])
	modelMessage := messages[1].(map[string]any)
	parts := interfaceSlice(modelMessage["parts"])
	var functionPart map[string]any
	for _, value := range parts {
		if p := value.(map[string]any); p["functionCall"] != nil {
			functionPart = p
		}
	}
	if functionPart == nil || functionPart["thoughtSignature"] != "signed-thought" {
		t.Fatalf("续轮丢失 Gemini functionCall 或签名：%#v", modelMessage)
	}
	function := functionPart["functionCall"].(map[string]any)
	if function["id"] != call.ID || function["name"] != call.Function.Name || function["args"].(map[string]any)["nodeId"] != "story" {
		t.Fatalf("工具调用转换不完整：%#v", function)
	}
	response := interfaceSlice(messages[2].(map[string]any)["parts"])[0].(map[string]any)["functionResponse"].(map[string]any)
	if response["id"] != call.ID || response["name"] != call.Function.Name {
		t.Fatalf("工具结果丢失原工具名称：%#v", response)
	}
	chat := canonicalAgentChatBody(&canonical, false)
	chatJSON, _ := json.Marshal(chat)
	if bytes.Contains(chatJSON, []byte("thoughtSignature")) {
		t.Fatalf("Gemini 签名泄漏到其他协议：%s", chatJSON)
	}
	if after, _ := json.Marshal(canonical); !bytes.Equal(before, after) {
		t.Fatal("协议转换修改了规范历史")
	}
}

func TestGeminiAgentKeepsParallelToolResultsInOneTurn(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		canonical := canonicalAgentRequest{Messages: []map[string]any{{"role": "user", "content": "读取两个节点"}}}
		calls := []any{}
		for _, id := range []string{"first", "second"} {
			if legacy {
				canonical.Messages = append(canonical.Messages, map[string]any{"type": "function_call", "call_id": id, "name": "canvas_get_state", "arguments": "{}"})
			} else {
				calls = append(calls, map[string]any{"id": id, "function": map[string]any{"name": "canvas_get_state", "arguments": "{}"}})
			}
		}
		if !legacy {
			canonical.Messages = append(canonical.Messages, map[string]any{"role": "assistant", "content": "", "tool_calls": calls})
		}
		for _, id := range []string{"first", "second"} {
			canonical.Messages = append(canonical.Messages, map[string]any{"role": "tool", "tool_call_id": id, "content": "{}"})
		}
		body := geminiAgentBodyForTest(&canonical)
		contents := interfaceSlice(body["contents"])
		if len(contents) != 3 {
			t.Fatalf("legacy=%v 工具批次被拆成多个回合：%#v", legacy, contents)
		}
		for i, field := range []string{"functionCall", "functionResponse"} {
			parts := interfaceSlice(contents[i+1].(map[string]any)["parts"])
			if len(parts) != 2 || parts[0].(map[string]any)[field] == nil || parts[1].(map[string]any)[field] == nil {
				t.Fatalf("legacy=%v 的 %s 没有保持成组：%#v", legacy, field, parts)
			}
		}
	}
}

func TestCanonicalAgentGeminiBodyUsesSupportedSchemaSubset(t *testing.T) {
	tools := cloudAgentAllToolSchemas()
	before, _ := json.Marshal(tools)
	body := geminiAgentBodyForTest(&canonicalAgentRequest{Tools: tools, ToolChoice: "auto"})
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
