package model

import "time"

type EmoticonItem struct {
	Id         int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`                                            // 表情包详情ID
	EmoticonId int       `gorm:"column:emoticon_id;index:idx_emoticon_item_emoticon_user_id,priority:1" json:"emoticon_id"` // 表情分组ID
	UserId     int       `gorm:"column:user_id;index:idx_emoticon_item_emoticon_user_id,priority:2" json:"user_id"`         // 用户ID（0：代码系统表情包）
	Describe   string    `gorm:"column:describe;type:varchar(255);" json:"describe"`                                        // 表情描述
	Url        string    `gorm:"column:url;type:varchar(255);" json:"url"`                                                  // 图片链接
	CreatedAt  time.Time `gorm:"column:created_at;" json:"created_at"`                                                      // 创建时间
	UpdatedAt  time.Time `gorm:"column:updated_at;" json:"updated_at"`                                                      // 更新时间
}

func (EmoticonItem) TableName() string {
	return "emoticon_item"
}
