package cron

import (
	"context"
	"log/slog"

	"github.com/gzydong/go-chat/internal/pkg/core/crontab"
	"github.com/gzydong/go-chat/internal/service"
)

var _ crontab.ICrontab = (*ExpireRedEnvelope)(nil)

type ExpireRedEnvelope struct {
	WalletService      service.IWalletService
	RedEnvelopeService service.IRedEnvelopeService
}

func (c *ExpireRedEnvelope) Name() string {
	return "red_envelope.expire"
}

// Spec 配置定时任务规则
// 每小时执行一次，检查并处理过期红包
// Cron表达式: "0 * * * *" - 在每小时的第0分钟执行 (例如: 01:00, 02:00, 03:00...)
func (c *ExpireRedEnvelope) Spec() string {
	return "0 * * * *"
}

func (c *ExpireRedEnvelope) Enable() bool {
	return true
}

func (c *ExpireRedEnvelope) Do(ctx context.Context) error {
	// 使用红包服务处理过期红包
	count, err := c.RedEnvelopeService.ExpireOverdue(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "红包过期处理失败", "error", err)
		return err
	}

	if count > 0 {
		slog.InfoContext(ctx, "红包过期处理完成", "expired_count", count)
	}

	return nil
}
