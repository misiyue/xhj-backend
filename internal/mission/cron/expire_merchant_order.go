package cron

import (
	"context"
	"log/slog"

	"github.com/gzydong/go-chat/internal/pkg/core/crontab"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ crontab.ICrontab = (*ExpireMerchantOrder)(nil)

type ExpireMerchantOrder struct {
	MerchantOrderRepo *repo.MerchantOrder
}

func (c *ExpireMerchantOrder) Name() string {
	return "merchant_order.expire_unpaid"
}

// Spec 每分钟检查超时未支付订单（30 分钟）
func (c *ExpireMerchantOrder) Spec() string {
	return "* * * * *"
}

func (c *ExpireMerchantOrder) Enable() bool {
	return true
}

func (c *ExpireMerchantOrder) Do(ctx context.Context) error {
	if c.MerchantOrderRepo == nil {
		return nil
	}
	n, err := c.MerchantOrderRepo.CancelExpiredUnpaid(ctx, repo.MerchantOrderUnpaidCancelTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "商户订单超时取消失败", "error", err)
		return err
	}
	if n > 0 {
		slog.InfoContext(ctx, "商户订单超时取消完成", "cancelled", n)
	}
	return nil
}
