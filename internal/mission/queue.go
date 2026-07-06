package mission

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
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

	runCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer stop()

	logger.Infof("queue worker started, topics: %v", topics)

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if err := runCtx.Err(); err != nil {
			logger.Infof("queue worker shutting down: %v", err)
			return nil
		}

		err := runQueueSubscribe(runCtx, app, topics)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.Infof("queue worker stopped")
			return nil
		}
		if err != nil {
			logger.Errorf("queue redis subscription error: %s, reconnecting in %s", err.Error(), backoff)
		} else {
			logger.Warnf("queue redis subscription closed, reconnecting in %s", backoff)
			backoff = time.Second
		}

		select {
		case <-runCtx.Done():
			logger.Infof("queue worker shutting down")
			return nil
		case <-time.After(backoff):
		}

		if err != nil {
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}
}

func runQueueSubscribe(ctx context.Context, app *QueueProvider, topics []string) error {
	sub := app.Redis.Subscribe(ctx, topics...)
	defer sub.Close()

	if _, err := sub.Receive(ctx); err != nil {
		return fmt.Errorf("redis subscribe receive: %w", err)
	}

	logger.Infof("queue subscribed to topics: %v", topics)

	ch := sub.Channel(redis.WithChannelHealthCheckInterval(10 * time.Second))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return errors.New("redis pubsub channel closed")
			}
			if msg == nil {
				continue
			}
			dispatchQueueMessage(app, msg)
		}
	}
}

func dispatchQueueMessage(app *QueueProvider, data *redis.Message) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("queue consumer panic: topic=%s err=%v", data.Channel, r)
		}
	}()

	bg := context.Background()
	switch data.Channel {
	case entity.LoginTopic:
		if err := app.Consumers.UserLoginConsumer.Do(bg, []byte(data.Payload), 1); err != nil {
			logger.Errorf("queue consumer %s error: %s", entity.LoginTopic, err.Error())
		}
	case entity.SysNoticeTopic:
		if err := app.Consumers.SysNoticeConsumer.Do(bg, []byte(data.Payload)); err != nil {
			logger.Errorf("queue consumer %s error: %s", entity.SysNoticeTopic, err.Error())
		}
	default:
		logger.Warnf("queue unknown topic: %s", data.Channel)
	}
}
