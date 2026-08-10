package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/external/push"
	"github.com/gzydong/go-chat/internal/apis/handler"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tidwall/sjson"
)

// NewRouter 初始化配置路由
func NewRouter(conf *config.Config, handler *handler.Handler, session *cache.JwtTokenStorage) *gin.Engine {
	if conf.Push != nil && conf.Push.Valid() {
		push.Init(conf.Push.AppID, conf.Push.Key, conf.Push.URL)
	}

	router := gin.New()

	router.Use(middleware.Cors(conf.Cors))
	router.Use(middleware.InjectClientIP())

	// 添加安全头中间件
	router.Use(middleware.SecurityHeadersMiddleware(conf.Security.SecurityHeaders))

	// 添加全局限流中间件
	router.Use(middleware.GlobalRateLimitMiddleware(conf.Security.GlobalRateLimit))

	// 添加 IP 限流中间件
	router.Use(middleware.RateLimitMiddleware(conf.Security.RateLimit))

	// 添加请求大小限制中间件
	router.Use(middleware.RequestSizeLimitMiddleware(conf.Security.RequestSize))

	// 添加 Prometheus 监控中间件
	router.Use(middleware.PrometheusMiddleware())

	if conf.Log.AccessLog {
		accessFilterRule := middleware.NewAccessFilterRule()
		accessFilterRule.Exclude("/api/v1/talk/records")
		accessFilterRule.Exclude("/api/v1/talk/history")
		accessFilterRule.Exclude("/api/v1/talk/forward")
		accessFilterRule.Exclude("/api/v1/talk/publish")
		accessFilterRule.AddRule("/api/v1/auth/login", func(info *middleware.RequestInfo) {
			info.RequestBody, _ = sjson.Set(info.RequestBody, `password`, "过滤敏感字段")
		})

		router.Use(middleware.AccessLog(
			logger.CreateFileWriter(conf.Log.LogFilePath("access.log")),
			accessFilterRule,
		))
	}

	router.Use(gin.RecoveryWithWriter(gin.DefaultWriter, func(c *gin.Context, err any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]any{"code": 500, "msg": "系统错误，请重试!!!"})
	}))

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, map[string]any{"code": 200, "message": "hello world"})
	})

	router.GET("/health/check", func(c *gin.Context) {
		c.JSON(200, map[string]any{"status": "ok"})
	})

	// Prometheus metrics 端点
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	RegisterWebRoute(conf, router, handler.Api, session)
	RegisterAdminRoute(conf.Jwt.Secret, router, handler.Admin, session)
	RegisterOpenRoute(router, handler.Open)

	// 注册 debug 路由
	if conf.Debug() {
		RegisterDebugRoute(router)
	}

	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, map[string]any{"code": 404, "message": "请求地址不存在"})
	})

	return router
}

func HandlerFunc(resp *Interceptor, fn func(ctx *gin.Context) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fn(c)
		if err != nil {
			resp.Error(c, err)
		} else if data != nil {
			resp.Success(c, data)
		}
	}
}
