package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSPAHandlerServesAssetsAndRouteFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("SPA index"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.dart.js"), []byte("compiled app"), 0o600); err != nil {
		t.Fatal(err)
	}

	spa := newSPAHandler(root)
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		if !spa.serveRoot(c) {
			c.Status(http.StatusNotFound)
		}
	})
	router.NoRoute(func(c *gin.Context) {
		if !spa.serveNoRoute(c) {
			c.Status(http.StatusNotFound)
		}
	})

	for _, test := range []struct {
		name, path, wantBody string
		wantStatus           int
	}{
		{name: "root", path: "/", wantBody: "SPA index", wantStatus: http.StatusOK},
		{name: "asset", path: "/main.dart.js", wantBody: "compiled app", wantStatus: http.StatusOK},
		{name: "wallet deep link", path: "/wallet/transfer/internal", wantBody: "SPA index", wantStatus: http.StatusOK},
		{name: "api remains unmatched", path: "/api/v1/missing", wantBody: "404 page not found", wantStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if body := response.Body.String(); body != test.wantBody {
				t.Fatalf("body = %q, want %q", body, test.wantBody)
			}
		})
	}
}
