package repo

import (
	"context"
	"errors"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantSession struct {
	db *gorm.DB
}

func NewMerchantSession(db *gorm.DB) *MerchantSession {
	return &MerchantSession{db: db}
}

func (r *MerchantSession) findActiveInTx(tx *gorm.DB, ownerID, peerID int) (*model.MerchantSession, error) {
	var row model.MerchantSession
	err := tx.Where("inviter_id = ? AND friend_id = ? AND delete_time = 0 AND status = ?", ownerID, peerID, 1).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindActiveByOwnerPeer 当前用户视角下未删除的会话（inviter_id=自己，friend_id=对方）
func (r *MerchantSession) FindActiveByOwnerPeer(ctx context.Context, ownerID, peerID int) (*model.MerchantSession, error) {
	return r.findActiveInTx(r.db.WithContext(ctx), ownerID, peerID)
}

func (r *MerchantSession) FindByID(ctx context.Context, id int) (*model.MerchantSession, error) {
	var row model.MerchantSession
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantSession) Create(ctx context.Context, row *model.MerchantSession) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// EnsurePair 确保买卖双方各有一条 delete_time=0 的会话；无则创建，共享 session_id 供消息关联
func (r *MerchantSession) EnsurePair(ctx context.Context, uid, peer, orderID int) (mine, theirs *model.MerchantSession, chatSessionID int, err error) {
	tag := model.MerchantSessionOrderTag(orderID)
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		mine, err = r.findActiveInTx(tx, uid, peer)
		if err != nil {
			return err
		}
		theirs, err = r.findActiveInTx(tx, peer, uid)
		if err != nil {
			return err
		}

		if mine == nil {
			linkID := 0
			if theirs != nil && theirs.SessionId > 0 {
				linkID = theirs.SessionId
			}
			mine = &model.MerchantSession{
				InviterId: uid,
				FriendId:  peer,
				IsTop:     2,
				Status:    1,
				Tags:      tag,
			}
			if err := tx.Create(mine).Error; err != nil {
				return err
			}
			if linkID == 0 {
				linkID = mine.Id
			}
			if err := tx.Model(mine).Update("session_id", linkID).Error; err != nil {
				return err
			}
			mine.SessionId = linkID
		}

		if theirs == nil {
			linkID := mine.SessionId
			if linkID == 0 {
				linkID = mine.Id
			}
			theirs = &model.MerchantSession{
				InviterId:  peer,
				FriendId:   uid,
				SessionId:  linkID,
				IsTop:      2,
				Status:     1,
				Tags:       tag,
			}
			if err := tx.Create(theirs).Error; err != nil {
				return err
			}
		}

		chatSessionID = mine.SessionId
		if chatSessionID == 0 {
			chatSessionID = mine.Id
		}
		if mine.SessionId != chatSessionID {
			if err := tx.Model(mine).Update("session_id", chatSessionID).Error; err != nil {
				return err
			}
			mine.SessionId = chatSessionID
		}
		if theirs.SessionId != chatSessionID {
			if err := tx.Model(theirs).Update("session_id", chatSessionID).Error; err != nil {
				return err
			}
			theirs.SessionId = chatSessionID
		}

		now := time.Now()
		for _, s := range []*model.MerchantSession{mine, theirs} {
			if err := tx.Model(s).Updates(map[string]any{
				"tags":       tag,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return mine, theirs, chatSessionID, err
}

// ChatSessionID 解析消息查询用的共享 session_id
func ChatSessionID(sess *model.MerchantSession) int {
	if sess == nil {
		return 0
	}
	if sess.SessionId > 0 {
		return sess.SessionId
	}
	return sess.Id
}

// ListVisibleByUser 当前用户的商户会话栏（inviter_id=自己 且未删除）
func (r *MerchantSession) ListVisibleByUser(ctx context.Context, userId int) ([]model.MerchantSession, error) {
	var rows []model.MerchantSession
	err := r.db.WithContext(ctx).Model(&model.MerchantSession{}).
		Where("inviter_id = ? AND delete_time = 0 AND status = ?", userId, 1).
		Order("updated_at DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
