package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type TalkUserMessage struct {
	core.Repo[model.TalkUserMessage]
}

func NewTalkRecordFriend(db *gorm.DB) *TalkUserMessage {
	return &TalkUserMessage{Repo: core.NewRepo[model.TalkUserMessage](db)}
}

func (t *TalkUserMessage) FindByMsgId(ctx context.Context, msgId string) (*model.TalkUserMessage, error) {
	return t.FindByWhere(ctx, "msg_id = ?", msgId)
}

// MarkScheduledDeleteExpired 按 talk_session.retain_days 将过期私聊消息标记为定时删除（is_deleted = -1）
func (t *TalkUserMessage) MarkScheduledDeleteExpired(ctx context.Context) (int64, error) {
	result := t.Db.WithContext(ctx).Exec(`
UPDATE talk_user_message AS m
INNER JOIN (
	SELECT session_id, MAX(retain_days) AS retain_days
	FROM talk_session
	WHERE retain_days > 0 AND talk_mode = 1 AND session_id > 0
	GROUP BY session_id
) AS s ON m.session_id = s.session_id
SET m.is_deleted = ?
WHERE m.is_deleted = ?
AND m.created_at < DATE_SUB(NOW(), INTERVAL s.retain_days DAY)
`, model.TalkUserMessageDeletedScheduled, model.No)
	return result.RowsAffected, result.Error
}
