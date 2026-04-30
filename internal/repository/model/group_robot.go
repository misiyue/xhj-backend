package model

import "time"

const (
	GroupRobotStatusActive   = 1 // 活跃
	GroupRobotStatusDisabled = 0 // 禁用
)

// GroupRobot 群机器人表
type GroupRobot struct {
	Id          int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`                                               // 机器人ID
	GroupId     int       `gorm:"column:group_id;index;index:idx_group_robot_group_status,priority:1;not null" json:"group_id"` // 群组ID
	RobotName   string    `gorm:"column:robot_name;type:varchar(64);not null" json:"robot_name"`                                // 机器人名称
	WebhookUrl  string    `gorm:"column:webhook_url;type:varchar(255);uniqueIndex" json:"webhook_url"`                          // Webhook URL
	Secret      string    `gorm:"column:secret;type:varchar(255)" json:"secret"`                                                // 签名密钥
	Description string    `gorm:"column:description;type:varchar(255)" json:"description"`                                      // 描述
	Status      int       `gorm:"column:status;default:1;index:idx_group_robot_group_status,priority:2" json:"status"`          // 状态 0:禁用 1:活跃
	CreatorId   int       `gorm:"column:creator_id;index" json:"creator_id"`                                                    // 创建者ID
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`                                                          // 创建时间
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`                                                          // 更新时间
}

func (GroupRobot) TableName() string {
	return "group_robot"
}

// GroupRobotMessage 机器人消息记录表
type GroupRobotMessage struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	RobotId   int       `gorm:"column:robot_id;index;index:idx_robot_message_robot_created,priority:1" json:"robot_id"` // 机器人ID
	GroupId   int       `gorm:"column:group_id;index" json:"group_id"`                                                  // 群组ID
	MsgType   string    `gorm:"column:msg_type;type:varchar(32)" json:"msg_type"`                                       // 消息类型 text, markdown, image
	Content   string    `gorm:"column:content;type:text" json:"content"`                                                // 消息内容
	Extra     string    `gorm:"column:extra;type:text" json:"extra"`                                                    // 额外数据(JSON)
	Status    int       `gorm:"column:status;default:1" json:"status"`                                                  // 发送状态 0:失败 1:成功
	SendAt    time.Time `gorm:"column:send_at" json:"send_at"`                                                          // 发送时间
	CreatedAt time.Time `gorm:"column:created_at;index:idx_robot_message_robot_created,priority:2" json:"created_at"`   // 创建时间
}

func (GroupRobotMessage) TableName() string {
	return "group_robot_message"
}
