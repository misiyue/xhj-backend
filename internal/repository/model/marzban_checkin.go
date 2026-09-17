package model

import "time"

// MarzbanCheckin records one successful daily reward per user.
type MarzbanCheckin struct {
	ID          uint64    `gorm:"primaryKey"`
	UserID      int       `gorm:"not null;uniqueIndex:uk_marzban_checkin_day"`
	CheckinDate string    `gorm:"type:char(10);not null;uniqueIndex:uk_marzban_checkin_day"`
	Expire      int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (MarzbanCheckin) TableName() string { return "marzban_checkin" }
