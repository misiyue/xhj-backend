package model

import (
	"encoding/json"
	"strings"
	"time"
)

// 商户审核状态
const (
	MerchantStatusApplyCancel = -1 // 申请注销
	MerchantStatusCancelled   = -2 // 注销
	MerchantStatusPending     = 0  // 待审核
	MerchantStatusApproved    = 1  // 审核通过
	MerchantStatusRejected    = 2  // 驳回
)

// MerchantIsOperating 是否为正常营业中的商户（审核通过且未进入注销流程）
func MerchantIsOperating(status int) bool {
	return status == MerchantStatusApproved
}

const (
	MerchantPayTypeKeyHd = "hd" // 宏达
	MerchantPayTypeKeyHm = "hm" // 汇美
)

// Merchant 对应表 merchant（建表/迁移见 provider/mysql.go、mission/migrate.go 中的 InnoDB table_options）
type Merchant struct {
	Id           int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId       int       `gorm:"column:user_id;index;not null" json:"user_id"`
	Nickname     string    `gorm:"column:nickname;type:varchar(100);not null" json:"nickname"`
	Realname     string    `gorm:"column:realname;type:varchar(100);not null" json:"realname"`
	Nation       string    `gorm:"column:nation;type:varchar(100);not null" json:"nation"`
	IdType       int       `gorm:"column:id_type;not null" json:"id_type"`
	Idcard       string    `gorm:"column:idcard;type:varchar(30);not null" json:"idcard"`
	Image        string    `gorm:"column:image;type:varchar(255);not null" json:"image"`
	Backimage    string    `gorm:"column:backimage;type:varchar(255)" json:"backimage"`
	Surety       float64   `gorm:"column:surety;type:decimal(12,4);default:0" json:"surety"`
	SuretyBillId int       `gorm:"column:surety_bill_id;default:0" json:"surety_bill_id"` // 保证金冻结凭证 id（钱包）
	Status       int       `gorm:"column:status;default:0" json:"status"`
	Reason       string    `gorm:"column:reason;type:varchar(255)" json:"reason"`
	IsLimit      int       `gorm:"column:is_limit;default:0" json:"is_limit"`
	LimitTime    int       `gorm:"column:limit_time;default:0" json:"limit_time"`
	IsFrozen     int       `gorm:"column:is_frozen;default:0" json:"is_frozen"`
	FrozenTime   int       `gorm:"column:frozen_time;default:0" json:"frozen_time"`
	IsClose      int        `gorm:"column:is_close;default:0" json:"is_close"`
	PayTypes      string     `gorm:"column:pay_types;type:varchar(255)" json:"pay_types"`
	CarReason     string     `gorm:"column:car_reason;type:varchar(255);not null;default:''" json:"car_reason"` // 注销申请驳回原因
	CancelApplyAt *time.Time `gorm:"column:cancel_apply_at" json:"cancel_apply_at"`                             // 注销申请时间
	CancelAt      *time.Time `gorm:"column:cancel_at" json:"cancel_at"`                                         // 注销时间
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Merchant) TableName() string {
	return "merchant"
}

// MerchantPayTypeLimit 支付类型配置（profile / merchant-info 返回）
type MerchantPayTypeLimit struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	PayType string  `json:"pay_type,omitempty"`
}

// ParseMerchantPayTypeRefs 解析 pay_types JSON：{"hd":1,"hm":2}，值为 merchant_payment.id
func ParseMerchantPayTypeRefs(payTypesJSON string) map[string]int {
	s := strings.TrimSpace(payTypesJSON)
	if s == "" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	out := make(map[string]int, len(raw))
	for key, val := range raw {
		var id int
		if err := json.Unmarshal(val, &id); err == nil && id > 0 {
			out[key] = id
		}
	}
	return out
}

// MerchantPayTypePaymentID 获取指定平台对应的 merchant_payment.id
func MerchantPayTypePaymentID(payTypesJSON, key string) (int, bool) {
	id, ok := ParseMerchantPayTypeRefs(payTypesJSON)[key]
	return id, ok && id > 0
}

// MerchantHasPayType 是否开通指定支付（hd / hm）
func MerchantHasPayType(payTypesJSON, key string) bool {
	_, ok := MerchantPayTypePaymentID(payTypesJSON, key)
	return ok
}
