package admin

import (
	"context"
	"sort"
	"time"

	"github.com/gzydong/go-chat/api/pb/admin/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/encrypt/rsautil"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mojocn/base64Captcha"
)

var _ admin.IAuthHandler = (*Auth)(nil)

type Auth struct {
	Config          *config.Config
	AdminRepo       *repo.Admin
	SysMenuRepo     *repo.SysMenu
	JwtTokenStorage *cache.JwtTokenStorage
	ICaptcha        *base64Captcha.Captcha
	Rsa             rsautil.IRsa
}

// Menus 获取管理员菜单
// @Summary 获取管理员菜单
// @Description 获取当前管理员的授权菜单树
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthMenusRequest true "菜单请求"
// @Success 200 {object} admin.AuthMenusResponse
// @Router /backend/auth/menus [post]
func (c *Auth) Menus(ctx context.Context, req *admin.AuthMenusRequest) (*admin.AuthMenusResponse, error) {
	//uid := middleware.FormContextAuthId[entity.AdminClaims](ctx)

	items, err := c.SysMenuRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Where("status = ?", 1)
		db.Where("menu_type in ?", []int32{1, 2})
		db.Order("id asc")
	})
	if err != nil {
		return nil, err
	}

	return &admin.AuthMenusResponse{
		Items: c.buildUserMenus(c.SysMenuRepo.BuildMenuTree(items)),
	}, nil
}

// Login 登录接口
// Login 管理员登录
// @Summary 管理员登录
// @Description 管理员登录接口，支持图形验证码
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Param request body admin.AuthLoginRequest true "登录请求"
// @Success 200 {object} admin.AuthLoginResponse
// @Router /backend/auth/login [post]
func (c *Auth) Login(ctx context.Context, in *admin.AuthLoginRequest) (*admin.AuthLoginResponse, error) {
	if !c.ICaptcha.Verify(in.CaptchaVoucher, in.Captcha, true) {
		return nil, errorx.New(400, "验证码填写不正确")
	}

	adminInfo, err := c.AdminRepo.FindByWhere(ctx, "email = ?", in.Username)
	if err != nil {
		if utils.IsSqlNoRows(err) {
			return nil, errorx.New(400, "账号不存在或密码填写错误!")
		}
		return nil, err
	}

	password, err := c.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	if !adminInfo.VerifyPassword(string(password)) {
		return nil, errorx.New(400, "账号不存在或密码填写错误!")
	}

	if adminInfo.Status != model.AdminStatusNormal {
		return nil, entity.ErrAccountDisabled
	}

	expiresAt := time.Now().Add(12 * time.Hour)

	token, err := jwtutil.NewTokenWithClaims(
		[]byte(c.Config.Jwt.Secret),
		entity.AdminClaims{
			AdminId: int32(adminInfo.Id),
		},
		func(c *jwt.RegisteredClaims) {
			c.Issuer = entity.JwtIssuerAdmin
			c.ExpiresAt = jwt.NewNumericDate(expiresAt)
		},
	)

	if err != nil {
		return nil, err
	}

	return &admin.AuthLoginResponse{
		Username: adminInfo.Username,
		Auth: &admin.AccessToken{
			Type:        "Bearer",
			AccessToken: token,
			ExpiresIn:   int32(expiresAt.Unix() - time.Now().Unix()),
		},
	}, nil
}

// Captcha 图形验证码
// Captcha 图形验证码
// @Summary 图形验证码
// @Description 获取登录所需的图形验证码
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Param request body admin.AuthCaptchaRequest true "验证码请求"
// @Success 200 {object} admin.AuthCaptchaResponse
// @Router /backend/auth/captcha [post]
func (c *Auth) Captcha(ctx context.Context, in *admin.AuthCaptchaRequest) (*admin.AuthCaptchaResponse, error) {
	voucher, captcha, _, err := c.ICaptcha.Generate()
	if err != nil {
		return nil, err
	}

	return &admin.AuthCaptchaResponse{
		Voucher: voucher,
		Captcha: captcha,
	}, nil
}

// Logout 退出登录接口
// Logout 退出登录
// @Summary 退出登录
// @Description 管理员退出登录接口，将当前 token 加入黑名单使其失效
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthLogoutRequest true "退出登录请求"
// @Success 200 {object} admin.AuthLogoutResponse
// @Router /backend/auth/logout [post]
func (c *Auth) Logout(ctx context.Context, in *admin.AuthLogoutRequest) (*admin.AuthLogoutResponse, error) {
	token := middleware.GetAuthTokenFromContext(ctx)
	if token == "" {
		return &admin.AuthLogoutResponse{Success: true, Message: "ok"}, nil
	}
	if c.JwtTokenStorage != nil {
		claims, err := jwtutil.ParseWithClaims[entity.AdminClaims]([]byte(c.Config.Jwt.Secret), token)
		if err == nil && claims.ExpiresAt != nil {
			exp := time.Until(claims.ExpiresAt.Time)
			if exp > 0 {
				if e := c.JwtTokenStorage.SetBlackList(ctx, token, exp); e != nil {
					return &admin.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
				}
			} else {
				if e := c.JwtTokenStorage.SetBlackList(ctx, token, 24*time.Hour); e != nil {
					return &admin.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
				}
			}
		} else {
			if e := c.JwtTokenStorage.SetBlackList(ctx, token, 24*time.Hour); e != nil {
				return &admin.AuthLogoutResponse{Success: false, Message: e.Error()}, nil
			}
		}
	}
	return &admin.AuthLogoutResponse{Success: true, Message: "ok"}, nil
}

