package repo

import (
	"context"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantOrderErrLog struct {
	db *gorm.DB
}

func NewMerchantOrderErrLog(db *gorm.DB) *MerchantOrderErrLog {
	return &MerchantOrderErrLog{db: db}
}

func (r *MerchantOrderErrLog) Create(ctx context.Context, row *model.MerchantOrderErrLog) error {
	if row == nil {
		return nil
	}
	return r.db.WithContext(ctx).Create(row).Error
}
