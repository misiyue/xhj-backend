package model

import "time"

const (
	UserReportDefeTypeUser  = 1 // 被诉：用户
	UserReportDefeTypeGroup = 2 // 被诉：群

	UserReportStatusPending  = 0 // 待处理
	UserReportStatusHandled  = 1 // 已处理

	UserReportResultBanAccount = 1 // 账号封禁
	UserReportResultGroupMute  = 2 // 群禁言
)

// UserReport 用户举报表
type UserReport struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	UserId    int       `gorm:"column:user_id;type:int(11)" json:"user_id"`       // 上诉人用户id
	DefeId    int       `gorm:"column:defe_id;type:int(11)" json:"defe_id"`       // 被诉人用户或群id
	DefeType  int       `gorm:"column:defe_type;type:tinyint(4)" json:"defe_type"` // 1-用户 2-群
	Reason    string    `gorm:"column:reason;type:varchar(255)" json:"reason"`
	Status    int       `gorm:"column:status;type:tinyint(4);default:0" json:"status"` // 0-待处理 1-已处理
	Result    int       `gorm:"column:result;type:tinyint(4);default:0" json:"result"` // 1-账号封禁 2-群禁言
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (UserReport) TableName() string {
	return "user_report"
}
