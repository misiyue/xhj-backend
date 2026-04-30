package repo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type GroupRobot struct {
	db *gorm.DB
}

func NewGroupRobot(db *gorm.DB) *GroupRobot {
	return &GroupRobot{db: db}
}

// Create 创建群机器人
func (g *GroupRobot) Create(ctx context.Context, robot *model.GroupRobot) error {
	// 生成webhook URL和密钥
	robot.WebhookUrl = g.GenerateWebhookUrl(robot.GroupId)
	robot.Secret = g.GenerateSecret()
	return g.db.WithContext(ctx).Create(robot).Error
}

// FindById 根据ID查找机器人
func (g *GroupRobot) FindById(ctx context.Context, id int) (*model.GroupRobot, error) {
	var robot model.GroupRobot
	err := g.db.WithContext(ctx).Where("id = ?", id).First(&robot).Error
	return &robot, err
}

// FindByWebhookUrl 根据Webhook URL查找机器人
func (g *GroupRobot) FindByWebhookUrl(ctx context.Context, webhookUrl string) (*model.GroupRobot, error) {
	var robot model.GroupRobot
	err := g.db.WithContext(ctx).Where("webhook_url = ?", webhookUrl).First(&robot).Error
	return &robot, err
}

// FindByGroupId 根据群组ID查找机器人列表
func (g *GroupRobot) FindByGroupId(ctx context.Context, groupId int) ([]*model.GroupRobot, error) {
	var robots []*model.GroupRobot
	err := g.db.WithContext(ctx).
		Where("group_id = ? AND status = ?", groupId, model.GroupRobotStatusActive).
		Find(&robots).Error
	return robots, err
}

// Update 更新机器人信息
func (g *GroupRobot) Update(ctx context.Context, id int, updates map[string]interface{}) error {
	return g.db.WithContext(ctx).
		Model(&model.GroupRobot{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Delete 删除机器人
func (g *GroupRobot) Delete(ctx context.Context, id int) error {
	return g.db.WithContext(ctx).
		Model(&model.GroupRobot{}).
		Where("id = ?", id).
		Update("status", model.GroupRobotStatusDisabled).Error
}

// SaveMessage 保存机器人消息记录
func (g *GroupRobot) SaveMessage(ctx context.Context, message *model.GroupRobotMessage) error {
	return g.db.WithContext(ctx).Create(message).Error
}

// GetMessages 获取机器人消息列表
func (g *GroupRobot) GetMessages(ctx context.Context, robotId int, limit int) ([]*model.GroupRobotMessage, error) {
	var messages []*model.GroupRobotMessage
	err := g.db.WithContext(ctx).
		Where("robot_id = ?", robotId).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GenerateWebhookUrl 生成Webhook URL
func (g *GroupRobot) GenerateWebhookUrl(groupId int) string {
	// 生成唯一的webhook标识
	timestamp := time.Now().UnixNano()
	data := fmt.Sprintf("%d-%d", groupId, timestamp)
	hash := sha256.Sum256([]byte(data))
	token := hex.EncodeToString(hash[:])[:32]
	
	return fmt.Sprintf("/api/v1/webhook/robot/%s", token)
}

// GenerateSecret 生成签名密钥
func (g *GroupRobot) GenerateSecret() string {
	timestamp := time.Now().UnixNano()
	data := fmt.Sprintf("secret-%d", timestamp)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// VerifySignature 验证签名
func (g *GroupRobot) VerifySignature(secret, timestamp, signature string) bool {
	// 使用HMAC-SHA256验证签名
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}
