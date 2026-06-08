package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type SysNotice struct {
	db *gorm.DB
}

func NewSysNotice(db *gorm.DB) *SysNotice {
	return &SysNotice{db: db}
}

// Create 创建系统通知
func (r *SysNotice) Create(ctx context.Context, row *model.SysNotice) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// ListByUserDesc 分页查询用户系统通知，按创建时间倒序
func (r *SysNotice) ListByUserDesc(ctx context.Context, userId int, page, pageSize int) ([]model.SysNotice, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.SysNotice{}).Where("user_id = ?", userId)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.SysNotice
	err := r.db.WithContext(ctx).Where("user_id = ?", userId).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CountUnread 统计用户未读通知数
func (r *SysNotice) CountUnread(ctx context.Context, userId int) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.SysNotice{}).
		Where("user_id = ? AND is_read = ?", userId, model.SysNoticeUnread).
		Count(&n).Error
	return n, err
}

// MarkAllRead 将用户全部未读通知标记为已读
func (r *SysNotice) MarkAllRead(ctx context.Context, userId int) error {
	return r.db.WithContext(ctx).Model(&model.SysNotice{}).
		Where("user_id = ? AND is_read = ?", userId, model.SysNoticeUnread).
		Update("is_read", model.SysNoticeRead).Error
}

// FindLatestByUser 查询用户最新一条通知
func (r *SysNotice) FindLatestByUser(ctx context.Context, userId int) (*model.SysNotice, error) {
	var row model.SysNotice
	err := r.db.WithContext(ctx).Where("user_id = ?", userId).
		Order("created_at DESC, id DESC").
		Limit(1).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
