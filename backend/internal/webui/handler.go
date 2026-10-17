package webui

import (
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"yingce/backend/internal/buildinfo"

	"github.com/gin-gonic/gin"
)

// NewHandler composes the embedded SPA with the existing API NoRoute handler.
// Development builds remain API-only and do not require a frontend build.
func NewHandler(dataDir string, fallback gin.HandlerFunc) (gin.HandlerFunc, error) {
	files, err := releaseFiles()
	if err != nil {
		return nil, err
	}
	if files == nil {
		return fallback, nil
	}
	info := buildinfo.Current()
	identity := base64.RawURLEncoding.EncodeToString([]byte(info.Version + "\n" + info.Commit + "\n" + info.BuildTime))
	return newHandler(files, dataDir, identity, fallback, time.Now())
}

func newHandler(files fs.FS, dataDir, identity string, fallback gin.HandlerFunc, now time.Time) (gin.HandlerFunc, error) {
	index, err := fs.Stat(files, "index.html")
	if err != nil || index.IsDir() || index.Size() == 0 {
		return nil, errors.New("embedded frontend index.html is missing or empty")
	}
	cache, err := publishAssets(files, dataDir, identity, now)
	if err != nil {
		return nil, err
	}
	staticDirectories := map[string]bool{"/assets/": true}
	if err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && name != "." && !pageAlias("/"+name) {
			staticDirectories["/"+name+"/"] = true
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") || requestPath == "/oauth" || strings.HasPrefix(requestPath, "/oauth/") {
			if fallback != nil {
				fallback(c)
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}
		c.Header("Referrer-Policy", "no-referrer")
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.Contains(requestPath, "\\") || hasPrivateSegment(requestPath) || path.Clean(requestPath) != strings.TrimSuffix(requestPath, "/") && requestPath != "/" {
			missing(c)
			return
		}
		name := strings.TrimPrefix(requestPath, "/")
		if requestPath == "/" || pageAlias(requestPath) || name == "index.html" {
			serveFile(c, files, "index.html", identity, false)
			return
		}
		if serveFile(c, files, name, identity, strings.HasPrefix(name, "assets/")) {
			return
		}
		if strings.HasPrefix(name, "assets/") {
			for _, version := range cache.versions {
				file, err := os.OpenInRoot(cache.directory, filepath.Join(version, filepath.FromSlash(name)))
				if err != nil {
					continue
				}
				stat, err := file.Stat()
				if err == nil && stat.Mode().IsRegular() {
					serveContent(c, name, version, true, stat.ModTime(), file)
					_ = file.Close()
					return
				}
				_ = file.Close()
			}
			missing(c)
			return
		}
		for directory := range staticDirectories {
			if requestPath == strings.TrimSuffix(directory, "/") || strings.HasPrefix(requestPath, directory) {
				missing(c)
				return
			}
		}
		if path.Ext(requestPath) != "" {
			missing(c)
			return
		}
		serveFile(c, files, "index.html", identity, false)
	}, nil
}

func pageAlias(name string) bool {
	switch strings.TrimSuffix(name, "/") {
	case "/assets", "/canvas", "/welcome", "/inspirations":
		return true
	default:
		return false
	}
}

func hasPrivateSegment(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func missing(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatus(http.StatusNotFound)
}

func serveFile(c *gin.Context, files fs.FS, name, identity string, immutable bool) bool {
	file, err := files.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		return false
	}
	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return true
	}
	serveContent(c, name, identity, immutable, stat.ModTime(), seeker)
	return true
}

func serveContent(c *gin.Context, name, identity string, immutable bool, modified time.Time, content io.ReadSeeker) {
	c.Status(http.StatusOK)
	cacheControl := "no-cache, must-revalidate"
	if immutable {
		cacheControl = "public, max-age=31536000, immutable"
	}
	c.Header("Cache-Control", cacheControl)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("ETag", `"`+base64.RawURLEncoding.EncodeToString([]byte(identity+":"+name))+`"`)
	contentType := mime.TypeByExtension(path.Ext(name))
	if path.Ext(name) == ".wasm" {
		contentType = "application/wasm"
	}
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	http.ServeContent(c.Writer, c.Request, path.Base(name), modified, content)
}
