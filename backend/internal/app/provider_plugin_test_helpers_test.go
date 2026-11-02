package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"yingce/backend/internal/protocol"
)

const officialGeminiAgentInterface = "gemini-generate-content"

func geminiManifestForTest(t *testing.T) protocol.Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "google-gemini-generate-content", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest protocol.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func geminiAgentBodyForTest(source *canonicalAgentRequest) map[string]any {
	conversation, err := canonicalAgentConversation(source)
	if err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "google-gemini-generate-content", "manifest.json"))
	if err != nil {
		panic(err)
	}
	adapter, err := protocol.LoadManifest(raw)
	if err != nil {
		panic(err)
	}
	spec, err := adapter.(protocol.AgentAdapter).BuildAgent(context.Background(), protocol.AgentRequestContext{Model: "gemini-test", Request: map[string]any{"conversation": conversation}})
	if err != nil {
		panic(err)
	}
	return protocolBodyObject(spec.Body)
}

func geminiCacheSpecForTest(t *testing.T) *protocol.PromptCacheSpec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "google-gemini-generate-content", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := protocol.LoadManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := adapter.(protocol.AgentAdapter).BuildAgent(context.Background(), protocol.AgentRequestContext{Model: "gemini-test", Request: map[string]any{"conversation": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	return spec.PromptCache
}
