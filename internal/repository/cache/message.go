package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/redis/go-redis/v9"
)

const (
	// lastMessageCacheKey 旧版全局 Hash（仅读兼容 / 删除时清理，不再写入）
	lastMessageCacheKey = "im:message:last_message"
	lastMessageKeyFmt   = "im:message:last:%s"
	lastMessageTTL      = 30 * 24 * time.Hour
)

type MessageStorage struct {
	redis *redis.Client
}

type LastCacheMessage struct {
	Content  string `json:"content"`
	Datetime string `json:"datetime"`
}

func NewMessageStorage(rds *redis.Client) *MessageStorage {
	return &MessageStorage{rds}
}

func (m *MessageStorage) Set(ctx context.Context, talkType int, sender int, receive int, message *LastCacheMessage) error {
	text := jsonutil.Encode(message)
	field := m.name(talkType, sender, receive)
	key := m.key(field)

	pipe := m.redis.Pipeline()
	pipe.Set(ctx, key, text, lastMessageTTL)
	// 迁移：新写入后从巨型 Hash 摘掉对应 field，避免双份占内存
	pipe.HDel(ctx, lastMessageCacheKey, field)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *MessageStorage) Get(ctx context.Context, talkType int, sender int, receive int) (*LastCacheMessage, error) {
	field := m.name(talkType, sender, receive)
	key := m.key(field)

	res, err := m.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// 兼容旧 Hash
		res, err = m.redis.HGet(ctx, lastMessageCacheKey, field).Result()
		if err != nil {
			return nil, err
		}
		msg := &LastCacheMessage{}
		if err = jsonutil.Unmarshal(res, msg); err != nil {
			return nil, err
		}
		// 读时迁移到独立 key
		_ = m.Set(ctx, talkType, sender, receive, msg)
		return msg, nil
	}
	if err != nil {
		return nil, err
	}

	msg := &LastCacheMessage{}
	if err = jsonutil.Unmarshal(res, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// Delete 清除会话最后一条消息缓存（私聊双方共用同一 field/key）
func (m *MessageStorage) Delete(ctx context.Context, talkType int, sender int, receive int) error {
	field := m.name(talkType, sender, receive)
	pipe := m.redis.Pipeline()
	pipe.Del(ctx, m.key(field))
	pipe.HDel(ctx, lastMessageCacheKey, field)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *MessageStorage) MGet(ctx context.Context, fields []string) ([]*LastCacheMessage, error) {
	if len(fields) == 0 {
		return []*LastCacheMessage{}, nil
	}

	keys := make([]string, len(fields))
	for i, field := range fields {
		keys[i] = m.key(field)
	}

	vals, err := m.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	items := make([]*LastCacheMessage, 0, len(fields))
	missing := make([]string, 0)
	missingIdx := make([]int, 0)

	for i, item := range vals {
		if val, ok := item.(string); ok && val != "" {
			msg := &LastCacheMessage{}
			if err := jsonutil.Unmarshal(val, msg); err != nil {
				continue
			}
			items = append(items, msg)
			continue
		}
		missing = append(missing, fields[i])
		missingIdx = append(missingIdx, i)
	}

	if len(missing) == 0 {
		return items, nil
	}

	// 兼容：未命中独立 key 时回落旧 Hash
	legacy, err := m.redis.HMGet(ctx, lastMessageCacheKey, missing...).Result()
	if err != nil {
		return items, nil
	}
	for _, item := range legacy {
		if val, ok := item.(string); ok && val != "" {
			msg := &LastCacheMessage{}
			if err := jsonutil.Unmarshal(val, msg); err != nil {
				continue
			}
			items = append(items, msg)
		}
	}

	_ = missingIdx // 仅用于兼容回落，不要求与 fields 一一对齐返回（历史 MGet 行为亦如此）
	return items, nil
}

func (m *MessageStorage) key(field string) string {
	return fmt.Sprintf(lastMessageKeyFmt, field)
}

func (m *MessageStorage) name(talkType int, sender int, receive int) string {
	if talkType == 2 {
		sender = 0
	}
	// 商户订单 C2C：按用户 + 会话维度，与 UnreadStorage 一致（receive 为 merchant_session.id）
	if talkType == entity.ChatMerchantMode {
		return fmt.Sprintf("%d_%d_%d", talkType, sender, receive)
	}

	if sender > receive {
		sender, receive = receive, sender
	}

	return fmt.Sprintf("%d_%d_%d", talkType, sender, receive)
}
