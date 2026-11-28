package repo

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type UserReport struct {
	db *gorm.DB
}

func NewUserReport(db *gorm.DB) *UserReport {
	return &UserReport{db: db}
}

func (r *UserReport) Create(ctx context.Context, row *model.UserReport) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// ExistsRecent 判断 uid 在 within 内是否已对同一 defe_id+defe_type 举报过
func (r *UserReport) ExistsRecent(ctx context.Context, uid, defeId, defeType int, within time.Duration) (bool, error) {
	var count int64
	since := time.Now().Add(-within)
	err := r.db.WithContext(ctx).Model(&model.UserReport{}).
		Where("user_id = ? AND defe_id = ? AND defe_type = ? AND created_at >= ?", uid, defeId, defeType, since).
		Count(&count).Error
	return count > 0, err
}
