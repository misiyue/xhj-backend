package model

import "time"

// YunxinCredential is the local ownership record for a Yunxin IM account.
// The token is encrypted before it reaches this table and must never be sent
// to another user.
type YunxinCredential struct {
	Id              int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId          int       `gorm:"column:user_id;uniqueIndex;not null" json:"user_id"`
	Accid           string    `gorm:"column:accid;size:64;uniqueIndex;not null" json:"accid"`
	TokenCiphertext string    `gorm:"column:token_ciphertext;type:text;not null" json:"-"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (YunxinCredential) TableName() string {
	return "yunxin_credential"
}
