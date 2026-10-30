package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yingce/backend/internal/model"
	"yingce/backend/internal/platform"
	"yingce/backend/internal/protocol"
)

const geminiStreamStart = `{"candidates":[{"index":0,"finishReason":"FINISH_REASON_UNSPECIFIED","content":{"parts":[{"thought":true,"text":"思考 "},{"text":"{\"scenes\":["}]} }],"usageMetadata":{"promptTokenCount":120,"cachedContentTokenCount":60}}`
const geminiStreamEnd = `{"candidates":[{"index":0,"content":{"parts":[{"text":"1]} "}]},"finishReason":"STOP"}],"usageMetadata":{"candidatesTokenCount":9,"thoughtsTokenCount":4}}`

func TestGeminiStreamingRequestPreservesPluginContract(t *testing.T) {
	spec := protocol.RequestSpec{
		Method: "POST", Path: "/v1beta/models/vendor%2Fmodel:generateContent?trace=kept&alt=json", ContentType: "application/json",
		Headers: map[string]string{"X-Trace": "kept"}, Query: map[string][]string{"alt": {"json"}, "other": {"kept"}},
		Auth: protocol.ManifestAuth{Type: "header", Field: "apiKey", Header: "x-goog-api-key"},
		Body: map[string]any{"stream": true, "cachedContent": "cachedContents/kept", "systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "kept"}}}},
	}
	before, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	got, err := geminiStreamingRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/v1beta/models/vendor%2Fmodel:streamGenerateContent?trace=kept" || !reflect.DeepEqual(got.Query["alt"], []string{"sse"}) || got.Query["other"][0] != "kept" {
		t.Fatalf("request path/query changed incorrectly: %#v", got)
	}
	body := protocolBodyObject(got.Body)
	if _, exists := body["stream"]; exists || body["cachedContent"] != "cachedContents/kept" || body["systemInstruction"] == nil || got.Auth != spec.Auth || got.Headers["X-Trace"] != "kept" {
		t.Fatalf("native Gemini request lost its contract: %#v", got)
	}
	after, _ := json.Marshal(spec)
	if string(before) != string(after) {
		t.Fatal("stream adaptation mutated the original plugin request")
	}
	if _, err := geminiStreamingRequest(got); err != nil {
		t.Fatalf("already-streaming endpoint rejected: %v", err)
	}
	spec.Path = "/v1beta/cachedContents"
	if _, err := geminiStreamingRequest(spec); err == nil {
		t.Fatal("unrelated Gemini endpoint was rewritten")
	}
}

func TestGeminiNativeTextStreamingIsIncremental(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstDelta := make(chan struct{}, 1)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" || r.URL.RawQuery != "alt=sse" || r.Header.Get("Accept") != "text/event-stream" || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("wrong native streaming request: %s %s, Accept=%q", r.Method, r.URL, r.Header.Get("Accept"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode Gemini request: %v", err)
		}
		if _, exists := body["stream"]; exists || body["contents"] == nil || body["systemInstruction"] == nil {
			t.Errorf("native body mapping lost: %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprintf(w, "data: %s\r\n\r\n", geminiStreamStart)
		w.(http.Flusher).Flush()
		select {
		case <-firstDelta:
		case <-ctx.Done():
			t.Error("Gemini output was buffered until upstream EOF")
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", geminiStreamEnd)
	}))
	defer server.Close()
	var visible, thoughts strings.Builder
	input := canvasGenerationInput{
		Mode: "text", Prompt: "写分镜 JSON", StreamText: true,
		Config: providerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "gemini-test", InterfaceType: "gemini-generate-content", SystemPrompt: "保持 JSON 完整"},
		OnTextDelta: func(delta string) {
			visible.WriteString(delta)
			select {
			case firstDelta <- struct{}{}:
			default:
			}
		},
		OnReasoningDelta: func(delta string) { thoughts.WriteString(delta) },
	}
	result, err := runTextTask(withProtocolRegistry(ctx, loadOfficialFallbackRegistry()), input)
	if err != nil {
		t.Fatal(err)
	}
	if result["text"] != `{"scenes":[1]} ` || visible.String() != result["text"] || result["reasoning"] != "思考 " || thoughts.String() != "思考 " || calls.Load() != 1 {
		t.Fatalf("incomplete/duplicated output: result=%#v text=%q reasoning=%q calls=%d", result, visible.String(), thoughts.String(), calls.Load())
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(result["text"].(string)), &document); err != nil {
		t.Fatalf("assembled drama JSON is invalid: %v", err)
	}
}

