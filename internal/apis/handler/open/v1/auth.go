package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
)

// Auth 供第三方系统校验前端传来的 token，并返回对应用户 ID
type Auth struct {
	Config          *config.Config
	JwtTokenStorage *cache.JwtTokenStorage
}

// NewAuth 创建 Open Auth 处理器
func NewAuth(c *config.Config, s *cache.JwtTokenStorage) *Auth {
	return &Auth{Config: c, JwtTokenStorage: s}
}

// VerifyTokenRequest 可选：若通过 body 传 token 可使用
type VerifyTokenRequest struct {
	Token string `json:"token" form:"token"`
}

// VerifyToken 第三方认证接口：根据 token 校验并返回用户 ID
// Token 可从 Header Authorization: Bearer <token> 或 Query ?token= 或 Body {"token":"..."} 传入
func (a *Auth) VerifyToken(ctx *core.Context) error {
	token := middleware.GetAuthToken(ctx.Context)
	if token == "" {
		var req VerifyTokenRequest
		_ = ctx.Context.ShouldBind(&req)
		token = req.Token
	}
	if token == "" {
		return ctx.Unauthorized("缺少 token")
	}

	secret := []byte(a.Config.Jwt.Secret)
	if len(secret) == 0 {
		return ctx.Unauthorized("服务未配置 JWT")
	}

	claims, err := jwtutil.ParseWithClaims[entity.WebClaims](secret, token)
	if err != nil {
		return ctx.Unauthorized("token 无效或已过期")
	}
	if claims.RegisteredClaims.Issuer != entity.JwtIssuerWeb {
		return ctx.Unauthorized("token 类型错误")
	}
	if a.JwtTokenStorage != nil && a.JwtTokenStorage.IsBlackList(ctx.Context.Request.Context(), token) {
		return ctx.Unauthorized("token 已失效")
	}

	return ctx.Success(gin.H{"user_id": claims.Metadata.UserId})
}
