package consume

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
)

func (h *Handler) onConsumeSysNotice(ctx context.Context, body []byte) {
	var in entity.SubEventSysNoticePayload
	if err := json.Unmarshal(body, &in); err != nil {
		slog.Error("[ChatSubscribe] onConsumeSysNotice Unmarshal err", "error", err.Error())
		return
	}
	if in.UserId <= 0 {
		return
	}

	data := Message(entity.PushEventSysNotice, entity.ImSysNoticePayload{
		Id:        in.Id,
		UserId:    in.UserId,
		Title:     in.Title,
		Content:   in.Content,
		Url:       in.Url,
		CreatedAt: in.CreatedAt,
	})

	for _, session := range h.serv.SessionManager().GetSessions(int64(in.UserId)) {
		if err := session.Write(data); err != nil {
			slog.Error("session write sys notice error", "error", err)
		}
	}
}
