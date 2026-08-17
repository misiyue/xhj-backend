package router

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler serves a compiled Flutter Web application without interfering
// with API, websocket, health, or metrics endpoints owned by this server.
type spaHandler struct {
	root      string
	indexFile string
	enabled   bool
}

func newSPAHandler(root string) spaHandler {
	if root == "" {
		return spaHandler{}
	}

	indexFile := filepath.Join(root, "index.html")
	info, err := os.Stat(indexFile)
	if err != nil || info.IsDir() {
		return spaHandler{}
	}

	return spaHandler{root: root, indexFile: indexFile, enabled: true}
}

func (s spaHandler) serveRoot(c *gin.Context) bool {
	if !s.enabled {
		return false
	}
	c.File(s.indexFile)
	return true
}

func (s spaHandler) serveNoRoute(c *gin.Context) bool {
	if !s.enabled || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
		return false
	}

	requestPath := path.Clean("/" + c.Request.URL.Path)
	if isReservedServerPath(requestPath) {
		return false
	}

	if file := s.staticFile(requestPath); file != "" {
		c.File(file)
		return true
	}

	if shouldServeSPAIndex(c.Request, requestPath) {
		c.File(s.indexFile)
		return true
	}
	return false
}

func (s spaHandler) staticFile(requestPath string) string {
	relative := strings.TrimPrefix(requestPath, "/")
	if relative == "" {
		return ""
	}

	file := filepath.Join(s.root, filepath.FromSlash(relative))
	resolvedRelative, err := filepath.Rel(s.root, file)
	if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
		return ""
	}

	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		return ""
	}
	return file
}

func isReservedServerPath(requestPath string) bool {
	for _, prefix := range []string{"/api", "/swagger", "/health", "/metrics", "/wss", "/admin", "/open"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}

func shouldServeSPAIndex(request *http.Request, requestPath string) bool {
	if path.Ext(requestPath) == "" {
		return true
	}
	return strings.Contains(request.Header.Get("Accept"), "text/html")
}
