package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yingce/backend/internal/app"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/database"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

func TestStorageReadOnlyGuardBlocksWritesButPreservesRecoveryRoutes(t *testing.T) {
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "guard.db")})
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	t.Cleanup(func() { _ = connection.Close() })
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	svc := app.New(repository.New(db), t.TempDir())
	t.Cleanup(func() { _ = svc.Close() })
	since := time.Now().Add(-8 * 24 * time.Hour)
	for _, row := range []any{
		&model.User{ID: "owner", Username: "owner", Role: model.UserRoleUser, Status: model.UserStatusActive},
		&model.AuthSession{ID: "session", UserID: "owner", TokenHash: auth.HashToken("test-token"), ExpiresAt: time.Now().Add(time.Hour)},
		&model.Resource{ID: "large", UserID: "owner", Status: model.ResourceStatusReady, Provider: "local", ObjectKey: "large", Size: 1000 << 30},
		&model.UserStorageState{UserID: "owner", OverLimitSince: &since},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCanvasAPI(router.Group("/api"), svc)
	for _, tc := range []struct {
		method, path, body string
		blocked            bool
	}{
		{"PATCH", "/api/canvas-projects/new/operations", `{"operations":[]}`, true},
		{"POST", "/api/tasks", `{}`, true},
		{"POST", "/api/assets/batch", `{"ids":[]}`, false},
		{"GET", "/api/resources/storage-grants", "", false},
		{"POST", "/api/feedbacks", `{"title":"issue","content":"help"}`, false},
		{"POST", "/api/payments/orders", `{}`, false},
		{"DELETE", "/api/assets/not-found", "", false},
	} {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "open_ai_canvas_session", Value: "session.test-token"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		blocked := w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "storage_read_only")
		if blocked != tc.blocked {
			t.Fatalf("%s %s: code=%d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
