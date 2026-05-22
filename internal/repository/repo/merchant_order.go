package repo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMerchantOrderSelfBuy           = errors.New("merchant_order: cannot buy own task")
	ErrMerchantOrderTaskUnavailable   = errors.New("merchant_order: task not available for purchase")
	ErrMerchantOrderInsufficientCount = errors.New("merchant_order: purchase quantity exceeds remaining listing quantity")
	ErrMerchantOrderAlreadyCancelled  = errors.New("merchant_order: already cancelled")
	ErrMerchantOrderNotCancellable    = errors.New("merchant_order: not cancellable")
	ErrMerchantOrderSkipExpire = errors.New("merchant_order: skip expire cancel")
)

type MerchantOrder struct {
	db *gorm.DB
}

func NewMerchantOrder(db *gorm.DB) *MerchantOrder {
	return &MerchantOrder{db: db}
}

func (r *MerchantOrder) FindByID(ctx context.Context, id int) (*model.MerchantOrder, error) {
	var row model.MerchantOrder
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// FindByOrderNo 按对外订单号 merchant_order.order_id 查询（创建订单时生成的字符串，非自增 id）
func (r *MerchantOrder) FindByOrderNo(ctx context.Context, orderNo string) (*model.MerchantOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, nil
	}
	var row model.MerchantOrder
	err := r.db.WithContext(ctx).Where("order_id = ?", orderNo).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantOrder) UpdateByID(ctx context.Context, id int, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.MerchantOrder{}).Where("id = ?", id).Updates(updates).Error
}

// ListByParticipant 分页：asBuyer=true 查 buyer_id，否则查 saler_id；statuses 非空时按 status IN 筛选
func (r *MerchantOrder) ListByParticipant(ctx context.Context, userId int, asBuyer bool, statuses []int, page, pageSize int) ([]model.MerchantOrder, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.MerchantOrder{})
	if asBuyer {
		q = q.Where("buyer_id = ?", userId)
	} else {
		q = q.Where("saler_id = ?", userId)
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.MerchantOrder
	err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CreateFromTask 创建订单：单事务内锁挂单 → 扣减 count → 创建订单（任一步失败整体回滚）
func (r *MerchantOrder) CreateFromTask(ctx context.Context, buyerID int, taskID int, counts float64, payType string, buyType int) (*model.MerchantOrder, error) {
	var out *model.MerchantOrder
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.MerchantTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("任务不存在")
			}
			return err
		}
		if task.UserId == buyerID {
			return ErrMerchantOrderSelfBuy
		}
		if !isMerchantTaskBuyable(&task) {
			return ErrMerchantOrderTaskUnavailable
		}
		if counts <= 0 || counts > task.Count+MerchantTaskCountEpsilon {
			return ErrMerchantOrderInsufficientCount
		}
		if err := deductMerchantTaskCountInTx(tx, taskID, counts); err != nil {
			return err
		}
		amount := math.Round(task.Price*counts*100) / 100
		payType = strings.TrimSpace(payType)
		if payType == "" {
			payType = "0"
		}
		oid := "MO" + strings.ReplaceAll(uuid.New().String(), "-", "")
		row := &model.MerchantOrder{
			OrderId:  oid,
			BuyerId:  buyerID,
			SalerId:  task.UserId,
			Amount:   amount,
			TaskId:   taskID,
			Counts:   counts,
			PayType:  payType,
			BuyType:  buyType,
			Status:   model.MerchantOrderStatusPendingPay,
			IsCancel: 0,
			IsAppeal: 0,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// cancelOrderInTx 在已有事务内取消订单并恢复挂单 count（订单行须已 FOR UPDATE 锁定）
func cancelOrderInTx(tx *gorm.DB, o *model.MerchantOrder, cancelID int, remark string, allowedStatus map[int]struct{}) error {
	if o.IsCancel != 0 {
		return ErrMerchantOrderAlreadyCancelled
	}
	if _, ok := allowedStatus[o.Status]; !ok {
		return ErrMerchantOrderNotCancellable
	}
	statusList := make([]int, 0, len(allowedStatus))
	for s := range allowedStatus {
		statusList = append(statusList, s)
	}
	now := int(time.Now().Unix())
	res := tx.Model(&model.MerchantOrder{}).
		Where("id = ? AND is_cancel = 0 AND status IN ?", o.Id, statusList).
		Updates(map[string]any{
			"status":      model.MerchantOrderStatusCancelled,
			"is_cancel":   1,
			"cancel_id":   cancelID,
			"remark":      remark,
			"cancel_time": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMerchantOrderNotCancellable
	}
	return restoreMerchantTaskCountOnCancel(tx, o.TaskId, o.Counts)
}

// CancelOrderTx 手动取消：待支付/已支付均可；订单状态更新与 count 回滚同一事务
func (r *MerchantOrder) CancelOrderTx(ctx context.Context, orderID int, cancelID int, remark string) error {
	allowed := map[int]struct{}{
		model.MerchantOrderStatusPendingPay: {},
		model.MerchantOrderStatusPaid:       {},
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o model.MerchantOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&o).Error; err != nil {
			return err
		}
		return cancelOrderInTx(tx, &o, cancelID, remark, allowed)
	})
}

const (
	MerchantOrderUnpaidCancelTimeout = 30 * time.Minute
	MerchantOrderAutoCancelRemark    = "超时未支付自动取消"
)

// cancelUnpaidExpiredInTx 定时任务用：事务内锁单并校验「待支付 + 已超时」后取消并回滚 count
func cancelUnpaidExpiredInTx(tx *gorm.DB, orderID int, notAfter time.Time, cancelID int, remark string) error {
	var o model.MerchantOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMerchantOrderSkipExpire
		}
		return err
	}
	if o.IsCancel != 0 {
		return ErrMerchantOrderSkipExpire
	}
	if o.Status != model.MerchantOrderStatusPendingPay {
		return ErrMerchantOrderSkipExpire
	}
	if o.CreatedAt.After(notAfter) {
		return ErrMerchantOrderSkipExpire
	}
	allowed := map[int]struct{}{model.MerchantOrderStatusPendingPay: {}}
	return cancelOrderInTx(tx, &o, cancelID, remark, allowed)
}

// CancelExpiredUnpaid 批量取消超时未支付订单，每笔独立事务并在事务内二次校验
func (r *MerchantOrder) CancelExpiredUnpaid(ctx context.Context, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		timeout = MerchantOrderUnpaidCancelTimeout
	}
	notAfter := time.Now().Add(-timeout)
	const batchSize = 50
	cancelled := 0
	for {
		var ids []int
		err := r.db.WithContext(ctx).Model(&model.MerchantOrder{}).
			Where("status = ? AND is_cancel = 0 AND created_at <= ?",
				model.MerchantOrderStatusPendingPay, notAfter).
			Order("id ASC").Limit(batchSize).
			Pluck("id", &ids).Error
		if err != nil {
			return cancelled, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				return cancelUnpaidExpiredInTx(tx, id, notAfter, 0, MerchantOrderAutoCancelRemark)
			})
			if err != nil {
				if errors.Is(err, ErrMerchantOrderSkipExpire) {
					continue
				}
				return cancelled, err
			}
			cancelled++
		}
		if len(ids) < batchSize {
			break
		}
	}
	return cancelled, nil
}
