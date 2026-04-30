package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
)

const JWTAuthID = "__JWT_AUTH_ID__"

// JWTTokenStringKey 用于在 context 中存储当前请求的 JWT 原始 token，供 Logout 等接口加入黑名单
type JWTTokenStringKey struct{}

type IStorage interface {
	// IsBlackList 判断是否是黑名单
	IsBlackList(ctx context.Context, token string) bool
}

type IClaims interface {
	GetAuthID() int
}

type AuthClaimsKey struct{}
type GinContextKey struct{}

func GetAuthToken(c *gin.Context) string {
	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	// Headers 中没有授权信息则读取 url 中的 token
	if token == "" {
		token = c.DefaultQuery("token", "")
	}

	return token
}

type JwtMiddlewareOption struct {
	// 访客路由
	ExclusionPaths []string
}

func NewJwtMiddleware[T IClaims](
	secret []byte,
	storage IStorage,
	fn func(ctx context.Context, claims *jwtutil.JwtClaims[T]) error,
	opts ...func(*JwtMiddlewareOption),
) gin.HandlerFunc {

	option := &JwtMiddlewareOption{}
	for _, opt := range opts {
		opt(option)
	}

	return func(c *gin.Context) {
		token := GetAuthToken(c)

		if token == "" {
			for _, path := range option.ExclusionPaths {
				if strings.HasSuffix(c.Request.URL.Path, path) {
					c.Next()
					return
				}
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "授权异常，请登录后操作!"})
			return
		}

		claims, err := jwtutil.ParseWithClaims[T](secret, token)
		if err != nil {
			for _, path := range option.ExclusionPaths {
				if strings.HasSuffix(c.Request.URL.Path, path) {
					c.Next()
					return
				}
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
			return
		}

		if storage.IsBlackList(c.Request.Context(), token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "授权异常，请登录后操作!"})
			return
		}

		if err = fn(c.Request.Context(), claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
			return
		}

		// 将用户信息与原始 token 放入上下文（token 供 Logout 加入黑名单）
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, AuthClaimsKey{}, claims.Metadata)
		ctx = context.WithValue(ctx, JWTTokenStringKey{}, token)
		c.Request = c.Request.WithContext(ctx)
		c.Set(JWTAuthID, claims.Metadata.GetAuthID())

		expiresAt := claims.ExpiresAt.Unix()
		// 提前15分钟刷新token
		if expiresAt-time.Now().Unix() < 900 {
			newToken, err := jwtutil.NewTokenWithClaims(secret, claims.Metadata, func(c *jwt.RegisteredClaims) {
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour * 2))
				c.Issuer = claims.Issuer
				c.Audience = claims.Audience
				c.Subject = claims.Subject
				c.NotBefore = claims.NotBefore
				c.IssuedAt = claims.IssuedAt
			})

			if err == nil {
				c.Header("Refresh-Access-Token", newToken)
				c.Header("Refresh-Access-Expires-At", "7200")
			}
		}

		c.Next()
	}
}

// FormContext 从上下文中获取用户信息
func FormContext[T IClaims](ctx context.Context) (T, error) {
	if ctx.Value(AuthClaimsKey{}) == nil {
		return *new(T), errors.New("claims is nil")
	}

	claims, ok := ctx.Value(AuthClaimsKey{}).(T)
	if !ok {
		return *new(T), errors.New("claims is nil")
	}

	return claims, nil
}

// FormContextAuthId 从上下文中获取用户ID
func FormContextAuthId[T IClaims](ctx context.Context) int {
	if ctx.Value(AuthClaimsKey{}) == nil {
		return 0
	}

	claims, ok := ctx.Value(AuthClaimsKey{}).(T)
	if !ok {
		return 0
	}

	return claims.GetAuthID()
}

// GetAuthTokenFromContext 从上下文中获取当前请求的 JWT 原始 token（由 JWT 中间件写入，供 Logout 加入黑名单）
func GetAuthTokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(JWTTokenStringKey{})
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
