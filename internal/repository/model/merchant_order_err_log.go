package model

import "time"

// MerchantOrderErrLog 第三方下单失败日志 merchant_order_err_log
type MerchantOrderErrLog struct {
	No        string    `gorm:"column:no;type:varchar(64);not null;index" json:"no"`
	Request   string    `gorm:"column:request;type:text" json:"request"`
	Respond   string    `gorm:"column:respond;type:text" json:"respond"`
	URL       string    `gorm:"column:url;type:varchar(255)" json:"url"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (MerchantOrderErrLog) TableName() string {
	return "merchant_order_err_log"
}
