package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const appNewsViewTTL = 30 * time.Second

type AppNewsViewStorage struct {
	redis *redis.Client
}

func NewAppNewsViewStorage(rds *redis.Client) *AppNewsViewStorage {
	return &AppNewsViewStorage{redis: rds}
}

// Acquire 对 news_id+user_id 设置 30s 缓存；返回 true 表示本次可写库，false 表示缓存已存在应跳过
func (s *AppNewsViewStorage) Acquire(ctx context.Context, newsId, userId int) (bool, error) {
	ok, err := s.redis.SetNX(ctx, s.key(newsId, userId), 1, appNewsViewTTL).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *AppNewsViewStorage) key(newsId, userId int) string {
	return fmt.Sprintf("im:app-news-view:%d:%d", newsId, userId)
}
