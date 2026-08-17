package repo

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppNewsCategory struct {
	db *gorm.DB
}

func NewAppNewsCategory(db *gorm.DB) *AppNewsCategory {
	return &AppNewsCategory{db: db}
}

// ListVisibleByCollect 返回已显示（status=1）分类，按 sort 倒序；collect 非空时筛选
func (r *AppNewsCategory) ListVisibleByCollect(ctx context.Context, collect string) ([]*model.AppNewsCategory, error) {
	collect = strings.TrimSpace(collect)
	q := r.db.WithContext(ctx).
		Where("status = ?", model.AppNewsCategoryStatusShow)
	if collect != "" {
		q = q.Where("collect = ?", collect)
	}
	var list []*model.AppNewsCategory
	err := q.Order("`sort` DESC, id DESC").Find(&list).Error
	return list, err
}
