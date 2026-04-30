package service

import (
	"context"
	"errors"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/email"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
)

var _ IEmailService = (*EmailService)(nil)

type IEmailService interface {
	// Verify 验证邮箱验证码
	Verify(ctx context.Context, channel string, email string, code string) bool
	// Delete 删除邮箱验证码记录
	Delete(ctx context.Context, channel string, email string)
	// Send 发送邮箱验证码
	Send(ctx context.Context, channel string, email string) (string, error)
}

type EmailService struct {
	Storage     *cache.EmailStorage
	EmailClient *email.Client
}

// Verify 验证邮箱验证码是否正确
func (e *EmailService) Verify(ctx context.Context, channel string, email string, code string) bool {
	return e.Storage.Verify(ctx, channel, email, code)
}

// Delete 删除邮箱验证码记录
func (e *EmailService) Delete(ctx context.Context, channel string, email string) {
	_ = e.Storage.Del(ctx, channel, email)
}

// Send 发送邮箱验证码
func (e *EmailService) Send(ctx context.Context, channel string, email string) (string, error) {
	// 检查是否在60秒内已发送过验证码
	if !e.Storage.CanSend(ctx, email) {
		return "", errors.New("验证码发送过于频繁，请60秒后再试")
	}

	code := strutil.GenValidateCode(6)

	// 添加发送记录
	if err := e.Storage.Set(ctx, channel, email, code, 15*time.Minute); err != nil {
		return "", err
	}

	// 记录发送时间，60秒内不允许重复发送
	if err := e.Storage.SetSendTime(ctx, email); err != nil {
		return "", err
	}

	// Email sending is handled by the caller (handler) to allow template customization
	// This service only manages the verification code storage

	// For development/testing: log the code
	logger.Infof("Email verification code for %s (channel: %s): %s", email, channel, code)

	return code, nil
}
