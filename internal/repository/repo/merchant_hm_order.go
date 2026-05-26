package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantHmOrder struct {
	db *gorm.DB
}

func NewMerchantHmOrder(db *gorm.DB) *MerchantHmOrder {
	return &MerchantHmOrder{db: db}
}

func (r *MerchantHmOrder) FindByOrderNo(ctx context.Context, orderNo string) (*model.MerchantHmOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, nil
	}
	var row model.MerchantHmOrder
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&row).Error
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

func (r *MerchantHmOrder) UpdateByOrderNo(ctx context.Context, orderNo string, updates map[string]any) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.MerchantHmOrder{}).Where("order_no = ?", orderNo).Updates(updates).Error
}

// ApplyNotifyAndMarkOrderPaid 更新汇美订单；支付成功时同步 merchant_order 为已支付（幂等）
func (r *MerchantHmOrder) ApplyNotifyAndMarkOrderPaid(ctx context.Context, orderNo string, hmUpdates map[string]any, payTimeUnix int) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return errors.New("order_no empty")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MerchantHmOrder{}).Where("order_no = ?", orderNo).Updates(hmUpdates).Error; err != nil {
			return err
		}
		status, _ := hmUpdates["status"].(string)
		if status != "success" {
			return nil
		}
		var mo model.MerchantOrder
		if err := tx.Where("order_id = ?", orderNo).First(&mo).Error; err != nil {
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
		return tx.Model(&model.MerchantOrder{}).Where("id = ? AND status = ?", mo.Id, model.MerchantOrderStatusPendingPay).
			Updates(map[string]any{
				"status":   model.MerchantOrderStatusPaid,
				"pay_time": payTimeUnix,
			}).Error
	})
}

// ParsePayedAt 解析回调支付时间
func ParseHmPayedAt(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t
		}
	}
	return nil
}
