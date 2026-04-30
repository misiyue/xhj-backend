package model

import "time"

type TalkGroupMessageDel struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	GroupId   int       `gorm:"column:group_id;" json:"group_id"`                                          // 用户ID
	UserId    int       `gorm:"column:user_id;index:idx_group_msg_del_user_msg,priority:1" json:"user_id"` // 用户ID
	MsgId     string    `gorm:"column:msg_id;index:idx_group_msg_del_user_msg,priority:2" json:"msg_id"`   // 聊天记录ID
	CreatedAt time.Time `gorm:"column:created_at;" json:"created_at"`                                      // 创建时间
}

func (t TalkGroupMessageDel) TableName() string {
	return "talk_group_message_del"
}
