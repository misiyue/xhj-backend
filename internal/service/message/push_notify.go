package message

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/external/push"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

// TryOneSignalTemplatePush 用户离线且已订阅时，按 notice_template.flag 发送 OneSignal 推送。
// vars 用于替换模板中的 {#key} 占位符；groupID > 0 时会检查群聊免打扰。
func TryOneSignalTemplatePush(
	ctx context.Context,
	userClient *cache.UserClient,
	usersRepo *repo.Users,
	noticeTemplateRepo *repo.NoticeTemplate,
	talkSessionRepo *repo.TalkSession,
	userID, groupID int,
	templateFlag string,
	vars map[string]string,
) {
	if userID <= 0 || userClient == nil || usersRepo == nil || noticeTemplateRepo == nil {
		return
	}
	if userClient.IsOnline(ctx, int64(userID)) {
		return
	}
	user, err := usersRepo.FindByIdWithCache(ctx, userID)
	if err != nil || user == nil || user.IsSubscribe != model.UsersSubscribeYes {
		return
	}
	if groupID > 0 && talkSessionRepo != nil && talkSessionRepo.IsDisturb(userID, groupID, entity.ChatGroupMode) {
		return
	}
	tpl, err := noticeTemplateRepo.FindByFlagCached(ctx, templateFlag)
	if err != nil || tpl == nil {
		if err != nil {
			logger.Errorf("notice_template %s load err: user_id=%d %s", templateFlag, userID, err.Error())
		}
		return
	}
	if err := push.SendToUser(userID, push.Message{
		Title:    applyNoticeTemplateVars(tpl.Title, vars),
		Subtitle: applyNoticeTemplateVars(tpl.Subtitle, vars),
		Contents: applyNoticeTemplateVars(tpl.Content, vars),
	}); err != nil {
		logger.Errorf("onesignal %s push err: user_id=%d %s", templateFlag, userID, err.Error())
	}
}

// TryOneSignalChatPush 用户离线且已订阅时，按 notice_template.flag 发送 OneSignal 推送。
// groupID > 0 时会检查群聊免打扰；私聊/商户聊天传 0。
func TryOneSignalChatPush(
	ctx context.Context,
	userClient *cache.UserClient,
	usersRepo *repo.Users,
	noticeTemplateRepo *repo.NoticeTemplate,
	talkSessionRepo *repo.TalkSession,
	userID, groupID int,
	templateFlag string,
) {
	TryOneSignalTemplatePush(ctx, userClient, usersRepo, noticeTemplateRepo, talkSessionRepo, userID, groupID, templateFlag, nil)
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

// tryOneSignalUserChat 普通私聊/群聊离线推送（userChat 模板）
func (s *Service) tryOneSignalUserChat(ctx context.Context, userID int, groupID int) {
	TryOneSignalChatPush(ctx, s.UserClient, s.UsersRepo, s.NoticeTemplateRepo, s.TalkSessionRepo, userID, groupID, model.NoticeTemplateFlagUserChat)
}
