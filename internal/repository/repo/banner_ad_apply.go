package repo

import (
	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type BannerAdApply struct {
	core.Repo[model.BannerAdApply]
}

func NewBannerAdApply(db *gorm.DB) *BannerAdApply {
	return &BannerAdApply{Repo: core.NewRepo[model.BannerAdApply](db)}
}
