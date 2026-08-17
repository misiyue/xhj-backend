package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type YunxinCredential struct {
	core.Repo[model.YunxinCredential]
}

func NewYunxinCredential(db *gorm.DB) *YunxinCredential {
	return &YunxinCredential{Repo: core.NewRepo[model.YunxinCredential](db)}
}

func (r *YunxinCredential) FindByUserId(ctx context.Context, userID int) (*model.YunxinCredential, error) {
	return r.Repo.FindByWhere(ctx, "user_id = ?", userID)
}
