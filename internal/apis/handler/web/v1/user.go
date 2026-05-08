package v1

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/encrypt/rsautil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

var _ web.IUserHandler = (*User)(nil)

type User struct {
	Redis               *redis.Client
	UsersRepo           *repo.Users
	MerchantRepo        *repo.Merchant
	MerchantTaskRepo    *repo.MerchantTask
	MerchantPaytypeRepo *repo.MerchantPaytype
	MerchantOrderRepo   *repo.MerchantOrder
	MerchantSessionRepo *repo.MerchantSession
	MerchantMessageRepo *repo.MerchantMessage
	PushMessage         *logic.PushMessage
	UnreadStorage       *cache.UnreadStorage
	OrganizeRepo        *repo.Organize
	UserService         service.IUserService
	SmsService          service.ISmsService
	EmailService        service.IEmailService
	Rsa                 rsautil.IRsa
}

// Detail 获取登录用户详情接口
//
//	@Summary		用户详情
//	@Description	获取当前登录用户的详细信息
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserDetailRequest	true	"用户详情请求"
//	@Success		200		{object}	web.UserDetailResponse
//	@Router			/api/v1/user/detail [post]
//	@Security		Bearer
func (u *User) Detail(ctx context.Context, _ *web.UserDetailRequest) (*web.UserDetailResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	user, err := u.UsersRepo.FindByIdWithCache(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	return &web.UserDetailResponse{
		Id:       int32(user.Id),
		UserId:   int32(user.UserId),
		Username: user.Username,
		Mobile:   lo.FromPtr(user.Mobile),
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Gender:   int32(user.Gender),
		Motto:    user.Motto,
		Email:    user.Email,
		Birthday: user.Birthday,
		Uuid:     int32(user.Uuid),
		UserCode: user.UserCode,
		IsTrans:  int32(user.IsTrans),
	}, nil
}

// Setting 获取用户配置信息接口
//
//	@Summary		用户设置
//	@Description	获取用户配置和个人资料设置
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserSettingRequest	true	"用户设置请求"
//	@Success		200		{object}	web.UserSettingResponse
//	@Router			/api/v1/user/setting [post]
//	@Security		Bearer
func (u *User) Setting(ctx context.Context, req *web.UserSettingRequest) (*web.UserSettingResponse, error) {
	session, err := middleware.FormContext[entity.WebClaims](ctx)
	if err != nil {
		return nil, err
	}

	user, err := u.UsersRepo.FindByIdWithCache(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	isOk, err := u.OrganizeRepo.IsQiyeMember(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	return &web.UserSettingResponse{
		UserInfo: &web.UserSettingResponse_UserInfo{
			Uid:      int32(user.Id),
			UserId:   int32(user.UserId),
			Username: user.Username,
			Nickname: user.Nickname,
			Avatar:   user.Avatar,
			Motto:    user.Motto,
			Gender:   int32(user.Gender),
			IsQiye:   isOk,
			Mobile:   lo.FromPtr(user.Mobile),
			Email:    user.Email,
			Uuid:     int32(user.Uuid),
			UserCode: user.UserCode,
			IsTrans:  int32(user.IsTrans),
		},
		Setting: &web.UserSettingResponse_ConfigInfo{},
	}, nil
}

// DetailUpdate 更新用户信息接口
//
//	@Summary		更新用户详情
//	@Description	更新用户个人资料信息，如昵称、头像、性别等
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserDetailUpdateRequest	true	"更新详情请求"
//	@Success		200		{object}	web.UserDetailUpdateResponse
//	@Router			/api/v1/user/detail-update [post]
//	@Security		Bearer
func (u *User) DetailUpdate(ctx context.Context, req *web.UserDetailUpdateRequest) (*web.UserDetailUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	if req.Birthday != "" {
		if !timeutil.IsDate(req.Birthday) {
			return nil, errorx.New(400, "birthday 错误")
		}
	}

	uid := session.UserId

	_, err := u.UsersRepo.UpdateById(ctx, uid, map[string]any{
		"nickname": strings.TrimSpace(strings.ReplaceAll(req.Nickname, " ", "")),
		"avatar":   req.Avatar,
		"gender":   req.Gender,
		"motto":    req.Motto,
		"birthday": req.Birthday,
	})

	if err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, int(uid))
	return &web.UserDetailUpdateResponse{}, nil
}

// PasswordUpdate 更新用户密码接口
//
//	@Summary		更新密码
//	@Description	修改用户登录密码
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserPasswordUpdateRequest	true	"更新密码请求"
//	@Success		200		{object}	web.UserPasswordUpdateResponse
//	@Router			/api/v1/user/password-update [post]
//	@Security		Bearer
func (u *User) PasswordUpdate(ctx context.Context, in *web.UserPasswordUpdateRequest) (*web.UserPasswordUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	uid := session.UserId
	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	oldPassword, err := u.Rsa.Decrypt(in.OldPassword)
	if err != nil {
		return nil, err
	}

	newPassword, err := u.Rsa.Decrypt(in.NewPassword)
	if err != nil {
		return nil, err
	}

	if err := u.UserService.UpdatePassword(ctx, int(uid), string(oldPassword), string(newPassword)); err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, int(uid))
	return nil, nil
}

// MobileUpdate 更新用户手机号接口
//
//	@Summary		更新手机号
//	@Description	修改用户绑定的手机号码
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserMobileUpdateRequest	true	"更新手机号请求"
//	@Success		200		{object}	web.UserMobileUpdateResponse
//	@Router			/api/v1/user/mobile-update [post]
//	@Security		Bearer
func (u *User) MobileUpdate(ctx context.Context, in *web.UserMobileUpdateRequest) (*web.UserMobileUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.UserId

	user, _ := u.UsersRepo.FindById(ctx, uid)
	if lo.FromPtr(user.Mobile) == in.Mobile {
		return nil, errorx.New(400, "手机号与原手机号一致无需修改")
	}

	password, err := u.Rsa.Decrypt(in.Password)
	if err != nil {
		return nil, err
	}

	if !encrypt.VerifyPassword(user.Password, string(password), user.Salt) {
		return nil, entity.ErrAccountOrPasswordError
	}

	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	if !u.SmsService.Verify(ctx, entity.SmsChangeAccountChannel, in.Mobile, in.SmsCode) {
		return nil, entity.ErrSmsCodeError
	}

	_, err = u.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"mobile": in.Mobile,
	})

	if err != nil {
		return nil, err
	}

	_ = u.UsersRepo.ClearTableCache(ctx, user.Id)
	return nil, nil
}

