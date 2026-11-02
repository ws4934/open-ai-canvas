package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"yingce/backend/internal/model"
	"yingce/backend/internal/protocol"
	"yingce/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeclarativePromptCacheRecoveryIsBoundedAndOptional(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, name := range []string{"create-forbidden", "missing", "rebuild-forbidden", "missing-again", "unrelated-404"} {
		t.Run(name, func(t *testing.T) {
			var creates, deletes, requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					deletes.Add(1)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if r.URL.Path == "/v1beta/cachedContents" {
					n := creates.Add(1)
					if name == "create-forbidden" || (name == "rebuild-forbidden" && n > 1) {
						w.WriteHeader(http.StatusForbidden)
						fmt.Fprint(w, `{"error":{"code":403,"message":"cache permission denied"}}`)
						return
					}
					fmt.Fprintf(w, `{"name":"cachedContents/cache-%d","expireTime":"2099-01-01T00:00:00Z"}`, n)
					return
				}
				n := requests.Add(1)
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if name == "unrelated-404" {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"error":{"code":404,"status":"NOT_FOUND","message":"model not found"}}`)
					return
				}
				if name != "create-forbidden" && (n == 1 || name == "missing-again") {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, `{"error":{"code":404,"status":"NOT_FOUND","message":"%s no longer exists"}}`, body["cachedContent"])
					return
				}
				if (name == "create-forbidden" || name == "rebuild-forbidden") && (body["systemInstruction"] == nil || body["cachedContent"] != nil) {
					t.Errorf("uncached request lost prefix: %#v", body)
				}
				if name == "missing" && (body["systemInstruction"] != nil || body["cachedContent"] != "cachedContents/cache-2") {
					t.Errorf("rebuilt cache not used: %#v", body)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"done\"}]},\"finishReason\":\"STOP\"}]}\n\n")
			}))
			defer server.Close()
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cache.sqlite")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sql, _ := db.DB()
			t.Cleanup(func() { _ = sql.Close() })
			if err := db.AutoMigrate(&model.SystemSetting{}, &model.ApiCallLog{}, &model.CloudAgentGeminiCache{}); err != nil {
				t.Fatal(err)
			}
			svc := New(repository.New(db), t.TempDir())
			ctx := context.WithValue(context.Background(), providerAnalyticsKey{}, providerAnalyticsContext{Service: svc, UserID: "cache-user"})
			manifest := geminiManifestForTest(t)
			manifest.Contributes.Providers[0].ID = "unrecognized-cache-provider"
			raw, _ := json.Marshal(manifest)
			adapter, err := protocol.LoadManifest(raw)
			if err != nil {
				t.Fatal(err)
			}
			spec, err := adapter.(protocol.AgentAdapter).BuildAgent(ctx, protocol.AgentRequestContext{Model: "gemini-test", Request: map[string]any{"conversation": map[string]any{"instructions": strings.Repeat("stable system instruction ", 1000)}}})
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(spec)
			input := canvasGenerationInput{Mode: "text", StreamText: true, Config: providerConfig{BaseURL: server.URL, Model: "gemini-test", InterfaceType: "unrecognized-cache-provider"}, AgentRequests: &agentToolRequests{}}
			result, err := executeDeclarativeAgentWithCache(ctx, input, spec)
			wantFailure := name == "missing-again" || name == "unrelated-404"
			if (err != nil) != wantFailure || (!wantFailure && !strings.Contains(string(result), "done")) {
				t.Fatalf("result=%s err=%v", result, err)
			}
			wantCreates, wantRequests, wantDeletes := int32(2), int32(2), int32(1)
			if name == "create-forbidden" || name == "unrelated-404" {
				wantCreates, wantRequests, wantDeletes = 1, 1, 0
			}
			if creates.Load() != wantCreates || requests.Load() != wantRequests || deletes.Load() != wantDeletes {
				t.Fatalf("create=%d model=%d delete=%d", creates.Load(), requests.Load(), deletes.Load())
			}
			after, _ := json.Marshal(spec)
			if string(before) != string(after) {
				t.Fatal("cache path mutated the original plugin request")
			}
		})
	}
}

func TestAgentPromptCacheIdentityIncludesUserCredentialsAndProtocol(t *testing.T) {
	spec := geminiCacheSpecForTest(t)
	base := canvasGenerationInput{Config: providerConfig{APIKey: "first", Model: "gemini-test", InterfaceType: "first-provider"}}
	prefix := map[string]any{"systemInstruction": "same"}
	first, _, err := agentPromptCacheKey(base, "user", prefix, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"user", "credential", "protocol", "contract"} {
		input, user, config := base, "user", *spec
		switch field {
		case "user":
			user = "other"
		case "credential":
			input.Config.APIKey = "second"
		case "protocol":
			input.Config.InterfaceType = "second-provider"
		case "contract":
			config.Definition.ApplyField = "newCacheField"
		}
		next, _, err := agentPromptCacheKey(input, user, prefix, &config)
		if err != nil || next == first {
			t.Fatalf("cache identity ignored %s: %v", field, err)
		}
	}
}
