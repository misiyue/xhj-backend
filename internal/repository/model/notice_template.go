package model

import "time"

const (
	NoticeTemplateFlagUserChat     = "userChat"
	NoticeTemplateFlagC2cChat      = "c2cChat"
	NoticeTemplateFlagC2cPaid      = "c2cPaid"
	NoticeTemplateFlagC2cTrans     = "c2cTrans"
	NoticeTemplateFlagContactApply = "contactApply"
	NoticeTemplateFlagGroupApply   = "groupApply"
)

// NoticeTemplate 通知模板 notice_template
type NoticeTemplate struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Flag      string    `gorm:"column:flag;type:varchar(32);uniqueIndex:flag_idx" json:"flag"`
	Title     string    `gorm:"column:title;type:varchar(255)" json:"title"`
	Subtitle  string    `gorm:"column:subtitle;type:varchar(255)" json:"subtitle"`
	Content   string    `gorm:"column:content;type:varchar(255)" json:"content"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (NoticeTemplate) TableName() string {
	return "notice_template"
}
