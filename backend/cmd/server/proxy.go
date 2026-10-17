package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Forwarded headers affect cookies and origin checks, not just access logs.
// Sanitize them before any middleware or handler consumes them.
func configureTrustedProxies(router *gin.Engine, raw string) (gin.HandlerFunc, error) {
	var trusted []netip.Prefix
	var entries []string
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			address, addressErr := netip.ParseAddr(value)
			if addressErr != nil {
				return nil, fmt.Errorf("CANVAS_TRUSTED_PROXIES 包含无效 IP/CIDR：%q", value)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		trusted = append(trusted, prefix.Masked())
		entries = append(entries, value)
	}
	if err := router.SetTrustedProxies(entries); err != nil {
		return nil, err
	}
	return func(c *gin.Context) {
		c.Header("Referrer-Policy", "no-referrer")
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		peer, err := netip.ParseAddr(host)
		isTrusted := false
		if err == nil {
			for _, prefix := range trusted {
				if prefix.Contains(peer) || prefix.Contains(peer.Unmap()) {
					isTrusted = true
					break
				}
			}
		}
		c.Request.Header.Del("Forwarded")
		if !isTrusted {
			for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
				c.Request.Header.Del(name)
			}
		} else {
			proto := strings.ToLower(firstForwardedValue(c.GetHeader("X-Forwarded-Proto")))
			if proto == "https" || proto == "http" {
				c.Request.Header.Set("X-Forwarded-Proto", proto)
			} else {
				c.Request.Header.Del("X-Forwarded-Proto")
			}
			forwardedHost := firstForwardedValue(c.GetHeader("X-Forwarded-Host"))
			parsed, parseErr := url.Parse("//" + forwardedHost)
			if parseErr == nil && parsed.Host != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" {
				c.Request.Header.Set("X-Forwarded-Host", parsed.Host)
			} else {
				c.Request.Header.Del("X-Forwarded-Host")
			}
		}
		c.Next()
	}, nil
}

func firstForwardedValue(value string) string {
	return strings.TrimSpace(strings.SplitN(value, ",", 2)[0])
}

func requestScheme(request *http.Request) string {
	if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
