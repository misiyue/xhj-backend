package model

import "time"

// 订单状态
const (
	MerchantOrderStatusPendingPay = 0 // 待支付
	MerchantOrderStatusPaid       = 1 // 已支付，待卖方放币
	MerchantOrderStatusDone       = 2 // 已完成
	MerchantOrderStatusCancelled  = 3 // 已取消
)

// 申诉方（与表字段 appeal_id 含义一致）
const (
	MerchantOrderAppealSideBuyer  = 1
	MerchantOrderAppealSideSeller = 2
)

// 支付类型 pay_type_id
const (
	MerchantPayTypeAlipay = 1 // 支付宝
	MerchantPayTypeWechat = 2 // 微信
	MerchantPayTypeBank   = 3 // 银行卡
	MerchantPayTypeHd     = 4 // 宏达
	MerchantPayTypeHm     = 5 // 汇美
)

// MerchantOrderPayTypeSupportsOnlinePay 订单是否可走第三方在线支付（后续可扩展更多 pay_type_id）
func MerchantOrderPayTypeSupportsOnlinePay(payTypeID int) bool {
	switch payTypeID {
	case MerchantPayTypeHd:
		return true
	default:
		return false
	}
}

// MerchantOrder 商户订单 merchant_order
type MerchantOrder struct {
	Id              int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderId         string    `gorm:"column:order_id;type:varchar(64);not null;uniqueIndex" json:"order_id"`
	BuyerId         int       `gorm:"column:buyer_id;index;not null" json:"buyer_id"`
	SalerId         int       `gorm:"column:saler_id;index;not null" json:"saler_id"`
	Amount          float64   `gorm:"column:amount;type:decimal(10,2);not null;default:0" json:"amount"`
	TaskId          int       `gorm:"column:task_id;index;not null;default:0" json:"task_id"`
	Counts          float64   `gorm:"column:counts;type:float(12,4);not null;default:0" json:"counts"`
	PayTypeInfo     string    `gorm:"column:pay_type_info;type:varchar(255);default:0" json:"pay_type_info"`
	PayTypeId       int       `gorm:"column:pay_type_id;not null;default:0" json:"pay_type_id"`
	BuyType         int       `gorm:"column:buy_type;not null;default:0" json:"buy_type"`
	Status          int       `gorm:"column:status;not null;default:0" json:"status"`
	PayImg          string    `gorm:"column:pay_img;type:varchar(255)" json:"pay_img"`
	IsCancel        int       `gorm:"column:is_cancel;default:0" json:"is_cancel"`
	IsAppeal        int       `gorm:"column:is_appeal;not null;default:0" json:"is_appeal"`
	AppealId        int       `gorm:"column:appeal_id;not null;default:0" json:"appeal_id"`
	AppealTime      int       `gorm:"column:appeal_time;not null;default:0" json:"appeal_time"`
	AppealReason    string    `gorm:"column:appeal_reason;type:varchar(255)" json:"appeal_reason"`
	AppealMaterials string    `gorm:"column:appeal_materials;type:text" json:"appeal_materials"`
	CancelId        int       `gorm:"column:cancel_id;not null;default:0" json:"cancel_id"`
	CancelReason    string    `gorm:"column:cancel_reason;type:varchar(255)" json:"cancel_reason"`
	PayTime         int       `gorm:"column:pay_time;default:0" json:"pay_time"`
	CancelTime      int       `gorm:"column:cancel_time;default:0" json:"cancel_time"`
	Wronger         int       `gorm:"column:wronger;not null;default:0" json:"wronger"`
	Judge           string    `gorm:"column:judge;type:varchar(255)" json:"judge"`
	JudgeTime       int       `gorm:"column:judge_time;not null;default:0" json:"judge_time"`
	Remark          string    `gorm:"column:remark;type:varchar(255)" json:"remark"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
}

func (MerchantOrder) TableName() string {
	return "merchant_order"
}
