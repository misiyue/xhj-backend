package group

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/sliceutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/gzydong/go-chat/internal/service/message"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

var _ web.IGroupHandler = (*Group)(nil)

type Group struct {
	RedisLock          *cache.RedisLock
	Repo               *repo.Source
	UsersRepo          *repo.Users
	GroupRepo          *repo.Group
	GroupMemberRepo    *repo.GroupMember
	GroupNoticeRepo    *repo.GroupNotice
	GroupFakerRepo     *repo.GroupFaker
	TalkSessionRepo    *repo.TalkSession
	GroupService       service.IGroupService
	GroupMemberService service.IGroupMemberService
	TalkSessionService service.ITalkSessionService
	UserService        service.IUserService
	ContactService     service.IContactService
	Message            message.IService
}

// List 群列表接口
//
//	@Summary		群列表
//	@Description	获取用户加入的群聊列表
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupListRequest	true	"群列表请求"
//	@Success		200		{object}	web.GroupListResponse
//	@Router			/api/v1/group/list [post]
//	@Security		Bearer
func (g Group) List(ctx context.Context, in *web.GroupListRequest) (*web.GroupListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	items, err := g.GroupService.List(ctx, uid)
	if err != nil {
		return nil, err
	}

	resp := &web.GroupListResponse{
		Items: make([]*web.GroupListResponse_Item, 0, len(items)),
	}

	for _, item := range items {
		resp.Items = append(resp.Items, &web.GroupListResponse_Item{
			GroupId:   int32(item.Id),
			GroupName: item.GroupName,
			Avatar:    item.Avatar,
			Profile:   item.Profile,
			Leader:    int32(item.Leader),
			CreatorId: int32(item.CreatorId),
		})
	}

	return resp, nil
}

// Create 创建群聊接口
//
//	@Summary		创建群聊
//	@Description	创建一个新的群聊
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupCreateRequest	true	"创建群聊请求"
//	@Success		200		{object}	web.GroupCreateResponse
//	@Router			/api/v1/group/create [post]
//	@Security		Bearer
func (g Group) Create(ctx context.Context, in *web.GroupCreateRequest) (*web.GroupCreateResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	uids := make([]int, 0)
	for _, id := range sliceutil.Unique(in.UserIds) {
		uids = append(uids, int(id))
	}

	if len(uids) < 2 {
		return nil, errorx.New(400, "创建群聊失败，至少需要两个用户")
	}

	if len(uids)+1 > model.GroupMemberMaxNum {
		return nil, errorx.New(400, fmt.Sprintf("群成员数量已达到%d上限！", model.GroupMemberMaxNum))
	}

	gid, err := g.GroupService.Create(ctx, &service.GroupCreateOpt{
		UserId:    uid,
		Name:      in.Name,
		MemberIds: uids,
	})

	if err != nil {
		return nil, err
	}

	return &web.GroupCreateResponse{GroupId: int32(gid)}, nil
}

// Detail 群聊详情接口
//
//	@Summary		群详情
//	@Description	获取群聊的详细信息
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupDetailRequest	true	"群详情请求"
//	@Success		200		{object}	web.GroupDetailResponse
//	@Router			/api/v1/group/detail [post]
//	@Security		Bearer
func (g Group) Detail(ctx context.Context, in *web.GroupDetailRequest) (*web.GroupDetailResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	groupInfo, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if groupInfo.Id == 0 {
		return nil, entity.ErrGroupNotExist
	}

	isAllowInvite := int32(groupInfo.IsAllowInvite)
	if isAllowInvite == 0 {
		isAllowInvite = 1 // 库中无该列或旧数据时默认允许互加
	}
	resp := &web.GroupDetailResponse{
		GroupId:       int32(groupInfo.Id),
		GroupName:     groupInfo.Name,
		Profile:       groupInfo.Profile,
		Avatar:        groupInfo.Avatar,
		CreatedAt:     timeutil.FormatDatetime(groupInfo.CreatedAt),
		IsManager:     uid == groupInfo.CreatorId,
		IsDisturb:     0,
		IsMute:        int32(groupInfo.IsMute),
		IsOvert:       int32(groupInfo.IsOvert),
		IsAllowInvite: isAllowInvite,
		VisitCard:     g.GroupMemberRepo.GetMemberRemark(ctx, int(in.GroupId), uid),
		Notice: &web.GroupDetailResponse_Notice{
			Content:        "",
			CreatedAt:      "",
			UpdatedAt:      "",
			ModifyUserName: "",
		},
	}

	notice, err := g.GroupNoticeRepo.GetLatestNotice(ctx, int(in.GroupId))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if notice != nil {
		resp.Notice = &web.GroupDetailResponse_Notice{
			Content:        notice.Content,
			CreatedAt:      timeutil.FormatDatetime(notice.CreatedAt),
			UpdatedAt:      timeutil.FormatDatetime(notice.UpdatedAt),
			ModifyUserName: "",
		}
	}

	if g.TalkSessionRepo.IsDisturb(ctx, uid, groupInfo.Id, 2) {
		resp.IsDisturb = 1
	}

	return resp, nil
}

