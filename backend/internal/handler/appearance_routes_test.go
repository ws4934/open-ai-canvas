package handler

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yingce/backend/internal/app"
	"yingce/backend/internal/database"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestAppearanceRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAppearanceRoutes(router.Group("/api"), &service.Service{})
	wanted := map[string]bool{
		"GET /api/public/appearance/updates/:id":              false,
		"GET /api/admin/settings/appearance/updates/:id":      false,
		"POST /api/admin/settings/appearance/live2d":          false,
		"GET /api/admin/settings/appearance/live2d/:id/*file": false,
		"GET /api/public/appearance/live2d/:id/*file":         false,
		"POST /api/admin/settings/appearance/avatar-bundles":  false,
		"GET /api/admin/settings/appearance/avatar/:id/*file": false,
		"GET /api/public/appearance/avatar/:id/*file":         false,
		"GET /api/public/appearance":                          false,
		"GET /api/public/appearance/assets/:slot":             false,
		"GET /api/admin/settings/appearance":                  false,
		"PATCH /api/admin/settings/appearance":                false,
		"DELETE /api/admin/settings/appearance":               false,
		"POST /api/admin/settings/appearance/assets/:slot":    false,
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
}

func TestAppearanceUpdateAnnouncementMediaRangeAndDisabledAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:appearance-handler-range?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "resources"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "resources", "fixture.mp4"), []byte("video-data"), 0640); err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "fixture-video", UserID: "admin", Kind: "video", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "fixture.mp4", MimeType: "video/mp4", Size: 10}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), dir)
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	setting, err := svc.AdminAppearance(admin)
	if err != nil {
		t.Fatal(err)
	}
	setting.Updates = app.UpdateAnnouncement{Enabled: true, CurrentVersion: "v2", Releases: []app.UpdateRelease{{ID: "r1", Version: "v2", Blocks: []app.UpdateBlock{{ID: "b1", Type: "video", ResourceID: resource.ID, Layout: "full"}}}}}
	if _, err := svc.UpdateAppearance(admin, setting.AppearanceSetting); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterAppearanceRoutes(router.Group("/api"), svc)
	req := httptest.NewRequest(http.MethodGet, "/api/public/appearance/updates/fixture-video", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Body.String() != "vide" || rec.Header().Get("Content-Range") != "bytes 0-3/10" {
		t.Fatalf("Range HTTP response: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	preview := httptest.NewRecorder()
	router.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/api/admin/settings/appearance/updates/fixture-video", nil))
	if preview.Code != 401 {
		t.Fatalf("anonymous admin preview: %d", preview.Code)
	}
	setting.Updates.Enabled = false
	if _, err := svc.UpdateAppearance(admin, setting.AppearanceSetting); err != nil {
		t.Fatal(err)
	}
	disabled := httptest.NewRecorder()
	router.ServeHTTP(disabled, req)
	if disabled.Code != 404 {
		t.Fatalf("disabled public media: %d", disabled.Code)
	}
}

func TestAppearanceAvatarBundleRangeAndDisabledAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "avatar-handler.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "resources"), 0o750); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	mp4 := []byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}
	for name, content := range map[string][]byte{
		"avatar.json": []byte(`{"version":1,"width":480,"height":432,"clips":{"idle":"idle.mp4"}}`),
		"idle.mp4":    mp4,
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "resources", "avatar.zip"), data.Bytes(), 0o640); err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "fixture-avatar", UserID: "admin", Kind: "agent-avatar-bundle", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "avatar.zip", MimeType: "application/zip", Size: int64(data.Len())}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.New(db), dir)
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	setting, err := svc.AdminAppearance(admin)
	if err != nil {
		t.Fatal(err)
	}
	setting.Canvas.AvatarType = "bundle"
	setting.Canvas.AvatarBundleResourceID = resource.ID
	setting.Canvas.AvatarClipStates = map[string]string{"idle": "idle"}
	if _, err := svc.UpdateAppearance(admin, setting.AppearanceSetting); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterAppearanceRoutes(router.Group("/api"), svc)
	req := httptest.NewRequest(http.MethodGet, "/api/public/appearance/avatar/fixture-avatar/idle.mp4", nil)
	req.Header.Set("Range", "bytes=4-7")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "ftyp" || rec.Header().Get("Content-Range") != "bytes 4-7/16" || rec.Header().Get("Content-Type") != "video/mp4" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("avatar range response: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodPost, "/api/admin/settings/appearance/avatar-bundles"},
		{http.MethodGet, "/api/admin/settings/appearance/avatar/fixture-avatar/idle.mp4"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(endpoint.method, endpoint.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s returned %d", endpoint.method, endpoint.path, response.Code)
		}
	}
	setting.Canvas.AvatarType = "orb"
	if _, err := svc.UpdateAppearance(admin, setting.AppearanceSetting); err != nil {
		t.Fatal(err)
	}
	disabled := httptest.NewRecorder()
	router.ServeHTTP(disabled, req)
	if disabled.Code != http.StatusNotFound {
		t.Fatalf("disabled public avatar response: %d", disabled.Code)
	}
}
