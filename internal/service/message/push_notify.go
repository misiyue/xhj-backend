package message

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/external/push"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

// TryOneSignalTemplatePush 用户已订阅且未开启会话免打扰时，按 notice_template.flag 发送 OneSignal 推送。
// vars 用于替换模板中的 {#key} 占位符。
// talkMode/receiverID 用于 talk_session 免打扰判断（私聊 receiverID 为对方 user_id，群聊为 group_id）；talkMode=0 时不检查。
func TryOneSignalTemplatePush(
	ctx context.Context,
	usersRepo *repo.Users,
	noticeTemplateRepo *repo.NoticeTemplate,
	talkSessionRepo *repo.TalkSession,
	userID, talkMode, receiverID int,
	templateFlag string,
	vars map[string]string,
) {
	if userID <= 0 || usersRepo == nil || noticeTemplateRepo == nil {
		return
	}
	user, err := usersRepo.FindByIdWithCache(ctx, userID)
	if err != nil || user == nil || user.IsSubscribe != model.UsersSubscribeYes {
		return
	}
	if talkMode > 0 && receiverID > 0 && talkSessionRepo != nil &&
		talkSessionRepo.IsDisturb(ctx, userID, receiverID, talkMode) {
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

// TryOneSignalChatPush 用户已订阅且未开启会话免打扰时，按 notice_template.flag 发送 OneSignal 推送。
func TryOneSignalChatPush(
	ctx context.Context,
	usersRepo *repo.Users,
	noticeTemplateRepo *repo.NoticeTemplate,
	talkSessionRepo *repo.TalkSession,
	userID, talkMode, receiverID int,
	templateFlag string,
) {
	TryOneSignalTemplatePush(ctx, usersRepo, noticeTemplateRepo, talkSessionRepo, userID, talkMode, receiverID, templateFlag, nil)
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
func (s *Service) tryOneSignalUserChat(ctx context.Context, userID, talkMode, receiverID int) {
	TryOneSignalChatPush(ctx, s.UsersRepo, s.NoticeTemplateRepo, s.TalkSessionRepo, userID, talkMode, receiverID, model.NoticeTemplateFlagUserChat)
}

// TryOneSignalRTCInvitePush rtc_invite 向对方（receiver_id）发送 OneSignal VoIP 推送。
func TryOneSignalRTCInvitePush(
	ctx context.Context,
	usersRepo *repo.Users,
	talkSessionRepo *repo.TalkSession,
	fromUserId, receiverId int,
) {
	if fromUserId <= 0 || receiverId <= 0 || usersRepo == nil {
		return
	}
	if push.GetClient() == nil {
		return
	}

	targetUser, err := usersRepo.FindByIdWithCache(ctx, receiverId)
	if err != nil || targetUser == nil || targetUser.IsSubscribe != model.UsersSubscribeYes {
		return
	}
	if talkSessionRepo != nil &&
		talkSessionRepo.IsDisturb(ctx, receiverId, fromUserId, entity.ChatPrivateMode) {
		return
	}

	fromUser, err := usersRepo.FindByIdWithCache(ctx, fromUserId)
	if err != nil || fromUser == nil {
		return
	}

	if err := push.SendRTCInviteVoIPToUser(receiverId, push.VoIPCallData{
		Event:          entity.PushEventImCallInvite,
		FromUserId:     fromUserId,
		ToUserId:       receiverId,
		FromUserName:   fromUser.Nickname,
		FromUserAvatar: fromUser.Avatar,
	}); err != nil {
		logger.Errorf("onesignal rtc_invite push err: from_id=%d receiver_id=%d %s", fromUserId, receiverId, err.Error())
	}
}

// TryOneSignalVoIPCallPush 发起音视频通话时向对方发送 OneSignal VoIP 推送。
func TryOneSignalVoIPCallPush(
	ctx context.Context,
	usersRepo *repo.Users,
	talkSessionRepo *repo.TalkSession,
	fromUserId, toUserId, callType int,
) {
	if fromUserId <= 0 || toUserId <= 0 || usersRepo == nil {
		return
	}
	if push.GetClient() == nil {
		return
	}

	targetUser, err := usersRepo.FindByIdWithCache(ctx, toUserId)
	if err != nil || targetUser == nil || targetUser.IsSubscribe != model.UsersSubscribeYes {
		return
	}
	if talkSessionRepo != nil &&
		talkSessionRepo.IsDisturb(ctx, toUserId, fromUserId, entity.ChatPrivateMode) {
		return
	}

	fromUser, err := usersRepo.FindByIdWithCache(ctx, fromUserId)
	if err != nil || fromUser == nil {
		return
	}

	if err := push.SendVoIPToUser(toUserId, push.VoIPCallData{
		Event:          entity.PushEventImCallInvite,
		FromUserId:     fromUserId,
		ToUserId:       toUserId,
		CallType:       callType,
		FromUserName:   fromUser.Nickname,
		FromUserAvatar: fromUser.Avatar,
	}); err != nil {
		logger.Errorf("onesignal voip call push err: from_id=%d to_id=%d %s", fromUserId, toUserId, err.Error())
	}
}

func (s *Service) tryOneSignalVoIPCall(ctx context.Context, fromUserId, toUserId, callType int) {
	TryOneSignalVoIPCallPush(ctx, s.UsersRepo, s.TalkSessionRepo, fromUserId, toUserId, callType)
}
