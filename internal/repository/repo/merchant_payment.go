package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantPayment struct {
	db *gorm.DB
}

func NewMerchantPayment(db *gorm.DB) *MerchantPayment {
	return &MerchantPayment{db: db}
}

func (r *MerchantPayment) FindByID(ctx context.Context, id int) (*model.MerchantPayment, error) {
	if id <= 0 {
		return nil, nil
	}
	var row model.MerchantPayment
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantPayment) MapByIDs(ctx context.Context, ids []int) (map[int]*model.MerchantPayment, error) {
	if len(ids) == 0 {
		return map[int]*model.MerchantPayment{}, nil
	}
	uniq := make([]int, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return map[int]*model.MerchantPayment{}, nil
	}
	var rows []model.MerchantPayment
	if err := r.db.WithContext(ctx).Where("id IN ?", uniq).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int]*model.MerchantPayment, len(rows))
	for i := range rows {
		out[rows[i].Id] = &rows[i]
	}
	return out, nil
}
