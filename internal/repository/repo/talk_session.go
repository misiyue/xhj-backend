package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type TalkSession struct {
	core.Repo[model.TalkSession]
}

func NewTalkSession(db *gorm.DB) *TalkSession {
	return &TalkSession{Repo: core.NewRepo[model.TalkSession](db)}
}

// IsDisturb 当前用户对某会话是否开启免打扰（is_disturb=1，与 session-disturb action=1 一致）
func (t *TalkSession) IsDisturb(ctx context.Context, uid int, receiverId int, talkType int) bool {
	if uid <= 0 || receiverId <= 0 || talkType <= 0 {
		return false
	}
	row, err := t.FindByWhere(ctx, "user_id = ? AND receiver_id = ? AND talk_mode = ?", uid, receiverId, talkType)
	if err != nil || row == nil || row.Id == 0 {
		return false
	}
	return row.IsDisturb == model.Yes
}

func (t *TalkSession) FindBySessionId(uid int, receiverId int, talkType int) int {

	resp, err := t.Repo.FindByWhere(context.TODO(), "user_id = ? and receiver_id = ? and talk_mode = ?", uid, receiverId, talkType)
	if err != nil {
		return 0
	}

	return resp.Id
}

// GetOrCreateSessionId 获取或创建 session_id，用于关联成对的会话
func (t *TalkSession) GetOrCreateSessionId(ctx context.Context, uid int, receiverId int, talkType int) (int, error) {
	resp, err := t.Repo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", uid, receiverId, talkType)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	if resp != nil && resp.SessionId > 0 {
		return resp.SessionId, nil
	}

	// 如果没有 session_id，返回当前记录的 ID（会自动关联）
	if resp != nil {
		return resp.Id, nil
	}

	return 0, nil
}
