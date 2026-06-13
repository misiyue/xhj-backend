package group

import (
	"context"
	"errors"
	"strings"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/sliceutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/gzydong/go-chat/internal/service/message"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var _ web.IGroupApplyHandler = (*Apply)(nil)

type Apply struct {
	Redis              *redis.Client
	GroupApplyStorage  *cache.GroupApplyStorage
	GroupRepo          *repo.Group
	GroupApplyRepo     *repo.GroupApply
	GroupMemberRepo    *repo.GroupMember
	GroupApplyService  service.IGroupApplyService
	GroupMemberService service.IGroupMemberService
	GroupService       service.IGroupService
	PushMessage        *logic.PushMessage
	UsersRepo          *repo.Users
	UserClient         *cache.UserClient
	NoticeTemplateRepo *repo.NoticeTemplate
}

// Create 创建群组申请接口
//
//	@Summary		申请入群
//	@Description	申请加入群聊
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyCreateRequest	true	"入群申请请求"
//	@Success		200		{object}	web.GroupApplyCreateResponse
//	@Router			/api/v1/group-apply/create [post]
//	@Security		Bearer
func (a Apply) Create(ctx context.Context, in *web.GroupApplyCreateRequest) (*web.GroupApplyCreateResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	apply, err := a.GroupApplyRepo.FindByWhere(ctx, "group_id = ? and user_id = ? and status = ?", in.GroupId, uid, model.GroupApplyStatusWait)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	applyId := 0
	if apply == nil || apply.Id == 0 {
		data := &model.GroupApply{
			GroupId: int(in.GroupId),
			UserId:  uid,
			Status:  model.GroupApplyStatusWait,
			Remark:  in.Remark,
		}

		err = a.GroupApplyRepo.Create(ctx, data)
		if err == nil {
			applyId = data.Id
		}
	} else {
		applyId = apply.Id
		data := map[string]any{
			"remark":     in.Remark,
			"updated_at": timeutil.DateTime(),
		}

		_, err = a.GroupApplyRepo.UpdateByWhere(ctx, data, "id = ?", apply.Id)
	}

	if err != nil {
		return nil, err
	}

	leaders, leaderErr := a.GroupMemberRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Select("user_id")
		db.Where("group_id = ?", in.GroupId)
		db.Where("leader in ?", []int{
			model.GroupMemberLeaderOwner,
			model.GroupMemberLeaderAdmin,
		})
		db.Where("is_quit = ?", model.No)
	})
	if leaderErr == nil {
		for _, leader := range leaders {
			a.GroupApplyStorage.Incr(ctx, leader.UserId)
			a.tryOneSignalGroupApply(ctx, leader.UserId, uid, int(in.GroupId))
		}
	}

	_ = a.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event: entity.SubEventGroupApply,
		Payload: jsonutil.Encode(entity.SubEventGroupApplyPayload{
			GroupId: int(in.GroupId),
			UserId:  uid,
			ApplyId: applyId,
		}),
	})

	return &web.GroupApplyCreateResponse{}, err
}

func (a Apply) tryOneSignalGroupApply(ctx context.Context, receiverID, senderID, groupID int) {
	if receiverID <= 0 || senderID <= 0 {
		return
	}
	senderName := "用户"
	if a.UsersRepo != nil {
		sender, err := a.UsersRepo.FindByIdWithCache(ctx, senderID)
		if err != nil {
			logger.Errorf("group_apply sender load err: sender_id=%d %s", senderID, err.Error())
		} else if sender != nil && strings.TrimSpace(sender.Nickname) != "" {
			senderName = strings.TrimSpace(sender.Nickname)
		}
	}
	groupName := "群聊"
	if a.GroupRepo != nil {
		group, err := a.GroupRepo.FindById(ctx, groupID)
		if err != nil {
			logger.Errorf("group_apply group load err: group_id=%d %s", groupID, err.Error())
		} else if group != nil && strings.TrimSpace(group.Name) != "" {
			groupName = strings.TrimSpace(group.Name)
		}
	}
	message.TryOneSignalTemplatePush(
		ctx,
		a.UserClient,
		a.UsersRepo,
		a.NoticeTemplateRepo,
		nil,
		receiverID,
		0,
		0,
		model.NoticeTemplateFlagGroupApply,
		map[string]string{"sender": senderName, "group": groupName},
	)
}

