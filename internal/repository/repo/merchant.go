package repo

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type Merchant struct {
	db *gorm.DB
}

func NewMerchant(db *gorm.DB) *Merchant {
	return &Merchant{db: db}
}

// Create 插入一条商户申请
func (m *Merchant) Create(ctx context.Context, row *model.Merchant) error {
	return m.db.WithContext(ctx).Create(row).Error
}

// FindLatestByUserId 用户最近一次商户申请（按主键倒序）
func (m *Merchant) FindLatestByUserId(ctx context.Context, userId int) (*model.Merchant, error) {
	var row model.Merchant
	err := m.db.WithContext(ctx).Where("user_id = ?", userId).Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// HasApprovedByUserId 是否存在任意一条已审核通过的商户记录
func (m *Merchant) HasApprovedByUserId(ctx context.Context, userId int) (bool, error) {
	var n int64
	err := m.db.WithContext(ctx).Model(&model.Merchant{}).
		Where("user_id = ? AND status = ?", userId, model.MerchantStatusApproved).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateById 按主键更新字段（驳回后重新提交等）
func (m *Merchant) UpdateById(ctx context.Context, id int, updates map[string]any) error {
	return m.db.WithContext(ctx).Model(&model.Merchant{}).Where("id = ?", id).Updates(updates).Error
}

// MapApprovedNicknameByUserIDs 批量取用户最近一条已审核通过商户的 nickname（user_id -> nickname）
func (m *Merchant) MapApprovedNicknameByUserIDs(ctx context.Context, userIds []int) (map[int]string, error) {
	out := make(map[int]string)
	if len(userIds) == 0 {
		return out, nil
	}
	var rows []model.Merchant
	err := m.db.WithContext(ctx).
		Where("user_id IN ? AND status = ?", userIds, model.MerchantStatusApproved).
		Order("id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if _, ok := out[rows[i].UserId]; !ok {
			out[rows[i].UserId] = rows[i].Nickname
		}
	}
	return out, nil
}

// FindLatestApprovedByUserId 用户最近一条已审核通过的商户（按 id 倒序）
func (m *Merchant) FindLatestApprovedByUserId(ctx context.Context, userId int) (*model.Merchant, error) {
	var row model.Merchant
	err := m.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userId, model.MerchantStatusApproved).
		Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
