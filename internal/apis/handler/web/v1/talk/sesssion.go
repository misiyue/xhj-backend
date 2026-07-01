package talk

import (
	"context"
	"fmt"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
)

var _ web.ITalkHandler = (*Session)(nil)

type Session struct {
	RedisLock          *cache.RedisLock
	MessageStorage     *cache.MessageStorage
	UnreadStorage      *cache.UnreadStorage
	MentionStorage     *cache.MentionStorage
	ContactRemark      *cache.ContactRemark
	ContactRepo        *repo.Contact
	UsersRepo          *repo.Users
	GroupRepo          *repo.Group
	PushMessage        *logic.PushMessage
	TalkService        service.ITalkService
	TalkSessionService service.ITalkSessionService
	UserService        service.IUserService
	GroupService       service.IGroupService
	AuthService        service.IAuthService
}

// SessionCreate 会话创建接口
//
//	@Summary		创建会话
//	@Description	与用户或群组创建一个新的聊天会话
//	@Tags			会话
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.TalkSessionCreateRequest	true	"创建会话请求"
//	@Success		200		{object}	web.TalkSessionCreateResponse
//	@Router			/api/v1/talk/session-create [post]
//	@Security		Bearer
func (s *Session) SessionCreate(ctx context.Context, in *web.TalkSessionCreateRequest) (*web.TalkSessionCreateResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	// Agent identifier for session tracking (currently not used, reserved for future client type tracking)
	agent := ""

	// 判断对方是否是自己
	if in.TalkMode == entity.ChatPrivateMode && int(in.ReceiverId) == uid {
		return nil, entity.ErrPermissionDenied
	}

	key := fmt.Sprintf("talk:list:%d-%d-%d-%s", uid, in.ReceiverId, in.TalkMode, agent)
	if !s.RedisLock.Lock(ctx, key, 10) {
		return nil, entity.ErrTooFrequentOperation
	}

	if s.AuthService.IsAuth(ctx, &service.AuthOption{
		TalkType:   int(in.TalkMode),
		UserId:     uid,
		ReceiverId: int(in.ReceiverId),
	}) != nil {
		return nil, entity.ErrPermissionDenied
	}

	result, err := s.TalkSessionService.Create(ctx, &service.TalkSessionCreateOpt{
		UserId:     uid,
		TalkType:   int(in.TalkMode),
		ReceiverId: int(in.ReceiverId),
	})
	if err != nil {
		return nil, err
	}

	item := &web.TalkSessionItem{
		Id:         int32(result.Id),
		TalkMode:   int32(result.TalkMode),
		ReceiverId: int32(result.ReceiverId),
		IsTop:      int32(result.IsTop),
		IsDisturb:  int32(result.IsDisturb),
		IsRobot:    int32(result.IsRobot),
		Name:       "",
		Avatar:     "",
		Remark:     "",
		UnreadNum:  0,
		MsgText:    "",
		UpdatedAt:  timeutil.DateTime(),
	}
	if item.TalkMode == entity.ChatPrivateMode {
		item.UnreadNum = int32(s.UnreadStorage.Get(ctx, uid, 1, int(in.ReceiverId)))

		item.Remark = s.ContactRepo.GetFriendRemark(ctx, uid, int(in.ReceiverId))
		if user, err := s.UsersRepo.FindById(ctx, result.ReceiverId); err == nil {
			item.Name = user.Nickname
			item.Avatar = user.Avatar
		}
	} else if result.TalkMode == entity.ChatGroupMode {
		if group, err := s.GroupRepo.FindById(ctx, int(in.ReceiverId)); err == nil {
			item.Name = group.Name
			item.Avatar = group.Avatar
		}
	}

	// 查询缓存消息
	if msg, err := s.MessageStorage.Get(ctx, result.TalkMode, uid, result.ReceiverId); err == nil {
		item.MsgText = msg.Content
		item.UpdatedAt = msg.Datetime
	}

	return &web.TalkSessionCreateResponse{
		Id:         item.Id,
		TalkMode:   item.TalkMode,
		ReceiverId: item.ReceiverId,
		IsTop:      item.IsTop,
		IsDisturb:  item.IsDisturb,
		IsRobot:    item.IsRobot,
		Name:       item.Name,
		Avatar:     item.Avatar,
		Remark:     item.Remark,
		UnreadNum:  item.UnreadNum,
		MsgText:    item.MsgText,
		UpdatedAt:  item.UpdatedAt,
	}, nil
}

// SessionDelete 会话删除接口
//
//	@Summary		删除会话
//	@Description	从列表中移除聊天会话
//	@Tags			会话
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.TalkSessionDeleteRequest	true	"删除会话请求"
//	@Success		200		{object}	web.TalkSessionDeleteResponse
//	@Router			/api/v1/talk/session-delete [post]
//	@Security		Bearer
func (s *Session) SessionDelete(ctx context.Context, in *web.TalkSessionDeleteRequest) (*web.TalkSessionDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := s.TalkSessionService.Delete(ctx, uid, int(in.TalkMode), int(in.ReceiverId)); err != nil {
		return nil, err
	}

	return &web.TalkSessionDeleteResponse{}, nil
}

