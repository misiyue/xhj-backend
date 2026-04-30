package config

import "github.com/gzydong/go-chat/internal/pkg/core/middleware"

// Security 安全配置
type Security struct {
	RateLimit       *middleware.RateLimitConfig       `json:"rate_limit" yaml:"rate_limit"`               // 限流配置
	GlobalRateLimit *middleware.RateLimitConfig       `json:"global_rate_limit" yaml:"global_rate_limit"` // 全局限流配置
	BruteForce      *middleware.BruteForceConfig      `json:"brute_force" yaml:"brute_force"`             // 暴力破解保护配置
	SecurityHeaders *middleware.SecurityHeadersConfig `json:"security_headers" yaml:"security_headers"`   // 安全头配置
	RequestSize     *middleware.RequestSizeConfig     `json:"request_size" yaml:"request_size"`           // 请求大小限制配置
}

// DefaultSecurity 默认安全配置
func DefaultSecurity() *Security {
	return &Security{
		RateLimit: middleware.DefaultRateLimitConfig(),
		GlobalRateLimit: &middleware.RateLimitConfig{
			Enabled:  true,
			Capacity: 1000, // 全局允许1000个突发请求
			Rate:     500,  // 全局每秒500个请求
		},
		BruteForce:      middleware.DefaultBruteForceConfig(),
		SecurityHeaders: middleware.DefaultSecurityHeadersConfig(),
		RequestSize:     middleware.DefaultRequestSizeConfig(),
	}
}
