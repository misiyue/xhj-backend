package model

import "time"

const (
	NoticeArticleStatusOff = 0 // 关闭
	NoticeArticleStatusOn  = 1 // 开启
)

// NoticeArticle 通知文章 notice_article
type NoticeArticle struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement;type:int(11)" json:"id"`
	Code      string    `gorm:"column:code;type:varchar(32);uniqueIndex:code_idx" json:"code"`
	Title     string    `gorm:"column:title;type:varchar(64)" json:"title"`
	Content   string    `gorm:"column:content;type:text" json:"content"`
	Status    int       `gorm:"column:status;type:tinyint(4)" json:"status"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (NoticeArticle) TableName() string {
	return "notice_article"
}
