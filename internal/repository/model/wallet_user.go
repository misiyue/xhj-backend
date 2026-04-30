package model

import (
	"time"
)

// WalletUser stores the mapping between IM user and wallet platform user.
type WalletUser struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	UserId    int       `gorm:"column:user_id;uniqueIndex" json:"user_id"`         // IM用户ID
	WalletUID int       `gorm:"column:wallet_uid" json:"wallet_uid"`               // 钱包平台用户ID (u_uid)
	UUID      string    `gorm:"column:uuid;size:64" json:"uuid"`                   // 钱包平台UUID
	Phone     string    `gorm:"column:phone;size:64" json:"phone"`                 // 注册手机号/账号
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (w WalletUser) TableName() string {
	return "wallet_user"
}
