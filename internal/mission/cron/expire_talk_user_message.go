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
	TalkUserMessageRepo  *repo.TalkUserMessage
	TalkGroupMessageRepo *repo.TalkGroupMessage
	TalkSessionRepo      *repo.TalkSession
	MessageStorage       *cache.MessageStorage
}

func (c *ExpireTalkUserMessage) Name() string {
	return "talk_user_message.expire_by_retain_days"
}

// Spec 每小时检查一次，按 retain_days 标记过期私聊/群聊消息
func (c *ExpireTalkUserMessage) Spec() string {
	return "0 * * * *"
}

func (c *ExpireTalkUserMessage) Enable() bool {
	return true
}

func (c *ExpireTalkUserMessage) Do(ctx context.Context) error {
	if err := c.expirePrivate(ctx); err != nil {
		return err
	}
	if err := c.expireGroup(ctx); err != nil {
		return err
	}
	return nil
}

func (c *ExpireTalkUserMessage) expirePrivate(ctx context.Context) error {
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

func (c *ExpireTalkUserMessage) expireGroup(ctx context.Context) error {
	if c.TalkGroupMessageRepo == nil {
		slog.WarnContext(ctx, "群聊消息 retain_days 定时删除跳过：TalkGroupMessageRepo 未注入，请重新生成 wire_gen.go")
		return nil
	}

	// 1. 他人消息：写入 talk_group_message_del，仅对该用户隐藏
	delN, err := c.TalkGroupMessageRepo.MarkOthersExpiredToDel(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "群聊他人消息写入 del 表失败", "error", err)
		return err
	}
	if delN > 0 {
		slog.InfoContext(ctx, "群聊他人过期消息已写入 del 表", "rows", delN)
	}

	// 2. 本人消息：is_revoked=3（过期删除），列表侧全局过滤
	revN, err := c.TalkGroupMessageRepo.MarkOwnExpiredRevoked(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "群聊本人消息标记过期删除失败", "error", err)
		return err
	}
	if revN > 0 {
		slog.InfoContext(ctx, "群聊本人过期消息已标记 is_revoked=3", "rows", revN)
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
