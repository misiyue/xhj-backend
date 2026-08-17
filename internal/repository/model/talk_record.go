package model

import "time"

type TalkRecord struct {
	MsgId      string    `gorm:"column:msg_id;" json:"msg_id"`           // 消息唯一ID
	TalkType   int       `gorm:"column:talk_type;" json:"talk_type"`     // 对话类型[1:私信;2:群聊;]
	MsgType    int       `gorm:"column:msg_type;" json:"msg_type"`       // 消息类型
	UserId     int       `gorm:"column:user_id;" json:"user_id"`         // 发送者ID[0:系统用户;]
	ReceiverId int       `gorm:"column:receiver_id;" json:"receiver_id"` // 接收者ID（用户ID 或 群ID）
	IsRevoke   int       `gorm:"column:is_revoked;" json:"is_revoked"`   // 是否撤回消息[0:否;1:是;]
	QuoteId    string    `gorm:"column:quote_id;" json:"quote_id"`       // 引用消息ID
	Extra      string    `gorm:"column:extra;" json:"extra"`             // 扩展信信息
	CreatedAt  time.Time `gorm:"column:created_at;" json:"created_at"`   // 创建时间
}

type TalkMessageRecord struct {
	Id        int64     `json:"id"`         // 记录ID（用于游标分页）
	TalkMode  int       `json:"talk_mode"`  // 对话类型 1:私聊 2:群聊
	FromId    int       `json:"from_id"`    // 消息发送者
	ReceiverId int      `json:"receiver_id"` // 消息接受者
	MsgId     string    `json:"msg_id"`     // 消息ID
	MsgType   int       `json:"msg_type"`   // 消息类型
	Nickname  string    `json:"nickname"`   // 发送者昵称
	Avatar    string    `json:"avatar"`     // 发送者头像
	IsRevoked int       `json:"is_revoked"` // 消息是否已撤销
	IsRead    int       `json:"is_read"`    // 是否已读：0-否，1-是（私聊）
	SendTime  time.Time `json:"send_time"`  // 发送时间
	Extra     string    `json:"extra"`      // 额外参数
	Quote     string    `json:"quote"`      // 消息引用
}
