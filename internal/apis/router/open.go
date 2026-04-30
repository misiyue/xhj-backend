package router

import (
	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/internal/apis/handler/open"
	"github.com/gzydong/go-chat/internal/pkg/core"
)

// RegisterOpenRoute 注册 Open 路由
func RegisterOpenRoute(router *gin.Engine, handler *open.Handler) {
	// v1 接口
	v1 := router.Group("/open/v1")
	{
		index := v1.Group("/index")
		{
			index.Any("", core.HandlerFunc(handler.V1.Index.Index))
		}
		// 第三方认证：根据 token 校验并返回用户 ID
		auth := v1.Group("/auth")
		{
			auth.Any("/verify-token", core.HandlerFunc(handler.V1.Auth.VerifyToken))
		}
	}
}
