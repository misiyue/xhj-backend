package entity

const (
	LoginTopic     = "im.user.login"
	SysNoticeTopic = "im.sys.notice"
)

// SysNoticeQueueMessage Redis 通道 im.sys.notice 的消息体
type SysNoticeQueueMessage struct {
	UserId  int    `json:"user_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Url     string `json:"url"`
}
