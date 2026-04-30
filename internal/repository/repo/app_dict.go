package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppDict struct {
	db *gorm.DB
}

func NewAppDict(db *gorm.DB) *AppDict {
	return &AppDict{db: db}
}

// ListEnabledByKeys 按 key 批量查询且仅 status=1（启用）
func (r *AppDict) ListEnabledByKeys(ctx context.Context, keys []string) ([]model.AppDict, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	var rows []model.AppDict
	err := r.db.WithContext(ctx).Where("`key` IN ? AND status = ?", keys, 1).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
