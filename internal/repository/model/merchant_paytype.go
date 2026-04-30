package model

import "time"

// MerchantPaytype 用户商户收款支付方式 merchant_paytype
type MerchantPaytype struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId    int       `gorm:"column:user_id;index;not null" json:"user_id"`
	TypeId    int       `gorm:"column:type_id;not null" json:"type_id"`
	Account   string    `gorm:"column:account;type:varchar(100)" json:"account"`
	Nickname  string    `gorm:"column:nickname;type:varchar(100)" json:"nickname"`
	OpenBank  string    `gorm:"column:open_bank;type:varchar(255)" json:"open_bank"`
	IsDelete  int       `gorm:"column:is_delete;not null;default:0" json:"is_delete"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MerchantPaytype) TableName() string {
	return "merchant_paytype"
}
