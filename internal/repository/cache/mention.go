package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// MentionStorage 用于存储和管理群聊中的@提及消息
type MentionStorage struct {
	redis *redis.Client
}

// NewMentionStorage 创建MentionStorage实例
func NewMentionStorage(rds *redis.Client) *MentionStorage {
	return &MentionStorage{rds}
}

// mentionExpireAt @提及消息过期时间 - 7天
const mentionExpireAt = 7 * 24 * time.Hour

// AddMention 添加一条@提及消息记录
// @params uid     被@的用户ID
// @params groupId 群组ID
// @params msgId   消息ID
func (m *MentionStorage) AddMention(ctx context.Context, uid, groupId int, msgId string) error {
	key := m.key(uid, groupId)
	// 使用时间戳作为score，最新的消息score最大
	score := float64(time.Now().UnixMilli())

	pipe := m.redis.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: msgId})
	pipe.Expire(ctx, key, mentionExpireAt)
	_, err := pipe.Exec(ctx)
	return err
}

// GetMentions 获取用户在某个群的所有未读@提及消息ID列表
// 返回按时间倒序排列的消息ID列表（最新的在前）
func (m *MentionStorage) GetMentions(ctx context.Context, uid, groupId int) ([]string, error) {
	key := m.key(uid, groupId)
	// 按score从大到小排序（最新的在前）
	return m.redis.ZRevRange(ctx, key, 0, -1).Result()
}

// GetMentionCount 获取用户在某个群的未读@提及消息数量
func (m *MentionStorage) GetMentionCount(ctx context.Context, uid, groupId int) (int64, error) {
	key := m.key(uid, groupId)
	return m.redis.ZCard(ctx, key).Result()
}

// RemoveMention 移除一条@提及消息记录（用户已读取）
func (m *MentionStorage) RemoveMention(ctx context.Context, uid, groupId int, msgId string) error {
	key := m.key(uid, groupId)
	return m.redis.ZRem(ctx, key, msgId).Err()
}

// RemoveMentions 批量移除@提及消息记录
func (m *MentionStorage) RemoveMentions(ctx context.Context, uid, groupId int, msgIds []string) error {
	if len(msgIds) == 0 {
		return nil
	}
	key := m.key(uid, groupId)
	members := make([]interface{}, len(msgIds))
	for i, id := range msgIds {
		members[i] = id
	}
	return m.redis.ZRem(ctx, key, members...).Err()
}

// ClearMentions 清空用户在某个群的所有@提及消息记录
func (m *MentionStorage) ClearMentions(ctx context.Context, uid, groupId int) error {
	key := m.key(uid, groupId)
	return m.redis.Del(ctx, key).Err()
}

// GetAllGroupMentions 获取用户在所有群的未读@提及消息统计
// 返回 map[groupId]count
func (m *MentionStorage) GetAllGroupMentions(ctx context.Context, uid int) (map[int]int64, error) {
	pattern := fmt.Sprintf("im:mention:%d:*", uid)
	result := make(map[int]int64)

	// Use count of 100 to limit keys per iteration for better performance
	iter := m.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		var groupId int
		_, err := fmt.Sscanf(key, fmt.Sprintf("im:mention:%d:%%d", uid), &groupId)
		if err != nil {
			continue
		}

		count, err := m.redis.ZCard(ctx, key).Result()
		if err != nil {
			continue
		}

		if count > 0 {
			result[groupId] = count
		}
	}

	return result, iter.Err()
}

// key 生成缓存键
// im:mention:uid:groupId
func (m *MentionStorage) key(uid, groupId int) string {
	return fmt.Sprintf("im:mention:%d:%d", uid, groupId)
}
