package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"yingce/backend/internal/auth"
	"yingce/backend/internal/database"
	"yingce/backend/internal/handler"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
	"yingce/backend/internal/service"
)

func TestCanvasRealtimeThroughBusinessCORSAndTrustedProxy(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("CANVAS_CORS_ORIGINS", "https://allowed.example")
	db, err := database.Open(database.Config{Driver: "sqlite", DSN: "file:realtime-proxy?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&model.User{ID: "user", Username: "proxy-user", Role: model.UserRoleUser, Status: model.UserStatusActive},
		&model.AuthSession{ID: "session", UserID: "user", TokenHash: auth.HashToken("proxy-token"), ExpiresAt: time.Now().Add(time.Hour)},
		&model.CanvasProject{ID: "canvas", UserID: "user", Revision: 1, Title: "Canvas", PayloadJSON: `{"id":"canvas","nodes":[],"connections":[]}`},
		&model.CanvasProject{ID: "foreign", UserID: "other", Revision: 1, PayloadJSON: `{}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := service.New(repository.New(db), t.TempDir())
	defer svc.Close()
	for _, test := range []struct {
		name, proxies, origin, canvas string
		forwarded, authenticated      bool
		want                          int
	}{
		{"direct same-origin", "", "", "canvas", false, true, 101},
		{"trusted HTTPS proxy", "127.0.0.1/32", "https://canvas.example", "canvas", true, true, 101},
		{"explicit allowed Origin", "", "https://allowed.example", "canvas", false, true, 101},
		{"untrusted forwarded headers", "127.0.0.2/32", "https://canvas.example", "canvas", true, true, 403},
		{"foreign Origin", "127.0.0.1/32", "https://foreign.example", "canvas", true, true, 403},
		{"missing login", "127.0.0.1/32", "https://canvas.example", "canvas", true, false, 401},
		{"foreign canvas", "127.0.0.1/32", "https://canvas.example", "foreign", true, true, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			proxy, err := configureTrustedProxies(router, test.proxies)
			if err != nil {
				t.Fatal(err)
			}
			router.Use(proxy)
			originPolicy, err := cors()
			if err != nil {
				t.Fatal(err)
			}
			business := router.Group("", originPolicy)
			handler.RegisterCanvasRealtimeRoutes(business.Group("/api"), svc)
			router.NoRoute(corsFallback(originPolicy, handler.SystemProxyNoRouteHandler(svc)))
			server := httptest.NewServer(router)
			defer server.Close()
			origin := test.origin
			if origin == "" {
				origin = server.URL
			}
			headers := http.Header{"Origin": {origin}}
			if test.authenticated {
				headers.Set("Cookie", service.SessionCookieName+"=session.proxy-token")
			}
			if test.forwarded {
				headers.Set("X-Forwarded-Host", "canvas.example")
				headers.Set("X-Forwarded-Proto", "https")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/canvas-projects/" + test.canvas + "/realtime"
			conn, response, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: headers})
			if conn != nil {
				defer conn.CloseNow()
			}
			if response == nil || response.StatusCode != test.want {
				t.Fatalf("handshake response = %v, error = %v; want %d", response, err, test.want)
			}
			if test.want != http.StatusSwitchingProtocols {
				if err == nil {
					t.Fatal("unauthorized WebSocket handshake succeeded")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var welcome struct {
				Type   string `json:"type"`
				SelfID string `json:"selfId"`
			}
			if err := wsjson.Read(ctx, conn, &welcome); err != nil || welcome.Type != "welcome" || welcome.SelfID == "" {
				t.Fatalf("missing realtime welcome: %+v, %v", welcome, err)
			}
		})
	}
}
