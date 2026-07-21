package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

var _ ITalkService = (*TalkService)(nil)

// TalkRevokeOption 撤回消息入参（由 HTTP Handler 从 MessageRevokeRequest + JWT 组装）
type TalkRevokeOption struct {
	UserId   int    // 当前操作人，须与库表里该 msg 的 from_id 一致
	TalkMode int    // 1 私聊 2 群聊
	MsgId    string // 被撤回消息的 msg_id
}

type TalkDeleteRecordOption struct {
	UserId     int
	TalkMode   int
	ReceiverId int
	MsgIds     []string
}

type ITalkService interface {
	DeleteRecord(ctx context.Context, opt *TalkDeleteRecordOption) error
	Revoke(ctx context.Context, opt *TalkRevokeOption) error
	MarkPrivateMessagesRead(ctx context.Context, readerId, peerId int, msgIds []string) ([]string, error)
	NotifyPrivateMessagesRead(ctx context.Context, readerId, senderId int, msgIds []string) ([]string, error)
	MarkGroupMessagesRead(ctx context.Context, readerId, groupId int, records []*model.TalkMessageRecord) error
}

type TalkService struct {
	*repo.Source
	GroupMemberRepo        *repo.GroupMember
	UserRepo               *repo.Users
	TalkGroupMsgReaderRepo *repo.TalkGroupMsgReader
	PushMessage            *logic.PushMessage
	MessageStorage         *cache.MessageStorage
}

// DeleteRecord 删除消息记录
func (t *TalkService) DeleteRecord(ctx context.Context, opt *TalkDeleteRecordOption) error {
	var db = t.Source.Db().WithContext(ctx)

	// 私有消息直接更新删除状态（不再需要 user_id，因为消息只存储一次）
	if opt.TalkMode == entity.ChatPrivateMode {
		return db.Model(model.TalkUserMessage{}).
			Where("msg_id in ?", opt.MsgIds).
			Update("is_deleted", model.Yes).Error
	}

	if !t.GroupMemberRepo.IsMember(ctx, opt.ReceiverId, opt.UserId, false) {
		return entity.ErrPermissionDenied
	}

	var findMsgIds []string
	db.Model(&model.TalkGroupMessage{}).
		Where("group_id = ? and msg_id in ?", opt.ReceiverId, opt.MsgIds).
		Pluck("msg_id", &findMsgIds)

	if len(opt.MsgIds) != len(findMsgIds) {
		return errors.New("删除异常! ")
	}

	items := make([]*model.TalkGroupMessageDel, 0, len(opt.MsgIds))
	for _, msgId := range opt.MsgIds {
		items = append(items, &model.TalkGroupMessageDel{
			MsgId:     msgId,
			GroupId:   opt.ReceiverId,
			UserId:    opt.UserId,
			CreatedAt: time.Now(),
		})
	}

	// 删除后清除最后一条记录
	return db.Create(items).Error
}

// Revoke 撤回消息核心业务：校验 → 更新 is_revoked →（成功时）更新会话最后一条文案 + Redis 通知 Comet 推 WebSocket
func (t *TalkService) Revoke(ctx context.Context, opt *TalkRevokeOption) (err error) {
	db := t.Source.Db().WithContext(ctx)

	// 供 defer 使用：会话列表「最后一条」缓存的维度（发送者视角的 fromId / 对端或群 ID）
	var (
		fromId     int
		receiverId int
	)

	// 仅在整次 Revoke 未报错时执行：更新 MessageStorage + 发布 sub.im.message.revoke
	defer func() {
		if err == nil {
			remark := "有消息已被撤回"

			// 用发送者昵称拼展示文案；同时给该会话写最后一条预览（会话列表用）
			user, _ := t.UserRepo.FindByIdWithCache(ctx, fromId)
			if user != nil {
				remark = fmt.Sprintf("【%s】撤回了一条消息", user.Nickname)

				_ = t.MessageStorage.Set(ctx, opt.TalkMode, fromId, receiverId, &cache.LastCacheMessage{
					Content:  remark,
					Datetime: time.Now().Format(time.DateTime),
				})
			}

			// Comet 订阅 ImTopicChat 后根据 Event 分发到 onConsumeTalkRevoke，再写 im.message.revoke 到各用户 WebSocket
			// 若 WebSocket 收不到撤回：请确认已单独启动 Comet 进程（lumenim comet），且与 API 使用同一 Redis
			e := t.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
				Event: entity.SubEventImMessageRevoke,
				Payload: jsonutil.Encode(entity.SubEventTalkRevokePayload{
					TalkMode: opt.TalkMode,
					MsgId:    opt.MsgId,
					Remark:   remark,
				}),
			})

			if e != nil {
				logger.Errorf("revoke push message error:%s", e.Error())
			} else {
				logger.Infof("revoke redis push ok: talk_mode=%d msg_id=%s", opt.TalkMode, opt.MsgId)
			}
		}
	}()

	switch opt.TalkMode {
	case entity.ChatPrivateMode:
		var record model.TalkUserMessage

		// 必须 msg_id + from_id 同时命中，防止撤回别人的消息
		err := db.First(&record, "msg_id = ? and from_id = ?", opt.MsgId, opt.UserId).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("消息ID不存在")
			}

			return err
		}

		if record.IsRevoked == model.Yes {
			return errors.New("消息已撤回")
		}

		// 业务规则：发送后 3 分钟内可撤
		if time.Now().Unix() > record.SendTime.Add(3*time.Minute).Unix() {
			return errors.New("超出有效撤回时间范围，无法进行撤销！")
		}

		fromId = record.FromId
		receiverId = record.ReceiverId

		// 私聊表仅一条物理消息，按 msg_id 打 is_revoked
		return db.Model(&model.TalkUserMessage{}).
			Where("msg_id = ?", opt.MsgId).
			Update("is_revoked", model.Yes).Error

	case entity.ChatGroupMode:
		var record model.TalkGroupMessage

		err := db.First(&record, "msg_id = ? and from_id = ?", opt.MsgId, opt.UserId).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("消息ID不存在")
			}

			return err
		}

		if record.IsRevoked == model.Yes {
			return errors.New("消息已撤回")
		}

		if time.Now().Unix() > record.SendTime.Add(3*time.Minute).Unix() {
			return errors.New("超出有效撤回时间范围，无法进行撤销！")
		}

		fromId = record.FromId
		receiverId = record.GroupId // 群聊时 receiverId 即 groupId，供 MessageStorage 维度使用

		return db.Model(&model.TalkGroupMessage{}).
			Where("msg_id = ?", record.MsgId).
			Update("is_revoked", model.Yes).Error
	}

	return errors.New("暂不支持撤回消息")
}

