package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppNews struct {
	core.Repo[model.AppNews]
}

func NewAppNews(db *gorm.DB) *AppNews {
	return &AppNews{Repo: core.NewRepo[model.AppNews](db)}
}

// ListPublished 分页查询已发布资讯，可按 category_id、is_index 筛选；按 publish_time、id 倒序（不含 content/source_url）
func (r *AppNews) ListPublished(ctx context.Context, page, pageSize int, categoryId int, isIndex *int) (int64, []*model.AppNews, error) {
	return r.Repo.Pagination(ctx, page, pageSize, func(tx *gorm.DB) *gorm.DB {
		tx = tx.Select("id, title, category_id, type_id, cover, upload_time, publish_time, status, is_index, created_at, updated_at").
			Where("status = ?", model.AppNewsStatusPublished)
		if categoryId > 0 {
			tx = tx.Where("category_id = ?", categoryId)
		}
		if isIndex != nil {
			tx = tx.Where("is_index = ?", *isIndex)
		}
		return tx.Order("publish_time DESC, id DESC")
	})
}

// FindPublishedById 按 id 查询已发布资讯详情
func (r *AppNews) FindPublishedById(ctx context.Context, id int) (*model.AppNews, error) {
	return r.Repo.FindByWhere(ctx, "id = ? AND status = ?", id, model.AppNewsStatusPublished)
}
