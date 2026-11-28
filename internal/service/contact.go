package service

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

var _ IContactService = (*ContactService)(nil)

type IContactService interface {
	UpdateRemark(ctx context.Context, uid int, friendId int, remark string) error
	Delete(ctx context.Context, uid, friendId int) error
	List(ctx context.Context, uid int) ([]*model.ContactListItem, error)
	GetContactIds(ctx context.Context, uid int) []int64
	MoveGroup(ctx context.Context, uid int, friendId int, groupId int) error
	Block(ctx context.Context, uid, friendId int) error
	Unblock(ctx context.Context, uid, friendId int) error
	BlockList(ctx context.Context, uid, page, pageSize int) ([]*model.ContactBlockListItem, int64, error)
}

type ContactService struct {
	*repo.Source
	ContactRepo *repo.Contact
}

// UpdateRemark 编辑联系人备注
// @params uid      用户ID
// @params friendId 联系人ID
func (s *ContactService) UpdateRemark(ctx context.Context, uid int, friendId int, remark string) error {

	_, err := s.ContactRepo.UpdateByWhere(ctx, map[string]any{"remark": remark}, "user_id = ? and friend_id = ?", uid, friendId)
	if err == nil {
		_ = s.ContactRepo.SetFriendRemark(ctx, uid, friendId, remark)
	}

	return err
}

// Delete 删除联系人
// @params uid      用户ID
// @params friendId 联系人ID
func (s *ContactService) Delete(ctx context.Context, uid, friendId int) error {

	find, err := s.ContactRepo.FindByWhere(ctx, "user_id = ? and friend_id = ?", uid, friendId)
	if err != nil {
		return err
	}

	return s.Source.Db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if find.GroupId > 0 {
			err := tx.Model(&model.ContactGroup{}).
				Where("id = ? and user_id = ?", find.GroupId, uid).
				Updates(map[string]any{"num": gorm.Expr("num - 1")}).Error

			if err != nil {
				return err
			}
		}
		s.ContactRepo.Relation.DelContactRelation(ctx, uid, friendId)
		return tx.Model(&model.Contact{}).Where("user_id = ? and friend_id = ?", uid, friendId).
			Update("status", model.ContactStatusDelete).Error
	})
}

// List 获取联系人列表
// @params uid      用户ID
func (s *ContactService) List(ctx context.Context, uid int) ([]*model.ContactListItem, error) {

	tx := s.ContactRepo.Model(ctx)
	tx.Select([]string{
		"users.id",
		"users.nickname",
		"users.avatar",
		"users.motto",
		"users.gender",
		"contact.remark",
		"contact.group_id",
	})
	tx.Joins("inner join `users` ON `users`.id = contact.friend_id")
	tx.Where("contact.user_id = ? and contact.status = ?", uid, model.ContactStatusNormal)

	var items []*model.ContactListItem
	if err := tx.Scan(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (s *ContactService) GetContactIds(ctx context.Context, uid int) []int64 {

	var ids []int64
	s.ContactRepo.Model(ctx).Where("user_id = ? and status = ?", uid, model.ContactStatusNormal).Pluck("friend_id", &ids)

	return ids
}

// Block 拉黑好友（单向）；对方仍可发消息，但本方不接收
func (s *ContactService) Block(ctx context.Context, uid, friendId int) error {
	if uid <= 0 || friendId <= 0 || uid == friendId {
		return errors.New("参数错误")
	}
	contact, err := s.ContactRepo.FindByWhere(ctx, "user_id = ? and friend_id = ? and status = ?", uid, friendId, model.ContactStatusNormal)
	if err != nil {
		return err
	}
	if contact == nil || contact.Id == 0 {
		return errors.New("好友关系不存在")
	}
	_, err = s.ContactRepo.UpdateByWhere(ctx, map[string]any{
		"is_black":   model.ContactIsBlackYes,
		"updated_at": gorm.Expr("NOW()"),
	}, "id = ?", contact.Id)
	return err
}

// Unblock 解除拉黑
func (s *ContactService) Unblock(ctx context.Context, uid, friendId int) error {
	if uid <= 0 || friendId <= 0 || uid == friendId {
		return errors.New("参数错误")
	}
	_, err := s.ContactRepo.UpdateByWhere(ctx, map[string]any{
		"is_black":   model.ContactIsBlackNo,
		"updated_at": gorm.Expr("NOW()"),
	}, "user_id = ? and friend_id = ? and status = ?", uid, friendId, model.ContactStatusNormal)
	return err
}

// BlockList 黑名单分页列表（is_black=1）
func (s *ContactService) BlockList(ctx context.Context, uid, page, pageSize int) ([]*model.ContactBlockListItem, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := "contact.user_id = ? AND contact.status = ? AND contact.is_black = ?"
	args := []any{uid, model.ContactStatusNormal, model.ContactIsBlackYes}

	var total int64
	if err := s.ContactRepo.Model(ctx).Where(where, args...).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.ContactBlockListItem
	err := s.ContactRepo.Model(ctx).
		Select([]string{"users.id", "users.nickname", "users.avatar", "contact.remark"}).
		Joins("inner join `users` ON `users`.id = contact.friend_id").
		Where(where, args...).
		Order("contact.updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *ContactService) MoveGroup(ctx context.Context, uid int, friendId int, groupId int) error {
	contact, err := s.ContactRepo.FindByWhere(ctx, "user_id = ? and friend_id  = ?", uid, friendId)
	if err != nil {
		return err
	}

	return s.Source.Db().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if contact.GroupId > 0 {
			err := tx.Model(&model.ContactGroup{}).Where("id = ? and user_id = ?", contact.GroupId, uid).Updates(map[string]any{
				"num": gorm.Expr("num - 1"),
			}).Error

			if err != nil {
				return err
			}
		}

		err := tx.Model(&model.Contact{}).Where("user_id = ? and friend_id = ? and group_id = ?", uid, friendId, contact.GroupId).UpdateColumn("group_id", groupId).Error
		if err != nil {
			return err
		}

		return tx.Model(&model.ContactGroup{}).Where("id = ? and user_id = ?", groupId, uid).Updates(map[string]any{
			"num": gorm.Expr("num + 1"),
		}).Error
	})
}
