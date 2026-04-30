package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/tencentyun/tls-sig-api-v2-golang/tencentyun"
)

type Trtc struct {
	Config *config.Config
}

func NewTrtc(config *config.Config) *Trtc {
	return &Trtc{Config: config}
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
