package model

import "time"

const (
	AppNewsStatusDraft     = 0  // 草稿
	AppNewsStatusPublished = 1  // 已发布
	AppNewsStatusOff       = -1 // 已下架

	AppNewsTypeImageText = 1 // 图文
	AppNewsTypeVideo     = 2 // 视频
)

// AppNews 对应表 app_news（火箭资讯）
type AppNews struct {
	Id          int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title       string     `gorm:"column:title;type:varchar(255);not null;default:''" json:"title"`
	CategoryId  int        `gorm:"column:category_id;type:int(11);not null;default:0" json:"category_id"`
	TypeId      *int       `gorm:"column:type_id;type:tinyint(4)" json:"type_id"` // 1-图文，2-视频
	Content     string     `gorm:"column:content;type:mediumtext" json:"content"`
	Cover       string     `gorm:"column:cover;type:varchar(255)" json:"cover"`
	SourceURL   string     `gorm:"column:source_url;type:varchar(512)" json:"source_url"`
	UploadTime  *time.Time `gorm:"column:upload_time" json:"upload_time"`
	PublishTime *time.Time `gorm:"column:publish_time;index:idx_publish_time" json:"publish_time"`
	Status      int        `gorm:"column:status;type:tinyint(4);not null;default:0;index:idx_status" json:"status"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (AppNews) TableName() string {
	return "app_news"
}
