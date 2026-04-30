package contact

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/repo"
	message2 "github.com/gzydong/go-chat/internal/service/message"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/service"
)

var _ web.IContactHandler = (*Contact)(nil)

type Contact struct {
	ContactRepo     *repo.Contact
	UsersRepo       *repo.Users
	OrganizeRepo    *repo.Organize
	TalkSessionRepo *repo.TalkSession
	ContactService  service.IContactService
	UserService     service.IUserService
	TalkListService service.ITalkSessionService
	Message         message2.IService
	UserClient      *cache.UserClient
}

// List 联系人列表接口
//
//	@Summary		联系人列表
//	@Description	获取用户联系人列表
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactListRequest	true	"联系人列表请求"
//	@Success		200		{object}	web.ContactListResponse
//	@Router			/api/v1/contact/list [post]
//	@Security		Bearer
func (c *Contact) List(ctx context.Context, _ *web.ContactListRequest) (*web.ContactListResponse, error) {
	list, err := c.ContactService.List(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx))
	if err != nil {
		return nil, err
	}

	items := make([]*web.ContactListResponse_Item, 0, len(list))
	for _, item := range list {
		items = append(items, &web.ContactListResponse_Item{
			UserId:   int32(item.Id),
			Nickname: item.Nickname,
			Gender:   int32(item.Gender),
			Motto:    item.Motto,
			Avatar:   item.Avatar,
			Remark:   item.Remark,
			GroupId:  int32(item.GroupId),
		})
	}

	return &web.ContactListResponse{Items: items}, nil
}

// Delete 联系人删除接口
//
//	@Summary		删除联系人
//	@Description	从联系人中移除用户
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactDeleteRequest	true	"删除联系人请求"
//	@Success		200		{object}	web.ContactDeleteResponse
//	@Router			/api/v1/contact/delete [post]
//	@Security		Bearer
func (c *Contact) Delete(ctx context.Context, in *web.ContactDeleteRequest) (*web.ContactDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if err := c.ContactService.Delete(ctx, uid, int(in.UserId)); err != nil {
		return nil, err
	}

	_ = c.Message.CreatePrivateSysMessage(ctx, message2.CreatePrivateSysMessageOption{
		FromId:   int(in.UserId),
		ReceiverId: uid,
		Content:  "你与对方已经解除了好友关系！",
	})

	if err := c.TalkListService.Delete(ctx, uid, entity.ChatPrivateMode, int(in.UserId)); err != nil {
		return nil, err
	}

	return &web.ContactDeleteResponse{}, nil
}

// EditRemark 联系人备注修改接口
//
//	@Summary		修改备注
//	@Description	修改联系人的备注名称
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactEditRemarkRequest	true	"修改备注请求"
//	@Success		200		{object}	web.ContactEditRemarkResponse
//	@Router			/api/v1/contact/edit-remark [post]
//	@Security		Bearer
func (c *Contact) EditRemark(ctx context.Context, in *web.ContactEditRemarkRequest) (*web.ContactEditRemarkResponse, error) {
	if err := c.ContactService.UpdateRemark(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx), int(in.UserId), in.Remark); err != nil {
		return nil, err
	}

	return &web.ContactEditRemarkResponse{}, nil
}

