package model

import "time"

const (
	AppNewsStatusDraft     = 0  // 草稿
	AppNewsStatusPublished = 1  // 已发布
	AppNewsStatusOff       = -1 // 已下架

	AppNewsCollectImageText = "image_text" // 图文
	AppNewsCollectVideo     = "video"      // 视频

	AppNewsTypeGlobalHot   = "global_hot"   // 火箭全球热讯
	AppNewsTypeCrypto      = "crypto"       // 加密货币
	AppNewsTypeRealtimeHot = "realtime_hot" // 实时热门

	AppNewsSourceYoutube  = "youtube"  // YouTube
	AppNewsSourceTwitter  = "twitter"  // 推特
	AppNewsSourceNytimes  = "nytimes"  // 纽约时报
	AppNewsSourceTelegram = "telegram" // TG 订阅号
)

// AppNews 对应表 app_news（火箭资讯）
type AppNews struct {
	Id          int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title       string     `gorm:"column:title;type:varchar(255);not null;default:''" json:"title"`
	CollectType string     `gorm:"column:collect_type;type:varchar(20);not null;default:image_text" json:"collect_type"`
	NewsType    string     `gorm:"column:news_type;type:varchar(20);not null;default:global_hot;index:idx_news_type" json:"news_type"`
	Source      string     `gorm:"column:source;type:varchar(20);not null;default:youtube;index:idx_source" json:"source"`
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
