package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type InviteCode struct {
	db *gorm.DB
}

func NewInviteCode(db *gorm.DB) *InviteCode {
	return &InviteCode{db: db}
}

// GenerateCode 生成随机邀请码
func (i *InviteCode) GenerateCode() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return fmt.Sprintf("INV%s", hex.EncodeToString(bytes))
}

// Create 创建邀请码
func (i *InviteCode) Create(ctx context.Context, userId int, expireDays int, maxUsage int) (*model.InviteCode, error) {
	code := i.GenerateCode()
	inviteCode := &model.InviteCode{
		Code:          code,
		UserId:        userId,
		Status:        model.InviteCodeStatusAvailable,
		ExpireAt:      time.Now().AddDate(0, 0, expireDays),
		MaxUsageCount: maxUsage,
		UsageCount:    0,
	}

	if err := i.db.WithContext(ctx).Create(inviteCode).Error; err != nil {
		return nil, err
	}

	return inviteCode, nil
}

// FindByCode 根据邀请码查找
func (i *InviteCode) FindByCode(ctx context.Context, code string) (*model.InviteCode, error) {
	var inviteCode model.InviteCode
	err := i.db.WithContext(ctx).
		Where("code = ?", code).
		First(&inviteCode).Error
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// FindByUserId 根据用户ID查找邀请码列表
func (i *InviteCode) FindByUserId(ctx context.Context, userId int) ([]*model.InviteCode, error) {
	var codes []*model.InviteCode
	err := i.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Find(&codes).Error
	return codes, err
}

// ValidateCode 验证邀请码是否有效
func (i *InviteCode) ValidateCode(ctx context.Context, code string) (bool, error) {
	inviteCode, err := i.FindByCode(ctx, code)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	// 检查状态
	if inviteCode.Status != model.InviteCodeStatusAvailable {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(inviteCode.ExpireAt) {
		return false, nil
	}

	// 检查使用次数
	// if inviteCode.UsageCount >= inviteCode.MaxUsageCount {
	// 	return false, nil
	// }

	return true, nil
}

// UseCode 使用邀请码
func (i *InviteCode) UseCode(ctx context.Context, code string, inviteeId int) error {
	return i.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁定记录
		var inviteCode model.InviteCode
		if err := tx.Clauses().
			Where("code = ?", code).
			First(&inviteCode).Error; err != nil {
			return err
		}

		// 验证有效性
		if inviteCode.Status != model.InviteCodeStatusAvailable {
			return fmt.Errorf("邀请码状态无效")
		}

		if time.Now().After(inviteCode.ExpireAt) {
			return fmt.Errorf("邀请码已过期")
		}

		// if inviteCode.UsageCount >= inviteCode.MaxUsageCount {
		// 	return fmt.Errorf("邀请码已达到使用上限")
		// }

		// 更新使用记录
		updates := map[string]any{
			"usage_count": gorm.Expr("usage_count + 1"),
			"invitee_id":  inviteeId,
			"used_at":     time.Now(),
		}

		// 如果达到最大使用次数，更新状态为已使用
		// if inviteCode.UsageCount+1 >= inviteCode.MaxUsageCount {
		// 	updates["status"] = model.InviteCodeStatusUsed
		// }

		return tx.Model(&model.InviteCode{}).
			Where("code = ?", code).
			Updates(updates).Error
	})
}

// DisableCode 禁用邀请码
func (i *InviteCode) DisableCode(ctx context.Context, code string) error {
	return i.db.WithContext(ctx).
		Model(&model.InviteCode{}).
		Where("code = ?", code).
		Update("status", model.InviteCodeStatusDisabled).Error
}

// GetInviteStats 获取用户邀请统计
func (i *InviteCode) GetInviteStats(ctx context.Context, userId int) (map[string]int, error) {
	var stats struct {
		TotalCodes       int64
		AvailableCodes   int64
		UsedCodes        int64
		TotalInvitations int64
	}

	// 总邀请码数
	if err := i.db.WithContext(ctx).
		Model(&model.InviteCode{}).
		Where("user_id = ?", userId).
		Count(&stats.TotalCodes).Error; err != nil {
		return nil, err
	}

	// 可用邀请码数
	if err := i.db.WithContext(ctx).
		Model(&model.InviteCode{}).
		Where("user_id = ? AND status = ?", userId, model.InviteCodeStatusAvailable).
		Count(&stats.AvailableCodes).Error; err != nil {
		return nil, err
	}

	// 已使用邀请码数
	if err := i.db.WithContext(ctx).
		Model(&model.InviteCode{}).
		Where("user_id = ? AND status = ?", userId, model.InviteCodeStatusUsed).
		Count(&stats.UsedCodes).Error; err != nil {
		return nil, err
	}

	// 总邀请人数
	if err := i.db.WithContext(ctx).
		Model(&model.InviteCode{}).
		Where("user_id = ?", userId).
		Select("COALESCE(SUM(usage_count), 0)").
		Scan(&stats.TotalInvitations).Error; err != nil {
		return nil, err
	}

	return map[string]int{
		"total_codes":       int(stats.TotalCodes),
		"available_codes":   int(stats.AvailableCodes),
		"used_codes":        int(stats.UsedCodes),
		"total_invitations": int(stats.TotalInvitations),
	}, nil
}