// Detail 联系人详情接口
//
//	@Summary		联系人详情
//	@Description	获取联系人的详细信息
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactDetailRequest	true	"联系人详情请求"
//	@Success		200		{object}	web.ContactDetailResponse
//	@Router			/api/v1/contact/detail [post]
//	@Security		Bearer
func (c *Contact) Detail(ctx context.Context, in *web.ContactDetailRequest) (*web.ContactDetailResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	user, err := c.UsersRepo.FindByIdWithCache(ctx, int(in.UserId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrUserNotExist
		}

		return nil, err
	}

	resp := &web.ContactDetailResponse{
		UserId:         int32(user.Id),
		Mobile:         lo.FromPtr(user.Mobile),
		Nickname:       user.Nickname,
		Username:       user.Username,
		Avatar:         user.Avatar,
		Gender:         int32(user.Gender),
		Motto:          user.Motto,
		Email:          user.Email,
		Relation:       1, // 关系 1陌生人 2好友 3企业同事 4本人
		ContactRemark:  "",
		ContactGroupId: 0,
		OnlineStatus:   "N",
	}

	if user.Id == uid {
		resp.Relation = 4
		resp.OnlineStatus = "Y"
		return resp, nil
	}

	isQiYeMember, _ := c.OrganizeRepo.IsQiyeMember(ctx, uid, user.Id)
	if isQiYeMember {
		if c.UserClient.IsOnline(ctx, int64(in.UserId)) {
			resp.OnlineStatus = "Y"
		}

		resp.Relation = 3
		return resp, nil
	}

	contact, err := c.ContactRepo.FindByWhere(ctx, "user_id = ? and friend_id = ?", uid, user.Id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	resp.Relation = 1
	if err == nil && contact.Status == 1 && c.ContactRepo.IsFriend(ctx, uid, user.Id, true) {
		resp.Relation = 2
		resp.ContactGroupId = int32(contact.GroupId)
		resp.ContactRemark = contact.Remark

		if c.UserClient.IsOnline(ctx, int64(in.UserId)) {
			resp.OnlineStatus = "Y"
		}
	}

	return resp, nil
}

// Search 联系人搜索接口
//
//	@Summary		搜索联系人
//	@Description	搜索用户以添加为联系人（仅支持账号的精确匹配：用户名 / 邮箱 / 手机号，不做模糊查询）
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactSearchRequest	true	"搜索联系人请求"
//	@Success		200		{object}	web.ContactSearchResponse
//	@Router			/api/v1/contact/search [post]
//	@Security		Bearer
func (c *Contact) Search(ctx context.Context, in *web.ContactSearchRequest) (*web.ContactSearchResponse, error) {
	// 添加好友时使用严格匹配：只按用户名 / 邮箱 / 手机号做等值查询，避免输入“2”时匹配到一堆包含 2 的邮箱/昵称
	user, err := c.UsersRepo.SearchByExactIdentifier(ctx, in.Mobile)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrUserNotExist
		}

		return nil, err
	}

	return &web.ContactSearchResponse{
		UserId:   int32(user.Id),
		Mobile:   lo.FromPtr[string](user.Mobile),
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Gender:   int32(user.Gender),
		Motto:    user.Motto,
	}, nil
}

// ChangeGroup 修改联系人分组接口
//
//	@Summary		修改联系人分组
//	@Description	将联系人移至不同的分组
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactChangeGroupRequest	true	"修改分组请求"
//	@Success		200		{object}	web.ContactChangeGroupResponse
//	@Router			/api/v1/contact/change-group [post]
//	@Security		Bearer
func (c *Contact) ChangeGroup(ctx context.Context, in *web.ContactChangeGroupRequest) (*web.ContactChangeGroupResponse, error) {
	err := c.ContactService.MoveGroup(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx), int(in.UserId), int(in.GroupId))
	if err != nil {
		return nil, err
	}

	return &web.ContactChangeGroupResponse{}, nil
}

// OnlineStatus 获取联系人在线状态接口
//
//	@Summary		联系人在线状态
//	@Description	检查联系人当前是否在线
//	@Tags			联系人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactOnlineStatusRequest	true	"在线状态请求"
//	@Success		200		{object}	web.ContactOnlineStatusResponse
//	@Router			/api/v1/contact/online-status [post]
//	@Security		Bearer
func (c *Contact) OnlineStatus(ctx context.Context, in *web.ContactOnlineStatusRequest) (*web.ContactOnlineStatusResponse, error) {
	resp := &web.ContactOnlineStatusResponse{
		OnlineStatus: "N",
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	ok := c.ContactRepo.IsFriend(ctx, uid, int(in.UserId), true)
	if ok && c.UserClient.IsOnline(ctx, int64(in.UserId)) {
		resp.OnlineStatus = "Y"
	}

	return resp, nil
}