// EmailUpdate 更新用户邮箱接口
//
//	@Summary		更新邮箱
//	@Description	修改用户绑定的邮箱地址
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UserEmailUpdateRequest	true	"更新邮箱请求"
//	@Success		200		{object}	web.UserEmailUpdateResponse
//	@Router			/api/v1/user/email-update [post]
//	@Security		Bearer
func (u *User) EmailUpdate(ctx context.Context, req *web.UserEmailUpdateRequest) (*web.UserEmailUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.UserId

	newEmail := strings.TrimSpace(req.Email)
	user, _ := u.UsersRepo.FindById(ctx, uid)
	if user.Email == newEmail {
		return nil, errorx.New(400, "邮箱与原邮箱一致无需修改")
	}

	password, err := u.Rsa.Decrypt(req.Password)
	if err != nil {
		return nil, err
	}

	if !encrypt.VerifyPassword(user.Password, string(password), user.Salt) {
		return nil, entity.ErrAccountOrPasswordError
	}

	if uid == 2054 || uid == 2055 {
		return nil, entity.ErrPermissionDenied
	}

	if !u.EmailService.Verify(ctx, entity.EmailVerifyChannel, newEmail, req.Code) {
		return nil, errorx.New(400, "邮箱验证码错误")
	}

	if other, _ := u.UsersRepo.FindByEmail(ctx, newEmail); other != nil && other.Id > 0 && other.Id != user.Id {
		return nil, errorx.New(400, "该邮箱已被其他账号使用")
	}
	if other, _ := u.UsersRepo.FindByUsername(ctx, newEmail); other != nil && other.Id > 0 && other.Id != user.Id {
		return nil, errorx.New(400, "该邮箱对应的登录名已被占用")
	}

	_, err = u.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"email":    newEmail,
		"username": newEmail,
	})

	if err != nil {
		return nil, err
	}

	u.EmailService.Delete(ctx, entity.EmailVerifyChannel, newEmail)

	_ = u.UsersRepo.ClearTableCache(ctx, user.Id)
	return &web.UserEmailUpdateResponse{}, nil
}

