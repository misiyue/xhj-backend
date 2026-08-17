package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppModule struct {
	db *gorm.DB
}

func NewAppModule(db *gorm.DB) *AppModule {
	return &AppModule{db: db}
}

// ListAll 返回全部功能模块，按 id 升序
func (r *AppModule) ListAll(ctx context.Context) ([]*model.AppModule, error) {
	var list []*model.AppModule
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
