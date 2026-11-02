package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "yingce/backend/internal/agent/runtime"
	"yingce/backend/internal/model"
)

func TestCloudAgentRuntimeCompletesGeminiStreamingToolRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	if _, err := agentRuntimeDirForTest(); err != nil {
		t.Skip(err.Error())
	}
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	t.Setenv("REDIS_URL", "")
	var mu sync.Mutex
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1beta/cachedContents" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":{"code":403,"message":"aiplatform.cachedContents.create denied"}}`)
			return
		}
		if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("Gemini 助手未使用原生 SSE：%s", r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		mu.Lock()
		requests++
		index := requests
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		foundCall, foundResult := false, false
		for _, value := range interfaceSlice(body["contents"]) {
			content := value.(map[string]any)
			for _, value := range interfaceSlice(content["parts"]) {
				part := value.(map[string]any)
				if call, ok := part["functionCall"].(map[string]any); ok {
					foundCall = call["name"] == "canvas_get_state" && part["thoughtSignature"] == "round-trip-signature"
				}
				if response, ok := part["functionResponse"].(map[string]any); ok {
					foundResult = response["name"] == "canvas_get_state"
				}
			}
		}
		if index > 1 && (!foundCall || !foundResult) {
			t.Errorf("真实 Node 续轮丢失调用、结果名称或签名：%#v", body["contents"])
		}
		if index <= 2 {
			fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"canvas_get_state\",\"args\":{}},\"thoughtSignature\":\"round-trip-signature\"}]},\"finishReason\":\"STOP\"}]}\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"已核对工具结果\"},{\"text\":\"画布已\"},{\"text\":\"检查完毕\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer upstream.Close()
	s, db, _, _ := creationTestService(t)
	s = New(s.repo, s.dataDir)
	t.Cleanup(func() { _ = s.Close() })
	if err := db.Model(&model.ModelChannel{}).Where("id = ?", "channel").Updates(map[string]any{"base_url": upstream.URL, "api_key": "test-only"}).Error; err != nil {
		t.Fatal(err)
	}
	config := mustEncodeModelCapabilityConfig(t, DefaultModelCapabilityConfigForModel(officialGeminiAgentInterface, "text-test"))
	if err := db.Model(&model.ChannelModel{}).Where("id = ?", "cm").Updates(map[string]any{"protocol": officialGeminiAgentInterface, "capability_config_json": config}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CanvasProject{ID: "agent-canvas", UserID: "user", PayloadJSON: `{"nodes":[]}`}).Error; err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = s.ProcessNextTask()
			time.Sleep(20 * time.Millisecond)
		}
	}()
	run, err := s.CreateCloudAgentRun("user", agentTestRequest(), "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		stored, err := s.repo.CloudAgent("user", run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cloudAgentRunTerminal(stored.Status) {
			if stored.Status != "completed" {
				t.Fatalf("Gemini 助手没有完成工具续轮：%s / %s", stored.Status, stored.FailureMessage)
			}
			state, err := cloudAgentDecode(stored)
			if err != nil {
				t.Fatal(err)
			}
			events, _ := json.Marshal(state.Events)
			if !strings.Contains(string(events), "画布已检查完毕") {
				t.Fatal("最终正文没有回传运行事件")
			}
			mu.Lock()
			count := requests
			mu.Unlock()
			if count != 3 {
				t.Fatalf("Gemini 工具往返意外重发了模型步骤：%d", count)
			}
			callIDs := map[string]bool{}
			for _, event := range state.Events {
				if event.Type == "tool_completed" && event.Payload["toolName"] == "canvas_get_state" {
					callIDs[stringValue(event.Payload["callId"])] = true
				}
			}
			if len(callIDs) != 2 {
				t.Fatalf("相同读取在续轮被误判为旧请求回放：%v", callIDs)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Gemini 助手工具往返没有结束")
}

// 端到端：生产入口 CreateCloudAgentRun → 真实 Node 运行时 → /model 桥接 → 任务 worker →
// 模拟上游（第一步返回工具调用，第二步返回正文）→ /tool 桥接 → 运行完成。
// 任何一层格式或并发问题都会让这里失败，而不是等用户在界面上一个个撞到。
func TestCloudAgentRuntimeCompletesToolRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	if _, err := agentRuntimeDirForTest(); err != nil {
		t.Skip(err.Error())
	}
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	t.Setenv("REDIS_URL", "")

	var mu sync.Mutex
	var bodies []map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode upstream body: %v", err)
		}
		mu.Lock()
		bodies = append(bodies, body)
		call := len(bodies)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			tool := firstUpstreamToolName(body)
			if tool == "" {
				t.Errorf("first model step exposed no tools: %v", body["tools"])
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"没有工具"}}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"` + tool + `","arguments":"{}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"画布已检查完毕"}}]}`))
	}))
	defer upstream.Close()

	s, db, _, _ := creationTestService(t)
	s = New(s.repo, s.dataDir)
	t.Cleanup(func() { _ = s.Close() })
	if err := db.Model(&model.ModelChannel{}).Where("id = ?", "channel").Updates(map[string]any{"base_url": upstream.URL, "api_key": "test-only"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CanvasProject{ID: "agent-canvas", UserID: "user", PayloadJSON: `{"nodes":[]}`}).Error; err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = s.ProcessNextTask()
			time.Sleep(20 * time.Millisecond)
		}
	}()

	run, err := s.CreateCloudAgentRun("user", agentTestRequest(), "")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(60 * time.Second)
	var execution *model.CloudAgentExecution
	for time.Now().Before(deadline) {
		execution, err = s.repo.CloudAgent("user", run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cloudAgentRunTerminal(execution.Status) || execution.Status == "waiting_approval" || execution.Status == "waiting_user" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if execution.Status != "completed" {
		t.Fatalf("run status=%s failure=%q", execution.Status, execution.FailureMessage)
	}

	mu.Lock()
	calls := len(bodies)
	second := map[string]any{}
	if calls >= 2 {
		second = bodies[1]
	}
	mu.Unlock()
	if calls != 2 {
		t.Fatalf("expected 2 upstream model calls, got %d", calls)
	}
	encoded, _ := json.Marshal(second["messages"])
	if !strings.Contains(string(encoded), `"tool_call_id":"call-1"`) {
		t.Fatalf("second model step lost the tool result: %s", encoded)
	}
	if strings.Contains(string(encoded), "unknown tool") || strings.Contains(string(encoded), `\"error\"`) {
		t.Fatalf("tool call did not succeed: %s", encoded)
	}
	if strings.Contains(string(encoded), "canvas_node_create") {
		t.Fatalf("model saw tools from a second registry: %s", encoded)
	}
	if strings.Contains(strings.ToLower(execution.FailureMessage), "pi") {
		t.Fatalf("failure message exposes runtime name: %q", execution.FailureMessage)
	}

	full, err := s.CloudAgentRun("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	rawEvents, _ := json.Marshal(full.Events)
	if !strings.Contains(string(rawEvents), "画布已检查完毕") {
		t.Fatalf("final reply missing from run events: %s", rawEvents)
	}
	if session, err := s.repo.CloudAgentPiSession("user", run.ID); err != nil || session.SessionJSONL == "" {
		t.Fatalf("session snapshot not persisted: %v", err)
	}
}

func firstUpstreamToolName(body map[string]any) string {
	tools, _ := body["tools"].([]any)
	for _, value := range tools {
		tool, _ := value.(map[string]any)
		fn, _ := tool["function"].(map[string]any)
		name := stringField(fn, "name")
		if strings.HasPrefix(name, "canvas_") && name != "canvas_apply_ops" {
			return name
		}
	}
	for _, value := range tools {
		tool, _ := value.(map[string]any)
		fn, _ := tool["function"].(map[string]any)
		if name := stringField(fn, "name"); name != "" {
			return name
		}
	}
	return ""
}

func agentRuntimeDirForTest() (string, error) {
	return agentruntime.RuntimeDir()
}

// 端到端：审批模式下写画布 → 等待审批 → 用户批准 → Go 执行一次写入 → 运行时恢复 → 完成。
func TestCloudAgentRuntimeApprovalWriteRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	if _, err := agentRuntimeDirForTest(); err != nil {
		t.Skip(err.Error())
	}
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	t.Setenv("REDIS_URL", "")

	canvas := model.CanvasProject{ID: "agent-canvas", UserID: "user", Title: "test", PayloadJSON: `{"nodes":[]}`}
	doc, err := creationDocument(canvas.PayloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	writeArgs, _ := json.Marshal(map[string]any{
		"snapshotHash": cloudAgentCanvasHash(doc),
		"ops":          []map[string]any{{"type": "add_node", "id": "agent-note", "nodeType": "text", "title": "Agent note", "content": "由 Agent 写入", "x": 24, "y": 48}},
	})
	writeCall, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": "", "tool_calls": []any{map[string]any{"id": "write-1", "type": "function", "function": map[string]any{"name": "canvas_apply_ops", "arguments": string(writeArgs)}}},
	}}}})

	var mu sync.Mutex
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write(writeCall)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"已写入画布"}}]}`))
	}))
	defer upstream.Close()

	s, db, _, _ := creationTestService(t)
	s = New(s.repo, s.dataDir)
	t.Cleanup(func() { _ = s.Close() })
	if err := db.Model(&model.ModelChannel{}).Where("id = ?", "channel").Updates(map[string]any{"base_url": upstream.URL, "api_key": "test-only"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = s.ProcessNextTask()
			time.Sleep(20 * time.Millisecond)
		}
	}()

	req := agentTestRequest()
	req.PermissionMode = "request_approval"
	run, err := s.CreateCloudAgentRun("user", req, "")
	if err != nil {
		t.Fatal(err)
	}
	waiting := waitCloudAgentStatus(t, s, run.ID, "waiting_approval")
	if waiting.Approval == nil {
		t.Fatalf("approval missing: status=%s", waiting.Status)
	}
	if err := s.DecideCloudAgentApproval("user", run.ID, waiting.Approval.ID, "approve", "确认写入"); err != nil {
		t.Fatal(err)
	}
	final := waitCloudAgentStatus(t, s, run.ID, "completed")
	var stored model.CanvasProject
	if err := db.First(&stored, "id = ? AND user_id = ?", canvas.ID, "user").Error; err != nil {
		t.Fatal(err)
	}
	if strings.Count(stored.PayloadJSON, "由 Agent 写入") != 1 {
		t.Fatalf("approved write must be applied exactly once: %s", stored.PayloadJSON)
	}
	raw, _ := json.Marshal(final.Events)
	if !strings.Contains(string(raw), "已写入画布") {
		t.Fatalf("final reply missing after approval: %s", raw)
	}
}

func waitCloudAgentStatus(t *testing.T, s *Service, runID, want string) *CloudAgentRun {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		execution, err := s.repo.CloudAgent("user", runID)
		if err != nil {
			t.Fatal(err)
		}
		if execution.Status == want {
			run, err := s.CloudAgentRun("user", runID)
			if err != nil {
				t.Fatal(err)
			}
			return run
		}
		if cloudAgentRunTerminal(execution.Status) {
			t.Fatalf("run ended as %s (want %s): %q", execution.Status, want, execution.FailureMessage)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", want)
	return nil
}
