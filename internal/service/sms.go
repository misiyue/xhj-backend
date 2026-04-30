package service

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
)

var _ ISmsService = (*SmsService)(nil)

type ISmsService interface {
	Verify(ctx context.Context, channel string, mobile string, code string) bool
	Delete(ctx context.Context, channel string, mobile string)
	Send(ctx context.Context, channel string, mobile string) (string, error)
}

type SmsService struct {
	Storage *cache.SmsStorage
}

// Verify 验证短信验证码是否正确
func (s *SmsService) Verify(ctx context.Context, channel string, mobile string, code string) bool {
	return s.Storage.Verify(ctx, channel, mobile, code)
}

// Delete 删除短信验证码记录
func (s *SmsService) Delete(ctx context.Context, channel string, mobile string) {
	_ = s.Storage.Del(ctx, channel, mobile)
}

// Send 发送短信
func (s *SmsService) Send(ctx context.Context, channel string, mobile string) (string, error) {

	code := strutil.GenValidateCode(6)

	// 添加发送记录
	if err := s.Storage.Set(ctx, channel, mobile, code, 15*time.Minute); err != nil {
		return "", err
	}

	// Send SMS via third-party service
	// Integration options:
	// 1. Aliyun SMS: https://www.aliyun.com/product/sms
	// 2. Tencent Cloud SMS: https://cloud.tencent.com/product/sms
	// 3. Twilio: https://www.twilio.com/sms
	// 
	// Example implementation with Aliyun SMS:
	// client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", "<accessKeyId>", "<accessKeySecret>")
	// request := dysmsapi.CreateSendSmsRequest()
	// request.PhoneNumbers = mobile
	// request.SignName = "YourSignName"
	// request.TemplateCode = "SMS_123456789"
	// request.TemplateParam = fmt.Sprintf(`{"code":"%s"}`, code)
	// response, err := client.SendSms(request)
	// if err != nil || response.Code != "OK" {
	//     return "", fmt.Errorf("SMS send failed: %v", err)
	// }
	
	// For development/testing: log the code instead of sending
	logger.Infof("SMS verification code for %s (channel: %s): %s", mobile, channel, code)

	return code, nil
}
