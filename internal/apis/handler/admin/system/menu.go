package system

import (
	"context"
	"slices"

	"github.com/gzydong/go-chat/api/pb/admin/v1"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"

	"github.com/samber/lo"
)

var _ admin.IMenuHandler = (*Menu)(nil)

type Menu struct {
	SysMenuRepo *repo.SysMenu
}

var tree = MenuTree{}

// List 菜单列表
// @Summary 菜单列表
// @Description 获取后台管理系统的所有菜单树
// @Tags 管理员后台-菜单
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.MenuListRequest true "列表请求"
// @Success 200 {object} admin.MenuListResponse
// @Router /backend/menu/list [post]
func (m *Menu) List(ctx context.Context, req *admin.MenuListRequest) (*admin.MenuListResponse, error) {
	items, err := m.SysMenuRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Order("id asc")
	})

	if err != nil {
		return nil, err
	}

	return &admin.MenuListResponse{
		Items: tree.Build(items),
	}, nil
}

// Create 创建菜单
// @Summary 创建菜单
// @Description 创建一个新的后台管理菜单
// @Tags 管理员后台-菜单
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.MenuCreateRequest true "创建请求"
// @Success 200 {object} admin.MenuCreateResponse
// @Router /backend/menu/create [post]
func (m *Menu) Create(ctx context.Context, in *admin.MenuCreateRequest) (*admin.MenuCreateResponse, error) {
	if in.ParentId > 0 {
		info, err := m.SysMenuRepo.FindById(ctx, in.ParentId)
		if err != nil {
			return nil, err
		}

		if in.MenuType == 3 && slices.Contains([]int32{1, 3}, info.MenuType) {
			return nil, errorx.New(400, "只能在页面菜单下添加按钮类型的子菜单")
		}
	} else {
		if in.MenuType == 3 {
			return nil, errorx.New(400, "只能在页面菜单下添加按钮类型的子菜单")
		}
	}

	data := &model.SysMenu{
		ParentId:  in.ParentId,
		Name:      in.Name,
		MenuType:  in.MenuType,
		Icon:      in.Icon,
		Path:      in.Path,
		Sort:      in.Sort,
		Hidden:    lo.Ternary(in.Hidden == "", "N", in.Hidden),
		UseLayout: lo.Ternary(in.UseLayout == "", "Y", in.UseLayout),
		AuthCode:  in.AuthCode,
		Status:    1,
	}

	err := m.SysMenuRepo.Create(ctx, data)
	if err != nil {
		return nil, err
	}

	return &admin.MenuCreateResponse{Id: data.Id}, nil
}

// Update 更新菜单
// @Summary 更新菜单
// @Description 更新现有后台管理菜单的信息
// @Tags 管理员后台-菜单
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.MenuUpdateRequest true "更新请求"
// @Success 200 {object} admin.MenuUpdateResponse
// @Router /backend/menu/update [post]
func (m *Menu) Update(ctx context.Context, in *admin.MenuUpdateRequest) (*admin.MenuUpdateResponse, error) {
	_, err := m.SysMenuRepo.UpdateByWhere(ctx, map[string]any{
		"parent_id":  in.ParentId,
		"name":       in.Name,
		"icon":       in.Icon,
		"path":       in.Path,
		"sort":       in.Sort,
		"hidden":     lo.Ternary(in.Hidden == "", "N", in.Hidden),
		"status":     in.Status,
		"use_layout": lo.Ternary(in.UseLayout == "", "Y", in.UseLayout),
		"auth_code":  in.AuthCode,
	}, "id = ?", in.Id)
	if err != nil {
		return nil, err
	}

	return &admin.MenuUpdateResponse{Id: in.Id}, nil
}

// Delete 删除菜单
// @Summary 删除菜单
// @Description 删除指定的后台管理菜单
// @Tags 管理员后台-菜单
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.MenuDeleteRequest true "删除请求"
// @Success 200 {object} admin.MenuDeleteResponse
// @Router /backend/menu/delete [post]
func (m *Menu) Delete(ctx context.Context, in *admin.MenuDeleteRequest) (*admin.MenuDeleteResponse, error) {
	info, err := m.SysMenuRepo.FindById(ctx, in.Id)
	if err != nil {
		return nil, err
	}

	if info.Status != 2 {
		return nil, errorx.New(400, "该菜单已启用，请先禁用后进行删除")
	}

	err = m.SysMenuRepo.Delete(ctx, in.Id)
	if err != nil {
		return nil, err
	}

	return &admin.MenuDeleteResponse{Id: in.Id}, nil
}
