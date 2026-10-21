package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yingce/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
)

func TestOpenAPIDocumentReferencesResolve(t *testing.T) {
	var document map[string]any
	if err := yaml.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatal(err)
	}
	var check func(any)
	check = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if ref, ok := value["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
				var target any = document
				for _, key := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					object, ok := target.(map[string]any)
					if !ok || object[key] == nil {
						t.Fatalf("unresolved OpenAPI reference %s", ref)
					}
					target = object[key]
				}
			}
			for _, child := range value {
				check(child)
			}
		case []any:
			for _, child := range value {
				check(child)
			}
		}
	}
	check(document)
}

func TestRegisterCanvasAPIExposesOpenAPIAndProjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), &service.Service{})

	wanted := map[string]bool{
		"GET /api/openapi.yaml":                         false,
		"GET /api/workbench/tools":                      false,
		"GET /api/workbench/tasks":                      false,
		"POST /api/workbench/tasks":                     false,
		"GET /api/projects":                             false,
		"POST /api/tasks":                               false,
		"POST /api/tasks/:id/recover-media":             false,
		"GET /api/resources":                            false,
		"GET /api/skills/presets":                       false,
		"GET /api/agent/skills/usage":                   false,
		"POST /api/admin/channels/export":               false,
		"POST /api/admin/channels/import":               false,
		"GET /api/admin/settings/billing-auto-refund":   false,
		"PATCH /api/admin/settings/billing-auto-refund": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("route %s is not registered", route)
		}
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "openapi: 3.0.3") || !strings.Contains(recorder.Body.String(), "url: /api") {
		t.Fatalf("openapi.yaml status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, path := range []string{"/admin/channels/export:", "/admin/channels/import:", "/admin/settings/billing-auto-refund:"} {
		if !strings.Contains(recorder.Body.String(), path) {
			t.Fatalf("OpenAPI omitted %s", path)
		}
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/skills/presets", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"presets"`) {
		t.Fatalf("skills presets status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
