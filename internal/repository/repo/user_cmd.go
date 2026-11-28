package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type UserCmd struct {
	db *gorm.DB
}

func NewUserCmd(db *gorm.DB) *UserCmd {
	return &UserCmd{db: db}
}

func (r *UserCmd) Create(ctx context.Context, row *model.UserCmd) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *UserCmd) FindOwned(ctx context.Context, id, userId int) (*model.UserCmd, error) {
	var row model.UserCmd
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userId).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *UserCmd) UpdateByID(ctx context.Context, id int, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.UserCmd{}).Where("id = ?", id).Updates(updates).Error
}

func (r *UserCmd) DeleteOwned(ctx context.Context, id, userId int) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userId).
		Delete(&model.UserCmd{}).Error
}

func (r *UserCmd) ListByUserID(ctx context.Context, userId, page, pageSize int) ([]model.UserCmd, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.UserCmd{}).Where("user_id = ?", userId)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.UserCmd
	offset := (page - 1) * pageSize
	err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