func TestGeminiTextStreamParserChunkBoundaries(t *testing.T) {
	parser := newStreamingAgentParser("gemini-generate-content", nil)
	stream := "data: " + geminiStreamStart + "\r\n\r\n: heartbeat\r\n\r\ndata: " + geminiStreamEnd
	for _, value := range []byte(stream) {
		parser.consume("text/event-stream", []byte{value})
	}
	if err := parser.completeGeminiStream("text/event-stream"); err != nil {
		t.Fatal(err)
	}
	result, err := parser.result()
	if err != nil || result["text"] != `{"scenes":[1]} ` || parser.geminiUsage["promptTokenCount"] != float64(120) || parser.geminiUsage["cachedContentTokenCount"] != float64(60) || parser.geminiUsage["candidatesTokenCount"] != float64(9) {
		t.Fatalf("result=%#v usage=%#v err=%v", result, parser.geminiUsage, err)
	}
}

func TestGeminiNativeTextStreamingFailures(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, tc := range []struct {
		name, frame, want string
		unexpectedEOF     bool
	}{
		{"missing_terminal", geminiStreamStart, "生成完成前中断", true},
		{"malformed_event", `{"candidates":`, "解析失败", false},
		{"upstream_error", `{"error":{"code":503,"message":"upstream unavailable"}}`, "", false},
		{"token_limit", `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`, "Token 上限", false},
		{"safety", `{"candidates":[{"finishReason":"SAFETY"}]}`, "安全审核", false},
		{"blocked_prompt", `{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"}}`, "安全审核", false},
		{"empty_stop", `{"candidates":[{"finishReason":"STOP"}]}`, "没有返回内容", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", tc.frame)
			}))
			defer server.Close()
			input := canvasGenerationInput{Mode: "text", Prompt: "hello", StreamText: true, Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "gemini-generate-content"}}
			result, err := runTextTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), input)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result != nil || calls.Load() != 1 {
				t.Fatalf("result=%#v err=%v calls=%d", result, err, calls.Load())
			}
			if tc.name == "upstream_error" {
				var payloadError providerPayloadError
				if !errors.As(err, &payloadError) || payloadError.raw != "upstream unavailable" {
					t.Fatalf("upstream error was lost: %v", err)
				}
			}
			if tc.unexpectedEOF && (!errors.Is(err, io.ErrUnexpectedEOF) || !billingFailureUncertain(err)) {
				t.Fatalf("truncated stream must retain uncertain-billing semantics: %v", err)
			}
		})
	}
}

func TestGeminiStreamingCancellationClosesUpstream(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", geminiStreamStart)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	input := canvasGenerationInput{Mode: "text", Prompt: "hello", StreamText: true, Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "gemini-generate-content"}, OnTextDelta: func(string) { cancel() }}
	_, err := runTextTask(withProtocolRegistry(ctx, loadOfficialFallbackRegistry()), input)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation changed to a stream parse error: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close the upstream request")
	}
}

