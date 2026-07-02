package cron

import (
	"context"
	"log/slog"

	"github.com/gzydong/go-chat/internal/pkg/core/crontab"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ crontab.ICrontab = (*ExpireTalkUserMessage)(nil)

type ExpireTalkUserMessage struct {
	TalkUserMessageRepo *repo.TalkUserMessage
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
	return nil
}
