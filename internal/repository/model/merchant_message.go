package model

import "time"

// MerchantMessage 商户对话消息 merchant_message
type MerchantMessage struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MsgId       string    `gorm:"column:msg_id;type:varchar(64)" json:"msg_id"`
	OrgMsgId    string    `gorm:"column:org_msg_id;type:varchar(64)" json:"org_msg_id"`
	SessionId   int       `gorm:"column:session_id;index" json:"session_id"`
	MsgType     int       `gorm:"column:msg_type" json:"msg_type"`
	UserId      int       `gorm:"column:user_id" json:"user_id"`
	ReceiverId  int       `gorm:"column:receiver_id" json:"receiver_id"`
	FromId      int       `gorm:"column:from_id" json:"from_id"`
	IsRevoked   int       `gorm:"column:is_revoked;default:2" json:"is_revoked"`
	IsDeleted   int       `gorm:"column:is_deleted;default:2" json:"is_deleted"`
	Extra       string    `gorm:"column:extra;type:text" json:"extra"`
	Quote       string    `gorm:"column:quote;type:text" json:"quote"`
	SendTime    time.Time `gorm:"column:send_time" json:"send_time"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (MerchantMessage) TableName() string {
	return "merchant_message"
}
