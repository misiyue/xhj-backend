package model

import "time"

// GroupTotop 群置顶推广 group_totop
type GroupTotop struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	Title     string    `gorm:"column:title;type:varchar(255)" json:"title"`
	Cover     string    `gorm:"column:cover;type:varchar(255)" json:"cover"`
	Intro     string    `gorm:"column:intro;type:varchar(255)" json:"intro"`
	Btn       string    `gorm:"column:btn;type:varchar(255)" json:"btn"`
	Url       string    `gorm:"column:url;type:varchar(255)" json:"url"`
	GroupIds  string    `gorm:"column:group_ids;type:varchar(255)" json:"group_ids"`
	Sort      int       `gorm:"column:sort;type:int(11)" json:"sort"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

func (GroupTotop) TableName() string {
	return "group_totop"
}
