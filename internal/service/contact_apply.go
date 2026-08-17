package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service/message"
	"gorm.io/gorm"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
)

var _ IContactApplyService = (*ContactApplyService)(nil)

type IContactApplyService interface {
	Create(ctx context.Context, opt *ContactApplyCreateOpt) error
	Accept(ctx context.Context, opt *ContactApplyAcceptOpt) (*model.ContactApply, error)
	Decline(ctx context.Context, opt *ContactApplyDeclineOpt) error
	List(ctx context.Context, uid int) ([]*model.ApplyItem, error)
	GetApplyUnreadNum(ctx context.Context, uid int) int
	ClearApplyUnreadNum(ctx context.Context, uid int)
}

type ContactApplyService struct {
	*repo.Source
	TalkSessionRepo    *repo.TalkSession
	PushMessage        *logic.PushMessage
	UsersRepo          *repo.Users
	UserClient         *cache.UserClient
	NoticeTemplateRepo *repo.NoticeTemplate
}

type ContactApplyCreateOpt struct {
	UserId      int
	Remarks     string
	FriendId    int
	ApplyReason string
}

func (s *ContactApplyService) Create(ctx context.Context, opt *ContactApplyCreateOpt) error {

	apply := &model.ContactApply{
		UserId:      opt.UserId,
		FriendId:    opt.FriendId,
		Remark:      opt.Remarks,
		ApplyReason: opt.ApplyReason,
	}

	if err := s.Source.Db().WithContext(ctx).Create(apply).Error; err != nil {
		return err
	}

	_ = s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventContactApply,
		Payload: jsonutil.Encode(entity.SubEventContactApplyPayload{
			ApplyId: apply.Id,
			Type:    1,
		}),
	})

	s.Source.Redis().Incr(ctx, fmt.Sprintf("im:contact:apply:%d", opt.FriendId))
	s.tryOneSignalContactApply(ctx, opt.FriendId, opt.UserId)
	return nil
}

func (s *ContactApplyService) tryOneSignalContactApply(ctx context.Context, receiverID, senderID int) {
	if receiverID <= 0 || senderID <= 0 {
		return
	}
	senderName := "用户"
	if s.UsersRepo != nil {
		sender, err := s.UsersRepo.FindByIdWithCache(ctx, senderID)
		if err != nil {
			logger.Errorf("contact_apply sender load err: sender_id=%d %s", senderID, err.Error())
		} else if sender != nil && strings.TrimSpace(sender.Nickname) != "" {
			senderName = strings.TrimSpace(sender.Nickname)
		}
	}
	message.TryOneSignalTemplatePush(
		ctx,
		s.UsersRepo,
		s.NoticeTemplateRepo,
		nil,
		receiverID,
		0,
		0,
		model.NoticeTemplateFlagContactApply,
		map[string]string{"sender": senderName},
	)
}

type ContactApplyAcceptOpt struct {
	UserId  int
	Remarks string
	ApplyId int
}