// MerchantApply 商户入驻申请
func (u *User) MerchantApply(ctx context.Context, in *web.UserMerchantApplyRequest) (*web.UserMerchantApplyResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	hasApproved, err := u.MerchantRepo.HasApprovedByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if hasApproved {
		return nil, errorx.New(400, "您已有审核通过的商户资质，无法再次申请")
	}

	latest, err := u.MerchantRepo.FindLatestByUserId(ctx, uid)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		if latest.Status == model.MerchantStatusPending {
			return nil, errorx.New(400, "您有待审核的商户申请，请勿重复提交")
		}
		if latest.Status == model.MerchantStatusRejected {
			updates := map[string]any{
				"nickname":  strings.TrimSpace(in.GetNickname()),
				"realname":  strings.TrimSpace(in.GetRealname()),
				"nation":    strings.TrimSpace(in.GetNation()),
				"id_type":   int(in.GetIdType()),
				"idcard":    strings.TrimSpace(in.GetIdcard()),
				"image":     strings.TrimSpace(in.GetImage()),
				"backimage": strings.TrimSpace(in.GetBackImage()),
				"surety":    in.GetSurety(),
				"status":    model.MerchantStatusPending,
				"reason":    "",
			}
			if err := u.MerchantRepo.UpdateById(ctx, latest.Id, updates); err != nil {
				return nil, err
			}
			return &web.UserMerchantApplyResponse{Id: int32(latest.Id)}, nil
		}
	}

	row := &model.Merchant{
		UserId:    uid,
		Nickname:  strings.TrimSpace(in.GetNickname()),
		Realname:  strings.TrimSpace(in.GetRealname()),
		Nation:    strings.TrimSpace(in.GetNation()),
		IdType:    int(in.GetIdType()),
		Idcard:    strings.TrimSpace(in.GetIdcard()),
		Image:     strings.TrimSpace(in.GetImage()),
		Backimage: strings.TrimSpace(in.GetBackImage()),
		Surety:    in.GetSurety(),
		Status:    model.MerchantStatusPending,
	}
	if err := u.MerchantRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &web.UserMerchantApplyResponse{Id: int32(row.Id)}, nil
}

// MerchantStatus 查询本人最近一次商户申请状态
func (u *User) MerchantStatus(ctx context.Context, _ *web.UserMerchantStatusRequest) (*web.UserMerchantStatusResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	latest, err := u.MerchantRepo.FindLatestByUserId(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return &web.UserMerchantStatusResponse{HasApplication: false}, nil
	}
	return &web.UserMerchantStatusResponse{
		HasApplication: true,
		Id:             int32(latest.Id),
		Nickname:       latest.Nickname,
		Realname:       latest.Realname,
		Nation:         latest.Nation,
		IdType:         int32(latest.IdType),
		Idcard:         latest.Idcard,
		Image:          latest.Image,
		BackImage:      latest.Backimage,
		Surety:         latest.Surety,
		Status:         int32(latest.Status),
		Reason:         latest.Reason,
		IsLimit:        int32(latest.IsLimit),
		LimitTime:      int32(latest.LimitTime),
		IsFrozen:       int32(latest.IsFrozen),
		FrozenTime:     int32(latest.FrozenTime),
		IsClose:        int32(latest.IsClose),
		CreatedAt:      timeutil.FormatDatetime(latest.CreatedAt),
		UpdatedAt:      timeutil.FormatDatetime(latest.UpdatedAt),
	}, nil
}

// MerchantProfile 获取本人商户资料（与 MerchantStatus 数据一致，供表单查看/编辑回填）
func (u *User) MerchantProfile(ctx context.Context, _ *web.UserMerchantProfileRequest) (*web.UserMerchantStatusResponse, error) {
	return u.MerchantStatus(ctx, &web.UserMerchantStatusRequest{})
}
