package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"yingce/backend/internal/service"
)

// ProgramUpdateIntakeMiddleware fences new external runs, not continuations of
// existing ones. Mount on the root router so the short model-proxy NoRoute form
// is covered as well as registered /api routes.
func ProgramUpdateIntakeMiddleware(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		startsWork := programUpdateStartsWork(c.Request.Method, c.Request.URL.Path)
		if !startsWork && c.FullPath() == "" && programUpdateWriteMethod(c.Request.Method) {
			_, _, startsWork = shortSystemProxyPath(c.Request.URL.Path)
		}
		if !startsWork {
			c.Next()
			return
		}
		release, err := svc.AcquireProgramUpdateIntake()
		if err != nil {
			c.Header("Retry-After", "5")
			failService(c, err)
			c.Abort()
			return
		}
		defer release()
		c.Next()
	}
}

func programUpdateStartsWork(method, path string) bool {
	if !programUpdateWriteMethod(method) {
		return false
	}
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	path = strings.TrimSuffix(path, "/")
	switch path {
	case "/api/tasks", "/api/workbench/tasks", "/api/timeline/transcriptions", "/api/timeline/renders", "/api/agent/runs", "/api/agent/memories/compact", "/api/creation-runs", "/api/ai/custom":
		return true
	}
	if strings.HasPrefix(path, "/api/tasks/") && (strings.HasSuffix(path, "/retry") || strings.HasSuffix(path, "/recover-media")) {
		return true
	}
	if strings.HasPrefix(path, "/api/agent/runs/") && (strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/interjections")) {
		return true
	}
	if strings.HasPrefix(path, "/api/creation-runs/") && strings.HasSuffix(path, "/execute") {
		return true
	}
	if strings.HasPrefix(path, "/api/ai/system/") {
		return true
	}
	return false
}

func programUpdateWriteMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions && method != http.MethodDelete
}
