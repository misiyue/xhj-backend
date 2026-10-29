package model

import (
	"time"
)

const (
	GroupMemberMaxNum = 500 // 最大成员数量

	GroupTypeNormal     = 1
	GroupTypeEnterprise = 2

	// ForceStatus 强制状态
	GroupForceStatusNone = 0 // 无
	GroupForceStatusMute = 1 // 强制群禁言
)

type Group struct {
	Id        int    `gorm:"column:id;primary_key;AUTO_INCREMENT;type:bigint(20)" json:"id"`                         // 群ID
	Type      int    `gorm:"column:type;type:tinyint(4)" json:"type"`                                                 // 群类型[1:普通群;2:企业群;]
	CreatorId int    `gorm:"column:creator_id;type:bigint(20)" json:"creator_id"`                                     // 创建者ID(群主ID)
	Name      string `gorm:"column:name;type:varchar(255)" json:"name"`                                               // 群名称
	Profile   string `gorm:"column:profile;type:varchar(255)" json:"profile"`                                         // 群介绍
	IsDismiss int    `gorm:"column:is_dismiss;type:tinyint(4);index:idx_group_overt_dismiss_created,priority:2" json:"is_dismiss"` // 是否已解散[1:否;2:是;]
	Avatar    string `gorm:"column:avatar;type:varchar(255)" json:"avatar"`                                           // 群头像
	MaxNum    int    `gorm:"column:max_num;type:int(11)" json:"max_num"`                                              // 最大群成员数量
	IsOvert   int    `gorm:"column:is_overt;type:tinyint(4);index:idx_group_overt_dismiss_created,priority:1" json:"is_overt"`     // 是否公开可见[1:否;2:是;]
	IsMute    int    `gorm:"column:is_mute;type:tinyint(4)" json:"is_mute"`                                           // 是否全员禁言 [1:否;2:是;] 提示:不包含群主或管理员
	// 群内是否允许成员互加好友 [1:允许;2:不允许;] 与接口 GroupDetailResponse.is_allow_invite 一致
	IsAllowInvite int `gorm:"column:is_allow_invite;type:tinyint(4);default:1" json:"is_allow_invite"`
	// 强制状态：0-无，1-群禁言
	ForceStatus int       `gorm:"column:force_status;type:tinyint(4);default:0" json:"force_status"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;default:CURRENT_TIMESTAMP;index:idx_group_overt_dismiss_created,priority:3" json:"created_at"` // 创建时间
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`                     // 更新时间
}

func (Group) TableName() string {
	return "group"
}

type GroupItem struct {
	Id        int    `json:"id"`
	GroupName string `json:"group_name"`
	Avatar    string `json:"avatar"`
	Profile   string `json:"profile"`
	Leader    int    `json:"leader"`
	IsDisturb int    `json:"is_disturb"`
	CreatorId int    `json:"creator_id"`
}
