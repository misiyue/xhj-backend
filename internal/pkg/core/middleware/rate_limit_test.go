package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		config         *RateLimitConfig
		requests       int
		expectedPassed int
		expectedFailed int
	}{
		{
			name: "Basic rate limiting",
			config: &RateLimitConfig{
				Enabled:  true,
				Capacity: 5,
				Rate:     2,
			},
			requests:       10,
			expectedPassed: 5,
			expectedFailed: 5,
		},
		{
			name: "Disabled rate limiting",
			config: &RateLimitConfig{
				Enabled:  false,
				Capacity: 1,
				Rate:     1,
			},
			requests:       10,
			expectedPassed: 10,
			expectedFailed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RateLimitMiddleware(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "ok"})
			})

			passed := 0
			failed := 0

			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "127.0.0.1:1234"
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code == 200 {
					passed++
				} else if w.Code == 429 {
					failed++
				}
			}

			assert.Equal(t, tt.expectedPassed, passed, "Expected passed requests")
			assert.Equal(t, tt.expectedFailed, failed, "Expected failed requests")
		})
	}
}

func TestGlobalRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &RateLimitConfig{
		Enabled:  true,
		Capacity: 3,
		Rate:     1,
	}

	router := gin.New()
	router.Use(GlobalRateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// First 3 requests should pass (using capacity)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Request %d should pass", i+1)
	}

	// 4th request should fail
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 503, w.Code, "Request should be rate limited")
}

func TestTokenBucket(t *testing.T) {
	bucket := NewTokenBucket(5, 2)

	// Should allow 5 requests initially (bucket capacity)
	for i := 0; i < 5; i++ {
		assert.True(t, bucket.Allow(), "Request %d should be allowed", i+1)
	}

	// 6th request should be denied
	assert.False(t, bucket.Allow(), "Request should be denied")

	// Wait for token refill (2 tokens per second)
	time.Sleep(1100 * time.Millisecond)

	// Should allow 2 more requests after refill
	assert.True(t, bucket.Allow(), "Request should be allowed after refill")
	assert.True(t, bucket.Allow(), "Request should be allowed after refill")
	assert.False(t, bucket.Allow(), "Request should be denied")
}

func TestRateLimiterCleanup(t *testing.T) {
	limiter := NewRateLimiter(5, 2)

	// Add some entries
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")
	limiter.Allow("192.168.1.3")

	// Check that entries exist
	count := 0
	limiter.buckets.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 3, count, "Should have 3 entries")
}

func TestRateLimitMiddlewareWithDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &RateLimitConfig{
		Enabled:  true,
		Capacity: 2,
		Rate:     1,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// Different IPs should have separate rate limits
	ips := []string{"192.168.1.1:1234", "192.168.1.2:5678", "192.168.1.3:9012"}

	for _, ip := range ips {
		// Each IP should be able to make 2 requests
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = ip
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code, "Request from %s should pass", ip)
		}

		// 3rd request from same IP should fail
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 429, w.Code, "Request from %s should be rate limited", ip)
	}
}
