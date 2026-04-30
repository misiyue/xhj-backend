package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBruteForceProtector(t *testing.T) {
	protector := NewBruteForceProtector(3, 5*time.Minute, 10*time.Minute)

	// Should not be blocked initially
	blocked, _ := protector.IsBlocked("test-key")
	assert.False(t, blocked, "Should not be blocked initially")

	// Record 2 failures
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")

	blocked, _ = protector.IsBlocked("test-key")
	assert.False(t, blocked, "Should not be blocked after 2 failures")

	// 3rd failure should trigger block
	protector.RecordFailure("test-key")
	blocked, blockedUntil := protector.IsBlocked("test-key")
	assert.True(t, blocked, "Should be blocked after 3 failures")
	assert.True(t, time.Until(blockedUntil) > 0, "Block should have future expiration")
}

func TestBruteForceProtectorSuccess(t *testing.T) {
	protector := NewBruteForceProtector(3, 5*time.Minute, 10*time.Minute)

	// Record some failures
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")

	// Record success should reset counter
	protector.RecordSuccess("test-key")

	// Should not be blocked
	blocked, _ := protector.IsBlocked("test-key")
	assert.False(t, blocked, "Should not be blocked after success")

	// Should be able to fail again from beginning
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")
	blocked, _ = protector.IsBlocked("test-key")
	assert.False(t, blocked, "Should not be blocked after 2 new failures")
}

func TestBruteForceProtectorReset(t *testing.T) {
	protector := NewBruteForceProtector(3, 5*time.Second, 1*time.Second)

	// Record failures
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")

	// Wait for reset duration
	time.Sleep(1100 * time.Millisecond)

	// Next failure should reset the counter
	protector.RecordFailure("test-key")

	remaining := protector.GetRemainingAttempts("test-key")
	assert.Equal(t, 2, remaining, "Should have 2 remaining attempts after reset")
}

func TestBruteForceProtectorExponentialBackoff(t *testing.T) {
	protector := NewBruteForceProtector(2, 1*time.Minute, 10*time.Minute)

	// First block (2 failures)
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")
	blocked, blockedUntil1 := protector.IsBlocked("test-key")
	assert.True(t, blocked, "Should be blocked after 2 failures")

	// Record more failures
	protector.RecordFailure("test-key")
	_, blockedUntil2 := protector.IsBlocked("test-key")

	// Second block should be longer
	assert.True(t, blockedUntil2.After(blockedUntil1), "Second block should be longer")
}

func TestBruteForceMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &BruteForceConfig{
		Enabled:       true,
		MaxAttempts:   3,
		BlockDuration: 1,
		ResetDuration: 5,
	}

	router := gin.New()
	router.Use(BruteForceMiddleware(config))
	router.GET("/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// First 3 requests should pass
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Request %d should pass", i+1)
	}
}

func TestBruteForceMiddlewareDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &BruteForceConfig{
		Enabled:       false,
		MaxAttempts:   1,
		BlockDuration: 1,
		ResetDuration: 1,
	}

	router := gin.New()
	router.Use(BruteForceMiddleware(config))
	router.GET("/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	// All requests should pass when disabled
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "All requests should pass when disabled")
	}
}

func TestRecordLoginAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &BruteForceConfig{
		Enabled:       true,
		MaxAttempts:   3,
		BlockDuration: 5,
		ResetDuration: 10,
	}

	router := gin.New()
	router.Use(BruteForceMiddleware(config))
	router.POST("/login", func(c *gin.Context) {
		// Simulate login failure
		RecordLoginAttempt(c, false)
		c.JSON(401, gin.H{"message": "login failed"})
	})

	// Make 3 failed login attempts
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// 4th attempt should be blocked by middleware
	req := httptest.NewRequest("POST", "/login", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code, "Should be blocked after 3 failures")
}

func TestGetRemainingAttempts(t *testing.T) {
	protector := NewBruteForceProtector(5, 5*time.Minute, 10*time.Minute)

	// Initially should have max attempts
	remaining := protector.GetRemainingAttempts("test-key")
	assert.Equal(t, 5, remaining)

	// After 2 failures, should have 3 remaining
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")
	remaining = protector.GetRemainingAttempts("test-key")
	assert.Equal(t, 3, remaining)

	// After all attempts used
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")
	protector.RecordFailure("test-key")
	remaining = protector.GetRemainingAttempts("test-key")
	assert.Equal(t, 0, remaining)
}
