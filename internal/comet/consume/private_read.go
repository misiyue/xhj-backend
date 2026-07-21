package consume

import (
	"context"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/model"
)

// markAndDeliverPrivateRead 标记私聊已读并向发送方推送 im.message.read（含 msg_ids）。
func (h *Handler) markAndDeliverPrivateRead(ctx context.Context, readerId, senderId int, msgIds []string) {
	if readerId <= 0 || senderId <= 0 || len(msgIds) == 0 {
		return
	}

	readMsgIds := h.markPrivateMessagesRead(ctx, readerId, senderId, msgIds)
	if len(readMsgIds) == 0 {
		return
	}

	h.deliverPrivateMessageRead(ctx, senderId, readerId, readMsgIds)
}

func (h *Handler) markPrivateMessagesRead(ctx context.Context, readerId, senderId int, msgIds []string) []string {
	if h.TalkService != nil {
		readMsgIds, err := h.TalkService.MarkPrivateMessagesRead(ctx, readerId, senderId, msgIds)
		if err != nil {
			logger.Errorf("mark private messages read err: reader_id=%d sender_id=%d %s", readerId, senderId, err.Error())
			return nil
		}
		return readMsgIds
	}
	if h.Source == nil {
		logger.Warnf("mark private messages read skipped: Source and TalkService are nil, reader_id=%d sender_id=%d", readerId, senderId)
		return nil
	}

	db := h.Source.Db().WithContext(ctx)
	result := db.Model(&model.TalkUserMessage{}).
		Where(
			"msg_id in ? and from_id = ? and receiver_id = ? and is_read = ?",
			msgIds, senderId, readerId, model.TalkUserMessageIsReadNo,
		).
		Update("is_read", model.TalkUserMessageIsReadYes)
	if result.Error != nil {
		logger.Errorf("mark private messages read db err: reader_id=%d sender_id=%d %s", readerId, senderId, result.Error.Error())
		return nil
	}
	if result.RowsAffected == 0 {
		return nil
	}

	var readMsgIds []string
	if err := db.Model(&model.TalkUserMessage{}).
		Where(
			"msg_id in ? and from_id = ? and receiver_id = ? and is_read = ?",
			msgIds, senderId, readerId, model.TalkUserMessageIsReadYes,
		).
		Pluck("msg_id", &readMsgIds).Error; err != nil {
		logger.Errorf("mark private messages read pluck err: reader_id=%d sender_id=%d %s", readerId, senderId, err.Error())
		return nil
	}
	return readMsgIds
}

// deliverPrivateMessageRead 向发送方推送已读事件，payload 含 msg_ids。
// 发送方若在本 Comet 节点在线则直接写 WebSocket；否则经 Redis 转发到其他节点。
func (h *Handler) deliverPrivateMessageRead(ctx context.Context, senderId, readerId int, msgIds []string) {
	if senderId <= 0 || len(msgIds) == 0 || h.serv == nil {
		return
	}

	data := buildPrivateMessageReadWS(senderId, readerId, msgIds)

	localWritten := 0
	for _, session := range h.serv.SessionManager().GetSessions(int64(senderId)) {
		if err := session.Write(data); err != nil {
			slog.Error("[MessageRead] private direct session write error", "error", err, "sender_id", senderId)
			continue
		}
		localWritten++
	}

	if localWritten > 0 {
		return
	}

	push := h.ensurePushMessage()
	if push == nil {
		logger.Warnf("private message read push skipped: PushMessage is nil, sender_id=%d reader_id=%d msg_ids=%v", senderId, readerId, msgIds)
		return
	}
	if err := push.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessageRead,
		Payload: jsonutil.Encode(entity.SubEventImMessageReadPayload{
			TalkMode:   entity.ChatPrivateMode,
			FromId:     senderId,
			ReceiverId: readerId,
			MsgIds:     msgIds,
		}),
	}); err != nil {
		logger.Errorf("private message read redis push err: sender_id=%d reader_id=%d %s", senderId, readerId, err.Error())
	}
}

func buildPrivateMessageReadWS(senderId, readerId int, msgIds []string) []byte {
	return buildMessageReadWS(entity.ChatPrivateMode, senderId, readerId, msgIds)
}

func buildMessageReadWS(talkMode, fromId, receiverId int, msgIds []string) []byte {
	return Message(entity.PushEventImMessageRead, entity.ImMessageReadPayload{
		TalkMode:   talkMode,
		FromId:     fromId,
		ReceiverId: receiverId,
		MsgIds:     msgIds,
	})
}

func (h *Handler) ensurePushMessage() *logic.PushMessage {
	if h.PushMessage != nil {
		return h.PushMessage
	}
	if h.Source == nil {
		return nil
	}
	h.PushMessage = &logic.PushMessage{Redis: h.Source.Redis()}
	return h.PushMessage
}
