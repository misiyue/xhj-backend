package model

import "time"

const (
	RootStatusDeleted = -1
	RootStatusNormal  = 0
	RootStatusDisable = 1
)

type Robot struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`                     // 机器人ID
	UserId    int       `gorm:"column:user_id;" json:"user_id"`                                     // 关联用户ID
	RobotName string    `gorm:"column:robot_name;type:varchar(64);" json:"robot_name"`              // 机器人名称
	Describe  string    `gorm:"column:describe;type:varchar(255);" json:"describe"`                 // 描述信息
	Logo      string    `gorm:"column:logo;type:varchar(255);" json:"logo"`                         // 机器人logo
	IsTalk    int       `gorm:"column:is_talk;" json:"is_talk"`                                     // 可发送消息[0:否;1:是;]
	Status    int       `gorm:"column:status;index:idx_robot_type_status,priority:2" json:"status"` // 状态[-1:已删除;0:正常;1:已禁用;]
	Type      int       `gorm:"column:type;index:idx_robot_type_status,priority:1" json:"type"`     // 机器人类型
	CreatedAt time.Time `gorm:"column:created_at;" json:"created_at"`                               // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at;" json:"updated_at"`                               // 更新时间
}

func (Robot) TableName() string {
	return "robot"
}
