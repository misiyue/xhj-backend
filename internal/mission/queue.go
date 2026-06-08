package mission

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/mission/queue"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v2"
)

type QueueProvider struct {
	Consumers *queue.Consumers
	Redis     *redis.Client
}

func Queue(ctx *cli.Context, app *QueueProvider) error {
	topics := []string{entity.LoginTopic, entity.SysNoticeTopic}

	sub := app.Redis.Subscribe(ctx.Context, topics...)

	// nolint
	defer sub.Close()

	logger.Infof("subscribed to topics: %v", topics)

	for data := range sub.Channel(redis.WithChannelHealthCheckInterval(10 * time.Second)) {
		switch data.Channel {
		case entity.LoginTopic:
			_ = app.Consumers.UserLoginConsumer.Do(context.Background(), []byte(data.Payload), 1)
		case entity.SysNoticeTopic:
			_ = app.Consumers.SysNoticeConsumer.Do(context.Background(), []byte(data.Payload))
		}
	}

	return nil
}
