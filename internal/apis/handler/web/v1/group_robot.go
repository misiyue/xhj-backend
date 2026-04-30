package v1

import (
	"context"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
)

type GroupRobot struct {
	GroupRobotService service.IGroupRobotService
}

// CreateRobot 创建群机器人
//
//	@Summary		创建群机器人
//	@Description	在群聊中创建通知机器人
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GroupRobotCreateRequest	true	"创建机器人请求"
//	@Success		200		{object}	GroupRobotCreateResponse
//	@Router			/api/v1/group/robot/create [post]
func (g *GroupRobot) CreateRobot(ctx context.Context, req *GroupRobotCreateRequest) (*GroupRobotCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.GroupId <= 0 {
		return nil, errorx.New(400, "群组ID无效")
	}

	if req.RobotName == "" {
		return nil, errorx.New(400, "机器人名称不能为空")
	}

	robot, err := g.GroupRobotService.CreateRobot(ctx, int(req.GroupId), req.RobotName, req.Description, int(userId))
	if err != nil {
		return nil, err
	}

	return &GroupRobotCreateResponse{
		RobotId:     int32(robot.Id),
		RobotName:   robot.RobotName,
		WebhookUrl:  robot.WebhookUrl,
		Secret:      robot.Secret,
		Description: robot.Description,
	}, nil
}

