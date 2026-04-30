package contact

import (
	"context"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/gzydong/go-chat/internal/service/message"
)

var _ web.IContactApplyHandler = (*Apply)(nil)

type Apply struct {
	ContactRepo         *repo.Contact
	ContactApplyService service.IContactApplyService
	UserService         service.IUserService
	ContactService      service.IContactService
	MessageService      message.IService
}

// Create 添加联系人申请接口
//
//	@Summary		申请添加好友
//	@Description	向另一名用户发送好友请求
//	@Tags			好友申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactApplyCreateRequest	true	"申请添加好友请求"
//	@Success		200		{object}	web.ContactApplyCreateResponse
//	@Router			/api/v1/contact-apply/create [post]
//	@Security		Bearer
func (a Apply) Create(ctx context.Context, in *web.ContactApplyCreateRequest) (*web.ContactApplyCreateResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if a.ContactRepo.IsFriend(ctx, uid, int(in.UserId), false) {
		return nil, nil
	}

	if err := a.ContactApplyService.Create(ctx, &service.ContactApplyCreateOpt{
		UserId:      uid,
		Remarks:     in.Remark,
		FriendId:    int(in.UserId),
		ApplyReason: in.ApplyReason,
	}); err != nil {
		return nil, err
	}

	return &web.ContactApplyCreateResponse{}, nil
}

// Accept 同意联系人申请接口
//
//	@Summary		同意好友申请
//	@Description	接受好友请求
//	@Tags			好友申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactApplyAcceptRequest	true	"同意好友申请请求"
//	@Success		200		{object}	web.ContactApplyAcceptResponse
//	@Router			/api/v1/contact-apply/accept [post]
//	@Security		Bearer
func (a Apply) Accept(ctx context.Context, in *web.ContactApplyAcceptRequest) (*web.ContactApplyAcceptResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	applyInfo, err := a.ContactApplyService.Accept(ctx, &service.ContactApplyAcceptOpt{
		Remarks: in.Remark,
		ApplyId: int(in.ApplyId),
		UserId:  uid,
	})

	if err != nil {
		return nil, err
	}

	_ = a.MessageService.CreatePrivateSysMessage(ctx, message.CreatePrivateSysMessageOption{
		FromId:     uid,
		ReceiverId: applyInfo.UserId,
		Content:    "你们已成为好友，可以开始聊天咯！",
	})

	// _ = a.MessageService.CreatePrivateSysMessage(ctx, message.CreatePrivateSysMessageOption{
	// 	FromId:   applyInfo.UserId,
	// 	ReceiverId: uid,
	// 	Content:  "你们已成为好友，可以开始聊天咯！",
	// })

	return &web.ContactApplyAcceptResponse{}, nil
}

// Decline 拒绝联系人申请接口
//
//	@Summary		拒绝好友申请
//	@Description	拒绝好友请求
//	@Tags			好友申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactApplyDeclineRequest	true	"拒绝好友申请请求"
//	@Success		200		{object}	web.ContactApplyDeclineResponse
//	@Router			/api/v1/contact-apply/decline [post]
//	@Security		Bearer
func (a Apply) Decline(ctx context.Context, in *web.ContactApplyDeclineRequest) (*web.ContactApplyDeclineResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := a.ContactApplyService.Decline(ctx, &service.ContactApplyDeclineOpt{
		UserId:  uid,
		Remarks: in.Remark,
		ApplyId: int(in.ApplyId),
	}); err != nil {
		return nil, err
	}

	return &web.ContactApplyDeclineResponse{}, nil
}

// List 联系人申请列表接口
//
//	@Summary		好友申请列表
//	@Description	获取收到的好友请求列表
//	@Tags			好友申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactApplyListRequest	true	"好友申请列表请求"
//	@Success		200		{object}	web.ContactApplyListResponse
//	@Router			/api/v1/contact-apply/list [post]
//	@Security		Bearer
func (a Apply) List(ctx context.Context, req *web.ContactApplyListRequest) (*web.ContactApplyListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	list, err := a.ContactApplyService.List(ctx, uid)
	if err != nil {
		return nil, err
	}

	items := make([]*web.ContactApplyListResponse_Item, 0, len(list))
	for _, item := range list {
		items = append(items, &web.ContactApplyListResponse_Item{
			Id:          int32(item.Id),
			UserId:      int32(item.UserId),
			FriendId:    int32(item.FriendId),
			Remark:      item.Remark,
			Nickname:    item.Nickname,
			Avatar:      item.Avatar,
			CreatedAt:   timeutil.FormatDatetime(item.CreatedAt),
			ApplyReason: item.ApplyReason,
		})
	}

	a.ContactApplyService.ClearApplyUnreadNum(ctx, uid)

	return &web.ContactApplyListResponse{Items: items}, nil
}

// UnreadNum 获取申请未读数
//
//	@Summary		好友申请未读数
//	@Description	获取未读的好友请求数量
//	@Tags			好友申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ContactApplyUnreadNumRequest	true	"未读数请求"
//	@Success		200		{object}	web.ContactApplyUnreadNumResponse
//	@Router			/api/v1/contact-apply/unread-num [post]
//	@Security		Bearer
func (a Apply) UnreadNum(ctx context.Context, req *web.ContactApplyUnreadNumRequest) (*web.ContactApplyUnreadNumResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	return &web.ContactApplyUnreadNumResponse{Num: int32(a.ContactApplyService.GetApplyUnreadNum(ctx, uid))}, nil
}
