package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type GroupTotop struct {
	db *gorm.DB
}

func NewGroupTotop(db *gorm.DB) *GroupTotop {
	return &GroupTotop{db: db}
}

// ListByGroupID 查询 group_ids 包含指定群 id 的记录，按 sort 倒序
func (r *GroupTotop) ListByGroupID(ctx context.Context, groupID int) ([]model.GroupTotop, error) {
	var rows []model.GroupTotop
	err := r.db.WithContext(ctx).Model(&model.GroupTotop{}).
		Where("FIND_IN_SET(?, group_ids)", groupID).
		Order("sort DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
