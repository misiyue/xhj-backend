package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppVersion struct {
	db *gorm.DB
}

func NewAppVersion(db *gorm.DB) *AppVersion {
	return &AppVersion{db: db}
}

// FindLatestPublished 取指定平台、发布时间已生效（published_at <= 当前时间）中 published_at 最新的一条
func (r *AppVersion) FindLatestPublished(ctx context.Context, platform string) (*model.AppVersion, error) {
	var row model.AppVersion
	err := r.db.WithContext(ctx).
		Where("platform = ? AND published_at <= ?", strings.ToLower(platform), time.Now()).
		Order("published_at DESC").
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
