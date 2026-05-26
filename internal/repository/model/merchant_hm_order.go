package model

import "time"

// MerchantHmOrder 汇美支付订单 merchant_hm_order
type MerchantHmOrder struct {
	Id            int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo       string     `gorm:"column:order_no;type:varchar(64);not null;index" json:"order_no"`
	LocalNo       string     `gorm:"column:local_no;type:varchar(64);not null;default:''" json:"local_no"`
	PayURL        string     `gorm:"column:pay_url;type:varchar(255);not null;default:''" json:"pay_url"`
	SubmitAmount  float64    `gorm:"column:submit_amount;type:decimal(10,2);not null;default:0" json:"submit_amount"`
	Status        string     `gorm:"column:status;type:varchar(16);not null;default:''" json:"status"`
	StatusText    string     `gorm:"column:status_text;type:varchar(16);not null;default:''" json:"status_text"`
	PayedAt       *time.Time `gorm:"column:payed_at" json:"payed_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (MerchantHmOrder) TableName() string {
	return "merchant_hm_order"
}