// MarkPrivateMessagesRead 接收方拉取私聊消息后，将对方发来的未读消息标记为已读。
// 返回本次实际标记为已读的 msg_id 列表（由调用方推送 im.message.read）。
func (t *TalkService) MarkPrivateMessagesRead(ctx context.Context, readerId, peerId int, msgIds []string) ([]string, error) {
	if readerId <= 0 || peerId <= 0 || len(msgIds) == 0 {
		return nil, nil
	}
	if t.Source == nil {
		return nil, errors.New("talk service source is nil")
	}

	db := t.Source.Db().WithContext(ctx)

	result := db.Model(&model.TalkUserMessage{}).
		Where(
			"msg_id in ? and from_id = ? and receiver_id = ? and is_read = ?",
			msgIds, peerId, readerId, model.TalkUserMessageIsReadNo,
		).
		Update("is_read", model.TalkUserMessageIsReadYes)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	var readMsgIds []string
	if err := db.Model(&model.TalkUserMessage{}).
		Where(
			"msg_id in ? and from_id = ? and receiver_id = ? and is_read = ?",
			msgIds, peerId, readerId, model.TalkUserMessageIsReadYes,
		).
		Pluck("msg_id", &readMsgIds).Error; err != nil {
		return nil, err
	}
	return readMsgIds, nil
}

// NotifyPrivateMessagesRead 标记私聊已读并向发送方推送 im.message.read。
func (t *TalkService) NotifyPrivateMessagesRead(ctx context.Context, readerId, senderId int, msgIds []string) ([]string, error) {
	readMsgIds, err := t.MarkPrivateMessagesRead(ctx, readerId, senderId, msgIds)
	if err != nil || len(readMsgIds) == 0 {
		return readMsgIds, err
	}
	if err := t.pushPrivateMessagesRead(ctx, senderId, readerId, readMsgIds); err != nil {
		logger.Errorf("notify private messages read push err: sender_id=%d reader_id=%d %s", senderId, readerId, err.Error())
	}
	return readMsgIds, nil
}

func (t *TalkService) pushPrivateMessagesRead(ctx context.Context, senderId, readerId int, msgIds []string) error {
	if len(msgIds) == 0 {
		return nil
	}
	if t.PushMessage == nil {
		logger.Warnf("private message read push skipped: PushMessage is nil, sender_id=%d reader_id=%d", senderId, readerId)
		return nil
	}
	return t.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessageRead,
		Payload: jsonutil.Encode(entity.SubEventImMessageReadPayload{
			TalkMode:   entity.ChatPrivateMode,
			FromId:     senderId,
			ReceiverId: readerId,
			MsgIds:     msgIds,
		}),
	})
}

// MarkGroupMessagesRead 群成员拉取历史消息后，将非本人发送且未撤回、未读的消息写入已读表并通知在线群成员。
func (t *TalkService) MarkGroupMessagesRead(ctx context.Context, readerId, groupId int, records []*model.TalkMessageRecord) error {
	if readerId <= 0 || groupId <= 0 || len(records) == 0 || t.TalkGroupMsgReaderRepo == nil {
		return nil
	}

	msgIds := make([]string, 0, len(records))
	for _, rec := range records {
		if rec != nil && rec.MsgId != "" {
			msgIds = append(msgIds, rec.MsgId)
		}
	}
	if len(msgIds) == 0 {
		return nil
	}

	readerMap, err := t.TalkGroupMsgReaderRepo.MapReaderUserIDsByMsgIDs(ctx, msgIds)
	if err != nil {
		return err
	}

	toInsert := make([]string, 0)
	for _, rec := range records {
		if rec == nil || rec.MsgId == "" {
			continue
		}
		if rec.FromId == readerId || rec.IsRevoked == model.Yes {
			continue
		}
		readers := readerMap[rec.MsgId]
		found := false
		for _, id := range readers {
			if id == readerId {
				found = true
				break
			}
		}
		if !found {
			toInsert = append(toInsert, rec.MsgId)
		}
	}
	if len(toInsert) == 0 {
		return nil
	}

	affected, err := t.TalkGroupMsgReaderRepo.BatchInsert(ctx, readerId, toInsert)
	if err != nil {
		return err
	}
	if affected == 0 || t.PushMessage == nil {
		return nil
	}

	if err := t.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessageRead,
		Payload: jsonutil.Encode(entity.SubEventImMessageReadPayload{
			TalkMode:   entity.ChatGroupMode,
			FromId:     readerId,
			ReceiverId: groupId,
			MsgIds:     toInsert,
		}),
	}); err != nil {
		logger.Errorf("mark group messages read push error: reader_id=%d group_id=%d %s", readerId, groupId, err.Error())
	}
	return nil
}
