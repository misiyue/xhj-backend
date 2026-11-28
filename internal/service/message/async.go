package message

import (
	"context"

	"github.com/gzydong/go-chat/internal/pkg/logger"
)

// goAfterSend 将发消息后的非关键路径异步化（OneSignal / @ / 未读等），缩短 HTTP 返回时间。
// 使用独立 Background context，避免请求结束后 ctx 取消导致推送中断。
func (s *Service) goAfterSend(name string, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("message async %s panic: %v", name, r)
			}
		}()
		fn(context.Background())
	}()
}
