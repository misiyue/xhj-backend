package model

import "time"

const (
	BannerAdApplyStatusWait   = 0  // 待审核
	BannerAdApplyStatusPass   = 1  // 通过
	BannerAdApplyStatusRefuse = -1 // 拒绝
)

type BannerAdApply struct {
	Id        int        `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	UserId    int        `gorm:"column:user_id;type:int(11)" json:"user_id"`
	Banner    string     `gorm:"column:banner;type:varchar(255)" json:"banner"`
	Title     string     `gorm:"column:title;type:varchar(255)" json:"title"`
	Status    int        `gorm:"column:status;type:tinyint(4);default:0" json:"status"`
	DealTime  *time.Time `gorm:"column:deal_time;type:datetime" json:"deal_time"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (BannerAdApply) TableName() string {
	return "banner_ad_apply"
}
