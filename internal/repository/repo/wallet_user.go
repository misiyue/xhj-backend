package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type WalletUser struct {
	core.Repo[model.WalletUser]
}

func NewWalletUser(db *gorm.DB) *WalletUser {
	return &WalletUser{
		Repo: core.NewRepo[model.WalletUser](db),
	}
}

// FindByUserId 根据IM用户ID查询钱包用户
func (w *WalletUser) FindByUserId(ctx context.Context, userId int) (*model.WalletUser, error) {
	return w.Repo.FindByWhere(ctx, "user_id = ?", userId)
}
