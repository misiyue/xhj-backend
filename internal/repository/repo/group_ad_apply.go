package repo

import (
	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type GroupAdApply struct {
	core.Repo[model.GroupAdApply]
}

func NewGroupAdApply(db *gorm.DB) *GroupAdApply {
	return &GroupAdApply{Repo: core.NewRepo[model.GroupAdApply](db)}
}
