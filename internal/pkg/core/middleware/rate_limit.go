package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenBucket 令牌桶结构
type TokenBucket struct {
	capacity   int64      // 桶的容量
	tokens     int64      // 当前令牌数
	rate       int64      // 每秒生成的令牌数
	lastRefill time.Time  // 上次填充时间
	mu         sync.Mutex // 互斥锁
}

// NewTokenBucket 创建新的令牌桶
func NewTokenBucket(capacity, rate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// 计算应该添加的令牌数
	tokensToAdd := int64(elapsed * float64(tb.rate))
	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
	}

	// 检查是否有可用令牌
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// RateLimiter 限流器结构
type RateLimiter struct {
	buckets  sync.Map // IP地址 -> TokenBucket
	capacity int64    // 桶容量
	rate     int64    // 令牌生成速率
	mu       sync.Mutex
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(capacity, rate int64) *RateLimiter {
	limiter := &RateLimiter{
		capacity: capacity,
		rate:     rate,
	}

	// 启动清理协程，定期清理过期的bucket
	go limiter.cleanup()

	return limiter
}

// cleanup 定期清理不活跃的令牌桶
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.buckets.Range(func(key, value interface{}) bool {
			bucket := value.(*TokenBucket)
			bucket.mu.Lock()
			// 如果令牌桶超过10分钟未使用，则删除
			if time.Since(bucket.lastRefill) > 10*time.Minute && bucket.tokens == bucket.capacity {
				rl.buckets.Delete(key)
			}
			bucket.mu.Unlock()
			return true
		})
	}
}

// Allow 检查特定IP是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	val, _ := rl.buckets.LoadOrStore(key, NewTokenBucket(rl.capacity, rl.rate))
	bucket := val.(*TokenBucket)
	return bucket.Allow()
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled  bool  `yaml:"enabled"`  // 是否启用限流
	Capacity int64 `yaml:"capacity"` // 桶容量（最大突发请求数）
	Rate     int64 `yaml:"rate"`     // 每秒生成的令牌数（QPS）
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:  true,
		Capacity: 100, // 允许100个突发请求
		Rate:     50,  // 每秒50个请求
	}
}

// RateLimitMiddleware 创建限流中间件
func RateLimitMiddleware(config *RateLimitConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	// 如果未启用，直接返回空中间件
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	limiter := NewRateLimiter(config.Capacity, config.Rate)

	return func(c *gin.Context) {
		// 使用客户端IP作为限流key
		clientIP := c.ClientIP()

		if !limiter.Allow(clientIP) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

// GlobalRateLimiter 全局限流器（用于整体QPS控制）
type GlobalRateLimiter struct {
	bucket *TokenBucket
}

// NewGlobalRateLimiter 创建全局限流器
func NewGlobalRateLimiter(capacity, rate int64) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		bucket: NewTokenBucket(capacity, rate),
	}
}

// GlobalRateLimitMiddleware 创建全局限流中间件
func GlobalRateLimitMiddleware(config *RateLimitConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	// 如果未启用，直接返回空中间件
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	limiter := NewGlobalRateLimiter(config.Capacity, config.Rate)

	return func(c *gin.Context) {
		if !limiter.bucket.Allow() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "系统繁忙，请稍后再试",
			})
			return
		}

		c.Next()
	}
}
