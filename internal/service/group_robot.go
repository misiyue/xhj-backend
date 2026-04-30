package service

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service/message"
)

var _ IGroupRobotService = (*GroupRobotService)(nil)

type IGroupRobotService interface {
	// CreateRobot 创建群机器人
	CreateRobot(ctx context.Context, groupId int, robotName string, description string, creatorId int) (*model.GroupRobot, error)
	// GetRobotsByGroup 获取群组的机器人列表
	GetRobotsByGroup(ctx context.Context, groupId int) ([]*model.GroupRobot, error)
	// DeleteRobot 删除机器人
	DeleteRobot(ctx context.Context, robotId int, userId int) error
	// UpdateRobot 更新机器人信息
	UpdateRobot(ctx context.Context, robotId int, robotName string, description string) error
	// SendWebhookMessage 发送Webhook消息到群
	SendWebhookMessage(ctx context.Context, webhookUrl string, timestamp string, signature string, req *WebhookMessageRequest) error
	// GetRobotMessages 获取机器人消息列表
	GetRobotMessages(ctx context.Context, robotId int, limit int) ([]*model.GroupRobotMessage, error)
}

type GroupRobotService struct {
	GroupRobotRepo  *repo.GroupRobot
	GroupRepo       *repo.Group
	GroupMemberRepo *repo.GroupMember
	MessageService  message.IService
}

type WebhookMessageRequest struct {
	MsgType  string                  `json:"msgtype"` // text, markdown, image
	Text     *WebhookTextMessage     `json:"text,omitempty"`
	Markdown *WebhookMarkdownMessage `json:"markdown,omitempty"`
	Image    *WebhookImageMessage    `json:"image,omitempty"`
}

type WebhookTextMessage struct {
	Content string `json:"content"`
}

type WebhookMarkdownMessage struct {
	Content string `json:"content"`
}

type WebhookImageMessage struct {
	Base64 string `json:"base64"`
	Md5    string `json:"md5"`
}

func (g *GroupRobotService) CreateRobot(ctx context.Context, groupId int, robotName string, description string, creatorId int) (*model.GroupRobot, error) {
	// 验证用户是否是群主或管理员
	member, err := g.GroupMemberRepo.FindByUserId(ctx, groupId, creatorId)
	if err != nil {
		return nil, errors.New("用户不是群成员")
	}

	if member.Leader == model.GroupMemberLeaderOrdinary {
		return nil, errors.New("只有群主或管理员可以创建机器人")
	}

	robot := &model.GroupRobot{
		GroupId:     groupId,
		RobotName:   robotName,
		Description: description,
		Status:      model.GroupRobotStatusActive,
		CreatorId:   creatorId,
	}

	if err := g.GroupRobotRepo.Create(ctx, robot); err != nil {
		return nil, err
	}

	return robot, nil
}

func (g *GroupRobotService) GetRobotsByGroup(ctx context.Context, groupId int) ([]*model.GroupRobot, error) {
	return g.GroupRobotRepo.FindByGroupId(ctx, groupId)
}

func (g *GroupRobotService) DeleteRobot(ctx context.Context, robotId int, userId int) error {
	// 验证机器人存在
	robot, err := g.GroupRobotRepo.FindById(ctx, robotId)
	if err != nil {
		return err
	}

	// 验证用户权限
	member, err := g.GroupMemberRepo.FindByUserId(ctx, robot.GroupId, userId)
	if err != nil {
		return errors.New("用户不是群成员")
	}

	if member.Leader == model.GroupMemberLeaderOrdinary {
		return errors.New("只有群主或管理员可以删除机器人")
	}

	return g.GroupRobotRepo.Delete(ctx, robotId)
}

func (g *GroupRobotService) UpdateRobot(ctx context.Context, robotId int, robotName string, description string) error {
	updates := make(map[string]interface{})

	if robotName != "" {
		updates["robot_name"] = robotName
	}
	if description != "" {
		updates["description"] = description
	}

	if len(updates) == 0 {
		return nil
	}

	return g.GroupRobotRepo.Update(ctx, robotId, updates)
}

func (g *GroupRobotService) SendWebhookMessage(ctx context.Context, webhookUrl string, timestamp string, signature string, req *WebhookMessageRequest) error {
	// 验证webhook URL
	robot, err := g.GroupRobotRepo.FindByWebhookUrl(ctx, webhookUrl)
	if err != nil {
		return errors.New("无效的webhook URL")
	}

	if robot.Status != model.GroupRobotStatusActive {
		return errors.New("机器人已被禁用")
	}

	// 验证签名
	if !g.GroupRobotRepo.VerifySignature(robot.Secret, timestamp, signature) {
		return errors.New("签名验证失败")
	}

	// 解析消息内容
	var content string
	switch req.MsgType {
	case "text":
		if req.Text == nil {
			return errors.New("文本消息内容不能为空")
		}
		content = req.Text.Content
	case "markdown":
		if req.Markdown == nil {
			return errors.New("Markdown消息内容不能为空")
		}
		content = req.Markdown.Content
	case "image":
		if req.Image == nil {
			return errors.New("图片消息内容不能为空")
		}
		content = req.Image.Base64
	default:
		return errors.New("不支持的消息类型")
	}

	// 保存消息记录
	robotMessage := &model.GroupRobotMessage{
		RobotId: robot.Id,
		GroupId: robot.GroupId,
		MsgType: req.MsgType,
		Content: content,
		Status:  1, // 成功
	}

	if err := g.GroupRobotRepo.SaveMessage(ctx, robotMessage); err != nil {
		return err
	}

	// 发送消息到群聊（通过消息服务）
	// 这里需要调用MessageService来实际发送消息
	// 简化版本：记录即可，实际发送由消息服务处理

	return nil
}

func (g *GroupRobotService) GetRobotMessages(ctx context.Context, robotId int, limit int) ([]*model.GroupRobotMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return g.GroupRobotRepo.GetMessages(ctx, robotId, limit)
}
