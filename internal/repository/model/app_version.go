package model

import (
	"time"
)

// AppVersion 对应表 app_version
type AppVersion struct {
	Id                int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	Platform          string    `gorm:"column:platform;type:varchar(20);not null;index:idx_platform_channel,priority:1" json:"platform"`
	Channel           string    `gorm:"column:channel;type:varchar(50);not null;index:idx_platform_channel,priority:2" json:"channel"`
	LatestVersionName string    `gorm:"column:latest_version_name;type:varchar(20);not null;index:idx_version" json:"latest_version_name"`
	UpgradeType       string    `gorm:"column:upgrade_type;type:varchar(20);not null;default:optional" json:"upgrade_type"`
	Title             string    `gorm:"column:title;type:varchar(100)" json:"title"`
	ReleaseNotes      string    `gorm:"column:release_notes;type:varchar(1024)" json:"release_notes"`
	DownloadUrls      string    `gorm:"column:download_urls;type:varchar(1024)" json:"download_urls"`
	PublishedAt       time.Time `gorm:"column:published_at;type:datetime;not null" json:"published_at"`
	// 与 DDL 一致：created_at DEFAULT CURRENT_TIMESTAMP；updated_at DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

func (AppVersion) TableName() string {
	return "app_version"
}
