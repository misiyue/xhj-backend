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

func (t *TalkSession) IsDisturb(uid int, receiverId int, talkType int) bool {
	resp, err := t.Repo.FindByWhere(context.TODO(), "user_id = ? and receiver_id = ? and talk_mode = ?", uid, receiverId, talkType)
	return err == nil && resp.IsDisturb == 1
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
