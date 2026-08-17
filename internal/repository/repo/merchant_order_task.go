package repo

import (
	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// merchantOrderActiveStatuses 进行中的订单（占用挂单库存，未完成也未取消）
var merchantOrderActiveStatuses = []int{
	model.MerchantOrderStatusPendingPay,
	model.MerchantOrderStatusPaid,
}

func countActiveOrdersOnTaskInTx(tx *gorm.DB, taskID int) (int64, error) {
	if taskID <= 0 {
		return 0, nil
	}
	var n int64
	err := tx.Model(&model.MerchantOrder{}).
		Where("task_id = ? AND is_cancel = 0 AND status IN ?", taskID, merchantOrderActiveStatuses).
		Count(&n).Error
	return n, err
}

// recalcMerchantTaskStatusInTx 根据 count 与进行中订单重算挂单 status（须在事务内调用，任务行已锁定或可再锁）
// count>0 → 待交易；count=0 且有进行中订单 → 交易中；count=0 且无进行中订单 → 完成并下架
func recalcMerchantTaskStatusInTx(tx *gorm.DB, taskID int) error {
	if taskID <= 0 {
		return nil
	}
	var task model.MerchantTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
		return err
	}
	if task.IsDeleted != 0 {
		return nil
	}
	active, err := countActiveOrdersOnTaskInTx(tx, taskID)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if task.Count > MerchantTaskCountEpsilon {
		updates["status"] = model.MerchantTaskStatusPending
	} else if active > 0 {
		updates["status"] = model.MerchantTaskStatusTrading
	} else {
		updates["status"] = model.MerchantTaskStatusDone
		updates["is_up"] = 0
	}
	return tx.Model(&model.MerchantTask{}).Where("id = ?", taskID).Updates(updates).Error
}

// deductMerchantTaskCountInTx 事务内扣减 count，并按规则更新 status（不在此处标记完成）
func deductMerchantTaskCountInTx(tx *gorm.DB, taskID int, counts float64) error {
	var task model.MerchantTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
		return err
	}
	newCount := task.Count - counts
	if newCount < MerchantTaskCountEpsilon {
		newCount = 0
	}
	res := tx.Model(&model.MerchantTask{}).
		Where("id = ? AND `count` >= ?", taskID, counts-MerchantTaskCountEpsilon).
		Updates(map[string]any{"count": newCount})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMerchantOrderInsufficientCount
	}
	return recalcMerchantTaskStatusInTx(tx, taskID)
}

// restoreMerchantTaskCountOnCancel 取消订单后在同一事务内回滚 count 并重算 status
func restoreMerchantTaskCountOnCancel(tx *gorm.DB, taskID int, orderCounts float64) error {
	if taskID <= 0 || orderCounts <= MerchantTaskCountEpsilon {
		return nil
	}
	var task model.MerchantTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
		return err
	}
	newCount := task.Count + orderCounts
	if task.Total > MerchantTaskCountEpsilon && newCount > task.Total+MerchantTaskCountEpsilon {
		newCount = task.Total
	}
	if err := tx.Model(&model.MerchantTask{}).Where("id = ?", taskID).Update("count", newCount).Error; err != nil {
		return err
	}
	return recalcMerchantTaskStatusInTx(tx, taskID)
}

// isMerchantTaskBuyable 挂单是否仍可下单（未完成、已上架、有剩余数量）
func isMerchantTaskBuyable(task *model.MerchantTask) bool {
	if task == nil {
		return false
	}
	if task.IsDeleted != 0 || task.IsUp != 1 {
		return false
	}
	if task.Status == model.MerchantTaskStatusDone {
		return false
	}
	return task.Count > MerchantTaskCountEpsilon
}
