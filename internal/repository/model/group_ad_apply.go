package model

import "time"

const (
	GroupAdApplyStatusWait   = 0  // 待审核
	GroupAdApplyStatusPass   = 1  // 通过
	GroupAdApplyStatusRefuse = -1 // 拒绝
)

type GroupAdApply struct {
	Id        int        `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	UserId    int        `gorm:"column:user_id;type:int(11)" json:"user_id"`
	GroupId   int        `gorm:"column:group_id;type:int(11)" json:"group_id"`
	Intro     string     `gorm:"column:intro;type:varchar(255)" json:"intro"`
	Status    int        `gorm:"column:status;type:tinyint(4);default:0" json:"status"`
	DealTime  *time.Time `gorm:"column:deal_time;type:datetime" json:"deal_time"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (GroupAdApply) TableName() string {
	return "group_ad_apply"
}
