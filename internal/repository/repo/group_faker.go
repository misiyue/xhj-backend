package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type GroupFaker struct {
	db *gorm.DB
}

func NewGroupFaker(db *gorm.DB) *GroupFaker {
	return &GroupFaker{db: db}
}

// ListUserFakersByGroupID 查询群内关联的马甲用户资料
func (r *GroupFaker) ListUserFakersByGroupID(ctx context.Context, groupID int) ([]*model.UserFaker, error) {
	if groupID <= 0 {
		return nil, nil
	}
	var list []*model.UserFaker
	err := r.db.WithContext(ctx).
		Table(model.UserFaker{}.TableName()+" uf").
		Select("uf.user_id, uf.username, uf.nickname, uf.avatar").
		Joins("INNER JOIN "+model.GroupFaker{}.TableName()+" gf ON gf.faker_id = uf.id").
		Where("gf.group_id = ?", groupID).
		Order("uf.id ASC").
		Scan(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
