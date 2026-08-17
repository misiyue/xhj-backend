package cron

import (
	"context"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/crontab"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ crontab.ICrontab = (*ExpireTalkUserMessage)(nil)

type ExpireTalkUserMessage struct {
	TalkUserMessageRepo *repo.TalkUserMessage
	TalkSessionRepo     *repo.TalkSession
	MessageStorage      *cache.MessageStorage
}

func (c *ExpireTalkUserMessage) Name() string {
	return "talk_user_message.expire_by_retain_days"
}

// Spec 每小时检查一次，按 retain_days 标记过期私聊消息
func (c *ExpireTalkUserMessage) Spec() string {
	return "0 * * * *"
}

func (c *ExpireTalkUserMessage) Enable() bool {
	return true
}

func (c *ExpireTalkUserMessage) Do(ctx context.Context) error {
	if c.TalkUserMessageRepo == nil {
		slog.WarnContext(ctx, "私聊消息 retain_days 定时删除跳过：TalkUserMessageRepo 未注入，请重新生成 wire_gen.go")
		return nil
	}
	n, err := c.TalkUserMessageRepo.MarkScheduledDeleteExpired(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "私聊消息按 retain_days 定时删除失败", "error", err)
		return err
	}
	if n > 0 {
		slog.InfoContext(ctx, "私聊消息按 retain_days 定时删除完成", "marked", n)
	}

	cleared, err := c.clearLastMessageForEmptySessions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "清除空会话最后一条消息缓存失败", "error", err)
		return err
	}
	if cleared > 0 {
		slog.InfoContext(ctx, "已清除空会话最后一条消息缓存", "sessions", cleared)
	}
	return nil
}

// clearLastMessageForEmptySessions 消息全部过期删除后，清除 session-list 使用的最后一条消息缓存
func (c *ExpireTalkUserMessage) clearLastMessageForEmptySessions(ctx context.Context) (int, error) {
	if c.MessageStorage == nil {
		return 0, nil
	}

	sessionIds, err := c.TalkUserMessageRepo.FindEmptyVisibleSessionIds(ctx)
	if err != nil {
		return 0, err
	}
	if len(sessionIds) == 0 {
		return 0, nil
	}

	cleared := 0
	for _, sessionID := range sessionIds {
		if sessionID <= 0 {
			continue
		}

		var userID, receiverID int
		if c.TalkSessionRepo != nil {
			rows, err := c.TalkSessionRepo.FindPrivatePairByLinkSessionId(ctx, sessionID)
			if err != nil {
				return cleared, err
			}
			if len(rows) > 0 {
				userID = rows[0].UserId
				receiverID = rows[0].ReceiverId
			}
		}

		if userID <= 0 || receiverID <= 0 {
			continue
		}

		if err := c.MessageStorage.Delete(ctx, entity.ChatPrivateMode, userID, receiverID); err != nil {
			return cleared, err
		}
		cleared++
	}

	return cleared, nil
}
