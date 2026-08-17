package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

type clientIPCtxKey struct{}

// InjectClientIP 将 gin ClientIP 写入 request context，供仅持有 context.Context 的 handler 使用。
func InjectClientIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ctx := context.WithValue(c.Request.Context(), clientIPCtxKey{}, ip)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ClientIPFromContext 读取客户端 IP；未注入时返回空字符串。
func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientIPCtxKey{}).(string)
	return v
}
