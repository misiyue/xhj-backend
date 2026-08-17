package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppNewsView struct {
	db *gorm.DB
}

func NewAppNewsView(db *gorm.DB) *AppNewsView {
	return &AppNewsView{db: db}
}

// RecordView 记录阅读：插入一条阅读记录；pv+1；若该用户对该资讯此前无记录则 uv+1
func (r *AppNewsView) RecordView(ctx context.Context, newsId, userId int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.AppNews{}).
			Where("id = ? AND status = ?", newsId, model.AppNewsStatusPublished).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return nil
		}

		var viewed int64
		if err := tx.Model(&model.AppNewsView{}).
			Where("news_id = ? AND user_id = ?", newsId, userId).
			Count(&viewed).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.AppNewsView{NewsId: newsId, UserId: userId}).Error; err != nil {
			return err
		}

		updates := map[string]any{
			"pv": gorm.Expr("IFNULL(pv,0) + 1"),
		}
		if viewed == 0 {
			updates["uv"] = gorm.Expr("IFNULL(uv,0) + 1")
		}

		return tx.Model(&model.AppNews{}).Where("id = ?", newsId).Updates(updates).Error
	})
}