// Delete 删除群组申请接口
//
//	@Summary		删除入群申请
//	@Description	删除加入请求（未实现）
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyDeleteRequest	true	"删除申请请求"
//	@Success		200		{object}	web.GroupApplyDeleteResponse
//	@Router			/api/v1/group-apply/delete [post]
//	@Security		Bearer
func (a Apply) Delete(ctx context.Context, req *web.GroupApplyDeleteRequest) (*web.GroupApplyDeleteResponse, error) {
	return nil, nil
}

// Agree 同意群组申请接口
//
//	@Summary		同意入群申请
//	@Description	通过用户的入群申请（仅限管理员）
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyAgreeRequest	true	"同意申请请求"
//	@Success		200		{object}	web.GroupApplyAgreeResponse
//	@Router			/api/v1/group-apply/agree [post]
//	@Security		Bearer
func (a Apply) Agree(ctx context.Context, in *web.GroupApplyAgreeRequest) (*web.GroupApplyAgreeResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	apply, err := a.GroupApplyRepo.FindById(ctx, int(in.ApplyId))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrDataNotFound
	}

	if !a.GroupMemberRepo.IsLeader(ctx, apply.GroupId, uid) {
		return nil, entity.ErrPermissionDenied
	}

	if apply.Status != model.GroupApplyStatusWait {
		return nil, nil
	}

	if !a.GroupMemberRepo.IsMember(ctx, apply.GroupId, apply.UserId, false) {
		err = a.GroupService.Invite(ctx, &service.GroupInviteOpt{
			UserId:    uid,
			GroupId:   apply.GroupId,
			MemberIds: []int{apply.UserId},
		})

		if err != nil {
			return nil, err
		}
	}

	data := map[string]any{
		"status":     model.GroupApplyStatusPass,
		"updated_at": timeutil.DateTime(),
	}

	_, err = a.GroupApplyRepo.UpdateByWhere(ctx, data, "id = ?", in.ApplyId)
	if err != nil {
		return nil, err
	}

	return &web.GroupApplyAgreeResponse{}, nil
}

// Decline 拒绝群组申请接口
//
//	@Summary		拒绝入群申请
//	@Description	拒绝用户的入群申请（仅限管理员）
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyDeclineRequest	true	"拒绝申请请求"
//	@Success		200		{object}	web.GroupApplyDeclineResponse
//	@Router			/api/v1/group-apply/decline [post]
//	@Security		Bearer
func (a Apply) Decline(ctx context.Context, in *web.GroupApplyDeclineRequest) (*web.GroupApplyDeclineResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	apply, err := a.GroupApplyRepo.FindById(ctx, int(in.ApplyId))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrDataNotFound
	}

	if !a.GroupMemberRepo.IsLeader(ctx, apply.GroupId, uid) {
		return nil, entity.ErrPermissionDenied
	}

	if apply.Status != model.GroupApplyStatusWait {
		return &web.GroupApplyDeclineResponse{}, nil
	}

	data := map[string]any{
		"status":     model.GroupApplyStatusRefuse,
		"reason":     in.Remark,
		"updated_at": timeutil.DateTime(),
	}

	_, err = a.GroupApplyRepo.UpdateByWhere(ctx, data, "id = ?", in.ApplyId)
	if err != nil {
		return nil, err
	}

	return &web.GroupApplyDeclineResponse{}, nil
}

