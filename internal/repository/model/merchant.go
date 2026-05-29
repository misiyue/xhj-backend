package model

import "time"

// 商户审核状态
const (
	MerchantStatusPending  = 0 // 待审核
	MerchantStatusApproved = 1 // 审核通过
	MerchantStatusRejected = 2 // 驳回
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
	IsClose      int       `gorm:"column:is_close;default:0" json:"is_close"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Merchant) TableName() string {
	return "merchant"
}
