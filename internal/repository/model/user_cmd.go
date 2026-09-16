package model

import "time"

// UserCmd 用户指令 user_cmd
type UserCmd struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	UserId    int       `gorm:"column:user_id;type:int(11)" json:"user_id"`
	Title     string    `gorm:"column:title;type:varchar(255)" json:"title"`
	Intro     string    `gorm:"column:intro;type:varchar(255)" json:"intro"`
	File      string    `gorm:"column:file;type:varchar(255)" json:"file"`
	Btns      string    `gorm:"column:btns;type:text" json:"btns"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

func (UserCmd) TableName() string {
	return "user_cmd"
}
