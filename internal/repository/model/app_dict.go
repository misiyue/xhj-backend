package model

import "time"

// AppDict 字典配置表 app_dict（列名 key 为 MySQL 保留字，结构体字段用 DictKey）
type AppDict struct {
	Title     string    `gorm:"column:title;type:varchar(32);not null;default:''" json:"title"`
	DictKey   string    `gorm:"column:key;type:varchar(64);primaryKey" json:"key"`
	Value     string    `gorm:"column:value;type:varchar(2048);not null;default:''" json:"value"`
	Type      string    `gorm:"column:type;type:varchar(16);not null;default:''" json:"type"`
	Status    int       `gorm:"column:status;not null;default:0" json:"status"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AppDict) TableName() string {
	return "app_dict"
}
