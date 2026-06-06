package model

import "time"

const (
	SysNoticeUnread = 0
	SysNoticeRead   = 1
)

// SysNotice 对应表 sys_notice
type SysNotice struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	UserId    int       `gorm:"column:user_id;type:int(11);not null;default:0" json:"user_id"`
	Title     string    `gorm:"column:title;type:varchar(32);not null;default:''" json:"title"`
	Content   string    `gorm:"column:content;type:varchar(255);not null;default:''" json:"content"`
	Url       string    `gorm:"column:url;type:varchar(255);not null;default:''" json:"url"`
	IsRead    int       `gorm:"column:is_read;type:tinyint(4);not null;default:0" json:"is_read"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SysNotice) TableName() string {
	return "sys_notice"
}
