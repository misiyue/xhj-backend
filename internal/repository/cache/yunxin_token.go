package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const yunxinTokenTTL = 2 * time.Hour

type YunxinTokenStorage struct {
	redis *redis.Client
}

func NewYunxinTokenStorage(rds *redis.Client) *YunxinTokenStorage {
	return &YunxinTokenStorage{redis: rds}
}

func (s *YunxinTokenStorage) Get(ctx context.Context, userId int) (string, error) {
	if s == nil || s.redis == nil {
		return "", nil
	}
	val, err := s.redis.Get(ctx, s.key(userId)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (s *YunxinTokenStorage) Set(ctx context.Context, userId int, token string) error {
	if s == nil || s.redis == nil {
		return nil
	}
	return s.redis.Set(ctx, s.key(userId), token, yunxinTokenTTL).Err()
}

func (s *YunxinTokenStorage) key(userId int) string {
	return fmt.Sprintf("im:yunxin:token:%d", userId)
}
