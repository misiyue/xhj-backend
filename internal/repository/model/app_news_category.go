package model

import "time"

const (
	AppNewsCategoryStatusHide = 0 // 不显示
	AppNewsCategoryStatusShow = 1 // 显示

	AppNewsCategoryCollectNews = "news" // 火箭资讯
)

// AppNewsCategory 对应表 app_news_category（资讯分类）
type AppNewsCategory struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"column:title;type:varchar(255)" json:"title"`
	Collect   string    `gorm:"column:collect;type:varchar(16)" json:"collect"` // 所属项目：news-火箭资讯
	Status    int       `gorm:"column:status;type:tinyint(4);default:0" json:"status"`
	Sort      int       `gorm:"column:sort;type:int(11);default:0" json:"sort"` // 越大越靠前
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppNewsCategory) TableName() string {
	return "app_news_category"
}
