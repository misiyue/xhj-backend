package queue

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

type SysNoticeConsumer struct {
	SysNoticeRepo *repo.SysNotice
	PushMessage   *logic.PushMessage
}

func (c *SysNoticeConsumer) Do(ctx context.Context, msg []byte) error {
	var in entity.SysNoticeQueueMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		logger.Errorf("sys_notice consumer unmarshal err: %s", err.Error())
		return err
	}
	if in.UserId <= 0 {
		logger.Warnf("sys_notice consumer skip: invalid user_id")
		return nil
	}

	row := &model.SysNotice{
		UserId:  in.UserId,
		Title:   truncateRunes(strings.TrimSpace(in.Title), 32),
		Content: truncateRunes(strings.TrimSpace(in.Content), 255),
		Url:     truncateRunes(strings.TrimSpace(in.Url), 255),
		IsRead:  model.SysNoticeUnread,
	}
	if err := c.SysNoticeRepo.Create(ctx, row); err != nil {
		return err
	}

	payload := entity.SubEventSysNoticePayload{
		UserId:    row.UserId,
		Id:        row.Id,
		Title:     row.Title,
		Content:   row.Content,
		Url:       row.Url,
		CreatedAt: timeutil.FormatDatetime(row.CreatedAt),
	}
	return c.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
		Event:   entity.SubEventSysNotice,
		Payload: jsonutil.Encode(payload),
	})
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	rs := []rune(s)
	return string(rs[:max])
}
