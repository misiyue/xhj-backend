package model

import (
	"strconv"
	"time"
)

// MerchantSessionOrderTag tags 存最近关联的订单 id（展示用）
func MerchantSessionOrderTag(orderID int) string {
	return strconv.Itoa(orderID)
}

// MerchantSession 商户对话列表 merchant_session
// 每人一条会话栏记录：inviter_id=自己，friend_id=对方；成对两条记录共享 session_id 关联消息
type MerchantSession struct {
	Id         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionId  int       `gorm:"column:session_id;default:0;index" json:"session_id"` // 成对会话共享 id（消息 merchant_message.session_id）
	InviterId  int       `gorm:"column:inviter_id;index:idx_merchant_session_owner_peer,priority:1" json:"inviter_id"` // 自己 users.id
	FriendId   int       `gorm:"column:friend_id;index:idx_merchant_session_owner_peer,priority:2" json:"friend_id"`   // 对方 users.id
	IsTop      int       `gorm:"column:is_top;not null;default:2" json:"is_top"`
	Status     int       `gorm:"column:status;not null;default:1" json:"status"` // 0 隐藏 1 展示
	Tags       string    `gorm:"column:tags;type:varchar(255);not null;default:''" json:"tags"`
	DeleteTime int       `gorm:"column:delete_time;not null;default:0" json:"delete_time"` // 0 未删除；非 0 表示已删，发消息时重开新记录
	Remark     string    `gorm:"column:remark;type:varchar(255)" json:"remark"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MerchantSession) TableName() string {
	return "merchant_session"
}
