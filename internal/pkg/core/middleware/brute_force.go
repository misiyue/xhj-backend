package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	Count        int       // 失败次数
	FirstAttempt time.Time // 第一次尝试时间
	LastAttempt  time.Time // 最后一次尝试时间
	BlockedUntil time.Time // 封禁到期时间
}

// BruteForceProtector 暴力破解保护器
type BruteForceProtector struct {
	attempts      sync.Map      // IP或用户 -> LoginAttempt
	maxAttempts   int           // 最大尝试次数
	blockDuration time.Duration // 封禁时长
	resetDuration time.Duration // 重置失败计数的时间
	mu            sync.Mutex
}

// NewBruteForceProtector 创建暴力破解保护器
func NewBruteForceProtector(maxAttempts int, blockDuration, resetDuration time.Duration) *BruteForceProtector {
	protector := &BruteForceProtector{
		maxAttempts:   maxAttempts,
		blockDuration: blockDuration,
		resetDuration: resetDuration,
	}

	// 启动清理协程
	go protector.cleanup()

	return protector
}

// cleanup 定期清理过期记录
func (bfp *BruteForceProtector) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		bfp.attempts.Range(func(key, value interface{}) bool {
			attempt := value.(*LoginAttempt)
			// 如果封禁已过期且超过重置时间，删除记录
			if time.Now().After(attempt.BlockedUntil) &&
				time.Since(attempt.LastAttempt) > bfp.resetDuration {
				bfp.attempts.Delete(key)
			}
			return true
		})
	}
}

// IsBlocked 检查是否被封禁
func (bfp *BruteForceProtector) IsBlocked(key string) (bool, time.Time) {
	val, exists := bfp.attempts.Load(key)
	if !exists {
		return false, time.Time{}
	}

	attempt := val.(*LoginAttempt)
	if time.Now().Before(attempt.BlockedUntil) {
		return true, attempt.BlockedUntil
	}

	return false, time.Time{}
}

// RecordFailure 记录失败尝试
func (bfp *BruteForceProtector) RecordFailure(key string) {
	now := time.Now()

	val, exists := bfp.attempts.Load(key)
	if !exists {
		// 第一次失败
		bfp.attempts.Store(key, &LoginAttempt{
			Count:        1,
			FirstAttempt: now,
			LastAttempt:  now,
		})
		return
	}

	attempt := val.(*LoginAttempt)

	// 如果距离上次失败超过重置时间，重置计数
	if now.Sub(attempt.LastAttempt) > bfp.resetDuration {
		bfp.attempts.Store(key, &LoginAttempt{
			Count:        1,
			FirstAttempt: now,
			LastAttempt:  now,
		})
		return
	}

	// 增加失败次数
	attempt.Count++
	attempt.LastAttempt = now

	// 如果超过最大尝试次数，封禁
	if attempt.Count >= bfp.maxAttempts {
		// 使用指数退避策略
		attemptsOverThreshold := (attempt.Count - bfp.maxAttempts + 1)
		blockDuration := bfp.blockDuration * time.Duration(attemptsOverThreshold)
		// 最大封禁1小时
		if blockDuration > time.Hour {
			blockDuration = time.Hour
		}
		attempt.BlockedUntil = now.Add(blockDuration)
	}

	bfp.attempts.Store(key, attempt)
}

// RecordSuccess 记录成功登录，重置计数
func (bfp *BruteForceProtector) RecordSuccess(key string) {
	bfp.attempts.Delete(key)
}

// GetRemainingAttempts 获取剩余尝试次数
func (bfp *BruteForceProtector) GetRemainingAttempts(key string) int {
	val, exists := bfp.attempts.Load(key)
	if !exists {
		return bfp.maxAttempts
	}

	attempt := val.(*LoginAttempt)
	// 如果超过重置时间，返回最大次数
	if time.Since(attempt.LastAttempt) > bfp.resetDuration {
		return bfp.maxAttempts
	}

	remaining := bfp.maxAttempts - attempt.Count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BruteForceConfig 暴力破解保护配置
type BruteForceConfig struct {
	Enabled       bool `yaml:"enabled"`        // 是否启用
	MaxAttempts   int  `yaml:"max_attempts"`   // 最大尝试次数
	BlockDuration int  `yaml:"block_duration"` // 封禁时长（分钟）
	ResetDuration int  `yaml:"reset_duration"` // 重置时长（分钟）
}

// DefaultBruteForceConfig 默认暴力破解保护配置
func DefaultBruteForceConfig() *BruteForceConfig {
	return &BruteForceConfig{
		Enabled:       true,
		MaxAttempts:   5,  // 5次失败后封禁
		BlockDuration: 15, // 封禁15分钟
		ResetDuration: 30, // 30分钟后重置计数
	}
}

// BruteForceMiddleware 创建暴力破解保护中间件
func BruteForceMiddleware(config *BruteForceConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultBruteForceConfig()
	}

	// 如果未启用，直接返回空中间件
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	protector := NewBruteForceProtector(
		config.MaxAttempts,
		time.Duration(config.BlockDuration)*time.Minute,
		time.Duration(config.ResetDuration)*time.Minute,
	)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 检查IP是否被封禁
		if blocked, blockedUntil := protector.IsBlocked(clientIP); blocked {
			remaining := int(time.Until(blockedUntil).Seconds())
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":            429,
				"message":         "登录尝试次数过多，请稍后再试",
				"blocked_seconds": remaining,
			})
			return
		}

		// 将protector放入context，供后续处理使用
		c.Set("brute_force_protector", protector)
		c.Set("brute_force_key", clientIP)

		c.Next()
	}
}

// RecordLoginAttempt 记录登录尝试结果（在登录处理函数中调用）
func RecordLoginAttempt(c *gin.Context, success bool) {
	protector, exists := c.Get("brute_force_protector")
	if !exists {
		return
	}

	key, exists := c.Get("brute_force_key")
	if !exists {
		return
	}

	bfp := protector.(*BruteForceProtector)
	keyStr := key.(string)

	if success {
		bfp.RecordSuccess(keyStr)
	} else {
		bfp.RecordFailure(keyStr)
	}
}
