package model

import (
	"time"
)

// AppVersion 对应表 app_version
type AppVersion struct {
	Id                int       `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	Platform          string    `gorm:"type:varchar(20);not null;column:platform" json:"platform"`
	Channel           string    `gorm:"type:varchar(50);not null;column:channel" json:"channel"`
	LatestVersionName string    `gorm:"type:varchar(20);not null;column:latest_version_name" json:"latest_version_name"`
	UpgradeType       string    `gorm:"type:varchar(20);not null;default:optional;column:upgrade_type" json:"upgrade_type"`
	Title             string    `gorm:"type:varchar(100);column:title" json:"title"` // 使用指针允许NULL
	ReleaseNotes      string    `gorm:"type:varchar(1024);column:release_notes" json:"release_notes"`
	DownloadUrls      string    `gorm:"type:varchar(1024);column:download_urls" json:"download_urls"`
	PublishedAt       time.Time `gorm:"not null;column:published_at" json:"published_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
	CreatedAt         time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

func (AppVersion) TableName() string {
	return "app_version"
}
