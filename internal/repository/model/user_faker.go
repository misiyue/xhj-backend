package model

import "time"

// UserFaker 对应表 user_faker
type UserFaker struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	UserId    int       `gorm:"column:user_id;type:int(11)" json:"user_id"`
	Username  string    `gorm:"column:username;type:varchar(255)" json:"username"`
	Nickname  string    `gorm:"column:nickname;type:varchar(255)" json:"nickname"`
	Avatar    string    `gorm:"column:avatar;type:varchar(255)" json:"avatar"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

func (UserFaker) TableName() string {
	return "user_faker"
}