// MemberList 群成员列表接口
//
//	@Summary		群成员列表
//	@Description	获取群聊中的成员列表
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupMemberListRequest	true	"成员列表请求"
//	@Success		200		{object}	web.GroupMemberListResponse
//	@Router			/api/v1/group/member-list [post]
//	@Security		Bearer
func (g Group) MemberList(ctx context.Context, in *web.GroupMemberListRequest) (*web.GroupMemberListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	group, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if group != nil && group.IsDismiss == model.Yes {
		return &web.GroupMemberListResponse{}, nil
	}

	if !g.GroupMemberRepo.IsMember(ctx, int(in.GroupId), uid, false) {
		return nil, entity.ErrPermissionDenied
	}

	list := g.GroupMemberRepo.GetMembers(ctx, int(in.GroupId))

	items := make([]*web.GroupMemberListResponse_Item, 0)
	for _, item := range list {
		items = append(items, &web.GroupMemberListResponse_Item{
			UserId:   int32(item.UserId),
			Nickname: item.Nickname,
			Avatar:   item.Avatar,
			Gender:   int32(item.Gender),
			Leader:   int32(item.Leader),
			IsMute:   int32(item.IsMute),
			Remark:   item.UserCard,
			Motto:    item.Motto,
		})
	}

	slices.SortFunc(items, func(a, b *web.GroupMemberListResponse_Item) int {
		return int(a.Leader - b.Leader)
	})

	return &web.GroupMemberListResponse{Items: items}, nil
}

// Dismiss 解散群聊接口
//
//	@Summary		解散群聊
//	@Description	解散群聊（仅限群主）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupDismissRequest	true	"解散群聊请求"
//	@Success		200		{object}	web.GroupDismissResponse
//	@Router			/api/v1/group/dismiss [post]
//	@Security		Bearer
func (g Group) Dismiss(ctx context.Context, in *web.GroupDismissRequest) (*web.GroupDismissResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if !g.GroupMemberRepo.IsMaster(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	if err := g.GroupService.Dismiss(ctx, int(in.GroupId), uid); err != nil {
		return nil, err
	}

	_ = g.Message.CreateGroupSysMessage(ctx, message.CreateGroupSysMessageOption{
		GroupId: int(in.GroupId),
		Content: "该群已被群主解散！",
	})

	return &web.GroupDismissResponse{}, nil
}

// Invite 邀请加入群聊接口
//
//	@Summary		邀请入群
//	@Description	邀请好友加入群聊
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupInviteRequest	true	"邀请请求"
//	@Success		200		{object}	web.GroupInviteResponse
//	@Router			/api/v1/group/invite [post]
//	@Security		Bearer
func (g Group) Invite(ctx context.Context, in *web.GroupInviteRequest) (*web.GroupInviteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	uids := make([]int, 0)
	for _, id := range sliceutil.Unique(in.UserIds) {
		uids = append(uids, int(id))
	}

	if len(uids) == 0 {
		return nil, errorx.New(400, "邀请好友列表不能为空")
	}

	if len(uids) > model.GroupMemberMaxNum {
		return nil, errorx.New(400, fmt.Sprintf("当前群成员数量已达到%d上限！", model.GroupMemberMaxNum))
	}

	key := fmt.Sprintf("group_join:%d", in.GroupId)
	if !g.RedisLock.Lock(ctx, key, 20) {
		return nil, entity.ErrTooFrequentOperation
	}

	defer g.RedisLock.UnLock(ctx, key)

	if !g.GroupMemberRepo.IsMember(ctx, int(in.GroupId), uid, true) {
		return nil, entity.ErrPermissionDenied
	}

	group, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if group != nil && group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}

	count, err := g.GroupMemberRepo.FindCount(ctx, "group_id = ? and is_quit = ?", in.GroupId, model.No)
	if err != nil {
		return nil, err
	}

	if int(count)+len(uids) >= model.GroupMemberMaxNum {
		return nil, entity.ErrGroupMemberLimit
	}

	if err := g.GroupService.Invite(ctx, &service.GroupInviteOpt{
		UserId:    uid,
		GroupId:   int(in.GroupId),
		MemberIds: uids,
	}); err != nil {
		return nil, err
	}

	return &web.GroupInviteResponse{}, nil
}

