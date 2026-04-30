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

func (s *Service) CreateGroupMessage(ctx context.Context, option CreateGroupMessageOption) error {
	quoteJsonText := "{}"

	// 处理引用消息：如果当前消息是对某条群消息的「回复」，
	// 就从群消息表中查出被引用的消息和发送者昵称，封装成 Quote JSON，
	// 方便前端展示「回复了谁的哪条消息」。
	if option.QuoteId != "" {
		quoteRecord := &model.TalkGroupMessage{}
		if err := s.Source.Db().First(quoteRecord, "msg_id = ?", option.QuoteId).Error; err != nil {
			return err
		}

		user := &model.Users{}
		if err := s.Source.Db().First(user, "id = ?", quoteRecord.FromId).Error; err != nil {
			return err
		}

		quote := &model.Quote{
			QuoteId: option.QuoteId,
			MsgType: 1,
		}

		quote.Nickname = user.Nickname
		quote.Content = s.getTextMessage(quoteRecord.MsgType, quoteRecord.Extra)
		quoteJsonText = jsonutil.Encode(quote)
	}

	// 如果当前用户之前对该群的会话做过「删除会话」，此处重新发消息时需要自动恢复：
	// 将 talk_session 中 user_id=发送者、receiver_id=群ID、talk_mode=2 的记录的 is_delete 从 1 改回 2。
	if s.TalkSessionRepo != nil {
		if sess, err := s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", option.FromId, option.ReceiverId, entity.ChatGroupMode); err == nil && sess.Id > 0 {
			if sess.IsDelete == model.Yes {
				_, _ = s.TalkSessionRepo.UpdateByWhere(ctx, map[string]any{
					"is_delete":  model.No,
					"updated_at": time.Now(),
				}, "id = ?", sess.Id)
				sess.IsDelete = model.No
			}
		}
	}

	// 构造群聊消息记录，注意：
	//   - MsgId 为空则自动生成（确保全局唯一）
	//   - GroupId 为群 ID，FromId 为发送者用户 ID
	//   - Extra 里存放具体消息体（文本/图片等，见 CreateTextMessage 等组装）
	item := &model.TalkGroupMessage{
		MsgId:     lo.Ternary(option.MsgId == "", strutil.NewMsgId(), option.MsgId),
		MsgType:   option.MsgType,
		GroupId:   option.ReceiverId,
		FromId:    option.FromId,
		Quote:     quoteJsonText,
		Extra:     option.Extra,
		IsRevoked: model.No,
		SendTime:  time.Now(),
	}

	if err := s.Source.Db().WithContext(ctx).Create(item).Error; err != nil {
		return err
	}

	// 通过 PushMessage 将消息投递到 ImTopicChat 主题，
	// comet / 长连接服务会订阅该主题，并把消息推送给在线客户端
	err := s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessage,
		Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
			TalkMode: entity.ChatGroupMode,
			Message:  jsonutil.Encode(item),
		}),
	})
	if err != nil {
		logger.Errorf("CreateGroupMessage publish message error:%s", err.Error())
	}

	// 取出当前群的所有成员，用于维护未读数 & @ 提醒
	memberIds := s.GroupMemberRepo.GetMemberIds(ctx, item.GroupId)

	pipe := s.Source.Redis().Pipeline()
	for _, uid := range memberIds {
		if uid != item.FromId {
			s.UnreadStorage.PipeIncr(ctx, pipe, uid, entity.ChatGroupMode, item.GroupId)
		}
	}
	_, _ = pipe.Exec(ctx)

	// 处理 @ 提及通知：将被 @ 用户写入 MentionStorage，并额外推送一条 @ 通知事件
	s.processMentions(ctx, item, option.Extra, memberIds)

	// 更新最后一条消息
	_ = s.MessageStorage.Set(ctx, entity.ChatGroupMode, item.FromId, item.GroupId, &cache.LastCacheMessage{
		Content:  s.getTextMessage(item.MsgType, option.Extra),
		Datetime: item.CreatedAt.Format(time.DateTime),
	})

	return nil
}

// processMentions 处理@提及通知
func (s *Service) processMentions(ctx context.Context, item *model.TalkGroupMessage, extra string, memberIds []int) {
	// 解析extra获取mentions列表
	var textExtra model.TalkRecordExtraText
	if err := jsonutil.Unmarshal(extra, &textExtra); err != nil {
		return
	}

	if len(textExtra.Mentions) == 0 {
		return
	}

	// 检查是否@所有人（mentions包含0表示@所有人）
	atAll := false
	mentionedUserIds := make([]int, 0)

	for _, uid := range textExtra.Mentions {
		if uid == 0 {
			atAll = true
		} else {
			mentionedUserIds = append(mentionedUserIds, uid)
		}
	}

	// 获取发送者信息
	var senderNickname, senderAvatar string
	if item.FromId > 0 {
		user, err := s.UsersRepo.FindById(ctx, item.FromId)
		if err == nil && user != nil {
			senderNickname = user.Nickname
			senderAvatar = user.Avatar
		}
	}

	// 获取群组名称
	var groupName string
	group := &model.Group{}
	if err := s.Source.Db().First(group, "id = ?", item.GroupId).Error; err == nil {
		groupName = group.Name
	}

	// 确定要通知的用户列表
	var usersToNotify []int
	if atAll {
		// @所有人时，通知所有群成员（除了发送者）
		for _, uid := range memberIds {
			if uid != item.FromId {
				usersToNotify = append(usersToNotify, uid)
			}
		}
	} else {
		// 只通知被@的用户（除了发送者）
		for _, uid := range mentionedUserIds {
			if uid != item.FromId {
				usersToNotify = append(usersToNotify, uid)
			}
		}
	}

	if len(usersToNotify) == 0 {
		return
	}

	// 为每个被@的用户添加mention记录并推送通知
	for _, uid := range usersToNotify {
		// 添加到mention缓存
		if err := s.MentionStorage.AddMention(ctx, uid, item.GroupId, item.MsgId); err != nil {
			logger.Errorf("AddMention error for user %d: %s", uid, err.Error())
			continue
		}

		// 获取该用户的所有未读mention消息ID
		msgIds, err := s.MentionStorage.GetMentions(ctx, uid, item.GroupId)
		if err != nil {
			logger.Errorf("GetMentions error for user %d: %s", uid, err.Error())
			continue
		}

		// 推送mention通知
		err = s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventImMessageMention,
			Payload: jsonutil.Encode(entity.SubEventImMessageMentionPayload{
				UserId:    uid,
				GroupId:   item.GroupId,
				GroupName: groupName,
				FromId:    item.FromId,
				Nickname:  senderNickname,
				Avatar:    senderAvatar,
				MsgIds:    msgIds,
				Count:     len(msgIds),
				AtAll:     atAll,
			}),
		})
		if err != nil {
			logger.Errorf("Push mention notification error for user %d: %s", uid, err.Error())
		}
	}
}

func (s *Service) CreateGroupSysMessage(ctx context.Context, option CreateGroupSysMessageOption) error {
	return s.CreateGroupMessage(ctx, CreateGroupMessageOption{
		MsgType:    entity.ChatMsgSysText,
		FromId:     0, // 0:系统消息
		ReceiverId: option.GroupId,
		Extra: jsonutil.Encode(model.TalkRecordExtraText{
			Content: option.Content,
		}),
	})
}
