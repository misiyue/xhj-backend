package model

import "time"

const (
	AppModuleOpenYes = 1 // 开放
	AppModuleOpenNo  = 0 // 不开放
)

// AppModule 对应表 app_module
type AppModule struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code      string    `gorm:"column:code" json:"code"`
	Title     string    `gorm:"column:title" json:"title"`
	IsOpen    int       `gorm:"column:is_open" json:"is_open"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppModule) TableName() string {
	return "app_module"
}
