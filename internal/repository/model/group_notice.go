package model

import (
	"time"
)

const (
	GroupNoticeIsDeleteNo  = 0
	GroupNoticeIsDeleteYes = 2
)

type GroupNotice struct {
	Id           int       `gorm:"column:id;primary_key;AUTO_INCREMENT;index:idx_group_notice_group_id,priority:2" json:"id"` // 公告ID
	GroupId      int       `gorm:"column:group_id;index:idx_group_notice_group_id,priority:1" json:"group_id"`                // 群组ID
	CreatorId    int       `gorm:"column:creator_id;" json:"creator_id"`                                                      // 创建者用户ID
	ModifyId     int       `gorm:"column:modify_id;" json:"modify_id"`                                                        // 创建者用户ID
	Content      string    `gorm:"column:content;type:text" json:"content"`                                                   // 公告内容
	ConfirmUsers string    `gorm:"column:confirm_users;type:varchar(1024)" json:"confirm_users"`                              // 已确认成员
	IsConfirm    int       `gorm:"column:is_confirm;" json:"is_confirm"`                                                      // 是否需群成员确认公告[1:否;2:是;]
	CreatedAt    time.Time `gorm:"column:created_at;" json:"created_at"`                                                      // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at;" json:"updated_at"`                                                      // 更新时间
}

func (GroupNotice) TableName() string {
	return "group_notice"
}

type SearchNoticeItem struct {
	Id           int       `json:"id" grom:"column:id"`
	CreatorId    int       `json:"creator_id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	IsTop        int       `json:"is_top"`
	IsConfirm    int       `json:"is_confirm"`
	ConfirmUsers string    `json:"confirm_users"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Avatar       string    `json:"avatar"`
	Nickname     string    `json:"nickname"`
}
