package consume

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
)

func (h *Handler) onConsumeSessionUnreadCleared(ctx context.Context, body []byte) {
	var in entity.SubEventImSessionUnreadClearedPayload
	if err := json.Unmarshal(body, &in); err != nil {
		logger.Errorf("[ChatSubscribe] onConsumeSessionUnreadCleared Unmarshal err: %s", err.Error())
		return
	}

	sessions := h.serv.SessionManager().GetSessions(int64(in.UserId))
	if len(sessions) == 0 {
		return
	}

	data := Message(entity.PushEventImSessionUnreadCleared, entity.ImSessionUnreadClearedPayload{
		UserId:     in.UserId,
		TalkMode:   in.TalkMode,
		ReceiverId: in.ReceiverId,
		UnreadNum:  in.UnreadNum,
	})

	for _, session := range sessions {
		if err := session.Write(data); err != nil {
			slog.Error("session write message error", "error", err)
		}
	}
}
