package talk

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/repo"
)

// resolveTalkReceiverId 将客户端传入的 receiver_id 解析为真实会话对端/群 ID。
// 兼容三种入参：
//  1. talk_session.receiver_id（正确：私聊对方 uid / 群聊 group_id）
//  2. talk_session.id（会话列表 id / 群聊 session_id 常等于此）
//  3. talk_session.session_id（私聊关联 id）
func resolveTalkReceiverId(ctx context.Context, sessions *repo.TalkSession, uid, talkMode, receiverId int) int {
	if sessions == nil || uid <= 0 || talkMode <= 0 || receiverId <= 0 {
		return receiverId
	}

	if sess, err := sessions.FindByWhere(ctx, "user_id = ? AND talk_mode = ? AND receiver_id = ?", uid, talkMode, receiverId); err == nil && sess != nil && sess.Id > 0 {
		return receiverId
	}

	if sess, err := sessions.FindByWhere(ctx, "id = ? AND user_id = ? AND talk_mode = ?", receiverId, uid, talkMode); err == nil && sess != nil && sess.Id > 0 && sess.ReceiverId > 0 {
		return sess.ReceiverId
	}

	if sess, err := sessions.FindByWhere(ctx, "session_id = ? AND user_id = ? AND talk_mode = ?", receiverId, uid, talkMode); err == nil && sess != nil && sess.Id > 0 && sess.ReceiverId > 0 {
		return sess.ReceiverId
	}

	return receiverId
}
