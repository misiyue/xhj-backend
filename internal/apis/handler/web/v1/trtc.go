package v1

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/external/push"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/tencentyun/tls-sig-api-v2-golang/tencentyun"
)

type Trtc struct {
	Config          *config.Config
	UsersRepo       *repo.Users
	TalkSessionRepo *repo.TalkSession
}

type TrtcSignatureResponse struct {
	SdkAppId int    `json:"sdk_app_id"`
	UserSig  string `json:"user_sig"`
}

// GetSignature 获取 TRTC UserSig
//
//	@Summary		获取 TRTC UserSig
//	@Description	获取 TRTC UserSig
//	@Tags			TRTC
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	TrtcSignatureResponse
//	@Router			/api/v1/trtc/user-sig [get]
func (h *Trtc) GetSignature(ctx *gin.Context) (any, error) {
	session, err := middleware.FormContext[entity.WebClaims](ctx.Request.Context())
	if err != nil {
		return nil, errorx.New(401, "未登录")
	}

	userId := session.UserId
	if userId == 0 {
		return nil, errorx.New(401, "未登录")
	}

	if h == nil || h.Config == nil || h.Config.Trtc == nil {
		return nil, errorx.New(500, "TRTC 配置未设置")
	}

	sdkAppId := h.Config.Trtc.SdkAppId
	secretKey := h.Config.Trtc.SecretKey

	// Convert userId to string as TRTC expects string user ID
	sig, err := tencentyun.GenUserSig(sdkAppId, secretKey, strconv.Itoa(int(userId)), 86400*7)
	if err != nil {
		return nil, errorx.New(500, "生成签名失败")
	}

	return &TrtcSignatureResponse{
		SdkAppId: sdkAppId,
		UserSig:  sig,
	}, nil
}

type CallPushRequest struct {
	ToUserId int `json:"to_user_id"`
	RoomId   int `json:"room_id"`
	CallType int `json:"call_type"` // 1: 语音 2: 视频
}

type CallPushResponse struct {
	Success bool `json:"success"`
}

// CallPush 发起音视频通话时向对方推送 VoIP 通知
//
//	@Summary		音视频通话 VoIP 推送
//	@Description	用户发起音视频通话时，向对方发送 OneSignal VoIP 推送
//	@Tags			TRTC
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CallPushRequest	true	"通话推送请求"
//	@Success		200		{object}	CallPushResponse
//	@Router			/api/v1/trtc/call-push [post]
func (h *Trtc) CallPush(ctx context.Context, req *CallPushRequest) (*CallPushResponse, error) {
	session, err := middleware.FormContext[entity.WebClaims](ctx)
	if err != nil {
		return nil, errorx.New(401, "未登录")
	}

	fromUserId := int(session.UserId)
	if fromUserId <= 0 {
		return nil, errorx.New(401, "未登录")
	}
	if req == nil {
		return nil, errorx.New(400, "请求参数错误")
	}
	if req.ToUserId <= 0 {
		return nil, errorx.New(400, "对方用户ID无效")
	}
	if req.ToUserId == fromUserId {
		return nil, errorx.New(400, "不能向自己发起通话")
	}
	if req.RoomId <= 0 {
		return nil, errorx.New(400, "房间ID无效")
	}
	if req.CallType != 1 && req.CallType != 2 {
		return nil, errorx.New(400, "通话类型无效")
	}
	if h == nil || h.Config == nil || h.Config.Push == nil || !h.Config.Push.Valid() {
		return nil, errorx.New(500, "推送服务未配置")
	}
	if h.UsersRepo == nil {
		return nil, errorx.New(500, "用户服务未初始化")
	}

	targetUser, err := h.UsersRepo.FindByIdWithCache(ctx, req.ToUserId)
	if err != nil {
		return nil, errorx.New(404, "对方用户不存在")
	}
	if targetUser == nil || targetUser.IsDisabled() {
		return nil, errorx.New(400, "对方用户不可用")
	}
	if targetUser.IsSubscribe != model.UsersSubscribeYes {
		return &CallPushResponse{Success: false}, nil
	}
	if h.TalkSessionRepo != nil &&
		h.TalkSessionRepo.IsDisturb(ctx, req.ToUserId, fromUserId, entity.ChatPrivateMode) {
		return &CallPushResponse{Success: false}, nil
	}

	fromUser, err := h.UsersRepo.FindByIdWithCache(ctx, fromUserId)
	if err != nil || fromUser == nil {
		return nil, errorx.New(500, "获取用户信息失败")
	}

	if err := push.SendVoIPToUser(req.ToUserId, push.VoIPCallData{
		Event:          entity.PushEventImCallInvite,
		FromUserId:     fromUserId,
		ToUserId:       req.ToUserId,
		RoomId:         req.RoomId,
		CallType:       req.CallType,
		FromUserName:   fromUser.Nickname,
		FromUserAvatar: fromUser.Avatar,
	}); err != nil {
		logger.Errorf("trtc call voip push err: from_id=%d to_id=%d %s", fromUserId, req.ToUserId, err.Error())
		return nil, errorx.New(500, "推送失败")
	}

	return &CallPushResponse{Success: true}, nil
}
