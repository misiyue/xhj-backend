package repo

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppModule struct {
	db *gorm.DB
}

func NewAppModule(db *gorm.DB) *AppModule {
	return &AppModule{db: db}
}

// List 返回功能模块列表，按 id 升序；end 非空时筛选 ends 字段（逗号分隔，如 h5,pc,app）
func (r *AppModule) List(ctx context.Context, end string) ([]*model.AppModule, error) {
	q := r.db.WithContext(ctx)
	end = strings.TrimSpace(end)
	if end != "" {
		q = q.Where("FIND_IN_SET(?, ends)", end)
	}

	var list []*model.AppModule
	err := q.Order("id ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
