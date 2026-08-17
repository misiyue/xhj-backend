package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type NoticeArticle struct {
	db *gorm.DB
}

func NewNoticeArticle(db *gorm.DB) *NoticeArticle {
	return &NoticeArticle{db: db}
}

// FindEnabledById 查询已开启的通知文章
func (r *NoticeArticle) FindEnabledById(ctx context.Context, id int) (*model.NoticeArticle, error) {
	var row model.NoticeArticle
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, model.NoticeArticleStatusOn).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
