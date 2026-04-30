package system

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/api/pb/admin/v1"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

var _ admin.IAdminHandler = (*Admin)(nil)

type Admin struct {
	AdminRepo *repo.Admin
}

// Create 创建管理员
// @Summary 创建管理员
// @Description 创建一个新的管理员账号
// @Tags 管理员后台-管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AdminCreateRequest true "创建请求"
// @Success 200 {object} admin.AdminCreateResponse
// @Router /backend/admin/create [post]
func (a *Admin) Create(ctx context.Context, in *admin.AdminCreateRequest) (*admin.AdminCreateResponse, error) {
	salt := encrypt.GenerateSalt()
	data := &model.Admin{
		Username:    in.Username,
		Password:    encrypt.HashPassword(in.Password, salt),
		Salt:        salt,
		Gender:      3,
		Email:       in.Email,
		Status:      1,
		LastLoginAt: time.Now(),
	}

	err := a.AdminRepo.Create(ctx, data)
	if err != nil {
		return nil, err
	}

	return &admin.AdminCreateResponse{Id: int32(data.Id)}, nil
}

// List 管理员列表
// @Summary 管理员列表
// @Description 获取管理员账号列表，支持分页和查询
// @Tags 管理员后台-管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AdminListRequest true "列表请求"
// @Success 200 {object} admin.AdminListResponse
// @Router /backend/admin/list [post]
func (a *Admin) List(ctx context.Context, in *admin.AdminListRequest) (*admin.AdminListResponse, error) {
	total, conditions, err := a.AdminRepo.Pagination(ctx, int(in.Page), int(in.PageSize), func(tx *gorm.DB) *gorm.DB {
		if in.Username != "" {
			tx = tx.Where("username = ?", in.Username)
		}

		if in.Email != "" {
			tx = tx.Where("email = ?", in.Email)
		}

		if in.Status > 0 {
			tx = tx.Where("status = ?", in.Status)
		}

		return tx.Order("id desc")
	})

	if err != nil {
		return nil, err
	}

	items := lo.Map(conditions, func(item *model.Admin, index int) *admin.AdminListResponse_Item {
		return &admin.AdminListResponse_Item{
			Id:          int32(item.Id),
			Username:    item.Username,
			Avatar:      item.Avatar,
			Mobile:      item.Mobile,
			Email:       item.Email,
			Status:      int32(item.Status),
			CreatedAt:   item.CreatedAt.Format(time.DateTime),
			UpdatedAt:   item.UpdatedAt.Format(time.DateTime),
			LastLoginAt: item.LastLoginAt.Format(time.DateTime),
			RoleName:    "test",
		}
	})

	return &admin.AdminListResponse{
		Items:     items,
		Total:     int32(total),
		Page:      in.Page,
		PageSize:  in.PageSize,
		PageTotal: int32(total) / in.PageSize,
	}, nil
}

// UpdateStatus 更新管理员状态
// @Summary 更新管理员状态
// @Description 启用或禁用管理员账号
// @Tags 管理员后台-管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AdminUpdateStatusRequest true "更新状态请求"
// @Success 200 {object} admin.AdminUpdateStatusResponse
// @Router /backend/admin/update-status [post]
func (a *Admin) UpdateStatus(ctx context.Context, in *admin.AdminUpdateStatusRequest) (*admin.AdminUpdateStatusResponse, error) {
	_, err := a.AdminRepo.UpdateById(ctx, in.GetId(), map[string]any{
		"status": in.Status,
	})

	if err != nil {
		return nil, err
	}

	return &admin.AdminUpdateStatusResponse{Id: in.Id}, nil
}

// ResetPassword 重置管理员密码
// @Summary 重置管理员密码
// @Description 重置指定管理员的登录密码
// @Tags 管理员后台-管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AdminResetPasswordRequest true "重置密码请求"
// @Success 200 {object} admin.AdminResetPasswordResponse
// @Router /backend/admin/reset-password [post]
func (a *Admin) ResetPassword(ctx context.Context, in *admin.AdminResetPasswordRequest) (*admin.AdminResetPasswordResponse, error) {
	salt := encrypt.GenerateSalt()
	_, err := a.AdminRepo.UpdateById(ctx, in.GetId(), map[string]any{
		"password": encrypt.HashPassword(in.Password, salt),
		"salt":     salt,
	})

	if err != nil {
		return nil, err
	}

	return &admin.AdminResetPasswordResponse{Id: in.Id}, nil
}
