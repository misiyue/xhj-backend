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

func (r *MerchantSession) FindByOrderTag(ctx context.Context, orderID int) (*model.MerchantSession, error) {
	tag := model.MerchantSessionOrderTag(orderID)
	var row model.MerchantSession
	err := r.db.WithContext(ctx).Where("tags = ? AND status = ?", tag, 1).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
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

func (r *MerchantSession) TouchUpdatedAt(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Model(&model.MerchantSession{}).Where("id = ?", id).Update("updated_at", time.Now()).Error
}

// ListVisibleByUser 商户会话列表：merchant_session 中 inviter_id 或 friend_id 等于 userId 的记录，
// 且 status=1（展示）。与「买卖双方任一方拉列表」语义一致。
func (r *MerchantSession) ListVisibleByUser(ctx context.Context, userId int) ([]model.MerchantSession, error) {
	var rows []model.MerchantSession
	err := r.db.WithContext(ctx).Model(&model.MerchantSession{}).
		Where("inviter_id = ? OR friend_id = ?", userId, userId).
		Where("status = ?", 1).
		Order("updated_at DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
