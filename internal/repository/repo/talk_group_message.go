package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type TalkGroupMessage struct {
	core.Repo[model.TalkGroupMessage]
}

func NewTalkRecordGroup(db *gorm.DB) *TalkGroupMessage {
	return &TalkGroupMessage{Repo: core.NewRepo[model.TalkGroupMessage](db)}
}

func (t *TalkGroupMessage) FindByMsgId(ctx context.Context, msgId string) (*model.TalkGroupMessage, error) {
	return t.FindByWhere(ctx, "msg_id =?", msgId)
}

// MarkOthersExpiredToDel 将群内非本人发送、已过 retain_days 的消息写入 talk_group_message_del（对该用户隐藏）
func (t *TalkGroupMessage) MarkOthersExpiredToDel(ctx context.Context) (int64, error) {
	result := t.Db.WithContext(ctx).Exec(`
INSERT IGNORE INTO talk_group_message_del (user_id, group_id, msg_id, created_at)
SELECT ts.user_id, m.group_id, m.msg_id, NOW()
FROM talk_session ts
INNER JOIN talk_group_message m ON m.group_id = ts.receiver_id
WHERE ts.talk_mode = 2
  AND ts.retain_days > 0
  AND m.from_id <> ts.user_id
  AND m.is_deleted = ?
  AND m.send_time < DATE_SUB(NOW(), INTERVAL ts.retain_days DAY)
`, model.No)
	return result.RowsAffected, result.Error
}

// MarkOwnExpiredDeleted 将本人发送、已过 retain_days 的群消息标记为删除（is_deleted = 1）
func (t *TalkGroupMessage) MarkOwnExpiredDeleted(ctx context.Context) (int64, error) {
	result := t.Db.WithContext(ctx).Exec(`
UPDATE talk_group_message AS m
INNER JOIN talk_session AS ts
  ON ts.talk_mode = 2
 AND ts.receiver_id = m.group_id
 AND ts.user_id = m.from_id
 AND ts.retain_days > 0
SET m.is_deleted = ?
WHERE m.is_deleted = ?
  AND m.send_time < DATE_SUB(NOW(), INTERVAL ts.retain_days DAY)
`, model.Yes, model.No)
	return result.RowsAffected, result.Error
}
