package repo

import (
	"context"
	"errors"
	"strings"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantHmOrder struct {
	db *gorm.DB
}

func NewMerchantHmOrder(db *gorm.DB) *MerchantHmOrder {
	return &MerchantHmOrder{db: db}
}

func (r *MerchantHmOrder) FindByOrderId(ctx context.Context, orderId string) (*model.MerchantHmOrder, error) {
	orderId = strings.TrimSpace(orderId)
	if orderId == "" {
		return nil, nil
	}
	var row model.MerchantHmOrder
	err := r.db.WithContext(ctx).Where("order_id = ?", orderId).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantHmOrder) Create(ctx context.Context, row *model.MerchantHmOrder) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *MerchantHmOrder) UpdateByOrderId(ctx context.Context, orderId string, updates map[string]any) error {
	orderId = strings.TrimSpace(orderId)
	if orderId == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.MerchantHmOrder{}).Where("order_id = ?", orderId).Updates(updates).Error
}

// ApplyNotifyAndMarkOrderPaid 更新汇美支付订单；restate=0 时同步 merchant_order 为已支付（幂等）
// 返回值 marked 表示本次调用将订单从待支付更新为已支付。
func (r *MerchantHmOrder) ApplyNotifyAndMarkOrderPaid(ctx context.Context, orderId string, restate int, payTimeUnix int) (bool, error) {
	orderId = strings.TrimSpace(orderId)
	if orderId == "" {
		return false, errors.New("order_id empty")
	}
	marked := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MerchantHmOrder{}).Where("order_id = ?", orderId).
			Update("restate", restate).Error; err != nil {
			return err
		}
		if restate != 0 {
			return nil
		}
		var mo model.MerchantOrder
		if err := tx.Where("order_id = ?", orderId).First(&mo).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if mo.IsCancel != 0 || mo.Status == model.MerchantOrderStatusCancelled {
			return nil
		}
		if mo.Status != model.MerchantOrderStatusPendingPay {
			return nil
		}
		res := tx.Model(&model.MerchantOrder{}).Where("id = ? AND status = ?", mo.Id, model.MerchantOrderStatusPendingPay).
			Updates(map[string]any{
				"status":   model.MerchantOrderStatusPaid,
				"pay_time": payTimeUnix,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			marked = true
		}
		return nil
	})
	return marked, err
}
