package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantMessage struct {
	db *gorm.DB
}

func NewMerchantMessage(db *gorm.DB) *MerchantMessage {
	return &MerchantMessage{db: db}
}

func (r *MerchantMessage) Create(ctx context.Context, row *model.MerchantMessage) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// ListBySessionDesc 分页，按 id 倒序（新消息在前）
func (r *MerchantMessage) ListBySessionDesc(ctx context.Context, sessionID int, page, pageSize int) ([]model.MerchantMessage, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.MerchantMessage{}).Where("session_id = ? AND is_deleted = ?", sessionID, model.No)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.MerchantMessage
	err := r.db.WithContext(ctx).Where("session_id = ? AND is_deleted = ?", sessionID, model.No).
		Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *MerchantMessage) FindLatestBySession(ctx context.Context, sessionID int) (*model.MerchantMessage, error) {
	var row model.MerchantMessage
	err := r.db.WithContext(ctx).Where("session_id = ? AND is_deleted = ?", sessionID, model.No).
		Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
