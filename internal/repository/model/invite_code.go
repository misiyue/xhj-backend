package model

import "time"

const (
	InviteCodeStatusAvailable = 0 // 可用
	InviteCodeStatusUsed      = 1 // 已使用
	InviteCodeStatusDisabled  = 2 // 已禁用
)

// InviteCode 邀请码表
type InviteCode struct {
	Id            int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`                                                                                 // 邀请码ID
	Code          string    `gorm:"column:code;type:char(16);uniqueIndex;not null" json:"code"`                                                                     // 邀请码
	UserId        int       `gorm:"column:user_id;index;index:idx_invite_code_user_status,priority:1;index:idx_invite_code_user_created,priority:1" json:"user_id"` // 生成者用户ID
	InviteeId     int       `gorm:"column:invitee_id;index" json:"invitee_id"`                                                                                      // 受邀者用户ID
	Status        int       `gorm:"column:status;default:0;index:idx_invite_code_user_status,priority:2" json:"status"`                                             // 状态 0:可用 1:已使用 2:已禁用
	UsedAt        time.Time `gorm:"column:used_at" json:"used_at"`                                                                                                  // 使用时间
	ExpireAt      time.Time `gorm:"column:expire_at" json:"expire_at"`                                                                                              // 过期时间
	MaxUsageCount int       `gorm:"column:max_usage_count;default:1" json:"max_usage_count"`                                                                        // 最大使用次数
	UsageCount    int       `gorm:"column:usage_count;default:0" json:"usage_count"`                                                                                // 已使用次数
	CreatedAt     time.Time `gorm:"column:created_at;index:idx_invite_code_user_created,priority:2" json:"created_at"`                                              // 创建时间
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`                                                                                            // 更新时间
}

func (InviteCode) TableName() string {
	return "invite_code"
}
