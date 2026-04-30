package middleware

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &SecurityHeadersConfig{
		Enabled:                 true,
		XFrameOptions:           "DENY",
		XContentTypeOptions:     "nosniff",
		XSSProtection:           "1; mode=block",
		StrictTransportSecurity: "max-age=31536000",
		ContentSecurityPolicy:   "default-src 'self'",
		ReferrerPolicy:          "no-referrer",
		PermissionsPolicy:       "geolocation=()",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "max-age=31536000", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "geolocation=()", w.Header().Get("Permissions-Policy"))
	assert.Equal(t, "", w.Header().Get("X-Powered-By"))
	assert.Equal(t, "", w.Header().Get("Server"))
}

func TestSecurityHeadersMiddlewareDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &SecurityHeadersConfig{
		Enabled:       false,
		XFrameOptions: "DENY",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "", w.Header().Get("X-Frame-Options"), "Headers should not be set when disabled")
}

func TestSecurityHeadersMiddlewareDefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(nil)) // Use default config
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))
}

func TestSecurityHeadersMiddlewarePartialConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &SecurityHeadersConfig{
		Enabled:       true,
		XFrameOptions: "SAMEORIGIN",
		// Other headers empty
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "", w.Header().Get("X-Content-Type-Options"))
}

func TestRequestSizeLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &RequestSizeConfig{
		Enabled:     true,
		MaxBodySize: 1024, // 1KB limit
	}

	router := gin.New()
	router.Use(RequestSizeLimitMiddleware(config))
	router.POST("/upload", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Small request should succeed
	smallBody := bytes.Repeat([]byte("a"), 512)
	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(smallBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code, "Small request should succeed")
}

func TestRequestSizeLimitMiddlewareDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &RequestSizeConfig{
		Enabled:     false,
		MaxBodySize: 10, // Very small limit
	}

	router := gin.New()
	router.Use(RequestSizeLimitMiddleware(config))
	router.POST("/upload", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Large request should succeed when disabled
	largeBody := bytes.Repeat([]byte("a"), 1024)
	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(largeBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code, "Large request should succeed when disabled")
}

func TestRequestSizeLimitMiddlewareDefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestSizeLimitMiddleware(nil)) // Use default config
	router.POST("/upload", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Default should allow reasonable sized requests
	body := bytes.Repeat([]byte("a"), 1024*1024) // 1MB
	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code, "1MB request should succeed with default config")
}

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	config := DefaultSecurityHeadersConfig()

	assert.True(t, config.Enabled)
	assert.NotEmpty(t, config.XFrameOptions)
	assert.NotEmpty(t, config.XContentTypeOptions)
	assert.NotEmpty(t, config.XSSProtection)
	assert.NotEmpty(t, config.StrictTransportSecurity)
	assert.NotEmpty(t, config.ContentSecurityPolicy)
	assert.NotEmpty(t, config.ReferrerPolicy)
	assert.NotEmpty(t, config.PermissionsPolicy)
}

func TestDefaultRequestSizeConfig(t *testing.T) {
	config := DefaultRequestSizeConfig()

	assert.True(t, config.Enabled)
	assert.Greater(t, config.MaxBodySize, int64(0))
	assert.Equal(t, int64(10*1024*1024), config.MaxBodySize) // 10MB default
}
