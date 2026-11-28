package comet

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/longnet"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/redis/go-redis/v9"
)

type Heartbeat struct {
	ServerStorage *cache.ServerStorage
	UserClient    *cache.UserClient
	Redis         *redis.Client
	startAt       string
}

type ServerInfo struct {
	StartAt         string `json:"start_at"`           // 启动时间
	ActiveAt        string `json:"active_at"`          // 心跳时间
	CurrConnNum     int32  `json:"curr_conn_num"`      // 当前连接数
	CurrConnUserNum int32  `json:"curr_conn_user_num"` // 当前用户数
	AutoConnId      int64  `json:"auto_conn_id"`       // 自动连接ID
	SendMessageNum  int64  `json:"send_message_num"`   // 发送消息数
	RecvMessageNum  int64  `json:"recv_message_num"`   // 接收消息数
}

func (h *Heartbeat) Start(ctx context.Context, serv longnet.IServer) error {
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()

	h.startAt = time.Now().Format(time.DateTime)
	logger.Infof("comet heartbeat started, server_id=%s", serv.ServerId())
	h.save(serv)

	for {
		select {
		case <-ctx.Done():
			bg := context.Background()
			if err := h.Redis.HDel(bg, cache.ServerInfoKey, serv.ServerId()).Err(); err != nil {
				logger.Errorf("comet heartbeat HDel %s: %v", cache.ServerInfoKey, err)
			}
			if h.ServerStorage != nil {
				_ = h.ServerStorage.Del(bg, serv.ServerId())
			}
			return nil
		case <-timer.C:
			h.save(serv)
		}
	}
}

func (h *Heartbeat) save(serv longnet.IServer) {
	if h == nil || h.Redis == nil {
		logger.Errorf("comet heartbeat skipped: redis is nil")
		return
	}

	bg := context.Background()
	sm := serv.SessionManager()
	if sm == nil {
		logger.Errorf("comet heartbeat skipped: session manager is nil")
		return
	}

	h.refreshClients(bg, serv, sm)

	info := &ServerInfo{
		StartAt:         h.startAt,
		ActiveAt:        time.Now().Format(time.DateTime),
		CurrConnNum:     sm.GetSessionNum(),
		CurrConnUserNum: sm.GetSessionUserNum(),
	}

	if err := h.Redis.HSet(bg, cache.ServerInfoKey, serv.ServerId(), jsonutil.Encode(info)).Err(); err != nil {
		logger.Errorf("comet heartbeat HSET %s failed: %v", cache.ServerInfoKey, err)
		return
	}
	if h.ServerStorage != nil {
		if err := h.ServerStorage.Set(bg, serv.ServerId(), time.Now().Unix()); err != nil {
			logger.Errorf("comet heartbeat server_ids failed: %v", err)
		}
	}
}

func (h *Heartbeat) refreshClients(ctx context.Context, serv longnet.IServer, sm longnet.ISessionManager) {
	if h.UserClient == nil {
		return
	}
	sid := serv.ServerId()
	for sess := range sm.Iterator() {
		if sess == nil || sess.UserId() <= 0 {
			continue
		}
		if err := h.UserClient.Bind(ctx, sid, sess.ConnId(), sess.UserId()); err != nil {
			logger.Errorf("comet heartbeat bind uid=%d: %v", sess.UserId(), err)
		}
	}
}
