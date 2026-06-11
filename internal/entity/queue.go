package entity

const (
	LoginTopic     = "im.user.login"
	SysNoticeTopic = "im.sys.notice"
)

// SysNoticeQueueMessage Redis 通道 im.sys.notice 的消息体（站内信已入库，消费端只负责推送）
type SysNoticeQueueMessage struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Url       string `json:"url"`
	CreatedAt string `json:"created_at,omitempty"`
}
