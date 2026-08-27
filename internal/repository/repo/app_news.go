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

// ListPublished 分页查询已发布资讯，可按 category_id 筛选；按 publish_time、id 倒序（不含 source_url）
func (r *AppNews) ListPublished(ctx context.Context, page, pageSize int, categoryId int) (int64, []*model.AppNews, error) {
	return r.Repo.Pagination(ctx, page, pageSize, func(tx *gorm.DB) *gorm.DB {
		tx = tx.Select("id, title, category_id, type_id, content, cover, upload_time, publish_time, status, pv, uv, created_at, updated_at").
			Where("status = ?", model.AppNewsStatusPublished)
		if categoryId > 0 {
			tx = tx.Where("category_id = ?", categoryId)
		}
		return tx.Order("publish_time DESC, id DESC")
	})
}

// FindPublishedById 按 id 查询已发布资讯详情
func (r *AppNews) FindPublishedById(ctx context.Context, id int) (*model.AppNews, error) {
	return r.Repo.FindByWhere(ctx, "id = ? AND status = ?", id, model.AppNewsStatusPublished)
}

// IncrPvByIds 批量增加已发布资讯 pv（不写 app_news_view）
func (r *AppNews) IncrPvByIds(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.Db.WithContext(ctx).Model(&model.AppNews{}).
		Where("id IN ? AND status = ?", ids, model.AppNewsStatusPublished).
		Update("pv", gorm.Expr("IFNULL(pv,0) + 1")).Error
}
