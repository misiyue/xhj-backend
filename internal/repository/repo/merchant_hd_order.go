package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantHdOrder struct {
	db *gorm.DB
}

func NewMerchantHdOrder(db *gorm.DB) *MerchantHdOrder {
	return &MerchantHdOrder{db: db}
}

func (r *MerchantHdOrder) FindByOrderNo(ctx context.Context, orderNo string) (*model.MerchantHdOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, nil
	}
	var row model.MerchantHdOrder
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantHdOrder) Create(ctx context.Context, row *model.MerchantHdOrder) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *MerchantHdOrder) UpdateByOrderNo(ctx context.Context, orderNo string, updates map[string]any) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.MerchantHdOrder{}).Where("order_no = ?", orderNo).Updates(updates).Error
}

// ApplyNotifyAndMarkOrderPaid 更新宏达支付订单；支付成功时同步 merchant_order 为已支付（幂等）
// 返回值 marked 表示本次调用将订单从待支付更新为已支付。
func (r *MerchantHdOrder) ApplyNotifyAndMarkOrderPaid(ctx context.Context, orderNo string, hdUpdates map[string]any, payTimeUnix int) (bool, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return false, errors.New("order_no empty")
	}
	marked := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MerchantHdOrder{}).Where("order_no = ?", orderNo).Updates(hdUpdates).Error; err != nil {
			return err
		}
		status, _ := hdUpdates["status"].(string)
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

// ParseHdPayedAt 解析回调支付时间
func ParseHdPayedAt(s string) *time.Time {
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
