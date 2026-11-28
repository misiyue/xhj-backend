package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type EmailStorage struct {
	redis *redis.Client
}

func NewEmailStorage(redis *redis.Client) *EmailStorage {
	return &EmailStorage{redis}
}

func (e *EmailStorage) Set(ctx context.Context, channel string, email string, code string, exp time.Duration) error {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)
	_, err := e.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, e.failName(channel, email))
		pipe.Set(ctx, e.name(channel, email), code, exp)
		return nil
	})
	return err
}

func (e *EmailStorage) Get(ctx context.Context, channel string, email string) (string, error) {
	return e.redis.Get(ctx, e.name(channel, normalizeEmail(email))).Result()
}

func (e *EmailStorage) Del(ctx context.Context, channel string, email string) error {
	return e.redis.Del(ctx, e.name(channel, normalizeEmail(email))).Err()
}

func (e *EmailStorage) Verify(ctx context.Context, channel string, email string, code string) bool {
	_, ok := e.VerifyAny(ctx, []string{channel}, email, code)
	return ok
}

// VerifyAny 按顺序在多个 channel 下查找验证码，命中则返回所用 channel
func (e *EmailStorage) VerifyAny(ctx context.Context, channels []string, email string, code string) (string, bool) {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)
	if email == "" || code == "" || len(channels) == 0 {
		return "", false
	}

	var lastStored string
	var lastChannel string
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}
		value, err := e.Get(ctx, channel, email)
		if err != nil || len(value) == 0 {
			logger.Infof("[EmailStorage.VerifyAny] miss - channel: %s, email: %s, err: %v", channel, email, err)
			continue
		}
		lastStored = value
		lastChannel = channel
		logger.Infof("[EmailStorage.VerifyAny] hit key - channel: %s, email: %s, stored: %s, provided: %s, match: %v",
			channel, email, value, code, value == code)
		if value == code {
			return channel, true
		}
	}

	if lastChannel == "" {
		logger.Infof("[EmailStorage.VerifyAny] no code found - email: %s, channels: %v", email, channels)
		return "", false
	}

	// 找到过 key 但验证码不匹配：累计失败次数（记在最后一个命中的 channel）
	num := e.redis.Incr(ctx, e.failName(lastChannel, email)).Val()
	logger.Infof("[EmailStorage.VerifyAny] mismatch - channel: %s, fail: %d, stored: %s, provided: %s", lastChannel, num, lastStored, code)
	if num >= 5 {
		_, _ = e.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, channel := range channels {
				pipe.Del(ctx, e.name(channel, email))
				pipe.Del(ctx, e.failName(channel, email))
			}
			return nil
		})
	} else if num == 1 {
		e.redis.Expire(ctx, e.failName(lastChannel, email), 3*time.Minute)
	}
	return "", false
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (e *EmailStorage) name(channel string, email string) string {
	return fmt.Sprintf("im:auth:email:%s:%s", channel, encrypt.Md5(email))
}

func (e *EmailStorage) failName(channel string, email string) string {
	return fmt.Sprintf("im:auth:email_fail:%s:%s", channel, encrypt.Md5(email))
}

func (e *EmailStorage) sendTimeName(email string) string {
	return fmt.Sprintf("im:auth:email_send_time:%s", encrypt.Md5(normalizeEmail(email)))
}

// CanSend 检查邮箱是否可以发送验证码（距离上次发送是否超过60秒）
func (e *EmailStorage) CanSend(ctx context.Context, email string) bool {
	ttl := e.redis.TTL(ctx, e.sendTimeName(email)).Val()
	return ttl <= 0 // TTL <= 0 表示key不存在或已过期，可以发送
}

// SetSendTime 记录邮箱验证码发送时间，60秒内不允许重复发送
func (e *EmailStorage) SetSendTime(ctx context.Context, email string) error {
	return e.redis.Set(ctx, e.sendTimeName(email), time.Now().Unix(), 60*time.Second).Err()
}
