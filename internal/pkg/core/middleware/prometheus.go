package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP请求总数
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTP请求持续时间
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// 当前正在处理的请求数
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	// 请求大小
	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)

	// 响应大小
	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "path"},
	)
)

// PrometheusMiddleware Prometheus监控中间件
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过metrics端点本身，避免循环统计
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// 记录开始时间
		start := time.Now()

		// 增加正在处理的请求计数
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		// 记录请求大小
		if c.Request.ContentLength > 0 {
			httpRequestSize.WithLabelValues(
				c.Request.Method,
				c.FullPath(),
			).Observe(float64(c.Request.ContentLength))
		}

		// 处理请求
		c.Next()

		// 计算请求持续时间
		duration := time.Since(start).Seconds()

		// 获取路径模板（如果有）
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 记录状态码
		status := strconv.Itoa(c.Writer.Status())

		// 更新指标
		httpRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Inc()

		httpRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Observe(duration)

		// 记录响应大小
		httpResponseSize.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(float64(c.Writer.Size()))
	}
}
