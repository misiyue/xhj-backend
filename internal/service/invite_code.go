package service

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ IInviteCodeService = (*InviteCodeService)(nil)

type IInviteCodeService interface {
	// GenerateInviteCode 生成邀请码
	GenerateInviteCode(ctx context.Context, userId int, expireDays int, maxUsage int) (*model.InviteCode, error)
	// ValidateInviteCode 验证邀请码
	ValidateInviteCode(ctx context.Context, code string) (bool, error)
	// ResolveInviter 若邀请码有效则返回邀请人用户 ID（invite_code.user_id）
	ResolveInviter(ctx context.Context, code string) (ok bool, inviterUserId int, err error)
	// UseInviteCode 使用邀请码
	UseInviteCode(ctx context.Context, code string, inviteeId int) error
	// GetUserInviteCodes 获取用户的邀请码列表
	GetUserInviteCodes(ctx context.Context, userId int) ([]*model.InviteCode, error)
	// GetInviteStats 获取邀请统计
	GetInviteStats(ctx context.Context, userId int) (map[string]int, error)
	// DisableInviteCode 禁用邀请码
	DisableInviteCode(ctx context.Context, code string, userId int) error
}

type InviteCodeService struct {
	InviteCodeRepo *repo.InviteCode
}

func (s *InviteCodeService) GenerateInviteCode(ctx context.Context, userId int, expireDays int, maxUsage int) (*model.InviteCode, error) {
	// 默认参数
	if expireDays <= 0 {
		expireDays = 365 // 默认1年有效期
	}
	if maxUsage <= 0 {
		maxUsage = 1 // 默认只能使用1次
	}

	return s.InviteCodeRepo.Create(ctx, userId, expireDays, maxUsage)
}

func (s *InviteCodeService) ValidateInviteCode(ctx context.Context, code string) (bool, error) {
	if code == "" {
		return false, errors.New("邀请码不能为空")
	}

	return s.InviteCodeRepo.ValidateCode(ctx, code)
}

// ResolveInviter 校验邀请码并返回邀请人（生成该码的用户）ID；code 为空返回 ok=false
func (s *InviteCodeService) ResolveInviter(ctx context.Context, code string) (ok bool, inviterUserId int, err error) {
	if code == "" {
		return false, 0, nil
	}
	valid, err := s.ValidateInviteCode(ctx, code)
	if err != nil {
		return false, 0, err
	}
	if !valid {
		return false, 0, nil
	}
	ic, err := s.InviteCodeRepo.FindByCode(ctx, code)
	if err != nil {
		return false, 0, err
	}
	return true, ic.UserId, nil
}

func (s *InviteCodeService) UseInviteCode(ctx context.Context, code string, inviteeId int) error {
	// 先验证邀请码
	valid, err := s.ValidateInviteCode(ctx, code)
	if err != nil {
		return err
	}

	if !valid {
		return errors.New("邀请码无效或已过期")
	}

	return s.InviteCodeRepo.UseCode(ctx, code, inviteeId)
}

func (s *InviteCodeService) GetUserInviteCodes(ctx context.Context, userId int) ([]*model.InviteCode, error) {
	return s.InviteCodeRepo.FindByUserId(ctx, userId)
}

func (s *InviteCodeService) GetInviteStats(ctx context.Context, userId int) (map[string]int, error) {
	return s.InviteCodeRepo.GetInviteStats(ctx, userId)
}

func (s *InviteCodeService) DisableInviteCode(ctx context.Context, code string, userId int) error {
	// 验证邀请码是否属于该用户
	inviteCode, err := s.InviteCodeRepo.FindByCode(ctx, code)
	if err != nil {
		return err
	}

	if inviteCode.UserId != userId {
		return errors.New("无权限操作此邀请码")
	}

	return s.InviteCodeRepo.DisableCode(ctx, code)
}
