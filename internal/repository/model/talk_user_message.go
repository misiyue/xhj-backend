package model

import "time"

type TalkUserMessage struct {
	Id         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`          // 聊天记录ID
	MsgId      string    `gorm:"column:msg_id;type:varchar(64);" json:"msg_id"`         // 消息ID
	OrgMsgId   string    `gorm:"column:org_msg_id;type:varchar(64);" json:"org_msg_id"` // 原消息ID
	SessionId  int       `gorm:"column:session_id;index;" json:"session_id"`            // 会话ID（关联talk_session.id）
	MsgType    int       `gorm:"column:msg_type;" json:"msg_type"`                      // 消息类型
	UserId     int       `gorm:"column:user_id;" json:"user_id"`                        // 消息所属用户ID
	ReceiverId int       `gorm:"column:receiver_id;" json:"receiver_id"`                // 对方用户ID
	FromId     int       `gorm:"column:from_id;" json:"from_id"`                        // 消息发送者ID
	IsRevoked  int       `gorm:"column:is_revoked;" json:"is_revoked"`                  // 是否撤回[1:否;2:是;]
	IsDeleted  int       `gorm:"column:is_deleted;" json:"is_deleted"`                  // 是否删除[1:否;2:是;]
	Extra      string    `gorm:"column:extra;type:text" json:"extra"`                   // 消息扩展字段
	Quote      string    `gorm:"column:quote;type:text" json:"quote"`                   // 引用消息
	SendTime   time.Time `gorm:"column:send_time;" json:"send_time"`                    // 发送时间
	CreatedAt  time.Time `gorm:"column:created_at;" json:"created_at"`                  // 创建时间
}

func (TalkUserMessage) TableName() string {
	return "talk_user_message"
}
