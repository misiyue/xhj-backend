package model

import "time"

const (
	AppExploreOpenYes = 1 // 开放
	AppExploreOpenNo  = 2 // 不开放
)

// AppExplore 对应表 app_explore
type AppExplore struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"column:title;type:varchar(64)" json:"title"`
	Image     string    `gorm:"column:image;type:varchar(255)" json:"image"`
	Digest    string    `gorm:"column:digest;type:varchar(128)" json:"digest"` // 描述
	Url       string    `gorm:"column:url;type:varchar(255)" json:"url"`
	Position  string    `gorm:"column:position;type:varchar(16)" json:"position"`
	IsOpen    int       `gorm:"column:is_open;type:tinyint(4);default:0" json:"is_open"`
	Sort      int       `gorm:"column:sort;type:int(11);default:0" json:"sort"` // 越大越靠前
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppExplore) TableName() string {
	return "app_explore"
}
