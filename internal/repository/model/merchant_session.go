package model

import (
	"strconv"
	"time"
)

// MerchantSessionOrderTag tags 字段与订单 id 对应
func MerchantSessionOrderTag(orderID int) string {
	return strconv.Itoa(orderID)
}

// MerchantSession 商户对话列表 merchant_session
// tags 存订单 id 字符串，与 merchant_order 关联；inviter_id/friend_id 固定为买家/卖家
type MerchantSession struct {
	Id         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	InviterId  int       `gorm:"column:inviter_id" json:"inviter_id"` // 买家 users.id
	FriendId   int       `gorm:"column:friend_id" json:"friend_id"`   // 卖家 users.id
	IsTop      int       `gorm:"column:is_top;not null;default:2" json:"is_top"`
	Status     int       `gorm:"column:status;not null;default:1" json:"status"` // 0 隐藏删除 1 展示
	Tags       string    `gorm:"column:tags;type:varchar(255);not null;default:'';uniqueIndex:uk_merchant_session_order_tag" json:"tags"`
	DeleteTime int       `gorm:"column:delete_time;not null;default:0" json:"delete_time"`
	Remark     string    `gorm:"column:remark;type:varchar(255)" json:"remark"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MerchantSession) TableName() string {
	return "merchant_session"
}
