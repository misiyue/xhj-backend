package consume

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/model"
)

func (h *Handler) onConsumeMerchantMessage(ctx context.Context, data []byte) {
	var sub entity.SubEventImMessageMerchantPayload
	if err := json.Unmarshal(data, &sub); err != nil {
		logger.Errorf("[ChatSubscribe] merchant message unmarshal err: %s", err.Error())
		return
	}

	var m model.MerchantMessage
	if err := json.Unmarshal([]byte(sub.Message), &m); err != nil {
		logger.Errorf("[ChatSubscribe] merchant message body unmarshal err: %s", err.Error())
		return
	}

	sessions := h.serv.SessionManager().GetSessions(int64(sub.InboxUserId))
	if len(sessions) == 0 {
		return
	}

	sendAt := m.SendTime
	if sendAt.IsZero() {
		sendAt = m.CreatedAt
	}
	body := entity.ImMessagePayloadBody{
		MsgId:     m.MsgId,
		MsgType:   m.MsgType,
		FromId:    m.FromId,
		IsRevoked: m.IsRevoked,
		SendTime:  sendAt.Format(time.DateTime),
		Extra:     m.Extra,
		Quote:     m.Quote,
	}

	if body.FromId > 0 {
		user, err := h.UserRepo.FindByIdWithCache(ctx, m.FromId)
		if err != nil {
			return
		}
		body.Nickname = user.Nickname
		body.Avatar = user.Avatar
	}

	payload := entity.ImMessagePayload{
		TalkMode:   entity.ChatMerchantMode,
		FromId:     m.FromId,
		ReceiverId: m.ReceiverId,
		SessionId:  m.SessionId,
		OrderId:    sub.OrderId,
		Body:       body,
	}

	msg := Message(entity.PushEventImMessageMerchantC2c, payload)
	for _, session := range sessions {
		if err := session.Write(msg); err != nil {
			slog.Error("merchant c2c session write error", "error", err)
		}
	}
}
