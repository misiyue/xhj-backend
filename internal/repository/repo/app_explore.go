package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppExplore struct {
	db *gorm.DB
}

func NewAppExplore(db *gorm.DB) *AppExplore {
	return &AppExplore{db: db}
}

// ListOpen 返回已开放（is_open = 1）的探索位列表，按 sort 降序（越大越靠前），同 sort 再按 id 倒序
func (r *AppExplore) ListOpen(ctx context.Context) ([]*model.AppExplore, error) {
	var list []*model.AppExplore
	err := r.db.WithContext(ctx).
		Where("is_open = ?", model.AppExploreOpenYes).
		Order("`sort` DESC, id DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
