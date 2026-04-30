package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantPaytype struct {
	db *gorm.DB
}

func NewMerchantPaytype(db *gorm.DB) *MerchantPaytype {
	return &MerchantPaytype{db: db}
}

func (r *MerchantPaytype) Create(ctx context.Context, row *model.MerchantPaytype) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *MerchantPaytype) FindOwned(ctx context.Context, id int, userId int) (*model.MerchantPaytype, error) {
	var row model.MerchantPaytype
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userId).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantPaytype) UpdateByID(ctx context.Context, id int, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.MerchantPaytype{}).Where("id = ?", id).Updates(updates).Error
}

// ListAllByUserID 当前用户全部收款方式（含已作废），不分页
func (r *MerchantPaytype) ListAllByUserID(ctx context.Context, userId int) ([]model.MerchantPaytype, error) {
	var rows []model.MerchantPaytype
	err := r.db.WithContext(ctx).Where("user_id = ?", userId).Order("id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