func TestGeminiStreamingDisabledRetainsJSONEndpoint(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" || r.URL.Query().Get("alt") != "" || r.Header.Get("Accept") == "text/event-stream" {
			t.Errorf("nonstreaming request changed: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	result, err := runTextTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), canvasGenerationInput{Mode: "text", Prompt: "hello", Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "gemini-generate-content"}})
	if err != nil || result["text"] != "hello" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestGeminiStreamUsageAndFailureAreRecordedBeforeSuccess(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, complete := range []bool{true, false} {
		t.Run(fmt.Sprint(complete), func(t *testing.T) {
			svc, db := newTimelineTaskTestService(t)
			svc.coordinator = platform.NewCoordinatorWithRedis(nil, "gemini-stream-test")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", geminiStreamStart)
				if complete {
					fmt.Fprintf(w, "data: %s\n\n", geminiStreamEnd)
				}
			}))
			defer server.Close()
			ctx := context.WithValue(context.Background(), providerAnalyticsKey{}, providerAnalyticsContext{Service: svc, UserID: "usr", Capability: "text", Model: "gemini-test"})
			_, _, err := executeProtocolCreateRequest(ctx, canvasGenerationInput{Mode: "text", StreamText: true, Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "gemini-generate-content"}}, protocol.RequestSpec{Method: "POST", Path: "/v1beta/models/gemini-test:generateContent", ContentType: "application/json", Body: map[string]any{"contents": []any{}}})
			if (err == nil) != complete {
				t.Fatalf("complete=%v err=%v", complete, err)
			}
			var logs []model.ApiCallLog
			if err := db.Find(&logs).Error; err != nil || len(logs) != 1 {
				t.Fatalf("logs=%#v err=%v", logs, err)
			}
			entry := logs[0]
			if !entry.UsageAvailable || entry.InputTokens != 120 || entry.CachedTokens != 60 {
				t.Fatalf("usage=%#v", entry)
			}
			if complete && (entry.Status != model.ApiCallStatusSucceeded || entry.OutputTokens != 9) {
				t.Fatalf("success usage=%#v", entry)
			}
			if !complete && entry.Status != model.ApiCallStatusFailed {
				t.Fatalf("partial stream was recorded as a successful billable call: %#v", entry)
			}
		})
	}
}

func TestGeminiTextStreamingHTTPFailuresDoNotRetry(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, status := range []int{524, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"not streamed"}]},"finishReason":"STOP"}]}`)
			}))
			defer server.Close()
			_, err := runTextTask(withProtocolRegistry(context.Background(), loadOfficialFallbackRegistry()), canvasGenerationInput{Mode: "text", Prompt: "hello", StreamText: true, Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "gemini-generate-content"}})
			if err == nil || calls.Load() != 1 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
			if status == 524 {
				var upstream providerHTTPError
				if !errors.As(err, &upstream) || upstream.StatusCode != 524 || !billingFailureUncertain(err) {
					t.Fatalf("524 semantics changed: %v", err)
				}
			} else if !strings.Contains(err.Error(), "未返回 SSE") {
				t.Fatalf("nonstreaming response was silently accepted: %v", err)
			}
		})
	}
}

func TestGeminiCanvasTextTaskPersistsDraftBeforeUpstreamFinishes(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	svc, db := newTimelineTaskTestService(t)
	svc.coordinator = platform.NewCoordinatorWithRedis(nil, "gemini-text-task-test")
	task := &model.Task{ID: "gemini-stream-task", UserID: "usr", Type: "canvas_text", Status: model.TaskStatusRunning}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	prefix := `{"scenes":["` + strings.Repeat("镜头", 1024)
	frame, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": prefix}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:streamGenerateContent" || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("real text task did not select native SSE: %s", r.URL)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", frame)
		w.(http.Flusher).Flush()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			var deltas []model.TaskTextDelta
			if err := db.Where("task_id = ?", task.ID).Find(&deltas).Error; err != nil {
				t.Errorf("read task delta: %v", err)
				return
			}
			if len(deltas) > 0 && deltas[0].Content == prefix {
				break
			}
			select {
			case <-ctx.Done():
				t.Error("native text delta was not persisted before upstream completion")
				return
			case <-ticker.C:
			}
		}
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"\\\"]}\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer server.Close()
	enabled := true
	input, err := json.Marshal(canvasGenerationInput{Mode: "text", TextOptions: canvasTextOptions{Stream: &enabled}, Config: providerConfig{APIFormat: "gemini", BaseURL: server.URL, APIKey: "test-key", Model: "gemini-test", InterfaceType: "gemini-generate-content"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.processCanvasGenerationTask(withTaskExecutionID(ctx, task.ID), "usr", "", "canvas_text", "写分镜 JSON", string(input))
	if err != nil || result["text"] != prefix+`"]}` {
		t.Fatalf("real task result=%#v err=%v", result, err)
	}
	replay, err := svc.TaskTextReplay("usr", task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var replayed strings.Builder
	replayed.WriteString(replay.TextDraft)
	for _, delta := range replay.Deltas {
		replayed.WriteString(delta.Content)
	}
	if replayed.String() != result["text"] || len(replay.Deltas) < 2 {
		t.Fatalf("draft not retained: %d events, %d bytes, want %d", len(replay.Deltas), replayed.Len(), len(result["text"].(string)))
	}
}
