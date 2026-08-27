package model

import "time"

// AppExplore 对应表 app_explore
type AppExplore struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	Title     string    `gorm:"column:title;type:varchar(64)" json:"title"`
	Image     string    `gorm:"column:image;type:varchar(255)" json:"image"`
	Digest    string    `gorm:"column:digest;type:varchar(128)" json:"digest"`
	Url       string    `gorm:"column:url;type:varchar(255)" json:"url"`
	Position  string    `gorm:"column:position;type:varchar(16)" json:"position"`
	Ends      string    `gorm:"column:ends;type:varchar(32)" json:"ends"`
	Sort      int       `gorm:"column:sort;type:int(11);default:0" json:"sort"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

func (AppExplore) TableName() string {
	return "app_explore"
}
