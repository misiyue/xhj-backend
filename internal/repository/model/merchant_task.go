package model

import "time"

// 商户挂售任务状态
const (
	MerchantTaskStatusPending = 0 // 待交易
	MerchantTaskStatusTrading = 1 // 交易中
	MerchantTaskStatusDone    = 2 // 完成交易
)

// 出售币种
const (
	MerchantTaskCurrencyUCoin = 1 // U币
)

// MerchantTask 对应表 merchant_task
type MerchantTask struct {
	Id           int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId       int       `gorm:"column:user_id;index;not null" json:"user_id"`
	CurrencyType int       `gorm:"column:currency_type;not null;default:1" json:"currency_type"`
	Price        float64   `gorm:"column:price;type:decimal(10,2);not null;default:0" json:"price"`
	Count        float64   `gorm:"column:count;type:float(16,4);not null;default:0" json:"count"`
	Paytype      string    `gorm:"column:paytype;type:varchar(32)" json:"paytype"`
	Status       int       `gorm:"column:status;not null;default:0" json:"status"`
	IsUp         int       `gorm:"column:is_up;not null;default:0" json:"is_up"`
	UpTime       int       `gorm:"column:up_time;not null;default:0" json:"up_time"`
	IsDeleted    int       `gorm:"column:is_deleted;not null;default:0" json:"is_deleted"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (MerchantTask) TableName() string {
	return "merchant_task"
}
