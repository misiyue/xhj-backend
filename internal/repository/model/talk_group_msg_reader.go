package model

import "time"

// TalkGroupMsgReader 群聊消息已读用户（talk_group_msg_reader）
type TalkGroupMsgReader struct {
	MsgId     string    `gorm:"column:msg_id;type:varchar(64);primaryKey" json:"msg_id"`
	UserId    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (TalkGroupMsgReader) TableName() string {
	return "talk_group_msg_reader"
}
