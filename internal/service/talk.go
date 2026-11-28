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
	ClearSessionRead(ctx context.Context, readerId, talkMode, receiverId int) error
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
			Where("msg_id in ? OR (org_msg_id in ? AND is_revoked = ?)", opt.MsgIds, opt.MsgIds, model.TalkMsgRevokedHidden).
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

	// 一并隐藏混合子段
	var segmentMsgIds []string
	db.Model(&model.TalkGroupMessage{}).
		Where("group_id = ? AND org_msg_id in ? AND is_revoked = ?", opt.ReceiverId, opt.MsgIds, model.TalkMsgRevokedHidden).
		Pluck("msg_id", &segmentMsgIds)

	allMsgIds := append(append([]string{}, opt.MsgIds...), segmentMsgIds...)
	items := make([]*model.TalkGroupMessageDel, 0, len(allMsgIds))
	for _, msgId := range allMsgIds {
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

// Revoke 撤回消息核心业务：校验 → 更新 is_revoked →（成功时）推 WebSocket；仅撤回「最新一条」时才改会话列表预览
func (t *TalkService) Revoke(ctx context.Context, opt *TalkRevokeOption) (err error) {
	db := t.Source.Db().WithContext(ctx)

	// 供 defer 使用：会话列表「最后一条」缓存的维度（发送者视角的 fromId / 对端或群 ID）
	var (
		fromId           int
		receiverId       int
		updateLastPreview bool // 仅当被撤消息是会话当前最新一条时更新外层预览
	)

	// 仅在整次 Revoke 未报错时执行：按需更新 MessageStorage + 发布 sub.im.message.revoke
	defer func() {
		if err == nil {
			remark := "有消息已被撤回"

			user, _ := t.UserRepo.FindByIdWithCache(ctx, fromId)
			if user != nil {
				remark = fmt.Sprintf("【%s】撤回了一条消息", user.Nickname)
			}

			// 撤回非最新消息时不改会话列表提示，避免覆盖真实最新消息摘要
			if updateLastPreview && t.MessageStorage != nil {
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
				logger.Infof("revoke redis push ok: talk_mode=%d msg_id=%s update_last=%v", opt.TalkMode, opt.MsgId, updateLastPreview)
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

		if record.IsRevoked == model.TalkMsgRevokedHidden {
			return errors.New("消息ID不存在")
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
		updateLastPreview = t.isLatestPrivateMessage(ctx, &record)

		// 父消息标记撤回；混合子段保持 is_revoked=0 并 is_deleted=1，避免进入会话列表
		if err := db.Model(&model.TalkUserMessage{}).
			Where("msg_id = ?", opt.MsgId).
			Update("is_revoked", model.Yes).Error; err != nil {
			return err
		}
		return db.Model(&model.TalkUserMessage{}).
			Where("org_msg_id = ? AND is_revoked = ?", opt.MsgId, model.TalkMsgRevokedHidden).
			Update("is_deleted", model.Yes).Error

	case entity.ChatGroupMode:
		var record model.TalkGroupMessage

		err := db.First(&record, "msg_id = ? and from_id = ?", opt.MsgId, opt.UserId).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("消息ID不存在")
			}

			return err
		}

		if record.IsRevoked == model.TalkMsgRevokedHidden {
			return errors.New("消息ID不存在")
		}
		if record.IsRevoked == model.Yes {
			return errors.New("消息已撤回")
		}
		if record.IsDeleted == model.Yes {
			return errors.New("消息已过期删除")
		}

		if time.Now().Unix() > record.SendTime.Add(3*time.Minute).Unix() {
			return errors.New("超出有效撤回时间范围，无法进行撤销！")
		}

		fromId = record.FromId
		receiverId = record.GroupId // 群聊时 receiverId 即 groupId，供 MessageStorage 维度使用
		updateLastPreview = t.isLatestGroupMessage(ctx, &record)

		if err := db.Model(&model.TalkGroupMessage{}).
			Where("msg_id = ?", record.MsgId).
			Update("is_revoked", model.Yes).Error; err != nil {
			return err
		}
		return db.Model(&model.TalkGroupMessage{}).
			Where("org_msg_id = ? AND is_revoked = ?", record.MsgId, model.TalkMsgRevokedHidden).
			Update("is_deleted", model.Yes).Error
	}

	return errors.New("暂不支持撤回消息")
}

// isLatestPrivateMessage 判断消息是否为该私聊会话当前最新一条（排除隐藏子段与已删除）
func (t *TalkService) isLatestPrivateMessage(ctx context.Context, record *model.TalkUserMessage) bool {
	if record == nil || record.MsgId == "" {
		return false
	}
	q := t.Source.Db().WithContext(ctx).Model(&model.TalkUserMessage{}).
		Where("is_deleted = ? AND is_revoked <> ?", model.No, model.TalkMsgRevokedHidden)
	if record.SessionId > 0 {
		q = q.Where("session_id = ?", record.SessionId)
	} else {
		// 无 session_id 时按双方用户维度兜底
		q = q.Where(
			"(from_id = ? AND receiver_id = ?) OR (from_id = ? AND receiver_id = ?)",
			record.FromId, record.ReceiverId, record.ReceiverId, record.FromId,
		)
	}
	var latestMsgId string
	if err := q.Select("msg_id").Order("id desc").Limit(1).Scan(&latestMsgId).Error; err != nil {
		return false
	}
	return latestMsgId == record.MsgId
}

// isLatestGroupMessage 判断消息是否为该群当前最新一条（排除隐藏子段与已删除）
func (t *TalkService) isLatestGroupMessage(ctx context.Context, record *model.TalkGroupMessage) bool {
	if record == nil || record.GroupId <= 0 || record.MsgId == "" {
		return false
	}
	var latestMsgId string
	err := t.Source.Db().WithContext(ctx).Model(&model.TalkGroupMessage{}).
		Where("group_id = ? AND is_deleted = ? AND is_revoked <> ?", record.GroupId, model.No, model.TalkMsgRevokedHidden).
		Select("msg_id").Order("id desc").Limit(1).Scan(&latestMsgId).Error
	if err != nil {
		return false
	}
	return latestMsgId == record.MsgId
}

// MarkPrivateMessagesRead 将指定私聊消息标记为已读，返回实际更新的 msg_id 列表。
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

// ClearSessionRead 清除会话未读：标记已读并推送 im.message.read（仅由 session-clear-unread-num 触发）。
func (t *TalkService) ClearSessionRead(ctx context.Context, readerId, talkMode, receiverId int) error {
	if readerId <= 0 || receiverId <= 0 {
		return nil
	}
	switch talkMode {
	case entity.ChatPrivateMode:
		return t.clearPrivateSessionRead(ctx, readerId, receiverId)
	case entity.ChatGroupMode:
		return t.clearGroupSessionRead(ctx, readerId, receiverId)
	default:
		return nil
	}
}

func (t *TalkService) clearPrivateSessionRead(ctx context.Context, readerId, peerId int) error {
	if t.Source == nil {
		return errors.New("talk service source is nil")
	}
	var msgIds []string
	if err := t.Source.Db().WithContext(ctx).Model(&model.TalkUserMessage{}).
		Where("from_id = ? and receiver_id = ? and is_read = ?", peerId, readerId, model.TalkUserMessageIsReadNo).
		Pluck("msg_id", &msgIds).Error; err != nil {
		return err
	}
	if len(msgIds) == 0 {
		return nil
	}
	readMsgIds, err := t.MarkPrivateMessagesRead(ctx, readerId, peerId, msgIds)
	if err != nil || len(readMsgIds) == 0 {
		return err
	}
	return t.pushMessageReadEvent(ctx, readerId, peerId, entity.ChatPrivateMode, readMsgIds)
}

func (t *TalkService) clearGroupSessionRead(ctx context.Context, readerId, groupId int) error {
	if t.Source == nil {
		return errors.New("talk service source is nil")
	}
	if t.TalkGroupMsgReaderRepo == nil {
		return errors.New("TalkGroupMsgReaderRepo is nil")
	}

	var candidates []string
	if err := t.Source.Db().WithContext(ctx).Model(&model.TalkGroupMessage{}).
		Where("group_id = ? and from_id != ? and is_revoked != ?", groupId, readerId, model.Yes).
		Pluck("msg_id", &candidates).Error; err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	unread, err := t.TalkGroupMsgReaderRepo.FilterUnreadMsgIDs(ctx, readerId, candidates)
	if err != nil {
		return err
	}
	if len(unread) == 0 {
		return nil
	}

	affected, err := t.TalkGroupMsgReaderRepo.BatchInsert(ctx, readerId, unread)
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	return t.pushMessageReadEvent(ctx, readerId, groupId, entity.ChatGroupMode, unread)
}

func (t *TalkService) pushMessageReadEvent(ctx context.Context, readerId, receiverId, talkMode int, msgIds []string) error {
	if len(msgIds) == 0 {
		return nil
	}
	if t.PushMessage == nil {
		logger.Warnf("message read push skipped: PushMessage is nil, reader_id=%d receiver_id=%d", readerId, receiverId)
		return nil
	}
	return t.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventImMessageRead,
		Payload: jsonutil.Encode(entity.SubEventImMessageReadPayload{
			TalkMode:   talkMode,
			FromId:     readerId,
			ReceiverId: receiverId,
			MsgIds:     msgIds,
		}),
	})
}