// List 群组申请列表接口
//
//	@Summary		群申请列表
//	@Description	获取特定群组的加入请求列表（仅限管理员）
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyListRequest	true	"申请列表请求"
//	@Success		200		{object}	web.GroupApplyListResponse
//	@Router			/api/v1/group-apply/list [post]
//	@Security		Bearer
func (a Apply) List(ctx context.Context, in *web.GroupApplyListRequest) (*web.GroupApplyListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if !a.GroupMemberRepo.IsLeader(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	list, err := a.GroupApplyRepo.List(ctx, []int{int(in.GroupId)})
	if err != nil {
		return nil, err
	}

	items := make([]*web.GroupApplyListResponse_Item, 0)
	for _, item := range list {
		items = append(items, &web.GroupApplyListResponse_Item{
			Id:        int32(item.Id),
			UserId:    int32(item.UserId),
			GroupId:   int32(item.GroupId),
			Remark:    item.Remark,
			Avatar:    item.Avatar,
			Nickname:  item.Nickname,
			CreatedAt: timeutil.FormatDatetime(item.CreatedAt),
		})
	}

	return &web.GroupApplyListResponse{Items: items}, nil
}

// All 所有群组申请列表接口
//
//	@Summary		所有群申请
//	@Description	获取用户管理的所有群组的待处理加入请求
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyAllRequest	true	"所有申请请求"
//	@Success		200		{object}	web.GroupApplyAllResponse
//	@Router			/api/v1/group-apply/all [post]
//	@Security		Bearer
func (a Apply) All(ctx context.Context, req *web.GroupApplyAllRequest) (*web.GroupApplyAllResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	all, err := a.GroupMemberRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Select("group_id")
		db.Where("user_id = ?", uid)
		db.Where("leader in ?", []int{
			model.GroupMemberLeaderOwner,
			model.GroupMemberLeaderAdmin,
		})
		db.Where("is_quit = ?", model.No)
	})

	if err != nil {
		return nil, err
	}

	groupIds := make([]int, 0, len(all))
	for _, m := range all {
		groupIds = append(groupIds, m.GroupId)
	}

	resp := &web.GroupApplyAllResponse{Items: make([]*web.GroupApplyAllResponse_Item, 0)}

	if len(groupIds) == 0 {
		a.GroupApplyStorage.Del(ctx, uid)
		return resp, nil
	}

	list, err := a.GroupApplyRepo.List(ctx, groupIds)
	if err != nil {
		return nil, err
	}

	groups, err := a.GroupRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Select("id,name")
		db.Where("id in ?", groupIds)
	})
	if err != nil {
		return nil, err
	}

	groupMap := sliceutil.ToMap(groups, func(t *model.Group) int {
		return t.Id
	})

	for _, item := range list {
		resp.Items = append(resp.Items, &web.GroupApplyAllResponse_Item{
			Id:        int32(item.Id),
			UserId:    int32(item.UserId),
			GroupName: groupMap[item.GroupId].Name,
			GroupId:   int32(item.GroupId),
			Remark:    item.Remark,
			Avatar:    item.Avatar,
			Nickname:  item.Nickname,
			CreatedAt: timeutil.FormatDatetime(item.CreatedAt),
		})
	}

	a.GroupApplyStorage.Del(ctx, uid)

	return resp, nil
}

// UnreadNum 获取群组申请未read数
//
//	@Summary		群申请未读数
//	@Description	获取未读的群组加入请求数量
//	@Tags			群申请
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupApplyUnreadNumRequest	true	"未读数请求"
//	@Success		200		{object}	web.GroupApplyUnreadNumResponse
//	@Router			/api/v1/group-apply/unread-num [post]
//	@Security		Bearer
func (a Apply) UnreadNum(ctx context.Context, req *web.GroupApplyUnreadNumRequest) (*web.GroupApplyUnreadNumResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	return &web.GroupApplyUnreadNumResponse{
		Num: int32(a.GroupApplyStorage.Get(ctx, uid)),
	}, nil
}
