package consume

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
)

// onConsumeMessageRead 私聊消息已读：通知发送方 WebSocket 更新已读状态
func (h *Handler) onConsumeMessageRead(ctx context.Context, body []byte) {
	var in entity.SubEventImMessageReadPayload
	if err := json.Unmarshal(body, &in); err != nil {
		logger.Errorf("[ChatSubscribe] onConsumeMessageRead Unmarshal err: %s", err.Error())
		return
	}
	if in.TalkMode != entity.ChatPrivateMode || in.FromId <= 0 || len(in.MsgIds) == 0 {
		return
	}

	data := Message(entity.PushEventImMessageRead, entity.ImMessageReadPayload{
		TalkMode:   in.TalkMode,
		FromId:     in.FromId,
		ReceiverId: in.ReceiverId,
		MsgIds:     in.MsgIds,
	})

	sessions := h.serv.SessionManager().GetSessions(int64(in.FromId))
	for _, session := range sessions {
		if err := session.Write(data); err != nil {
			slog.Error("[MessageRead] session write error", "error", err, "from_id", in.FromId)
		}
	}
}
