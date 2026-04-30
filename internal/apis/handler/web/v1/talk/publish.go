package talk

import (
	"context"
	"html"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/gzydong/go-chat/internal/service/message"
)

// mapping 保存「消息类型字符串」到具体处理函数的映射，
// 比如 type=text → onSendText、type=image → onSendImage。
// 这样可以通过统一的 Send 入口，根据 type 动态分发到不同的发送逻辑。
var mapping map[string]func(ctx *gin.Context) error

// Publish 是 `/api/v1/message/send` 的 HTTP Handler，
// 负责：
//  1. 解析前端传入的基础参数（type / talk_mode / receiver_id / msg_id / quote_id）
//  2. 通过 AuthService 做会话权限校验（是不是好友、是不是群成员、是否被禁言等）
//  3. 根据消息类型将请求分发到不同的 onSendXXX 方法
//  4. onSendXXX 再调用 MessageService，把消息真正写入数据库、更新未读、推送到长连接
type Publish struct {
	AuthService    service.IAuthService
	MessageService message.IService
}

type BaseMessageRequest struct {
	Type       string `json:"type" binding:"required"`             // 消息类型 text:文本消息 image:图片消息 voice:语音消息 video:视频消息 file:文件消息 location:位置消息
	TalkMode   int    `json:"talk_mode" binding:"required,gt=0"`   // 对话类型 1:私聊 2:群聊
	ReceiverId int    `json:"receiver_id" binding:"required,gt=0"` // 接受者ID (好友ID或者群ID)
	QuoteId    string `json:"quote_id"`                            // 引用的消息ID
	MsgId      string `json:"msg_id"`                              // 消息ID（可由前端生成，用于去重、防止重复发送）
}

// Send 发送消息接口
//
//	@Summary		发送消息
//	@Description	发送各种类型的消息（文本、图片、文件等）
//	@Tags			消息
//	@Accept			json
//	@Produce		json
//	@Param			request	body		talk.BaseMessageRequest	true	"发送消息请求"
//	@Success		200		{object}	map[string]string
//	@Router			/api/v1/message/send [post]
//	@Security		Bearer
func (c *Publish) Send(ctx *gin.Context) (any, error) {
	// 1. 解析基础参数（不包含各消息类型的 body，只负责通用字段）
	in := &BaseMessageRequest{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return nil, errorx.New(400, err.Error())
	}

	// 2. 如果前端自带 msg_id，则校验长度，用来保证全局唯一性/可追踪性
	if in.MsgId != "" && len(in.MsgId) < 30 {
		return nil, errorx.New(400, "msg_id 长度必须为30个字符")
	}

	// 3. 从 JWT 中解析当前登录用户 ID
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())

	// 4. 会话权限校验：是否允许在当前会话（单聊/群聊）发送消息
	//    - 单聊：是否互为好友
	//    - 群聊：是否是群成员、是否被禁言等
	if err := c.AuthService.IsAuth(ctx.Request.Context(), &service.AuthOption{
		TalkType:          in.TalkMode,
		UserId:            uid,
		ReceiverId:        in.ReceiverId,
		IsVerifyGroupMute: true,
	}); err != nil {
		return nil, err
	}

	// 5. 根据消息 type 分发到对应的 onSendXXX 逻辑
	err := c.transfer(ctx, in.Type)
	if err != nil {
		return nil, err
	}

	// 6. 这里仅返回简单的状态给 HTTP 调用方，
	//    实际消息内容通过 WebSocket / 推送下发到在线客户端
	return map[string]string{"status": "ok"}, nil
}

type onSendTextMessage struct {
	BaseMessageRequest
	Body struct {
		Content  string `json:"content" binding:"required"`
		Mentions []int  `json:"mentions"`
	} `json:"body" binding:"required"`
}

