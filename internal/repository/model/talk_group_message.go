package model

import "time"

type TalkGroupMessage struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`            // 聊天记录ID
	MsgId     string    `gorm:"column:msg_id;type:varchar(64);" json:"msg_id"`           // 消息ID
	OrgMsgId  string    `gorm:"column:org_msg_id;type:varchar(64);default:''" json:"org_msg_id"` // 原消息ID（混合子段指向父消息 msg_id）
	MsgType   int       `gorm:"column:msg_type;" json:"msg_type"`                        // 消息类型
	GroupId   int       `gorm:"column:group_id;index;" json:"group_id"`                  // 群组ID
	FromId    int       `gorm:"column:from_id;" json:"from_id"`                          // 消息发送者ID
	IsRevoked int       `gorm:"column:is_revoked;" json:"is_revoked"`                    // 是否撤回[0:隐藏子段;1:是;2:否]
	IsDeleted int       `gorm:"column:is_deleted;default:2" json:"is_deleted"`           // 是否删除[1:是;2:否]
	Extra     string    `gorm:"column:extra;type:text" json:"extra"`                     // 消息扩展字段
	Quote     string    `gorm:"column:quote;type:text" json:"quote"`                     // 引用消息
	SendTime  time.Time `gorm:"column:send_time;" json:"send_time"`                      // 发送时间
	CreatedAt time.Time `gorm:"column:created_at;" json:"created_at"`                    // 创建时间
}

func (TalkGroupMessage) TableName() string {
	return "talk_group_message"
}
