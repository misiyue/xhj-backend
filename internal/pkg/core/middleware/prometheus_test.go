package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestPrometheusMiddleware(t *testing.T) {
	// Set a shorter collection interval for testing
	originalInterval := collectionInterval
	collectionInterval = 500 * time.Millisecond
	defer func() {
		collectionInterval = originalInterval
	}()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start system metrics collector
	StartSystemMetricsCollector(ctx)

	// Create a simple gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add Prometheus middleware
	router.Use(PrometheusMiddleware())

	// Add some test routes
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello world"})
	})

	router.GET("/test", func(c *gin.Context) {
		time.Sleep(50 * time.Millisecond) // Simulate some work
		c.JSON(200, gin.H{"message": "test endpoint"})
	})

	// Add metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Make some test requests
	resp, err := http.Get(ts.URL + "/")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/notfound")
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()

	// Wait for at least one system metrics collection cycle
	time.Sleep(1 * time.Second)

	// Check metrics endpoint
	resp, err = http.Get(ts.URL + "/metrics")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	metricsStr := string(body)

	// Check for expected metrics
	expectedMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_requests_in_flight",
		"system_cpu_usage_percent",
		"system_memory_usage_bytes",
		"system_goroutines_count",
		"system_go_heap_alloc_bytes",
	}

	for _, metric := range expectedMetrics {
		assert.True(t, strings.Contains(metricsStr, metric), "Expected metric %s not found", metric)
	}
}
