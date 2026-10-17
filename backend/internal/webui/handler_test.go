package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"
)

func frontend(version string) fstest.MapFS {
	return fstest.MapFS{
		"index.html":                         {Data: []byte("<!doctype html>" + version)},
		"assets/app-" + version + ".js":      {Data: []byte("version=" + version)},
		"assets/engine-" + version + ".wasm": {Data: []byte("0123456789")},
		"canvas/models/face.tflite":          {Data: []byte("model")},
		"welcome/cover.png":                  {Data: []byte("cover")},
		"inspirations/list.json":             {Data: []byte("[]")},
	}
}

func routerFor(t *testing.T, directory, version string, now time.Time) *gin.Engine {
	t.Helper()
	fallback := func(c *gin.Context) { c.JSON(http.StatusTeapot, gin.H{"path": c.Request.URL.Path}) }
	handler, err := newHandler(frontend(version), directory, version, fallback, now)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/api/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.NoRoute(handler)
	return router
}

func get(router http.Handler, method, url string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, url, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestSPARoutesAndAPIProtection(t *testing.T) {
	router := routerFor(t, t.TempDir(), "A", time.Now())
	for _, route := range []string{"/", "/index.html", "/canvas", "/canvas/", "/canvas/project-1", "/assets", "/assets/", "/welcome", "/welcome/", "/inspirations", "/inspirations/", "/admin/settings/system-update", "/share/canvas/token"} {
		response := get(router, "GET", route, nil)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "<!doctype html>A") {
			t.Errorf("%s: %d %s", route, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-cache, must-revalidate" {
			t.Errorf("%s: unexpected cache header", route)
		}
	}
	for _, route := range []string{"/api", "/api/missing", "/api/channel-id/chat/completions", "/api/tasks/unknown", "/oauth/linuxdo/unknown"} {
		if response := get(router, "GET", route, nil); response.Code != http.StatusTeapot {
			t.Errorf("%s bypassed API fallback: %d", route, response.Code)
		}
	}
	if response := get(router, "GET", "/api/health", nil); response.Code != 204 {
		t.Fatal("registered route shadowed")
	}
	if response := get(router, "POST", "/canvas", nil); response.Code != 404 {
		t.Fatal("POST returned SPA")
	}
}

func TestStaticHeadersRangeAndMissingFiles(t *testing.T) {
	router := routerFor(t, t.TempDir(), "A", time.Now())
	response := get(router, "GET", "/assets/app-A.js", nil)
	if response.Code != 200 || response.Body.String() != "version=A" || !strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %#v", response)
	}
	if response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("referrer policy missing")
	}
	if conditional := get(router, "GET", "/assets/app-A.js", map[string]string{"If-None-Match": response.Header().Get("ETag")}); conditional.Code != 304 {
		t.Fatalf("conditional status: %d", conditional.Code)
	}
	if head := get(router, "HEAD", "/assets/app-A.js", nil); head.Code != 200 || head.Body.Len() != 0 {
		t.Fatalf("HEAD: %d %s", head.Code, head.Body.String())
	}
	ranged := get(router, "GET", "/assets/engine-A.wasm", map[string]string{"Range": "bytes=2-4"})
	if ranged.Code != 206 || ranged.Body.String() != "234" || ranged.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("range: %d %#v %s", ranged.Code, ranged.Header(), ranged.Body.String())
	}
	if response := get(router, "GET", "/assets/engine-A.wasm", map[string]string{"Range": "bytes=20-30"}); response.Code != 416 {
		t.Fatal("invalid range accepted")
	}
	for _, route := range []string{"/assets/missing.js", "/assets/missing", "/canvas/models/missing.tflite", "/canvas/models", "/missing.js", "/.env", "/canvas/../index.html", "/canvas/%2e%2e/index.html"} {
		response := get(router, "GET", route, nil)
		if response.Code != 404 || response.Body.Len() != 0 || response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %s", route, response.Code, response.Body.String())
		}
	}
}

func TestOldAssetsSurviveUpgradeAndRollback(t *testing.T) {
	directory := t.TempDir()
	start := time.Now().Add(-30 * 24 * time.Hour)
	routerFor(t, directory, "A", start)
	routerFor(t, directory, "B", start.Add(time.Hour))
	router := routerFor(t, directory, "C", time.Now())
	if response := get(router, "GET", "/assets/app-A.js", nil); response.Code != 404 {
		t.Fatal("expired third version was retained")
	}
	if response := get(router, "GET", "/assets/app-B.js", nil); response.Code != 200 || response.Body.String() != "version=B" {
		t.Fatal("previous version asset lost")
	}
	if response := get(router, "GET", "/", nil); response.Body.String() != "<!doctype html>C" {
		t.Fatal("stale entry served")
	}
	router = routerFor(t, directory, "B", time.Now())
	if response := get(router, "GET", "/", nil); response.Body.String() != "<!doctype html>B" {
		t.Fatal("rollback entry not selected")
	}
	if response := get(router, "GET", "/assets/app-C.js", nil); response.Code != 200 {
		t.Fatal("rollback discarded former current assets")
	}
	if err := os.WriteFile(filepath.Join(directory, "keep.txt"), []byte("business data"), 0o600); err != nil {
		t.Fatal(err)
	}
	routerFor(t, directory, "D", time.Now().Add(8*24*time.Hour))
	if _, err := os.Stat(filepath.Join(directory, "keep.txt")); err != nil {
		t.Fatal("asset pruning touched business data")
	}
}

func TestFrontendPublicationRejectsIncompleteBuild(t *testing.T) {
	files := frontend("A")
	delete(files, "index.html")
	if _, err := newHandler(files, t.TempDir(), "A", nil, time.Now()); err == nil {
		t.Fatal("missing entry accepted")
	}
	files = frontend("A")
	if _, err := newHandler(files, t.TempDir(), "../escape", nil, time.Now()); err == nil {
		t.Fatal("invalid cache identity accepted")
	}
}