// GetInviteFriends 获取可邀请好友列表接口
//
//	@Summary		获取可邀请好友
//	@Description	获取可以被邀请加入群组的好友列表
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GetInviteFriendsRequest	true	"获取好友请求"
//	@Success		200		{object}	web.GetInviteFriendsResponse
//	@Router			/api/v1/group/get-invite-friends [post]
//	@Security		Bearer
func (g Group) GetInviteFriends(ctx context.Context, in *web.GetInviteFriendsRequest) (*web.GetInviteFriendsResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	items, err := g.ContactService.List(ctx, uid)
	if err != nil {
		return nil, err
	}

	data := make([]*web.GetInviteFriendsResponse_Item, 0)
	if in.GroupId <= 0 {
		for _, item := range items {
			data = append(data, &web.GetInviteFriendsResponse_Item{
				UserId:   int32(item.Id),
				Nickname: item.Nickname,
				Avatar:   item.Avatar,
				Gender:   int32(item.Gender),
				Remark:   item.Remark,
			})
		}

		return &web.GetInviteFriendsResponse{
			Items: data,
		}, nil
	}

	mids := g.GroupMemberRepo.GetMemberIds(ctx, int(in.GroupId))
	if len(mids) == 0 {
		return &web.GetInviteFriendsResponse{
			Items: data,
		}, nil
	}

	for i := 0; i < len(items); i++ {
		if !slices.Contains(mids, items[i].Id) {
			data = append(data, &web.GetInviteFriendsResponse_Item{
				UserId:   int32(items[i].Id),
				Nickname: items[i].Nickname,
				Avatar:   items[i].Avatar,
				Gender:   int32(items[i].Gender),
				Remark:   items[i].Remark,
			})
		}
	}

	return &web.GetInviteFriendsResponse{
		Items: data,
	}, nil
}

// Secede 退出群聊接口
//
//	@Summary		退出群聊
//	@Description	退出群聊
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupSecedeRequest	true	"退出群聊请求"
//	@Success		200		{object}	web.GroupSecedeResponse
//	@Router			/api/v1/group/secede [post]
//	@Security		Bearer
func (g Group) Secede(ctx context.Context, in *web.GroupSecedeRequest) (*web.GroupSecedeResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := g.GroupService.Secede(ctx, int(in.GroupId), uid); err != nil {
		return nil, err
	}

	_ = g.TalkSessionService.Delete(ctx, uid, entity.ChatGroupMode, int(in.GroupId))

	return &web.GroupSecedeResponse{}, nil
}

