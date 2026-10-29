package repo

import (
	"context"
	"strconv"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type Contact struct {
	core.Repo[model.Contact]
	cache    *cache.ContactRemark
	Relation *cache.Relation
}

func NewContact(db *gorm.DB, cache *cache.ContactRemark, relation *cache.Relation) *Contact {
	return &Contact{Repo: core.NewRepo[model.Contact](db), cache: cache, Relation: relation}
}

func (c *Contact) Remarks(ctx context.Context, uid int, fids []int) (map[int]string, error) {

	if !c.cache.Exist(ctx, uid) {
		_ = c.LoadContactCache(ctx, uid)
	}

	return c.cache.MGet(ctx, uid, fids)
}

// IsFriend 判断是否为好友关系
func (c *Contact) IsFriend(ctx context.Context, uid int, friendId int, cache bool) bool {

	if cache && c.Relation.IsContactRelation(ctx, uid, friendId) == nil {
		return true
	}

	count, err := c.Repo.FindCount(ctx, "((user_id = ? and friend_id = ?) or (user_id = ? and friend_id = ?)) and status = ?", uid, friendId, friendId, uid, model.ContactStatusNormal)
	if err != nil {
		return false
	}

	if count == 2 {
		c.Relation.SetContactRelation(ctx, uid, friendId)
	} else {
		c.Relation.DelContactRelation(ctx, uid, friendId)
	}

	return count == 2
}

func (c *Contact) GetFriendRemark(ctx context.Context, uid int, friendId int) string {

	if c.cache.Exist(ctx, uid) {
		return c.cache.Get(ctx, uid, friendId)
	}

	var remark string
	c.Repo.Model(ctx).Where("user_id = ? and friend_id = ?", uid, friendId).Pluck("remark", &remark)

	return remark
}

func (c *Contact) SetFriendRemark(ctx context.Context, uid int, friendId int, remark string) error {
	return c.cache.Set(ctx, uid, friendId, remark)
}

// IsBlocked 判断 uid 是否拉黑了 friendId（单向：contact.user_id=uid）
func (c *Contact) IsBlocked(ctx context.Context, uid, friendId int) bool {
	if uid <= 0 || friendId <= 0 {
		return false
	}
	count, err := c.Repo.FindCount(ctx, "user_id = ? AND friend_id = ? AND status = ? AND is_black = ?",
		uid, friendId, model.ContactStatusNormal, model.ContactIsBlackYes)
	return err == nil && count > 0
}

// GetIsBlack 获取 uid 对 friendId 的拉黑状态，无关系返回 0
func (c *Contact) GetIsBlack(ctx context.Context, uid, friendId int) int {
	row, err := c.Repo.FindByWhere(ctx, "user_id = ? AND friend_id = ? AND status = ?", uid, friendId, model.ContactStatusNormal)
	if err != nil || row == nil {
		return model.ContactIsBlackNo
	}
	return row.IsBlack
}

// IsBlackMap 批量获取 uid 对好友的拉黑状态，key=friend_id，无记录视为 0
func (c *Contact) IsBlackMap(ctx context.Context, uid int, friendIds []int) map[int]int {
	out := make(map[int]int, len(friendIds))
	if uid <= 0 || len(friendIds) == 0 {
		return out
	}
	type row struct {
		FriendId int `gorm:"column:friend_id"`
		IsBlack  int `gorm:"column:is_black"`
	}
	var rows []row
	err := c.Repo.Model(ctx).
		Select("friend_id, is_black").
		Where("user_id = ? AND status = ? AND friend_id IN ?", uid, model.ContactStatusNormal, friendIds).
		Scan(&rows).Error
	if err != nil {
		return out
	}
	for _, r := range rows {
		out[r.FriendId] = r.IsBlack
	}
	return out
}

func (c *Contact) LoadContactCache(ctx context.Context, uid int) error {

	all, err := c.Repo.FindAll(ctx, func(db *gorm.DB) {
		db.Select("friend_id,remark").Where("user_id = ? and status = ?", uid, model.ContactStatusNormal)
	})

	if err != nil {
		return err
	}

	items := make(map[string]any)
	for _, value := range all {
		if len(value.Remark) > 0 {
			items[strconv.Itoa(value.FriendId)] = value.Remark
		}
	}

	return c.cache.MSet(ctx, uid, items)
}
