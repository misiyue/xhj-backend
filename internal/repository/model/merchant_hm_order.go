package model

import "time"

// MerchantHmOrder 汇美支付订单 merchant_hm_order
type MerchantHmOrder struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderId   string    `gorm:"column:order_id;type:varchar(64);not null;index" json:"order_id"`
	PayURL    string    `gorm:"column:pay_url;type:varchar(255);not null;default:''" json:"pay_url"`
	Restate   *int      `gorm:"column:restate" json:"restate"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MerchantHmOrder) TableName() string {
	return "merchant_hm_order"
}