// Detail 获取管理员详情接口
// Detail 管理员详情
// @Summary 管理员详情
// @Description 获取当前登录管理员的详细信息
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthDetailRequest true "详情请求"
// @Success 200 {object} admin.AuthDetailResponse
// @Router /backend/auth/detail [post]
func (c *Auth) Detail(ctx context.Context, in *admin.AuthDetailRequest) (*admin.AuthDetailResponse, error) {
	uid := middleware.FormContextAuthId[entity.AdminClaims](ctx)

	info, err := c.AdminRepo.FindById(ctx, uid)
	if err != nil {
		return nil, err
	}

	return &admin.AuthDetailResponse{
		Username:        info.Username,
		Email:           info.Email,
		Mobile:          info.Mobile,
		Address:         info.Address,
		TwoFactorEnable: "N",
	}, nil
}

// UpdatePassword 更新密码接口
// UpdatePassword 更新密码
// @Summary 更新密码
// @Description 更新当前管理员的登录密码
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthUpdatePasswordRequest true "更新密码请求"
// @Success 200 {object} admin.AuthUpdatePasswordResponse
// @Router /backend/auth/update-password [post]
func (c *Auth) UpdatePassword(ctx context.Context, in *admin.AuthUpdatePasswordRequest) (*admin.AuthUpdatePasswordResponse, error) {
	uid := middleware.FormContextAuthId[entity.AdminClaims](ctx)
	adminInfo, err := c.AdminRepo.FindById(ctx, uid)
	if err != nil {
		return nil, err
	}

	oldPassword, err := c.Rsa.Decrypt(in.OldPassword)
	if err != nil {
		return nil, err
	}

	newPassword, err := c.Rsa.Decrypt(in.NewPassword)
	if err != nil {
		return nil, err
	}

	if !adminInfo.VerifyPassword(string(oldPassword)) {
		return nil, errorx.New(400, "密码错误")
	}

	if string(oldPassword) == string(newPassword) {
		return nil, errorx.New(400, "新密码不能与旧密码相同")
	}

	newSalt := encrypt.GenerateSalt()
	_, err = c.AdminRepo.UpdateByWhere(ctx, map[string]any{
		"password": encrypt.HashPassword(string(newPassword), newSalt),
		"salt":     newSalt,
	}, "id = ?", uid)
	if err != nil {
		return nil, err
	}

	return &admin.AuthUpdatePasswordResponse{}, nil
}

// UpdateDetail 更新详情接口
// UpdateDetail 更新详情
// @Summary 更新详情
// @Description 更新当前管理员的基本信息
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthUpdateDetailRequest true "更新详情请求"
// @Success 200 {object} admin.AuthUpdateDetailResponse
// @Router /backend/auth/update-detail [post]
func (c *Auth) UpdateDetail(ctx context.Context, in *admin.AuthUpdateDetailRequest) (*admin.AuthUpdateDetailResponse, error) {
	uid := middleware.FormContextAuthId[entity.AdminClaims](ctx)

	_, err := c.AdminRepo.UpdateByWhere(ctx, map[string]any{
		"username": in.Username,
		"mobile":   in.Mobile,
		"address":  in.Address,
	}, "id = ?", uid)
	if err != nil {
		return nil, err
	}

	return &admin.AuthUpdateDetailResponse{}, nil
}

// Refresh 刷新Token接口
// Refresh 刷新Token
// @Summary 刷新Token
// @Description 刷新管理员的访问令牌
// @Tags 管理员后台-认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body admin.AuthRefreshRequest true "刷新Token请求"
// @Success 200 {object} admin.AuthRefreshResponse
// @Router /backend/auth/refresh [post]
func (c *Auth) Refresh(ctx context.Context, in *admin.AuthRefreshRequest) (*admin.AuthRefreshResponse, error) {
	// Note: Need to implement token refresh logic
	return nil, errorx.New(500, "需要实现Token刷新逻辑")
}

// buildUserMenus 递归构建UserMenus结构
func (c *Auth) buildUserMenus(menuItems []*repo.MenuItem) []*admin.Menus {
	var userMenus []*admin.Menus

	for _, item := range menuItems {
		if item.Status != 1 { // 假设1为启用状态
			continue
		}

		userMenu := &admin.Menus{
			Path: item.Path,
			Name: item.Name,
			Meta: &admin.Meta{},
		}

		// 设置Meta信息
		userMenu.Meta.Icon = item.Icon
		userMenu.Meta.Title = item.Name
		userMenu.Meta.Sort = item.Sort
		userMenu.Meta.Hidden = item.Hidden
		userMenu.Meta.UseLayout = item.UseLayout
		userMenu.Meta.FrameSrc = ""

		// 如果有子菜单，递归处理
		if len(item.Children) > 0 {
			userMenu.Children = c.buildUserMenus(item.Children)
		}

		userMenus = append(userMenus, userMenu)
	}

	sort.Slice(userMenus, func(i, j int) bool {
		return userMenus[i].Meta.Sort < userMenus[j].Meta.Sort
	})

	return userMenus
}
