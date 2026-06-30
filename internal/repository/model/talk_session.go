package model

import "time"

type TalkSession struct {
	Id         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                               // 聊天列表ID
	SessionId  int       `gorm:"column:session_id;" json:"session_id"`                                                       // 会话ID（关联成对的两条记录）
	TalkMode   int       `gorm:"column:talk_mode;index:idx_talk_session_user_receiver_mode,priority:3" json:"talk_mode"`     // 聊天类型[1:私信;2:群聊;]
	UserId     int       `gorm:"column:user_id;index:idx_talk_session_user_receiver_mode,priority:1" json:"user_id"`         // 用户ID
	ReceiverId int       `gorm:"column:receiver_id;index:idx_talk_session_user_receiver_mode,priority:2" json:"receiver_id"` // 接收者ID（用户ID 或 群ID）
	IsTop      int       `gorm:"column:is_top;" json:"is_top"`                                                               // 是否置顶[1:否;2:是;]
	IsDisturb  int       `gorm:"column:is_disturb;" json:"is_disturb"`                                                       // 消息免打扰[1:否;2:是;]
	IsDelete   int       `gorm:"column:is_delete;" json:"is_delete"`                                                         // 是否删除[1:否;2:是;]
	IsRobot    int       `gorm:"column:is_robot;" json:"is_robot"`                                                           // 是否机器人[1:否;2:是;]
	RetainDays int       `gorm:"column:retain_days;type:tinyint(4);default:0" json:"retain_days"`                           // 消息保留天数，0 表示不限制
	CreatedAt  time.Time `gorm:"column:created_at;" json:"created_at"`                                                       // 创建时间
	UpdatedAt  time.Time `gorm:"column:updated_at;" json:"updated_at"`                                                       // 更新时间
}

func (TalkSession) TableName() string {
	return "talk_session"
}

type TalkSessionDisplay struct {
	Id          int       `json:"id"`
	TalkMode    int       `json:"talk_mode"`
	ReceiverId  int       `json:"receiver_id"`
	IsDelete    int       `json:"is_delete"`
	IsTop       int       `json:"is_top"`
	IsRobot     int       `json:"is_robot"`
	IsDisturb   int       `json:"is_disturb"`
	Avatar      string    `json:"avatar"`
	Nickname    string    `json:"nickname"`
	GroupName   string    `json:"group_name"`
	GroupAvatar string    `json:"group_avatar"`
	UpdatedAt   time.Time `json:"updated_at"`
}