// GetRobotList 获取群机器人列表
//
//	@Summary		获取群机器人列表
//	@Description	获取指定群聊的机器人列表
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GroupRobotListRequest	true	"获取列表请求"
//	@Success		200		{object}	GroupRobotListResponse
//	@Router			/api/v1/group/robot/list [post]
func (g *GroupRobot) GetRobotList(ctx context.Context, req *GroupRobotListRequest) (*GroupRobotListResponse, error) {
	if req.GroupId <= 0 {
		return nil, errorx.New(400, "群组ID无效")
	}

	robots, err := g.GroupRobotService.GetRobotsByGroup(ctx, int(req.GroupId))
	if err != nil {
		return nil, err
	}

	items := make([]*GroupRobotItem, 0, len(robots))
	for _, robot := range robots {
		items = append(items, &GroupRobotItem{
			RobotId:     int32(robot.Id),
			RobotName:   robot.RobotName,
			WebhookUrl:  robot.WebhookUrl,
			Description: robot.Description,
			Status:      int32(robot.Status),
			CreatedAt:   robot.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GroupRobotListResponse{
		Items: items,
	}, nil
}

// DeleteRobot 删除群机器人
//
//	@Summary		删除群机器人
//	@Description	删除指定的群机器人
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GroupRobotDeleteRequest	true	"删除机器人请求"
//	@Success		200		{object}	GroupRobotDeleteResponse
//	@Router			/api/v1/group/robot/delete [post]
func (g *GroupRobot) DeleteRobot(ctx context.Context, req *GroupRobotDeleteRequest) (*GroupRobotDeleteResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.RobotId <= 0 {
		return nil, errorx.New(400, "机器人ID无效")
	}

	if err := g.GroupRobotService.DeleteRobot(ctx, int(req.RobotId), int(userId)); err != nil {
		return nil, err
	}

	return &GroupRobotDeleteResponse{
		Success: true,
	}, nil
}

// UpdateRobot 更新群机器人
//
//	@Summary		更新群机器人
//	@Description	更新机器人信息
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GroupRobotUpdateRequest	true	"更新机器人请求"
//	@Success		200		{object}	GroupRobotUpdateResponse
//	@Router			/api/v1/group/robot/update [post]
func (g *GroupRobot) UpdateRobot(ctx context.Context, req *GroupRobotUpdateRequest) (*GroupRobotUpdateResponse, error) {
	if req.RobotId <= 0 {
		return nil, errorx.New(400, "机器人ID无效")
	}

	if err := g.GroupRobotService.UpdateRobot(ctx, int(req.RobotId), req.RobotName, req.Description); err != nil {
		return nil, err
	}

	return &GroupRobotUpdateResponse{
		Success: true,
	}, nil
}

// SendWebhookMessage Webhook发送消息
//
//	@Summary		Webhook发送消息
//	@Description	通过Webhook向群聊发送消息
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			webhook_url	path		string					true	"Webhook URL"
//	@Param			timestamp	header		string					true	"时间戳"
//	@Param			signature	header		string					true	"签名"
//	@Param			request		body		service.WebhookMessageRequest	true	"消息内容"
//	@Success		200			{object}	WebhookMessageResponse
//	@Router			/api/v1/webhook/robot/{webhook_url} [post]
func (g *GroupRobot) SendWebhookMessage(ctx context.Context, webhookUrl string, req *WebhookSendRequest) (*WebhookMessageResponse, error) {
	if webhookUrl == "" {
		return nil, errorx.New(400, "Webhook URL不能为空")
	}

	if req.Timestamp == "" {
		return nil, errorx.New(400, "时间戳不能为空")
	}

	if req.Signature == "" {
		return nil, errorx.New(400, "签名不能为空")
	}

	if err := g.GroupRobotService.SendWebhookMessage(ctx, webhookUrl, req.Timestamp, req.Signature, &req.Message); err != nil {
		return nil, err
	}

	return &WebhookMessageResponse{
		Success: true,
		Message: "消息发送成功",
	}, nil
}

// GetRobotMessages 获取机器人消息记录
//
//	@Summary		获取机器人消息记录
//	@Description	获取机器人的消息发送记录
//	@Tags			群机器人
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GroupRobotMessagesRequest	true	"获取消息记录请求"
//	@Success		200		{object}	GroupRobotMessagesResponse
//	@Router			/api/v1/group/robot/messages [post]
func (g *GroupRobot) GetRobotMessages(ctx context.Context, req *GroupRobotMessagesRequest) (*GroupRobotMessagesResponse, error) {
	if req.RobotId <= 0 {
		return nil, errorx.New(400, "机器人ID无效")
	}

	messages, err := g.GroupRobotService.GetRobotMessages(ctx, int(req.RobotId), int(req.Limit))
	if err != nil {
		return nil, err
	}

	items := make([]*GroupRobotMessageItem, 0, len(messages))
	for _, msg := range messages {
		items = append(items, &GroupRobotMessageItem{
			Id:        int32(msg.Id),
			MsgType:   msg.MsgType,
			Content:   msg.Content,
			Status:    int32(msg.Status),
			CreatedAt: msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GroupRobotMessagesResponse{
		Items: items,
	}, nil
}

// Helper functions

// Request and Response types

type GroupRobotCreateRequest struct {
	GroupId     int32  `json:"group_id"`
	RobotName   string `json:"robot_name"`
	Description string `json:"description"`
}

type GroupRobotCreateResponse struct {
	RobotId     int32  `json:"robot_id"`
	RobotName   string `json:"robot_name"`
	WebhookUrl  string `json:"webhook_url"`
	Secret      string `json:"secret"`
	Description string `json:"description"`
}

type GroupRobotListRequest struct {
	GroupId int32 `json:"group_id"`
}

type GroupRobotListResponse struct {
	Items []*GroupRobotItem `json:"items"`
}

type GroupRobotItem struct {
	RobotId     int32  `json:"robot_id"`
	RobotName   string `json:"robot_name"`
	WebhookUrl  string `json:"webhook_url"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type GroupRobotDeleteRequest struct {
	RobotId int32 `json:"robot_id"`
}

type GroupRobotDeleteResponse struct {
	Success bool `json:"success"`
}

type GroupRobotUpdateRequest struct {
	RobotId     int32  `json:"robot_id"`
	RobotName   string `json:"robot_name"`
	Description string `json:"description"`
}

type GroupRobotUpdateResponse struct {
	Success bool `json:"success"`
}

type WebhookSendRequest struct {
	Timestamp string                        `json:"-"` // From header
	Signature string                        `json:"-"` // From header
	Message   service.WebhookMessageRequest `json:"message"`
}

type WebhookMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type GroupRobotMessagesRequest struct {
	RobotId int32 `json:"robot_id"`
	Limit   int32 `json:"limit"`
}

type GroupRobotMessagesResponse struct {
	Items []*GroupRobotMessageItem `json:"items"`
}

type GroupRobotMessageItem struct {
	Id        int32  `json:"id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
	Status    int32  `json:"status"`
	CreatedAt string `json:"created_at"`
}
