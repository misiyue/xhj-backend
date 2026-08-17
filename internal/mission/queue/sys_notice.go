package queue

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gzydong/go-chat/external/push"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

type SysNoticeConsumer struct {
	UsersRepo   *repo.Users
	UserClient  *cache.UserClient
	PushMessage *logic.PushMessage
}

func (c *SysNoticeConsumer) Do(ctx context.Context, msg []byte) error {
	var in entity.SysNoticeQueueMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		logger.Errorf("notice_letter consumer unmarshal err: %s", err.Error())
		return err
	}
	if in.UserId <= 0 || in.Id <= 0 {
		logger.Warnf("notice_letter consumer skip: invalid payload")
		return nil
	}

	title := strings.TrimSpace(in.Title)
	content := strings.TrimSpace(in.Content)
	url := strings.TrimSpace(in.Url)

	c.tryOneSignalNotice(ctx, in.UserId, title, content)

	payload := entity.SubEventSysNoticePayload{
		UserId:    in.UserId,
		Id:        in.Id,
		Title:     title,
		Content:   content,
		Url:       url,
		CreatedAt: strings.TrimSpace(in.CreatedAt),
	}
	return c.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event:   entity.SubEventSysNotice,
		Payload: jsonutil.Encode(payload),
	})
}

func (c *SysNoticeConsumer) tryOneSignalNotice(ctx context.Context, userID int, title, content string) {
	if userID <= 0 || c.UsersRepo == nil {
		return
	}
	user, err := c.UsersRepo.FindByIdWithCache(ctx, userID)
	if err != nil || user == nil || user.IsSubscribe != model.UsersSubscribeYes {
		return
	}
	if err := push.SendToUser(userID, push.Message{
		Title:    title,
		Subtitle: "",
		Contents: content,
	}); err != nil {
		logger.Errorf("notice_letter onesignal push err: user_id=%d %s", userID, err.Error())
	}
}
