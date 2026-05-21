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

// CreateFromTask 创建订单并在事务内扣减挂单剩余数量（counts 须 >0 且不超过当前 count）
func (r *MerchantOrder) CreateFromTask(ctx context.Context, buyerID int, taskID int, counts float64, payType, buyType int) (*model.MerchantOrder, error) {
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
		if task.IsDeleted != 0 || task.IsUp != 1 || task.Status != model.MerchantTaskStatusPending {
			return ErrMerchantOrderTaskUnavailable
		}
		if task.Count <= MerchantTaskCountEpsilon {
			return ErrMerchantOrderTaskUnavailable
		}
		if counts <= 0 || counts > task.Count+MerchantTaskCountEpsilon {
			return ErrMerchantOrderInsufficientCount
		}
		newCount := task.Count - counts
		if newCount < MerchantTaskCountEpsilon {
			newCount = 0
		}
		amount := math.Round(task.Price*counts*100) / 100
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
		taskUpdates := map[string]any{"count": newCount}
		if newCount <= MerchantTaskCountEpsilon {
			taskUpdates["count"] = 0
			taskUpdates["status"] = model.MerchantTaskStatusDone
			taskUpdates["is_up"] = 0
		}
		if err := tx.Model(&model.MerchantTask{}).Where("id = ?", taskID).Updates(taskUpdates).Error; err != nil {
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

// CancelOrderTx 取消订单并恢复挂单为待交易（仅待支付/已支付可取消）
func (r *MerchantOrder) CancelOrderTx(ctx context.Context, orderID int, cancelID int, remark string) error {
	now := int(time.Now().Unix())
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o model.MerchantOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&o).Error; err != nil {
			return err
		}
		if o.IsCancel != 0 {
			return fmt.Errorf("订单已取消")
		}
		if o.Status != model.MerchantOrderStatusPendingPay && o.Status != model.MerchantOrderStatusPaid {
			return fmt.Errorf("当前状态不可取消")
		}
		if err := tx.Model(&model.MerchantOrder{}).Where("id = ?", orderID).Updates(map[string]any{
			"status":      model.MerchantOrderStatusCancelled,
			"is_cancel":   1,
			"cancel_id":   cancelID,
			"remark":      remark,
			"cancel_time": now,
		}).Error; err != nil {
			return err
		}
		if o.TaskId > 0 {
			var task model.MerchantTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", o.TaskId).First(&task).Error; err != nil {
				return err
			}
			newCount := task.Count + o.Counts
			if task.Total > MerchantTaskCountEpsilon && newCount > task.Total+MerchantTaskCountEpsilon {
				newCount = task.Total
			}
			updates := map[string]any{"count": newCount}
			if task.Status == model.MerchantTaskStatusDone && newCount > MerchantTaskCountEpsilon {
				updates["status"] = model.MerchantTaskStatusPending
			}
			if err := tx.Model(&model.MerchantTask{}).Where("id = ?", o.TaskId).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
