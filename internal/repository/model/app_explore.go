package model

import "time"

const (
	AppExploreOpenYes = 1 // 开放
	AppExploreOpenNo  = 2 // 不开放
)

// AppExplore 对应表 app_explore
type AppExplore struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"column:title" json:"title"`
	Image     string    `gorm:"column:image" json:"image"`
	Url       string    `gorm:"column:url" json:"url"`
	Position  string    `gorm:"column:position" json:"position"`
	IsOpen    int       `gorm:"column:is_open" json:"is_open"`
	Sort      int       `gorm:"column:sort;default:0" json:"sort"` // 越大越靠前
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AppExplore) TableName() string {
	return "app_explore"
}
