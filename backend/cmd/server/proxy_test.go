package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrustedProxyHeaders(t *testing.T) {
	for _, test := range []struct {
		name, proxies, remote, wantHost, wantProto, wantIP string
	}{
		{"direct", "", "203.0.113.1:1234", "", "", "203.0.113.1"},
		{"untrusted", "127.0.0.1", "203.0.113.1:1234", "", "", "203.0.113.1"},
		{"trusted", "127.0.0.1", "127.0.0.1:1234", "canvas.example.com", "https", "198.51.100.7"},
		{"ipv6", "::1/128", "[::1]:1234", "canvas.example.com", "https", "198.51.100.7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			middleware, err := configureTrustedProxies(router, test.proxies)
			if err != nil {
				t.Fatal(err)
			}
			router.Use(middleware)
			router.GET("/", func(c *gin.Context) {
				if got := c.GetHeader("X-Forwarded-Host"); got != test.wantHost {
					t.Errorf("forwarded host = %q", got)
				}
				if got := c.GetHeader("X-Forwarded-Proto"); got != test.wantProto {
					t.Errorf("forwarded proto = %q", got)
				}
				if got := c.ClientIP(); got != test.wantIP {
					t.Errorf("client IP = %q", got)
				}
				c.Status(http.StatusOK)
			})
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = test.remote
			request.Header.Set("X-Forwarded-Host", "canvas.example.com, internal")
			request.Header.Set("X-Forwarded-Proto", "https, http")
			request.Header.Set("X-Forwarded-For", "198.51.100.7")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("missing referrer protection")
			}
		})
	}
}

func TestTrustedProxyRejectsInvalidConfiguration(t *testing.T) {
	if _, err := configureTrustedProxies(gin.New(), "proxy.example.com"); err == nil {
		t.Fatal("proxy configuration must contain IPs or CIDRs")
	}
}
