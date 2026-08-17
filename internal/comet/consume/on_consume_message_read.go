package consume

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
)

// onConsumeMessageRead 消息已读：私聊通知发送方，群聊通知在线群成员
func (h *Handler) onConsumeMessageRead(ctx context.Context, body []byte) {
	var in entity.SubEventImMessageReadPayload
	if err := json.Unmarshal(body, &in); err != nil {
		logger.Errorf("[ChatSubscribe] onConsumeMessageRead Unmarshal err: %s", err.Error())
		return
	}
	if len(in.MsgIds) == 0 {
		return
	}

	data := buildMessageReadWS(in.TalkMode, in.FromId, in.ReceiverId, in.MsgIds)

	switch in.TalkMode {
	case entity.ChatPrivateMode:
		if in.ReceiverId <= 0 {
			return
		}
		for _, session := range h.serv.SessionManager().GetSessions(int64(in.ReceiverId)) {
			if err := session.Write(data); err != nil {
				slog.Error("[MessageRead] private session write error", "error", err, "receiver_id", in.ReceiverId)
			}
		}
	case entity.ChatGroupMode:
		if in.ReceiverId <= 0 || h.GroupMemberRepo == nil {
			return
		}
		memberIds := h.GroupMemberRepo.GetMemberIds(ctx, in.ReceiverId)
		for _, uid := range memberIds {
			for _, session := range h.serv.SessionManager().GetSessions(int64(uid)) {
				if err := session.Write(data); err != nil {
					slog.Error("[MessageRead] group session write error", "error", err, "user_id", uid, "group_id", in.ReceiverId)
				}
			}
		}
	}
}
