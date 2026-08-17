package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	registerIPKeyFmt     = "register:ip:%s"
	registerDeviceKeyFmt = "register:device:%s"
	registerIPTTL        = 24 * time.Hour
)

var (
	ErrRegisterIPExceeded     = errors.New("register ip limit exceeded")
	ErrRegisterDeviceExceeded = errors.New("register device limit exceeded")
)

// RegisterLimiter 注册频控（Redis）。成功占用后若业务失败须调用返回的 release。
type RegisterLimiter struct {
	RDB *redis.Client
}

func NewRegisterLimiter(rdb *redis.Client) *RegisterLimiter {
	if rdb == nil {
		return nil
	}
	return &RegisterLimiter{RDB: rdb}
}

func registerIPRedisKey(ip string) string {
	s := strings.TrimSpace(ip)
	if s == "" {
		s = "unknown"
	}
	return fmt.Sprintf(registerIPKeyFmt, s)
}

func registerDeviceRedisKey(deviceCode string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(deviceCode)))
	return fmt.Sprintf(registerDeviceKeyFmt, hex.EncodeToString(sum[:]))
}

// TryAcquireIP 为当前注册尝试占用 1 次 IP 计数（24h 滑动窗口自首次计数起 TTL）。
func (l *RegisterLimiter) TryAcquireIP(ctx context.Context, ip string, limit int) (release func(context.Context), err error) {
	if l == nil || l.RDB == nil || limit <= 0 {
		return func(context.Context) {}, nil
	}
	key := registerIPRedisKey(ip)
	n, err := l.RDB.Incr(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if n == 1 {
		_ = l.RDB.Expire(ctx, key, registerIPTTL).Err()
	}
	if int(n) > limit {
		_, _ = l.RDB.Decr(ctx, key).Result()
		return nil, ErrRegisterIPExceeded
	}
	return func(c context.Context) {
		if c == nil {
			c = context.Background()
		}
		_, _ = l.RDB.Decr(c, key).Result()
	}, nil
}

// TryAcquireDevice 占用 1 次设备计数（无 TTL，永久累计）。
func (l *RegisterLimiter) TryAcquireDevice(ctx context.Context, deviceCode string, limit int) (release func(context.Context), err error) {
	if l == nil || l.RDB == nil || limit <= 0 {
		return func(context.Context) {}, nil
	}
	key := registerDeviceRedisKey(deviceCode)
	n, err := l.RDB.Incr(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if int(n) > limit {
		_, _ = l.RDB.Decr(ctx, key).Result()
		return nil, ErrRegisterDeviceExceeded
	}
	return func(c context.Context) {
		if c == nil {
			c = context.Background()
		}
		_, _ = l.RDB.Decr(c, key).Result()
	}, nil
}