// Accept 同意好友申请
func (s *ContactApplyService) Accept(ctx context.Context, opt *ContactApplyAcceptOpt) (*model.ContactApply, error) {

	db := s.Source.Db().WithContext(ctx)

	var applyInfo model.ContactApply
	if err := db.First(&applyInfo, "id = ? and friend_id = ?", opt.ApplyId, opt.UserId).Error; err != nil {
		return nil, err
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		addFriendFunc := func(uid, fid int, remark string) error {
			var contact model.Contact
			err := tx.Where("user_id = ? and friend_id = ?", uid, fid).First(&contact).Error

			// 数据存在则更新
			if err == nil {
				return tx.Model(&model.Contact{}).Where("id = ?", contact.Id).Updates(&model.Contact{
					Remark: remark,
					Status: 1,
				}).Error
			}

			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			return tx.Create(&model.Contact{
				UserId:    uid,
				FriendId:  fid,
				Remark:    remark,
				Status:    1,
				GroupId:   0,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).Error
		}

		var user model.Users
		if err := tx.Select("id", "nickname").First(&user, applyInfo.FriendId).Error; err != nil {
			return err
		}

		if err := addFriendFunc(applyInfo.UserId, applyInfo.FriendId, user.Nickname); err != nil {
			return err
		}

		if err := addFriendFunc(applyInfo.FriendId, applyInfo.UserId, opt.Remarks); err != nil {
			return err
		}

		// 创建双向talk_session记录
		now := time.Now()

		// 为申请人创建talk_session
		session1 := &model.TalkSession{
			TalkMode:   entity.ChatPrivateMode,
			UserId:     applyInfo.UserId,
			ReceiverId: applyInfo.FriendId,
			IsTop:      model.No,
			IsDelete:   model.No,
			IsDisturb:  model.No,
			IsRobot:    model.No,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		// 如果已存在则忽略
		if err := tx.Create(session1).Error; err != nil {
			// 如果记录已存在，则获取现有记录
			tx.Where("user_id = ? and receiver_id = ? and talk_mode = ?", session1.UserId, session1.ReceiverId, session1.TalkMode).First(&session1)
		}

		// 为被申请人创建talk_session
		session2 := &model.TalkSession{
			TalkMode:   entity.ChatPrivateMode,
			UserId:     applyInfo.FriendId,
			ReceiverId: applyInfo.UserId,
			IsTop:      model.No,
			IsDelete:   model.No,
			IsDisturb:  model.No,
			IsRobot:    model.No,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		// 如果已存在则忽略
		if err := tx.Create(session2).Error; err != nil {
			// 如果记录已存在，则获取现有记录
			tx.Where("user_id = ? and receiver_id = ? and talk_mode = ?", session2.UserId, session2.ReceiverId, session2.TalkMode).First(&session2)
		}

		// 设置session_id，使两条记录关联
		if session1.Id > 0 && session2.Id > 0 {
			sessionId := session1.Id
			if err := tx.Model(&model.TalkSession{}).Where("id = ?", session2.Id).Update("session_id", sessionId).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.TalkSession{}).Where("id = ?", session1.Id).Update("session_id", sessionId).Error; err != nil {
				return err
			}
		}

		// 删除申请记录
		return tx.Delete(&model.ContactApply{}, "user_id = ? and friend_id = ?", applyInfo.UserId, applyInfo.FriendId).Error
	})

	if err != nil {
		return &applyInfo, err
	}

	// 通知申请人：好友申请已被接受
	var user model.Users
	if userErr := s.Source.Db().WithContext(ctx).Select("id", "nickname").First(&user, opt.UserId).Error; userErr == nil {
		_ = s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventContactApplyResult,
			Payload: jsonutil.Encode(entity.SubEventContactApplyResultPayload{
				ApplierId: applyInfo.UserId, // 申请人ID (接收通知的人)
				UserId:    opt.UserId,       // 接受申请的人的ID
				Nickname:  user.Nickname,
				Result:    1, // 1: 同意
			}),
		})
	}

	return &applyInfo, err
}

type ContactApplyDeclineOpt struct {
	UserId  int
	Remarks string
	ApplyId int
}

// Decline 拒绝好友申请
func (s *ContactApplyService) Decline(ctx context.Context, opt *ContactApplyDeclineOpt) error {
	err := s.Source.Db().WithContext(ctx).Delete(&model.ContactApply{}, "id = ? and friend_id = ?", opt.ApplyId, opt.UserId).Error
	if err != nil {
		return err
	}

	_ = s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventContactApply,
		Payload: jsonutil.Encode(entity.SubEventContactApplyPayload{
			ApplyId: opt.ApplyId,
			Type:    2,
		}),
	})
	return nil
}

// List 联系人申请列表
func (s *ContactApplyService) List(ctx context.Context, uid int) ([]*model.ApplyItem, error) {
	fields := []string{
		"contact_apply.id",
		"contact_apply.remark",
		"contact_apply.apply_reason",
		"users.nickname",
		"users.avatar",
		"users.mobile",
		"contact_apply.user_id",
		"contact_apply.friend_id",
		"contact_apply.created_at",
	}

	// 子查询：每个 user_id 只取最大 id（最新一条）
	dedup := s.Source.Db().WithContext(ctx).
		Model(&model.ContactApply{}).
		Select("user_id, MAX(id) AS max_id").
		Where("friend_id = ?", uid).
		Group("user_id")

	tx := s.Source.Db().WithContext(ctx).
		Table("(?) AS ca_dedup", dedup).
		Joins("INNER JOIN contact_apply ON contact_apply.id = ca_dedup.max_id").
		Joins("LEFT JOIN `users` ON `users`.id = contact_apply.user_id")

	var items []*model.ApplyItem
	if err := tx.Select(fields).Order("contact_apply.id DESC").Scan(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *ContactApplyService) GetApplyUnreadNum(ctx context.Context, uid int) int {

	num, err := s.Source.Redis().Get(ctx, fmt.Sprintf("im:contact:apply:%d", uid)).Int()
	if err != nil {
		return 0
	}

	return num
}

func (s *ContactApplyService) ClearApplyUnreadNum(ctx context.Context, uid int) {
	s.Source.Redis().Del(ctx, fmt.Sprintf("im:contact:apply:%d", uid))
}
