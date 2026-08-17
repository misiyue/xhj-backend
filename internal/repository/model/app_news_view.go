package model

import "time"

// AppNewsView 对应表 app_news_view（资讯阅读记录，无主键，每次阅读插入一条）
type AppNewsView struct {
	NewsId    int       `gorm:"column:news_id;type:int(11);not null;default:0" json:"news_id"`
	UserId    int       `gorm:"column:user_id;type:int(11);not null;default:0" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AppNewsView) TableName() string {
	return "app_news_view"
}
