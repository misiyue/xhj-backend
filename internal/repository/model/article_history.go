package model

import (
	"time"
)

type ArticleHistory struct {
	Id        int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`                                        // 文章ID
	UserId    int       `gorm:"column:user_id;index:idx_article_history_user_article,priority:1" json:"user_id"`       // 用户ID
	ArticleId int       `gorm:"column:article_id;index:idx_article_history_user_article,priority:2" json:"article_id"` // 笔记ID
	Content   string    `gorm:"column:content;type:text" json:"content"`                                               // Markdown 内容
	CreatedAt time.Time `gorm:"column:created_at;" json:"created_at"`                                                  // 创建时间
}

func (ArticleHistory) TableName() string {
	return "article_history"
}
