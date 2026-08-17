package model

import (
	"encoding/json"
	"strings"
	"time"
)

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
	Total        float64   `gorm:"column:total;type:float(16,4)" json:"total"`
	Paytype      string    `gorm:"column:paytype;type:varchar(1024)" json:"paytype"`
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

// MerchantTaskPaytypeConfigured 挂单 paytype 是否配置了有效支付方式
func MerchantTaskPaytypeConfigured(paytype string) bool {
	s := strings.TrimSpace(paytype)
	if s == "" {
		return false
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &obj); err == nil {
		if len(obj) == 0 {
			return false
		}
		for _, raw := range obj {
			if merchantTaskPaytypeValueConfigured(raw) {
				return true
			}
		}
		return false
	}

	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		if len(arr) == 0 {
			return false
		}
		for _, raw := range arr {
			if merchantTaskPaytypeValueConfigured(raw) {
				return true
			}
		}
		return false
	}

	return false
}

func merchantTaskPaytypeValueConfigured(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "0" || s == "false" {
		return false
	}
	if s == "{}" || s == "[]" {
		return false
	}

	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		f, _ := n.Float64()
		return f != 0
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str) != ""
	}

	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err == nil {
		if len(nested) == 0 {
			return false
		}
		for _, v := range nested {
			if merchantTaskPaytypeValueConfigured(v) {
				return true
			}
		}
		return false
	}

	return true
}
