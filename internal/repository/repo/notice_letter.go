package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type NoticeLetter struct {
	db *gorm.DB
}

func NewNoticeLetter(db *gorm.DB) *NoticeLetter {
	return &NoticeLetter{db: db}
}

func (r *NoticeLetter) Create(ctx context.Context, row *model.NoticeLetter) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *NoticeLetter) ListByUserDesc(ctx context.Context, userId int, page, pageSize int) ([]model.NoticeLetter, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.NoticeLetter{}).Where("user_id = ?", userId)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.NoticeLetter
	err := r.db.WithContext(ctx).Where("user_id = ?", userId).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *NoticeLetter) CountUnread(ctx context.Context, userId int) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.NoticeLetter{}).
		Where("user_id = ? AND is_read = ?", userId, model.NoticeLetterUnread).
		Count(&n).Error
	return n, err
}

func (r *NoticeLetter) MarkAllRead(ctx context.Context, userId int) error {
	return r.db.WithContext(ctx).Model(&model.NoticeLetter{}).
		Where("user_id = ? AND is_read = ?", userId, model.NoticeLetterUnread).
		Update("is_read", model.NoticeLetterRead).Error
}

func (r *NoticeLetter) FindLatestByUser(ctx context.Context, userId int) (*model.NoticeLetter, error) {
	var row model.NoticeLetter
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
