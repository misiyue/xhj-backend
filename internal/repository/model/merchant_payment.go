package model

import "time"

const (
	MerchantPaymentPlatformHd = "hd" // 宏达
	MerchantPaymentPlatformHm = "hm" // 汇美
)

// MerchantPayment 第三方支付通道 merchant_payment
type MerchantPayment struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"column:title;type:varchar(16)" json:"title"`
	Code      string    `gorm:"column:code;type:varchar(8);not null;default:'';uniqueIndex:code_platform_idx,priority:1" json:"code"`
	Min       *int      `gorm:"column:min" json:"min"`
	Max       *int      `gorm:"column:max" json:"max"`
	Platform  string    `gorm:"column:platform;type:varchar(4);uniqueIndex:code_platform_idx,priority:2" json:"platform"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (MerchantPayment) TableName() string {
	return "merchant_payment"
}
