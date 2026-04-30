package cache

import (
	"context"
	"fmt"
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
	_, err := e.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, e.failName(channel, email))
		pipe.Set(ctx, e.name(channel, email), code, exp)
		return nil
	})
	return err
}

func (e *EmailStorage) Get(ctx context.Context, channel string, email string) (string, error) {
	return e.redis.Get(ctx, e.name(channel, email)).Result()
}

func (e *EmailStorage) Del(ctx context.Context, channel string, email string) error {
	return e.redis.Del(ctx, e.name(channel, email)).Err()
}

func (e *EmailStorage) Verify(ctx context.Context, channel string, email string, code string) bool {
	value, err := e.Get(ctx, channel, email)
	if err != nil || len(value) == 0 {
		logger.Infof("[EmailStorage.Verify] Failed to get code from Redis - channel: %s, email: %s, error: %v", channel, email, err)
		return false
	}

	logger.Infof("[EmailStorage.Verify] Retrieved code from Redis - channel: %s, email: %s, stored: %s, provided: %s, match: %v", channel, email, value, code, value == code)

	if value == code {
		return true
	}

	// 3分钟内同一个邮箱验证码错误次数超过5次，删除验证码
	num := e.redis.Incr(ctx, e.failName(channel, email)).Val()
	logger.Infof("[EmailStorage.Verify] Code mismatch - fail count: %d", num)
	if num >= 5 {
		logger.Infof("[EmailStorage.Verify] Max failures reached, deleting code")
		_, _ = e.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, e.name(channel, email))
			pipe.Del(ctx, e.failName(channel, email))
			return nil
		})
	} else if num == 1 {
		e.redis.Expire(ctx, e.failName(channel, email), 3*time.Minute)
	}

	return false
}

func (e *EmailStorage) name(channel string, email string) string {
	return fmt.Sprintf("im:auth:email:%s:%s", channel, encrypt.Md5(email))
}

func (e *EmailStorage) failName(channel string, email string) string {
	return fmt.Sprintf("im:auth:email_fail:%s:%s", channel, encrypt.Md5(email))
}

func (e *EmailStorage) sendTimeName(email string) string {
	return fmt.Sprintf("im:auth:email_send_time:%s", encrypt.Md5(email))
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
