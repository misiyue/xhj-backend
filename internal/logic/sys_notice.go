package logic

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/redis/go-redis/v9"
)

type SysNotice struct {
	Redis              *redis.Client
	NoticeLetterRepo   *repo.NoticeLetter
	NoticeTemplateRepo *repo.NoticeTemplate
}

// PublishFromTemplate 按模板写入 notice_letter，并发布到 im.sys.notice（消费端仅推送，不再入库）
func (s *SysNotice) PublishFromTemplate(ctx context.Context, userID int, flag string, vars map[string]string, url string) error {
	if userID <= 0 || s == nil || s.Redis == nil || s.NoticeLetterRepo == nil || s.NoticeTemplateRepo == nil {
		return nil
	}
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return nil
	}
	tpl, err := s.NoticeTemplateRepo.FindByFlagCached(ctx, flag)
	if err != nil {
		return err
	}
	if tpl == nil {
		logger.Warnf("sys_notice template missing: flag=%s user_id=%d", flag, userID)
		return nil
	}
	title := truncateNoticeRunes(applyNoticeTemplateVars(tpl.Title, vars), 32)
	content := truncateNoticeRunes(applyNoticeTemplateVars(tpl.Content, vars), 255)
	row := &model.NoticeLetter{
		UserId:  userID,
		Title:   title,
		Content: content,
		Url:     truncateNoticeRunes(strings.TrimSpace(url), 255),
		IsRead:  model.NoticeLetterUnread,
	}
	if err := s.NoticeLetterRepo.Create(ctx, row); err != nil {
		return err
	}
	msg := entity.SysNoticeQueueMessage{
		Id:        row.Id,
		UserId:    row.UserId,
		Title:     row.Title,
		Content:   row.Content,
		Url:       row.Url,
		CreatedAt: timeutil.FormatDatetime(row.CreatedAt),
	}
	return s.Redis.Publish(ctx, entity.SysNoticeTopic, jsonutil.Encode(msg)).Err()
}

func applyNoticeTemplateVars(s string, vars map[string]string) string {
	if s == "" || len(vars) == 0 {
		return s
	}
	out := s
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{#"+k+"}", v)
	}
	return out
}

func truncateNoticeRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	rs := []rune(s)
	return string(rs[:max])
}
