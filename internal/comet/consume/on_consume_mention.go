package consume

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
)

// onConsumeMention 处理@提及通知
func (h *Handler) onConsumeMention(ctx context.Context, body []byte) {
	var in entity.SubEventImMessageMentionPayload
	if err := json.Unmarshal(body, &in); err != nil {
		slog.Error("[ChatSubscribe] onConsumeMention Unmarshal err", "error", err.Error())
		return
	}

	// 获取被@用户的所有session
	sessions := h.serv.SessionManager().GetSessions(int64(in.UserId))
	if len(sessions) == 0 {
		return
	}

	// 构建推送消息
	data := Message(entity.PushEventImMessageMention, entity.ImMessageMentionPayload{
		GroupId:   in.GroupId,
		GroupName: in.GroupName,
		FromId:    in.FromId,
		Nickname:  in.Nickname,
		Avatar:    in.Avatar,
		MsgIds:    in.MsgIds,
		Count:     in.Count,
		AtAll:     in.AtAll,
	})

	// 推送到用户的所有session
	for _, session := range sessions {
		if err := session.Write(data); err != nil {
			slog.Error("session write mention message error", "error", err)
		}
	}
}
