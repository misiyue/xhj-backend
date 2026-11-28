package model

import "time"

// AppModule 对应表 app_module
type AppModule struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	Code      string    `gorm:"column:code;type:varchar(32)" json:"code"`
	Title     string    `gorm:"column:title;type:varchar(32)" json:"title"`
	Ends      string    `gorm:"column:ends;type:varchar(32)" json:"ends"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

func (AppModule) TableName() string {
	return "app_module"
}
