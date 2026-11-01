package message

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/samber/lo"
)

func (s *Service) CreatePrivateMessage(ctx context.Context, option CreatePrivateMessageOption) error {
	var (
		orgMsgId      = strutil.NewMsgId()
		quoteJsonText = "{}"
		now           = time.Now()
	)

	// 如果带有引用消息（回复某条消息），则从消息表和用户表中查出被引用的内容，
	// 并组装成 Quote 结构，序列化到当前消息的 Quote 字段中，方便前端展示「回复了谁的哪条消息」。
	if option.QuoteId != "" {
		quoteRecord := &model.TalkUserMessage{}
		if err := s.Source.Db().First(quoteRecord, "msg_id = ?", option.QuoteId).Error; err != nil {
			return err
		}

		user := &model.Users{}
		if err := s.Source.Db().First(user, "id = ?", quoteRecord.FromId).Error; err != nil {
			return err
		}

		queue := &model.Quote{
			QuoteId: option.QuoteId,
			MsgType: 1,
		}

		queue.Nickname = user.Nickname
		queue.Content = s.getTextMessage(quoteRecord.MsgType, quoteRecord.Extra)
		quoteJsonText = jsonutil.Encode(queue)
	}

	// 接收方是否拉黑了发送方：被拉黑方可发且落库到自己视角(user_id=from)，拉黑方不接收
	blockedByReceiver := s.ContactRepo != nil && s.ContactRepo.IsBlocked(ctx, option.ReceiverId, option.FromId)

	// 确保双方会话存在，并写入统一的 session_id（避免对方拉历史/会话列表漏消息）
	sessionId := s.resolvePrivateLinkSessionId(ctx, option.FromId, option.ReceiverId, !blockedByReceiver)

	// 正常消息 user_id=0 双方可见；被拉黑时 user_id=发送方，仅发送方查询可见
	ownerUserId := 0
	if blockedByReceiver {
		ownerUserId = option.FromId
	}

	message := &model.TalkUserMessage{
		MsgId:      lo.Ternary(option.MsgId == "", strutil.NewMsgId(), option.MsgId),
		MsgType:    option.MsgType,
		UserId:     ownerUserId,
		ReceiverId: option.ReceiverId,
		FromId:     option.FromId,
		SessionId:  sessionId,
		Extra:      option.Extra,
		Quote:      quoteJsonText,
		OrgMsgId:   orgMsgId,
		SendTime:   now,
		IsRevoked:  model.No,
		IsDeleted:  model.No,
	}

	if err := s.Db().WithContext(ctx).Create(message).Error; err != nil {
		return err
	}

	// 推送：始终推送给发送方；未被拉黑时再推接收方
	pipe := s.Source.Redis().Pipeline()

	senderMessage := *message
	senderMessage.UserId = option.FromId
	senderContent := &entity.SubscribeMessage{
		Event: entity.SubEventImMessage,
		Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
			TalkMode: entity.ChatPrivateMode,
			Message:  jsonutil.Encode(senderMessage),
		}),
	}
	pipe.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(senderContent))

	if !blockedByReceiver {
		if option.MsgType != entity.ChatMsgTypeRTCCall {
			s.tryOneSignalUserChat(ctx, option.ReceiverId, entity.ChatPrivateMode, option.FromId)
		}

		receiverMessage := *message
		receiverMessage.UserId = option.ReceiverId
		receiverContent := &entity.SubscribeMessage{
			Event: entity.SubEventImMessage,
			Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
				TalkMode: entity.ChatPrivateMode,
				Message:  jsonutil.Encode(receiverMessage),
			}),
		}
		pipe.Publish(ctx, entity.ImTopicChat, jsonutil.Encode(receiverContent))
		s.UnreadStorage.PipeIncr(ctx, pipe, option.ReceiverId, entity.ChatPrivateMode, option.FromId)

		// 双方共用 last_message 缓存；仅正常送达时更新，避免拉黑方会话预览被刷新
		msgContent := s.getTextMessage(message.MsgType, option.Extra)
		msgTime := message.CreatedAt.Format(time.DateTime)
		_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, option.FromId, option.ReceiverId, &cache.LastCacheMessage{
			Content:  msgContent,
			Datetime: msgTime,
		})
	}

	_, _ = pipe.Exec(ctx)

	return nil
}

func (s *Service) CreateToUserPrivateMessage(ctx context.Context, data *model.TalkUserMessage) error {
	if data.MsgId == "" {
		data.MsgId = strutil.NewMsgId()
	}

	if data.OrgMsgId == "" {
		data.OrgMsgId = data.MsgId
	}

	if data.Quote == "" {
		data.Quote = "{}"
	}

	if data.SendTime.IsZero() {
		data.SendTime = time.Now()
	}

	// 如果没有 session_id，解析/创建双方共享 session_id，避免对方拉历史漏消息
	if data.SessionId == 0 {
		fromId := data.FromId
		if fromId <= 0 {
			fromId = data.UserId
		}
		data.SessionId = s.resolvePrivateLinkSessionId(ctx, fromId, data.ReceiverId, true)
	}

	data.IsRevoked = model.No
	data.IsDeleted = model.No

	if err := s.Db().WithContext(ctx).Create(data).Error; err != nil {
		return err
	}

	s.tryOneSignalUserChat(ctx, data.ReceiverId, entity.ChatPrivateMode, data.FromId)

	err := s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessage,
		Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
			TalkMode: entity.ChatPrivateMode,
			Message:  jsonutil.Encode(data),
		}),
	})
	if err != nil {
		logger.Errorf("SendToUserPrivateLetter redis push err:%s", err.Error())
	}

	s.UnreadStorage.Incr(ctx, data.UserId, entity.ChatPrivateMode, data.ReceiverId)

	// 更新最后一条消息
	_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, data.UserId, data.ReceiverId, &cache.LastCacheMessage{
		Content:  s.getTextMessage(data.MsgType, data.Extra),
		Datetime: data.CreatedAt.Format(time.DateTime),
	})

	return nil
}

func (s *Service) CreatePrivateSysMessage(ctx context.Context, option CreatePrivateSysMessageOption) error {
	return s.CreateToUserPrivateMessage(ctx, &model.TalkUserMessage{
		MsgId:      strutil.NewMsgId(),
		MsgType:    entity.ChatMsgSysText,
		UserId:     option.FromId,
		ReceiverId: option.ReceiverId,
		FromId:     0,
		Extra: jsonutil.Encode(model.TalkRecordExtraText{
			Content: option.Content,
		}),
		Quote:    "{}",
		SendTime: time.Now(),
	})
}

func (s *Service) CreatePrivateRetainDaysSetMessage(ctx context.Context, fromId, receiverId, retainDays int) error {
	return s.CreatePrivateMessage(ctx, CreatePrivateMessageOption{
		MsgType:    entity.ChatMsgSysRetainDaysSet,
		FromId:     fromId,
		ReceiverId: receiverId,
		Extra:      jsonutil.Encode(map[string]int{"retain_days": retainDays}),
	})
}