// 文本消息
func (c *Publish) onSendText(ctx *gin.Context) error {
	in := &onSendTextMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateTextMessage(ctx.Request.Context(), message.CreateTextMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		Content:    html.EscapeString(in.Body.Content),
		QuoteId:    in.QuoteId,
		Mentions:   in.Body.Mentions,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendImageMessage struct {
	BaseMessageRequest
	Body struct {
		Url    string `json:"url" binding:"required"`
		Width  int    `json:"width" binding:"required"`
		Height int    `json:"height" binding:"required"`
		Size   int    `json:"size" binding:"required"`
	} `json:"body" binding:"required"`
}

// 图片消息
func (c *Publish) onSendImage(ctx *gin.Context) error {
	in := &onSendImageMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateImageMessage(ctx.Request.Context(), message.CreateImageMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		QuoteId:    in.QuoteId,
		Url:        in.Body.Url,
		Width:      in.Body.Width,
		Height:     in.Body.Height,
		Size:       in.Body.Size,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendVoiceMessage struct {
	BaseMessageRequest
	Body struct {
		Url      string `json:"url" binding:"required"`
		Duration int    `json:"duration" binding:"required"`
		Size     int    `json:"size" binding:"required"`
	} `json:"body" binding:"required"`
}

// 语音消息
func (c *Publish) onSendVoice(ctx *gin.Context) error {
	in := &onSendVoiceMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateVoiceMessage(ctx.Request.Context(), message.CreateVoiceMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		Url:        in.Body.Url,
		Duration:   in.Body.Duration,
		Size:       in.Body.Size,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendVideoMessage struct {
	BaseMessageRequest
	Body struct {
		Url      string `json:"url" binding:"required"`
		Duration int    `json:"duration" binding:"required"`
		Size     int    `json:"size" binding:"required"`
		Cover    string `json:"cover"`
	} `json:"body" binding:"required"`
}

// 视频消息
func (c *Publish) onSendVideo(ctx *gin.Context) error {
	in := &onSendVideoMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateVideoMessage(ctx.Request.Context(), message.CreateVideoMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		Url:        in.Body.Url,
		Duration:   in.Body.Duration,
		Size:       in.Body.Size,
		Cover:      in.Body.Cover,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendFileMessage struct {
	BaseMessageRequest
	Body struct {
		UploadId string `json:"upload_id"`
		Url      string `json:"url"`
		Name     string `json:"name"`
		Size     int    `json:"size"`
	} `json:"body" binding:"required"`
}

// 文件消息
func (c *Publish) onSendFile(ctx *gin.Context) error {
	in := &onSendFileMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateFileMessage(ctx.Request.Context(), message.CreateFileMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		UploadId:   in.Body.UploadId,
		Url:        in.Body.Url,
		Name:       in.Body.Name,
		Size:       in.Body.Size,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendCodeMessage struct {
	BaseMessageRequest
	Body struct {
		Code string `json:"code" binding:"required"`
		Lang string `json:"lang" binding:"required"`
	} `json:"body" binding:"required"`
}

// 代码消息
func (c *Publish) onSendCode(ctx *gin.Context) error {
	in := &onSendCodeMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateCodeMessage(ctx.Request.Context(), message.CreateCodeMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		Code:       in.Body.Code,
		Lang:       in.Body.Lang,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendLocationMessage struct {
	BaseMessageRequest
	Body struct {
		Latitude    string `json:"latitude" binding:"required"`
		Longitude   string `json:"longitude" binding:"required"`
		Description string `json:"description" binding:"required"`
	} `json:"body" binding:"required"`
}

// 位置消息
func (c *Publish) onSendLocation(ctx *gin.Context) error {
	in := &onSendLocationMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateLocationMessage(ctx.Request.Context(), message.CreateLocationMessage{
		MsgId:       in.MsgId,
		TalkMode:    in.TalkMode,
		FromId:      uid,
		ReceiverId:  in.ReceiverId,
		Longitude:   in.Body.Longitude,
		Latitude:    in.Body.Latitude,
		Description: in.Body.Description,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendForwardMessage struct {
	BaseMessageRequest
	Body struct {
		UserIds  []int    `json:"user_ids"`                   // 好友ID列表
		GroupIds []int    `json:"group_ids"`                  // 群ID列表
		MsgIds   []string `json:"msg_ids" binding:"required"` // 消息ID列表
		Action   int32    `json:"action" binding:"required"`  // 转发模式
	} `json:"body" binding:"required"`
}

// 转发消息
func (c *Publish) onSendForward(ctx *gin.Context) error {
	in := &onSendForwardMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	if len(in.Body.MsgIds) == 0 {
		return errorx.New(400, "请选择要转发的消息")
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	go func() {
		err := c.MessageService.CreateForwardMessage(context.Background(), message.CreateForwardMessage{
			TalkMode:   in.TalkMode,
			FromId:     uid,
			ReceiverId: in.ReceiverId,
			Action:     int(in.Body.Action),
			MsgIds:     in.Body.MsgIds,
			Gids:       in.Body.GroupIds,
			Uids:       in.Body.UserIds,
			UserId:     uid,
		})
		if err != nil {
			logger.Errorf(err.Error())
		}
	}()

	return nil
}

type onSendEmoticonMessage struct {
	BaseMessageRequest
	Body struct {
		EmoticonId int `json:"emoticon_id" binding:"required"`
	} `json:"body" binding:"required"`
}

// 表情消息
func (c *Publish) onSendEmoticon(ctx *gin.Context) error {
	in := &onSendEmoticonMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateEmoticonMessage(ctx.Request.Context(), message.CreateEmoticonMessage{
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		EmoticonId: in.Body.EmoticonId,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendCardMessage struct {
	BaseMessageRequest
	Body struct {
		UserId int `json:"user_id" binding:"required"`
	} `json:"body" binding:"required"`
}

// 名片消息
func (c *Publish) onSendCard(ctx *gin.Context) error {
	in := &onSendCardMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateBusinessCardMessage(ctx.Request.Context(), message.CreateBusinessCardMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		UserId:     in.Body.UserId,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onMixedMessageMessage struct {
	BaseMessageRequest
	Body struct {
		Items []struct {
			Type    int    `json:"type" binding:"required"`
			Content string `json:"content" binding:"required"`
		} `json:"items" binding:"required"`
	} `json:"body" binding:"required"`
}

// 图文消息
func (c *Publish) onMixedMessage(ctx *gin.Context) error {
	in := &onMixedMessageMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	items := make([]message.CreateMixedMessageItem, 0)
	for _, item := range in.Body.Items {
		items = append(items, message.CreateMixedMessageItem{
			Type:    item.Type,
			Content: item.Content,
		})
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateMixedMessage(ctx.Request.Context(), message.CreateMixedMessage{
		MsgId:       in.MsgId,
		TalkMode:    in.TalkMode,
		FromId:      uid,
		ReceiverId:  in.ReceiverId,
		QuoteId:     in.QuoteId,
		MessageList: items,
	})
	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendRTCCallMessage struct {
	BaseMessageRequest
	Body struct {
		Type     int `json:"type" binding:"required"`   // 1:语音 2:视频
		Status   int `json:"status" binding:"required"` // 1:已取消 2:未接听 3:已拒绝 4:已接通/已结束
		Duration int `json:"duration"`                  // 通话时长
	} `json:"body" binding:"required"`
}

// 音视频通话消息
func (c *Publish) onSendRTCCall(ctx *gin.Context) error {
	in := &onSendRTCCallMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateRTCCallMessage(ctx.Request.Context(), message.CreateRTCCallMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		Type:       in.Body.Type,
		Status:     in.Body.Status,
		Duration:   in.Body.Duration,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendRedEnvelopeMessage struct {
	BaseMessageRequest
	Body struct {
		EnvelopeId string  `json:"envelope_id" binding:"required"` // 红包ID
		Amount     float64 `json:"amount" binding:"required"`      // 红包金额（单位：分）
		Count      int     `json:"count" binding:"required"`       // 红包个数
		Type       string  `json:"type" binding:"required"`        // 红包类型 normal:普通红包 lucky:拼手气红包
		Greeting   string  `json:"greeting"`                       // 红包祝福语
	} `json:"body" binding:"required"`
}

// 红包消息
func (c *Publish) onSendRedEnvelope(ctx *gin.Context) error {
	in := &onSendRedEnvelopeMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateRedEnvelopeMessage(ctx.Request.Context(), message.CreateRedEnvelopeMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		EnvelopeId: in.Body.EnvelopeId,
		Amount:     in.Body.Amount,
		Count:      in.Body.Count,
		Type:       in.Body.Type,
		Greeting:   in.Body.Greeting,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

type onSendTransferMessage struct {
	BaseMessageRequest
	Body struct {
		TransferId string  `json:"transfer_id" binding:"required"` // 转账ID
		Amount     float64 `json:"amount" binding:"required"`      // 转账金额（单位：分）
		Remark     string  `json:"remark"`                         // 转账备注
	} `json:"body" binding:"required"`
}

// 转账消息
func (c *Publish) onSendTransfer(ctx *gin.Context) error {
	in := &onSendTransferMessage{}
	if err := ctx.ShouldBindBodyWith(in, binding.JSON); err != nil {
		return errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err := c.MessageService.CreateTransferMessage(ctx.Request.Context(), message.CreateTransferMessage{
		MsgId:      in.MsgId,
		TalkMode:   in.TalkMode,
		FromId:     uid,
		ReceiverId: in.ReceiverId,
		TransferId: in.Body.TransferId,
		Amount:     in.Body.Amount,
		Remark:     in.Body.Remark,
	})

	if err != nil {
		return ctx.Error(err)
	}

	return nil
}

func (c *Publish) transfer(ctx *gin.Context, typeValue string) error {
	if mapping == nil {
		mapping = make(map[string]func(ctx *gin.Context) error)
		mapping["text"] = c.onSendText
		mapping["code"] = c.onSendCode
		mapping["location"] = c.onSendLocation
		mapping["emoticon"] = c.onSendEmoticon
		mapping["image"] = c.onSendImage
		mapping["voice"] = c.onSendVoice
		mapping["video"] = c.onSendVideo
		mapping["file"] = c.onSendFile
		mapping["card"] = c.onSendCard
		mapping["forward"] = c.onSendForward
		mapping["mixed"] = c.onMixedMessage
		mapping["rtc"] = c.onSendRTCCall
		mapping["red_envelope"] = c.onSendRedEnvelope
		mapping["transfer"] = c.onSendTransfer
	}

	if call, ok := mapping[typeValue]; ok {
		return call(ctx)
	}

	return nil
}

