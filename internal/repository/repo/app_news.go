package repo

import (
	"context"
	"strings"

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

// ListPublished 分页查询已发布资讯，可按 news_type、source 筛选；按 publish_time、id 倒序（不含 content/source_url）
func (r *AppNews) ListPublished(ctx context.Context, page, pageSize int, newsType, source string) (int64, []*model.AppNews, error) {
	newsType = strings.TrimSpace(newsType)
	source = strings.TrimSpace(source)
	return r.Repo.Pagination(ctx, page, pageSize, func(tx *gorm.DB) *gorm.DB {
		tx = tx.Select("id, title, collect_type, news_type, source, cover, upload_time, publish_time, status, created_at, updated_at").
			Where("status = ?", model.AppNewsStatusPublished)
		if newsType != "" {
			tx = tx.Where("news_type = ?", newsType)
		}
		if source != "" {
			tx = tx.Where("source = ?", source)
		}
		return tx.Order("publish_time DESC, id DESC")
	})
}

// FindPublishedById 按 id 查询已发布资讯详情
func (r *AppNews) FindPublishedById(ctx context.Context, id int) (*model.AppNews, error) {
	return r.Repo.FindByWhere(ctx, "id = ? AND status = ?", id, model.AppNewsStatusPublished)
}
