package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig 安全头配置
type SecurityHeadersConfig struct {
	Enabled                 bool   `yaml:"enabled"`                   // 是否启用
	XFrameOptions           string `yaml:"x_frame_options"`           // X-Frame-Options
	XContentTypeOptions     string `yaml:"x_content_type_options"`    // X-Content-Type-Options
	XSSProtection           string `yaml:"xss_protection"`            // X-XSS-Protection
	StrictTransportSecurity string `yaml:"strict_transport_security"` // Strict-Transport-Security (HSTS)
	ContentSecurityPolicy   string `yaml:"content_security_policy"`   // Content-Security-Policy
	ReferrerPolicy          string `yaml:"referrer_policy"`           // Referrer-Policy
	PermissionsPolicy       string `yaml:"permissions_policy"`        // Permissions-Policy
}

// DefaultSecurityHeadersConfig 默认安全头配置
func DefaultSecurityHeadersConfig() *SecurityHeadersConfig {
	return &SecurityHeadersConfig{
		Enabled:                 true,
		XFrameOptions:           "SAMEORIGIN",
		XContentTypeOptions:     "nosniff",
		XSSProtection:           "1; mode=block",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		PermissionsPolicy:       "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeadersMiddleware 创建安全头中间件
func SecurityHeadersMiddleware(config *SecurityHeadersConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultSecurityHeadersConfig()
	}

	// 如果未启用，直接返回空中间件
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// X-Frame-Options: 防止点击劫持攻击
		if config.XFrameOptions != "" {
			c.Header("X-Frame-Options", config.XFrameOptions)
		}

		// X-Content-Type-Options: 防止MIME类型嗅探
		if config.XContentTypeOptions != "" {
			c.Header("X-Content-Type-Options", config.XContentTypeOptions)
		}

		// X-XSS-Protection: 启用浏览器XSS过滤器
		if config.XSSProtection != "" {
			c.Header("X-XSS-Protection", config.XSSProtection)
		}

		// Strict-Transport-Security: 强制使用HTTPS
		if config.StrictTransportSecurity != "" {
			c.Header("Strict-Transport-Security", config.StrictTransportSecurity)
		}

		// Content-Security-Policy: 内容安全策略
		if config.ContentSecurityPolicy != "" {
			c.Header("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		// Referrer-Policy: 控制Referer信息
		if config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		// Permissions-Policy: 控制浏览器功能
		if config.PermissionsPolicy != "" {
			c.Header("Permissions-Policy", config.PermissionsPolicy)
		}

		// 移除可能泄露服务器信息的头
		c.Header("X-Powered-By", "")
		c.Header("Server", "")

		c.Next()
	}
}

// RequestSizeConfig 请求大小限制配置
type RequestSizeConfig struct {
	Enabled     bool  `yaml:"enabled"`       // 是否启用
	MaxBodySize int64 `yaml:"max_body_size"` // 最大请求体大小（字节）
}

// DefaultRequestSizeConfig 默认请求大小限制配置
func DefaultRequestSizeConfig() *RequestSizeConfig {
	return &RequestSizeConfig{
		Enabled:     true,
		MaxBodySize: 10 * 1024 * 1024, // 10MB
	}
}

// RequestSizeLimitMiddleware 创建请求大小限制中间件
func RequestSizeLimitMiddleware(config *RequestSizeConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRequestSizeConfig()
	}

	// 如果未启用，直接返回空中间件
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		// 限制请求体大小
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.MaxBodySize)

		c.Next()
	}
}
