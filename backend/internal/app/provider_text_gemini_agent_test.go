package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/protocol"
)

func geminiAgentTestInput(baseURL string, stream bool) canvasGenerationInput {
	return canvasGenerationInput{
		Mode: "text", StreamText: stream,
		Config:        providerConfig{BaseURL: baseURL, APIKey: "test-key", Model: "gemini-test", InterfaceType: officialGeminiAgentInterface},
		AgentRequests: &agentToolRequests{Canonical: &canonicalAgentRequest{ToolChoice: "auto", Messages: []map[string]any{{"role": "user", "content": "读取分镜后继续"}}}},
	}
}

func TestDeclarativeAgentWithUnrecognizedProviderID(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	manifest := geminiManifestForTest(t)
	manifest.Contributes.Providers[0].ID = "new-unknown-text-provider"
	raw, _ := json.Marshal(manifest)
	adapter, err := protocol.LoadManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := protocol.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") || body["contents"] == nil || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("plugin mapping lost: %s / %#v", r.URL, body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"继续\"},{\"functionCall\":{\"name\":\"canvas_get_state\",\"args\":{}},\"thoughtSignature\":\"signature\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer server.Close()
	input := geminiAgentTestInput(server.URL, true)
	input.Config.InterfaceType = "new-unknown-text-provider"
	var delta strings.Builder
	input.OnTextDelta = func(s string) { delta.WriteString(s) }
	ctx := withTaskExecutionID(withProtocolRegistry(context.Background(), registry), "unknown-step")
	result, err := runAgentToolTask(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	calls, _ := json.Marshal(result["toolCalls"])
	if result["text"] != "继续" || delta.String() != "继续" || !strings.Contains(string(calls), "call_unknown-step_0") || !strings.Contains(string(calls), "signature") {
		t.Fatalf("unknown provider failed: %#v / %s", result, calls)
	}
}

func TestGeminiAgentReadsAllVisibleParts(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"内部推理"},{"text":"继续"},{"text":"制作"}]}}]}`)
	}))
	defer server.Close()
	result, err := runAgentToolTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), geminiAgentTestInput(server.URL, false))
	if err != nil {
		t.Fatal(err)
	}
	if result["text"] != "继续制作" || result["reasoning"] != "内部推理" {
		t.Fatalf("Gemini Agent 丢失正文或把推理当作回答：%#v", result)
	}
}

func TestGeminiAgentStreamsToolCallsWithSignatures(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstDelta := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") || r.URL.Query().Get("alt") != "sse" || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Agent 没有使用 Gemini 原生流式：%s / %s", r.URL, r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"推理\"},{\"text\":\"继续\"}]}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-firstDelta:
		case <-ctx.Done():
			t.Error("Agent 没有增量反馈")
			return
		}
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"制作\"},{\"functionCall\":{\"name\":\"canvas_get_state\",\"args\":{}},\"thoughtSignature\":\"kept-signature\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer server.Close()
	input := geminiAgentTestInput(server.URL, true)
	var visible, reasoning strings.Builder
	input.OnTextDelta = func(delta string) {
		visible.WriteString(delta)
		select {
		case firstDelta <- struct{}{}:
		default:
		}
	}
	input.OnReasoningDelta = func(delta string) { reasoning.WriteString(delta) }
	result, err := runAgentToolTask(withProtocolRegistry(ctx, loadOfficialFallbackRegistry()), input)
	if err != nil {
		t.Fatal(err)
	}
	if result["text"] != "继续制作" || visible.String() != "继续制作" || reasoning.String() != "推理" {
		t.Fatalf("增量和最终正文不一致：%#v / %q / %q", result, visible.String(), reasoning.String())
	}
	raw, _ := json.Marshal(result["toolCalls"])
	var calls []cloudAgentCall
	if err := json.Unmarshal(raw, &calls); err != nil || len(calls) != 1 || calls[0].ID == "" || calls[0].Function.Name != "canvas_get_state" || calls[0].ThoughtSignature != "kept-signature" {
		t.Fatalf("工具调用或签名丢失：%s / %v", raw, err)
	}
}

func TestGeminiAgentReportsFinishReasonInsteadOfEmptyOutput(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, reason := range []string{"MAX_TOKENS", "SAFETY"} {
		t.Run(reason, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"candidates":[{"finishReason":%q}]}`, reason)
			}))
			defer server.Close()
			_, err := runAgentToolTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), geminiAgentTestInput(server.URL, false))
			if err == nil || strings.Contains(err.Error(), "没有返回内容") {
				t.Fatalf("丢失 Gemini 结束原因 %s：%v", reason, err)
			}
		})
	}
}

func TestGeminiAgentRejectsInterruptedAndBlockedToolStreams(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, test := range []struct{ name, frames, want string }{
		{"interrupted", `data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"canvas_get_state","args":{}}}]}}]}` + "\n\n", "生成完成前中断"},
		{"blocked", `data: {"promptFeedback":{"blockReason":"SAFETY"}}` + "\n\n", "安全审核"},
		{"truncated", `data: {"candidates":[{"finishReason":"MAX_TOKENS"}]}` + "\n\n", "Token 上限"},
		{"malformed", "data: {not-json}\n\n", "事件解析失败"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, test.frames)
			}))
			defer server.Close()
			result, err := runAgentToolTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), geminiAgentTestInput(server.URL, true))
			if err == nil || !strings.Contains(err.Error(), test.want) || result != nil {
				t.Fatalf("异常流被当成可执行的半批工具：result=%#v err=%v", result, err)
			}
		})
	}
}

func TestGeminiAgentAcceptsCompleteJSONAndAppliesOutputBudget(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		config, _ := body["generationConfig"].(map[string]any)
		if config["maxOutputTokens"] != float64(4096) {
			t.Errorf("输出预算丢失或覆盖了插件参数：%#v", config)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"finishReason":"STOP","content":{"parts":[{}, {"text":"完整响应"}]}}]}`)
	}))
	defer server.Close()
	input := geminiAgentTestInput(server.URL, true)
	input.MaxOutputTokens = 4096
	input.TextOptions.MaxOutputTokens = 8192
	result, err := runAgentToolTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), input)
	if err != nil || result["text"] != "完整响应" {
		t.Fatalf("完整 JSON 不应被误判为空响应：%#v / %v", result, err)
	}
}

func TestGeminiAgentGeneratedCallIDsAreScopedToModelStep(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"canvas_get_state","args":{}}}]}}]}`)
	}))
	defer server.Close()
	ids := []string{}
	for _, stepID := range []string{"step-a", "step-b", "step-a"} {
		ctx := withTaskExecutionID(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), stepID)
		result, err := runAgentToolTask(ctx, geminiAgentTestInput(server.URL, false))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(result["toolCalls"])
		var calls []cloudAgentCall
		if err := json.Unmarshal(raw, &calls); err != nil || len(calls) != 1 {
			t.Fatalf("工具返回无效：%s / %v", raw, err)
		}
		ids = append(ids, calls[0].ID)
	}
	if ids[0] == ids[1] || ids[0] != ids[2] {
		t.Fatalf("不同模型步不能共用回放 ID，同一步恢复则必须稳定：%v", ids)
	}
}