// SessionTop 会话置顶接口
//
// 完整调用链：
//
//  1. 路由注册：api/proto/web/v1/talk.proto → talk.bff.go 生成 r.POST("/api/v1/talk/session-top", ...)
//
//  2. Gin 入口：/api/v1/talk/session-top → 本方法 SessionTop（已挂到 web.IUserHandler 上）
//
//  3. 认证：middleware 在进入 Handler 前，从 JWT 中解析出当前登录用户的 WebClaims
//
//  4. 业务：调用 TalkSessionService.Top，将当前用户在 talk_session 表中与 (talk_mode, receiver_id) 对应的那一行的 is_top 字段改为 1/2
//     - action = 1 → 置顶：is_top = 1 (model.Yes)
//     - action = 2 → 取消置顶：is_top = 2 (model.No)
//
//  5. 返回：返回最新的 is_top，前端可以直接用它更新会话列表项的置顶状态
//
//     @Summary		置顶会话
//     @Description	将聊天会话置顶或取消置顶（仅对当前登录用户生效）
//     @Tags			会话
//     @Accept			json
//     @Produce		json
//     @Param			request	body		web.TalkSessionTopRequest	true	"置顶会话请求"
//     @Success		200		{object}	web.TalkSessionTopResponse
//     @Router			/api/v1/talk/session-top [post]
//     @Security		Bearer
func (s *Session) SessionTop(ctx context.Context, in *web.TalkSessionTopRequest) (*web.TalkSessionTopResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	isTop, err := s.TalkSessionService.Top(ctx, &service.TalkSessionTopOpt{
		UserId:     uid,
		TalkMode:   int(in.TalkMode),
		ReceiverId: int(in.ReceiverId),
		Action:     int(in.Action),
	})
	if err != nil {
		return nil, err
	}

	return &web.TalkSessionTopResponse{
		IsTop: int32(isTop),
	}, nil
}

// SessionDisturb 会话免打扰接口
//
// 完整调用链与 SessionTop 类似：
//
//  1. 路由：/api/v1/talk/session-disturb → 本方法 SessionDisturb
//
//  2. 从 JWT 中取出当前登录用户 uid
//
//  3. 调用 TalkSessionService.Disturb，将 talk_session 中当前用户指定会话的 is_disturb 改为 1/2
//     - action = 1 → 开启免打扰：is_disturb = 1 (model.Yes)
//     - action = 2 → 关闭免打扰：is_disturb = 2 (model.No)
//
//  4. 前端可以根据返回的 is_disturb 更新 UI 上的“消息免打扰”开关
//
//     @Summary		会话免打扰
//     @Description	为聊天会话启用或禁用免打扰模式（仅对当前登录用户生效）
//     @Tags			会话
//     @Accept			json
//     @Produce		json
//     @Param			request	body		web.TalkSessionDisturbRequest	true	"免打扰会话请求"
//     @Success		200		{object}	web.TalkSessionDisturbResponse
//     @Router			/api/v1/talk/session-disturb [post]
//     @Security		Bearer
func (s *Session) SessionDisturb(ctx context.Context, in *web.TalkSessionDisturbRequest) (*web.TalkSessionDisturbResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	isDisturb, err := s.TalkSessionService.Disturb(ctx, &service.TalkSessionDisturbOpt{
		UserId:     uid,
		TalkMode:   int(in.TalkMode),
		ReceiverId: int(in.ReceiverId),
		Action:     int(in.Action),
	})
	if err != nil {
		return nil, err
	}

	return &web.TalkSessionDisturbResponse{
		IsDisturb: int32(isDisturb),
	}, nil
}

// SessionDetail 会话详情接口
//
//	@Summary		会话详情
//	@Description	获取聊天会话的详细设置信息
//	@Tags			会话
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.TalkSessionDetailRequest	true	"会话详情请求"
//	@Success		200		{object}	web.TalkSessionDetailResponse
//	@Router			/api/v1/talk/session-detail [post]
//	@Security		Bearer
func (s *Session) SessionDetail(ctx context.Context, in *web.TalkSessionDetailRequest) (*web.TalkSessionDetailResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	detail, err := s.TalkSessionService.SessionDetail(ctx, uid, int(in.TalkMode), int(in.ReceiverId))
	if err != nil {
		return nil, err
	}

	return &web.TalkSessionDetailResponse{
		IsTop:      int32(detail.IsTop),
		IsDisturb:  int32(detail.IsDisturb),
		RetainDays: int32(detail.RetainDays),
		SessionId:  int32(linkSessionId(detail)),
	}, nil
}

func linkSessionId(detail *model.TalkSession) int {
	if detail == nil {
		return 0
	}
	if detail.SessionId > 0 {
		return detail.SessionId
	}
	return detail.Id
}

