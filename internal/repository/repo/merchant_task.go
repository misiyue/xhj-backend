package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type MerchantTask struct {
	db *gorm.DB
}

const MerchantTaskCountEpsilon = 1e-4

func NewMerchantTask(db *gorm.DB) *MerchantTask {
	return &MerchantTask{db: db}
}

// marketListedScope 市场挂单：已上架、未删除、未完成、剩余数量必须大于 0
func marketListedScope(db *gorm.DB) *gorm.DB {
	return db.Model(&model.MerchantTask{}).
		Where("is_deleted = 0 AND is_up = 1 AND status <> ? AND `count` > ?",
			model.MerchantTaskStatusDone, MerchantTaskCountEpsilon)
}

func (r *MerchantTask) Create(ctx context.Context, row *model.MerchantTask) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *MerchantTask) FindByID(ctx context.Context, id int) (*model.MerchantTask, error) {
	var row model.MerchantTask
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantTask) FindOwned(ctx context.Context, id int, userId int) (*model.MerchantTask, error) {
	var row model.MerchantTask
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userId).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *MerchantTask) UpdateByID(ctx context.Context, id int, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.MerchantTask{}).Where("id = ?", id).Updates(updates).Error
}

// SumListedActiveCount 已上架且待交易/交易中的出售数量合计（不含 excludeID 为 0 时不排除）
func (r *MerchantTask) SumListedActiveCount(ctx context.Context, userId int, excludeID int) (float64, error) {
	q := r.db.WithContext(ctx).Model(&model.MerchantTask{}).
		Select("COALESCE(SUM(`count`),0)").
		Where("user_id = ? AND is_deleted = 0 AND is_up = 1 AND status <> ? AND `count` > ?",
			userId, model.MerchantTaskStatusDone, MerchantTaskCountEpsilon)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var sum float64
	if err := q.Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *MerchantTask) ListByUserID(ctx context.Context, userId int, page, pageSize int) ([]model.MerchantTask, int64, error) {
	var total int64
	base := r.db.WithContext(ctx).Model(&model.MerchantTask{}).Where("user_id = ?", userId)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.MerchantTask
	err := r.db.WithContext(ctx).Where("user_id = ?", userId).
		Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *MerchantTask) ListMarketPending(ctx context.Context, page, pageSize int) ([]model.MerchantTask, int64, error) {
	var total int64
	base := marketListedScope(r.db.WithContext(ctx))
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var rows []model.MerchantTask
	err := marketListedScope(r.db.WithContext(ctx)).
		Order("up_time DESC, id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