// Setting 设置群聊接口
//
//	@Summary		群设置
//	@Description	更新群聊设置
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupSettingRequest	true	"群设置请求"
//	@Success		200		{object}	web.GroupSettingResponse
//	@Router			/api/v1/group/setting [post]
//	@Security		Bearer
func (g Group) Setting(ctx context.Context, req *web.GroupSettingRequest) (*web.GroupSettingResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	// Check if user is the group master (creator/owner)
	if !g.GroupMemberRepo.IsMaster(ctx, int(req.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	// Prepare update data
	data := make(map[string]any)
	if len(req.GroupName) > 0 {
		data["name"] = req.GroupName
	}
	if len(req.Avatar) > 0 {
		data["avatar"] = req.Avatar
	}
	if len(req.Profile) > 0 {
		data["profile"] = req.Profile
	}
	data["updated_at"] = time.Now()

	// Update group settings
	_, err := g.GroupRepo.UpdateById(ctx, int(req.GroupId), data)
	if err != nil {
		return nil, err
	}

	return &web.GroupSettingResponse{}, nil
}

// RemarkUpdate 群聊名片更新接口
//
//	@Summary		更新群名片
//	@Description	更新用户在群聊中的名片（备注）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupRemarkUpdateRequest	true	"更新名片请求"
//	@Success		200		{object}	web.GroupRemarkUpdateResponse
//	@Router			/api/v1/group/remark-update [post]
//	@Security		Bearer
func (g Group) RemarkUpdate(ctx context.Context, in *web.GroupRemarkUpdateRequest) (*web.GroupRemarkUpdateResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	_, err := g.GroupMemberRepo.UpdateByWhere(ctx, map[string]any{
		"user_card": in.Remark,
	}, "group_id = ? and user_id = ?", in.GroupId, uid)
	if err != nil {
		return nil, err
	}

	return &web.GroupRemarkUpdateResponse{}, nil
}

// RemoveMember 移出群成员接口
//
//	@Summary		移除群成员
//	@Description	从群聊中移除用户（仅限管理员）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupRemoveMemberRequest	true	"移除成员请求"
//	@Success		200		{object}	web.GroupRemoveMemberResponse
//	@Router			/api/v1/group/remove-member [post]
//	@Security		Bearer
func (g Group) RemoveMember(ctx context.Context, in *web.GroupRemoveMemberRequest) (*web.GroupRemoveMemberResponse, error) {
	uids := make([]int, 0)
	for _, id := range sliceutil.Unique(in.UserIds) {
		uids = append(uids, int(id))
	}

	if len(uids) == 0 {
		return nil, errorx.New(400, "移除成员列表不能为空！")
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if !g.GroupMemberRepo.IsLeader(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	err := g.GroupService.RemoveMember(ctx, &service.GroupRemoveMembersOpt{
		UserId:    uid,
		GroupId:   int(in.GroupId),
		MemberIds: uids,
	})

	if err != nil {
		return nil, err
	}

	return &web.GroupRemoveMemberResponse{}, nil
}

// OvertList 公开群聊列表接口
//
//	@Summary		公开群列表
//	@Description	获取公开群聊列表
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupOvertListRequest	true	"公开列表请求"
//	@Success		200		{object}	web.GroupOvertListResponse
//	@Router			/api/v1/group/overt-list [post]
//	@Security		Bearer
func (g Group) OvertList(ctx context.Context, in *web.GroupOvertListRequest) (*web.GroupOvertListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	list, err := g.GroupRepo.SearchOvertList(ctx, &repo.SearchOvertListOpt{
		Name:   in.Name,
		UserId: uid,
		Page:   int(in.Page),
		Size:   20,
	})
	if err != nil {
		return nil, err
	}

	resp := &web.GroupOvertListResponse{}
	resp.Items = make([]*web.GroupOvertListResponse_Item, 0)

	if len(list) == 0 {
		return resp, nil
	}

	ids := make([]int, 0)
	for _, val := range list {
		ids = append(ids, val.Id)
	}

	count, err := g.GroupMemberRepo.CountGroupMemberNum(ids)
	if err != nil {
		return nil, err
	}

	countMap := make(map[int]int)
	for _, member := range count {
		countMap[member.GroupId] = member.Count
	}

	for i, value := range list {
		if i >= 19 {
			break
		}

		resp.Items = append(resp.Items, &web.GroupOvertListResponse_Item{
			GroupId:   int32(value.Id),
			Type:      int32(value.Type),
			Name:      value.Name,
			Avatar:    value.Avatar,
			Profile:   value.Profile,
			Count:     int32(countMap[value.Id]),
			MaxNum:    int32(value.MaxNum),
			CreatedAt: timeutil.FormatDatetime(value.CreatedAt),
		})
	}

	resp.Next = len(list) > 19

	return resp, nil
}

// Handover 群主更换接口
//
//	@Summary		转让群主
//	@Description	将群主权限转让给另一名成员（仅限群主）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupHandoverRequest	true	"转让请求"
//	@Success		200		{object}	web.GroupHandoverResponse
//	@Router			/api/v1/group/handover [post]
//	@Security		Bearer
func (g Group) Handover(ctx context.Context, in *web.GroupHandoverRequest) (*web.GroupHandoverResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if !g.GroupMemberRepo.IsMaster(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	if uid == int(in.UserId) {
		return nil, entity.ErrPermissionDenied
	}

	err := g.GroupMemberService.Handover(ctx, int(in.GroupId), uid, int(in.UserId))
	if err != nil {
		return nil, err
	}

	members := make([]model.TalkRecordExtraGroupMember, 0)
	g.Repo.Db().Model(&model.Users{}).Select("id as user_id", "nickname").Where("id in ?", []int{uid, int(in.UserId)}).Scan(&members)

	extra := model.TalkRecordExtraTransferGroup{}
	for _, member := range members {
		if member.UserId == uid {
			extra.OldOwnerId = member.UserId
			extra.OldOwnerName = member.Nickname
		} else {
			extra.NewOwnerId = member.UserId
			extra.NewOwnerName = member.Nickname
		}
	}

	_ = g.Message.CreateGroupMessage(ctx, message.CreateGroupMessageOption{
		MsgType:    entity.ChatMsgSysGroupTransfer,
		FromId:     uid,
		ReceiverId: int(in.GroupId),
		Extra:      jsonutil.Encode(extra),
	})

	return &web.GroupHandoverResponse{}, nil
}

// AssignAdmin 分配管理员接口
//
//	@Summary		分配管理员
//	@Description	为群成员分配或移除管理员角色（仅限群主）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupAssignAdminRequest	true	"分配管理员请求"
//	@Success		200		{object}	web.GroupAssignAdminResponse
//	@Router			/api/v1/group/assign-admin [post]
//	@Security		Bearer
func (g Group) AssignAdmin(ctx context.Context, in *web.GroupAssignAdminRequest) (*web.GroupAssignAdminResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if !g.GroupMemberRepo.IsMaster(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	leader := lo.Ternary(in.Action == 1, model.GroupMemberLeaderAdmin, model.GroupMemberLeaderOrdinary)

	err := g.GroupMemberService.SetLeaderStatus(ctx, int(in.GroupId), int(in.UserId), leader)
	if err != nil {
		return nil, err
	}

	return &web.GroupAssignAdminResponse{}, nil
}

// NoSpeak 群成员禁言接口
//
//	@Summary		成员禁言
//	@Description	对特定群成员进行禁言或取消禁言（仅限管理员）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupNoSpeakRequest	true	"成员禁言请求"
//	@Success		200		{object}	web.GroupNoSpeakResponse
//	@Router			/api/v1/group/no-speak [post]
//	@Security		Bearer
func (g Group) NoSpeak(ctx context.Context, in *web.GroupNoSpeakRequest) (*web.GroupNoSpeakResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if !g.GroupMemberRepo.IsLeader(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	status := lo.Ternary(in.Action == 1, model.Yes, model.No)

	err := g.GroupMemberService.SetMuteStatus(ctx, int(in.GroupId), int(in.UserId), status)
	if err != nil {
		return nil, err
	}

	members := make([]model.TalkRecordExtraGroupMember, 0)
	g.Repo.Db().Model(&model.Users{}).Select("id as user_id", "nickname").Where("id = ?", in.UserId).Scan(&members)

	user, err := g.UsersRepo.FindByIdWithCache(ctx, uid)
	if err != nil {
		return nil, err
	}

	data := message.CreateGroupMessageOption{
		FromId:     uid,
		ReceiverId: int(in.GroupId),
	}

	if status == model.Yes {
		data.MsgType = entity.ChatMsgSysGroupMemberMuted
		data.Extra = jsonutil.Encode(model.TalkRecordExtraGroupMemberCancelMuted{
			OwnerId:   uid,
			OwnerName: user.Nickname,
			Members:   members,
		})
	} else {
		data.MsgType = entity.ChatMsgSysGroupMemberCancelMuted
		data.Extra = jsonutil.Encode(model.TalkRecordExtraGroupMemberCancelMuted{
			OwnerId:   uid,
			OwnerName: user.Nickname,
			Members:   members,
		})
	}

	_ = g.Message.CreateGroupMessage(ctx, data)

	return &web.GroupNoSpeakResponse{}, nil
}

// Mute 群禁言接口
//
//	@Summary		全员禁言
//	@Description	对整个群聊进行禁言或取消禁言（仅限管理员）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupMuteRequest	true	"全员禁言请求"
//	@Success		200		{object}	web.GroupMuteResponse
//	@Router			/api/v1/group/mute [post]
//	@Security		Bearer
func (g Group) Mute(ctx context.Context, in *web.GroupMuteRequest) (*web.GroupMuteResponse, error) {

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	group, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}

	if !g.GroupMemberRepo.IsLeader(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	data := map[string]any{
		"is_mute":    in.Action,
		"updated_at": time.Now(),
	}

	affected, err := g.GroupRepo.UpdateByWhere(ctx, data, "id = ?", in.GroupId)
	if err != nil {
		return nil, err
	}

	if affected == 0 {
		return &web.GroupMuteResponse{}, nil
	}

	user, err := g.UsersRepo.FindById(ctx, uid)
	if err != nil {
		return nil, err
	}

	var extra any
	var msgType int
	if in.Action == model.Yes {
		msgType = entity.ChatMsgSysGroupMuted
		extra = model.TalkRecordExtraGroupMuted{
			OwnerId:   user.Id,
			OwnerName: user.Nickname,
		}
	} else {
		msgType = entity.ChatMsgSysGroupCancelMuted
		extra = model.TalkRecordExtraGroupCancelMuted{
			OwnerId:   user.Id,
			OwnerName: user.Nickname,
		}
	}

	_ = g.Message.CreateGroupMessage(ctx, message.CreateGroupMessageOption{
		MsgType:    msgType,
		FromId:     uid,
		ReceiverId: int(in.GroupId),
		Extra:      jsonutil.Encode(extra),
	})

	return &web.GroupMuteResponse{}, nil
}

// AllowInvite 群内是否允许成员互加好友（群主/管理员）
//
//	@Summary		群内互加好友开关
//	@Description	设置是否允许群内成员互加好友（群主或管理员）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupAllowInviteRequest	true	"请求：action 1 不允许 2 允许"
//	@Success		200		{object}	web.GroupAllowInviteResponse
//	@Router			/api/v1/group/allowInvite [post]
//	@Security		Bearer
func (g Group) AllowInvite(ctx context.Context, in *web.GroupAllowInviteRequest) (*web.GroupAllowInviteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if in.Action != 1 && in.Action != 2 {
		return nil, errorx.New(400, "参数错误")
	}

	group, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}

	if !g.GroupMemberRepo.IsLeader(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	// 请求 action：1 不允许 2 允许；库字段 is_allow_invite：1 允许 2 不允许
	var isAllowInvite int
	if in.Action == 1 {
		isAllowInvite = 2
	} else {
		isAllowInvite = 1
	}

	_, err = g.GroupRepo.UpdateByWhere(ctx, map[string]any{
		"is_allow_invite": isAllowInvite,
		"updated_at":      time.Now(),
	}, "id = ?", in.GroupId)
	if err != nil {
		return nil, err
	}

	return &web.GroupAllowInviteResponse{}, nil
}

// Overt 群公开修改接口
//
//	@Summary		更新群可见性
//	@Description	在公开和私密之间更改群聊状态（仅限群主）
//	@Tags			群组
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GroupOvertRequest	true	"更新可见性请求"
//	@Success		200		{object}	web.GroupOvertResponse
//	@Router			/api/v1/group/overt [post]
//	@Security		Bearer
func (g Group) Overt(ctx context.Context, in *web.GroupOvertRequest) (*web.GroupOvertResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	group, err := g.GroupRepo.FindById(ctx, int(in.GroupId))
	if err != nil {
		return nil, err
	}

	if group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}

	if !g.GroupMemberRepo.IsMaster(ctx, int(in.GroupId), uid) {
		return nil, entity.ErrPermissionDenied
	}

	_, err = g.GroupRepo.UpdateByWhere(ctx, map[string]any{
		"is_overt":   in.Action,
		"updated_at": time.Now(),
	}, "id = ?", in.GroupId)

	if err != nil {
		return nil, err
	}

	return &web.GroupOvertResponse{}, nil
}

// Fakers 群马甲用户列表（group_faker 关联的 user_faker 资料）
func (g Group) Fakers(ctx context.Context, in *web.GroupFakersRequest) (*web.GroupFakersResponse, error) {
	if g.GroupFakerRepo == nil {
		return nil, errors.New("GroupFakerRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	groupID := int(in.GetGroupId())

	group, err := g.GroupRepo.FindById(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group != nil && group.IsDismiss == model.Yes {
		return &web.GroupFakersResponse{Items: []*web.GroupFakersResponse_Item{}}, nil
	}
	if !g.GroupMemberRepo.IsMember(ctx, groupID, uid, false) {
		return nil, entity.ErrPermissionDenied
	}

	list, err := g.GroupFakerRepo.ListUserFakersByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	items := make([]*web.GroupFakersResponse_Item, 0, len(list))
	for _, row := range list {
		items = append(items, &web.GroupFakersResponse_Item{
			UserId:   int32(row.UserId),
			Username: row.Username,
			Nickname: row.Nickname,
			Avatar:   row.Avatar,
		})
	}
	return &web.GroupFakersResponse{Items: items}, nil
}
