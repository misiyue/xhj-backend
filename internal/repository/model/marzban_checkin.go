package model

import "time"

// MarzbanCheckin stores each user's latest successful reward on each date.
type MarzbanCheckin struct {
	ID          uint64    `gorm:"primaryKey"`
	UserID      int       `gorm:"not null;uniqueIndex:uk_marzban_checkin_day"`
	CheckinDate string    `gorm:"type:char(10);not null;uniqueIndex:uk_marzban_checkin_day"`
	Expire      int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (MarzbanCheckin) TableName() string { return "marzban_checkin" }

// MarzbanCheckinState serializes rewards and stores the rolling cooldown per user.
type MarzbanCheckinState struct {
	UserID        int       `gorm:"primaryKey;autoIncrement:false"`
	NextCheckinAt int64     `gorm:"not null;default:0"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (MarzbanCheckinState) TableName() string { return "marzban_checkin_state" }
