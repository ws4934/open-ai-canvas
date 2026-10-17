package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBusinessCORSRoutesAndPreflight(t *testing.T) {
	t.Setenv("CANVAS_CORS_ORIGINS", "https://allowed.example")
	middleware, err := cors()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	business := router.Group("", middleware)
	protected := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	business.GET("/api/resources/:id/file", protected)
	business.GET("/api/private.js", protected)
	business.GET("/oauth/linuxdo/callback", protected)
	router.NoRoute(corsFallback(middleware, func(c *gin.Context) { c.Status(http.StatusNotFound) }))
	// These routes model the public route chain; webui separately exercises the
	// actual embedded asset and SPA handlers with unauthorized Origins.
	router.GET("/assets/app.js", func(c *gin.Context) { c.Data(200, "text/javascript", []byte("public")) })
	router.GET("/canvas/project", func(c *gin.Context) { c.Data(200, "text/html", []byte("public")) })
	for _, test := range []struct {
		method, path, origin string
		want                 int
	}{
		{"GET", "/assets/app.js", "https://untrusted.example", 200},
		{"GET", "/canvas/project", "https://untrusted.example", 200},
		{"GET", "/api/resources/asset/file", "https://untrusted.example", 403},
		{"GET", "/api/private.js", "https://untrusted.example", 403},
		{"GET", "/oauth/linuxdo/callback", "https://untrusted.example", 403},
		{"GET", "/api", "https://untrusted.example", 403},
		{"GET", "/oauth", "https://untrusted.example", 403},
		{"GET", "/api/channel/chat/completions", "https://untrusted.example", 403},
		{"GET", "/api/unknown", "https://untrusted.example", 403},
		{"GET", "/oauth/unknown", "https://untrusted.example", 403},
		{"OPTIONS", "/api/resources/asset/file", "https://untrusted.example", 403},
		{"OPTIONS", "/api/channel/chat/completions", "https://untrusted.example", 403},
		{"OPTIONS", "/oauth/linuxdo/callback", "https://untrusted.example", 403},
		{"OPTIONS", "/api/resources/asset/file", "https://allowed.example", 204},
		{"OPTIONS", "/api/channel/chat/completions", "https://allowed.example", 204},
		{"OPTIONS", "/oauth/linuxdo/callback", "https://allowed.example", 204},
		{"GET", "/api/resources/asset/file", "https://allowed.example", 401},
		{"GET", "/oauth/linuxdo/callback", "https://allowed.example", 401},
		{"GET", "/api/unknown", "https://allowed.example", 404},
	} {
		t.Run(test.method+" "+test.path+" "+test.origin, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", "POST")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.want, response.Body.String())
			}
			if test.want == 200 && response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("public frontend response passed through business CORS")
			}
			if test.origin == "https://allowed.example" && response.Header().Get("Access-Control-Allow-Origin") != test.origin {
				t.Fatal("allowed business Origin response header was lost")
			}
		})
	}
}