// SessionSetRetainDays 设置私聊消息保留天数（按 session_id 同步己方与对方）
func (s *Session) SessionSetRetainDays(ctx context.Context, in *web.TalkSessionSetRetainDaysRequest) (*web.TalkSessionSetRetainDaysResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	retainDays, err := s.TalkSessionService.SetRetainDays(ctx, &service.TalkSessionSetRetainDaysOpt{
		UserId:        uid,
		LinkSessionId: int(in.GetSessionId()),
		RetainDays:    int(in.GetRetainDays()),
	})
	if err != nil {
		return nil, err
	}
	return &web.TalkSessionSetRetainDaysResponse{RetainDays: int32(retainDays)}, nil
}

// SessionList 会话列表接口
//
// 完整流程：
//  1. 路由：api.go → RegisterTalkHandler → POST /api/v1/talk/session-list（见 api/pb/web/v1/talk.bff.go）
//  2. 中间件：解析 JWT、ShouldProto 解析请求体（可为空）
//  3. 本方法：取当前用户 uid → TalkSessionService.List 查会话 → 补全备注/未读数/最后一条消息/at_me_user_count → 返回 TalkSessionListResponse
//  4. 响应：BFF 将返回值交给 core.Success → 若为 proto.Message 则用 protojson（MarshalOptions）序列化后写入 HTTP JSON
//
// 注意：at_me_user_count 依赖 api/proto/web/v1/talk.proto 中 TalkSessionItem 的字段 14；
// 若接口返回里没有 at_me_user_count，请执行 make proto 重新生成 api/pb/web/v1/talk.pb.go（含描述符），再重新编译部署。
//
//	@Summary		会话列表
//	@Description	获取用户的所有聊天会话列表
//	@Tags			会话
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.TalkSessionListRequest	true	"会话列表请求"
//	@Success		200		{object}	web.TalkSessionListResponse
//	@Router			/api/v1/talk/session-list [post]
//	@Security		Bearer
func (s *Session) SessionList(ctx context.Context, req *web.TalkSessionListRequest) (*web.TalkSessionListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	data, err := s.TalkSessionService.List(ctx, uid)
	if err != nil {
		return nil, err
	}

	friends := make([]int, 0)
	for _, item := range data {
		if item.TalkMode == 1 {
			friends = append(friends, item.ReceiverId)
		}
	}

	// 获取好友备注
	remarks, _ := s.ContactRepo.Remarks(ctx, uid, friends)

	items := make([]*web.TalkSessionItem, 0)
	for _, item := range data {
		atMeUserCount := int32(0)
		if item.TalkMode == entity.ChatGroupMode && s.MentionStorage != nil {
			if n, err := s.MentionStorage.GetMentionCount(ctx, uid, item.ReceiverId); err == nil {
				atMeUserCount = int32(n)
			}
		}
		value := &web.TalkSessionItem{
			Id:            int32(item.Id),
			TalkMode:      int32(item.TalkMode),
			ReceiverId:    int32(item.ReceiverId),
			IsTop:         int32(item.IsTop),
			IsDisturb:     int32(item.IsDisturb),
			IsRobot:       int32(item.IsRobot),
			Avatar:        item.Avatar,
			MsgText:       "...",
			UpdatedAt:     timeutil.FormatDatetime(item.UpdatedAt),
			UnreadNum:     int32(s.UnreadStorage.Get(ctx, uid, item.TalkMode, item.ReceiverId)),
			AtMeUserCount: atMeUserCount,
		}

		if item.TalkMode == entity.ChatPrivateMode {
			value.Name = item.Nickname
			value.Avatar = item.Avatar
			value.Remark = remarks[item.ReceiverId]
		} else {
			value.Name = item.GroupName
			value.Avatar = item.GroupAvatar
		}

		// 查询缓存消息
		if msg, err := s.MessageStorage.Get(ctx, item.TalkMode, uid, item.ReceiverId); err == nil {
			value.MsgText = msg.Content
			value.UpdatedAt = msg.Datetime
		}

		items = append(items, value)
	}

	return &web.TalkSessionListResponse{Items: items}, nil
}

// SessionClearUnreadNum 会话未读数清除接口
//
//	@Summary		清除会话未读数
//	@Description	将某个会话中的所有消息标记为已读
//	@Tags			会话
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.TalkSessionClearUnreadNumRequest	true	"清除未读数请求"
//	@Success		200		{object}	web.TalkSessionClearUnreadNumResponse
//	@Router			/api/v1/talk/session-clear-unread-num [post]
//	@Security		Bearer
func (s *Session) SessionClearUnreadNum(ctx context.Context, in *web.TalkSessionClearUnreadNumRequest) (*web.TalkSessionClearUnreadNumResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	s.UnreadStorage.Reset(ctx, uid, int(in.TalkMode), int(in.ReceiverId))

	if s.PushMessage != nil {
		_ = s.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventImSessionUnreadCleared,
			Payload: jsonutil.Encode(entity.SubEventImSessionUnreadClearedPayload{
				UserId:     uid,
				TalkMode:   int(in.TalkMode),
				ReceiverId: int(in.ReceiverId),
				UnreadNum:  0,
			}),
		})
	}

	return &web.TalkSessionClearUnreadNumResponse{}, nil
}
