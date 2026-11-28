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

// 聊天消息事件
func (h *Handler) onConsumeTalk(ctx context.Context, body []byte) {
	var in entity.SubEventImMessagePayload
	if err := json.Unmarshal(body, &in); err != nil {
		logger.Errorf("[ChatSubscribe] onConsumeTalk Unmarshal err: %s", err.Error())
		return
	}

	if in.TalkMode == entity.ChatPrivateMode {
		h.onConsumeTalkPrivateMessage(ctx, in)
	} else if in.TalkMode == entity.ChatGroupMode {
		h.onConsumeTalkGroupMessage(ctx, in)
	}
}

// 私有消息(点对点消息)
func (h *Handler) onConsumeTalkPrivateMessage(ctx context.Context, in entity.SubEventImMessagePayload) {
	message := model.TalkUserMessage{}
	if err := json.Unmarshal([]byte(in.Message), &message); err != nil {
		return
	}

	sessions := h.serv.SessionManager().GetSessions(int64(message.UserId))
	if len(sessions) == 0 {
		return
	}

	body := entity.ImMessagePayloadBody{
		MsgId:     message.MsgId,
		MsgType:   message.MsgType,
		FromId:    message.FromId,
		IsRevoked: message.IsRevoked,
		SendTime:  message.CreatedAt.Format(time.DateTime),
		Extra:     message.Extra,
		Quote:     message.Quote,
	}

	if body.FromId > 0 {
		user, err := h.UserRepo.FindByIdWithCache(ctx, message.FromId)
		if err != nil {
			return
		}

		body.Nickname = user.Nickname
		body.Avatar = user.Avatar
	}

	data := Message(entity.PushEventImMessage, entity.ImMessagePayload{
		TalkMode:   entity.ChatPrivateMode,
		ReceiverId: message.ReceiverId,
		FromId:     message.FromId,
		Body:       body,
	})

	for _, session := range sessions {
		if err := session.Write(data); err != nil {
			slog.Error("session write message error", "error", err)
		}
	}
}

// 群消息
func (h *Handler) onConsumeTalkGroupMessage(ctx context.Context, in entity.SubEventImMessagePayload) {
	message := model.TalkGroupMessage{}
	if err := json.Unmarshal([]byte(in.Message), &message); err != nil {
		return
	}

	memberIds := h.GroupMemberRepo.GetMemberIds(ctx, message.GroupId)
	if len(memberIds) == 0 {
		return
	}

	// 指定投递目标时，仅推送给这些用户（用于仅本人可见的系统提示）
	if len(in.TargetUserIds) > 0 {
		allow := make(map[int]struct{}, len(in.TargetUserIds))
		for _, uid := range in.TargetUserIds {
			allow[uid] = struct{}{}
		}
		filtered := make([]int, 0, len(in.TargetUserIds))
		for _, memberId := range memberIds {
			if _, ok := allow[memberId]; ok {
				filtered = append(filtered, memberId)
			}
		}
		memberIds = filtered
	}
	if len(memberIds) == 0 {
		return
	}

	var clientIds []int64
	for _, memberId := range memberIds {
		ids := h.serv.SessionManager().GetConnIds(int64(memberId))
		if len(ids) == 0 {
			continue
		}

		clientIds = append(clientIds, ids...)
	}

	data := entity.ImMessagePayloadBody{
		MsgId:     message.MsgId,
		MsgType:   message.MsgType,
		FromId:    message.FromId,
		IsRevoked: message.IsRevoked,
		SendTime:  message.SendTime.Format(time.DateTime),
		Extra:     message.Extra,
		Quote:     message.Quote,
	}

	if data.FromId > 0 {
		user, err := h.UserRepo.FindByIdWithCache(ctx, message.FromId)
		if err != nil {
			return
		}

		data.Nickname = user.Nickname
		data.Avatar = user.Avatar
	}

	msg := Message(entity.PushEventImMessage, entity.ImMessagePayload{
		TalkMode:   entity.ChatGroupMode,
		ReceiverId: message.GroupId,
		FromId:     message.FromId,
		Body:       data,
	})

	for _, cid := range clientIds {
		session, err := h.serv.SessionManager().GetSession(cid)
		if err != nil {
			continue
		}

		if err := session.Write(msg); err != nil {
			slog.Error("session write message error", "error", err)
		}
	}
}
