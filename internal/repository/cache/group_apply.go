package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const groupApplyUnreadTTL = 90 * 24 * time.Hour

type GroupApplyStorage struct {
	redis *redis.Client
}

func NewGroupApplyStorage(rds *redis.Client) *GroupApplyStorage {
	return &GroupApplyStorage{rds}
}

func (g *GroupApplyStorage) Incr(ctx context.Context, uid int) {
	key := g.name(uid)
	pipe := g.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, groupApplyUnreadTTL)
	_, _ = pipe.Exec(ctx)
}

func (g *GroupApplyStorage) Get(ctx context.Context, uid int) int {
	val, err := g.redis.Get(ctx, g.name(uid)).Int()
	if err != nil {
		return 0
	}

	return val
}

func (g *GroupApplyStorage) Del(ctx context.Context, uid int) {
	g.redis.Del(ctx, g.name(uid))
}

func (g *GroupApplyStorage) Decr(ctx context.Context, uid int) {
	key := g.name(uid)
	n, err := g.redis.Decr(ctx, key).Result()
	if err != nil || n <= 0 {
		_ = g.redis.Del(ctx, key).Err()
	}
}

func (g *GroupApplyStorage) name(uid int) string {
	return fmt.Sprintf("im:group:apply:unread:uid_%d", uid)
}
