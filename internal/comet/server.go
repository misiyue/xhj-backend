package comet

import (
	"context"
	"fmt"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/longnet"
	"github.com/gzydong/go-chat/internal/pkg/sysinfo"
	"github.com/gzydong/go-chat/internal/provider"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	Config    *config.Config
	Subscribe *Subscribe
	Handler   *Handler
	Heartbeat *Heartbeat
	Authorize provider.UserJwtAuthorize
	Redis     *redis.Client
}

func (s *Server) Start(ctx context.Context) error {
	// 确定最大连接数
	maxOpenConns := s.Config.Server.MaxOpenConns
	if maxOpenConns <= 0 {
		// 如果没有配置或配置为0，则自动检测
		reservePercent := s.Config.Server.ReservePercent
		if reservePercent <= 0 || reservePercent > 100 {
			reservePercent = 35 // 默认预留35%资源
		}

		autoMaxConns, err := sysinfo.AutoConfigureMaxConnections(reservePercent)
		if err != nil {
			return fmt.Errorf("自动配置最大连接数失败: %w", err)
		}
		maxOpenConns = autoMaxConns
	} else {
		logger.Infof("使用配置文件指定的最大连接数: %d", maxOpenConns)
	}

	// Get WebSocket CORS origins from config
	wsOrigins := "*"
	if s.Config.Cors != nil {
		wsOrigins = s.Config.Cors.GetWebsocketOrigins()
	}

	// Log warning if CORS is set to "*"
	if wsOrigins == "*" {
		logger.Warnf("WebSocket CORS is set to '*' - allowing connections from all origins. This is not recommended for production environments.")
	}

	serv := longnet.New(longnet.Options{
		MaxOpenConns:  maxOpenConns,
		MaxPacketSize: 2 << 20,
		WSSConfig: &longnet.WSSConfig{
			Addr:    s.Config.Server.WebsocketAddr,
			Path:    "/wss/default.io",
			Origins: wsOrigins,
		},
	})

	// Set snowflake-based ID generator for HA support
	// This ensures unique connection IDs across multiple instances
	serv.SetIdGenerator(longnet.NewSnowflakeGenerator(s.Redis))

	serv.SetAuthorize(s.onAuthorize)
	serv.SetHandler(s.Handler)
	serv.SetEncoder(longnet.NewEncoder(longnet.EncoderOptions{
		MaxPacketSize:         2 << 20,   // 2M
		MinCompressPacketSize: 10 * 1024, // 10KB
	}, nil, nil))

	serv.SetCustomProcess(s.Heartbeat)
	serv.SetCustomProcess(s.Subscribe)

	return serv.Start(ctx)
}

// onTcpAuthorize 授权认证
func (s *Server) onAuthorize(ctx context.Context, token string) (int64, error) {
	claims, err := s.Authorize.Valid(token)
	if err != nil {
		return 0, err
	}

	return int64(claims.Metadata.UserId), nil
}
