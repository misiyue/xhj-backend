package message

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

// createMixedSegmentRows 将混合消息各子项拆成独立行写入原表：
// type→msg_type，content→extra，is_revoked=0（会话隐藏），org_msg_id=父消息 msg_id
func (s *Service) createMixedSegmentRows(ctx context.Context, talkMode, fromId, receiverId int, parentMsgId string, segments []CreateMixedMessageItem) error {
	if len(segments) == 0 || parentMsgId == "" {
		return nil
	}

	now := time.Now()
	db := s.Source.Db().WithContext(ctx)

	if talkMode == entity.ChatPrivateMode {
		sessionId := 0
		if s.TalkSessionRepo != nil {
			if sess, err := s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", fromId, receiverId, entity.ChatPrivateMode); err == nil && sess != nil {
				sessionId = sess.SessionId
				if sessionId == 0 {
					sessionId = sess.Id
				}
			}
		}

		ownerUserId := 0
		if s.ContactRepo != nil && s.ContactRepo.IsBlocked(ctx, receiverId, fromId) {
			ownerUserId = fromId
		}

		rows := make([]model.TalkUserMessage, 0, len(segments))
		for _, seg := range segments {
			rows = append(rows, model.TalkUserMessage{
				MsgId:      strutil.NewMsgId(),
				OrgMsgId:   parentMsgId,
				SessionId:  sessionId,
				MsgType:    seg.Type,
				UserId:     ownerUserId,
				ReceiverId: receiverId,
				FromId:     fromId,
				Extra:      seg.Content,
				Quote:      "{}",
				SendTime:   now,
				IsRevoked:  model.TalkMsgRevokedHidden,
				IsDeleted:  model.No,
			})
		}
		return db.Create(&rows).Error
	}

	if talkMode == entity.ChatGroupMode {
		rows := make([]model.TalkGroupMessage, 0, len(segments))
		for _, seg := range segments {
			rows = append(rows, model.TalkGroupMessage{
				MsgId:     strutil.NewMsgId(),
				OrgMsgId:  parentMsgId,
				MsgType:   seg.Type,
				GroupId:   receiverId,
				FromId:    fromId,
				Extra:     seg.Content,
				Quote:     "{}",
				SendTime:  now,
				IsRevoked: model.TalkMsgRevokedHidden,
				IsDeleted: model.No,
			})
		}
		return db.Create(&rows).Error
	}

	return nil
}
