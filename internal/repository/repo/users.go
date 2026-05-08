package repo

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Users struct {
	core.Repo[model.Users]
	tableCache core.TableCache[model.Users, int]
}

func NewUsers(db *gorm.DB, rds *redis.Client) *Users {
	return &Users{
		Repo:       core.NewRepo[model.Users](db),
		tableCache: core.NewTableCache[model.Users, int](rds),
	}
}

// FindByMobile 手机号查询
func (u *Users) FindByMobile(ctx context.Context, mobile string) (*model.Users, error) {
	return u.Repo.FindByWhere(ctx, "mobile = ?", mobile)
}

// IsMobileExist 判断手机号是否存在
func (u *Users) IsMobileExist(ctx context.Context, mobile string) bool {
	exist, _ := u.Repo.IsExist(ctx, "mobile = ?", mobile)
	return exist
}

// FindByEmail 邮箱查询
func (u *Users) FindByEmail(ctx context.Context, email string) (*model.Users, error) {
	return u.Repo.FindByWhere(ctx, "email = ?", email)
}

// IsEmailExist 判断邮箱是否存在
func (u *Users) IsEmailExist(ctx context.Context, email string) bool {
	exist, _ := u.Repo.IsExist(ctx, "email = ?", email)
	return exist
}

func (u *Users) FindByIdWithCache(ctx context.Context, id int) (*model.Users, error) {
	return u.tableCache.GetOrSet(ctx, id, func(ctx context.Context) (*model.Users, error) {
		return u.Repo.FindById(ctx, id)
	})
}

func (u *Users) ClearTableCache(ctx context.Context, id int) error {
	return u.tableCache.Del(ctx, id)
}

// FindByUsername 按登录名 users.username 精确查询
func (u *Users) FindByUsername(ctx context.Context, username string) (*model.Users, error) {
	return u.Repo.FindByWhere(ctx, "username = ?", username)
}

// SearchByKeyword 通过关键词搜索用户（登录用：先精确再模糊）
func (u *Users) SearchByKeyword(ctx context.Context, keyword string) (*model.Users, error) {
	// 先尝试精确匹配用户名
	user, err := u.Repo.FindByWhere(ctx, "username = ?", keyword)
	if err == nil && user != nil {
		return user, nil
	}

	// 再尝试精确匹配邮箱
	user, err = u.FindByEmail(ctx, keyword)
	if err == nil && user != nil {
		return user, nil
	}

	// 再尝试精确匹配手机号
	user, err = u.FindByMobile(ctx, keyword)
	if err == nil && user != nil {
		return user, nil
	}

	// 转义 SQL LIKE 特殊字符以防止注入和意外匹配
	// 将 % 替换为 \%，将 _ 替换为 \_
	escapedKeyword := keyword
	escapedKeyword = strings.ReplaceAll(escapedKeyword, "\\", "\\\\")
	escapedKeyword = strings.ReplaceAll(escapedKeyword, "%", "\\%")
	escapedKeyword = strings.ReplaceAll(escapedKeyword, "_", "\\_")

	// 最后尝试模糊匹配（用户名、手机号、邮箱或昵称）
	return u.Repo.FindByWhere(ctx, "username LIKE ? OR mobile LIKE ? OR email LIKE ? OR nickname LIKE ?",
		"%"+escapedKeyword+"%", "%"+escapedKeyword+"%", "%"+escapedKeyword+"%", "%"+escapedKeyword+"%")
}

// SearchByExactIdentifier 通过精确账号搜索用户（添加好友用，只做完全匹配）
// 规则：先按 username，再按 email，再按 mobile，全部是精确等值匹配，不做模糊 LIKE。
func (u *Users) SearchByExactIdentifier(ctx context.Context, keyword string) (*model.Users, error) {
	// 精确用户名
	user, err := u.Repo.FindByWhere(ctx, "username = ?", keyword)
	if err == nil && user != nil {
		return user, nil
	}

	// 精确邮箱
	user, err = u.FindByEmail(ctx, keyword)
	if err == nil && user != nil {
		return user, nil
	}

	// 精确手机号
	return u.FindByMobile(ctx, keyword)
}

// PaginationByInviteUserId 分页查询被我邀请注册的用户（users.invite_user_id = inviterId），按 id 倒序
func (u *Users) PaginationByInviteUserId(ctx context.Context, inviterId int, page, pageSize int) (total int64, items []*model.Users, err error) {
	return u.Repo.Pagination(ctx, page, pageSize, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("invite_user_id = ?", inviterId).Order("id DESC")
	})
}
