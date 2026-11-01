package service

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

var _ IAuthService = (*AuthService)(nil)

type IAuthService interface {
	IsAuth(ctx context.Context, opt *AuthOption) error
}

type AuthService struct {
	OrganizeRepo    *repo.Organize
	ContactRepo     *repo.Contact
	GroupRepo       *repo.Group
	GroupMemberRepo *repo.GroupMember
}

type AuthOption struct {
	TalkType          int
	UserId            int
	ReceiverId        int
	IsVerifyGroupMute bool
}

func (a *AuthService) IsAuth(ctx context.Context, opt *AuthOption) error {

	if opt.TalkType == entity.ChatPrivateMode {
		if isOk, err := a.OrganizeRepo.IsQiyeMember(ctx, opt.UserId, opt.ReceiverId); err != nil {
			return errors.New("系统繁忙，请稍后再试！！！")
		} else if isOk {
			return nil
		}

		if a.ContactRepo.IsFriend(ctx, opt.UserId, opt.ReceiverId, false) {
			return nil
		}

		return errors.New("对方还不是你的好友！")
	}

	groupInfo, err := a.GroupRepo.FindById(ctx, opt.ReceiverId)
	if err != nil {
		return err
	}
	if groupInfo == nil {
		return errors.New("此群聊不存在！")
	}

	if groupInfo.IsDismiss == model.Yes {
		return errors.New("此群聊已解散！")
	}

	memberInfo, err := a.GroupMemberRepo.FindByUserId(ctx, opt.ReceiverId, opt.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("暂无权限发送消息！")
		}

		return errors.New("系统繁忙，请稍后再试！！！")
	}
	if memberInfo == nil {
		return errors.New("暂无权限发送消息！")
	}

	if memberInfo.IsQuit == model.Yes {
		return errors.New("暂无权限发送消息！")
	}

	// 成员个人禁言（对所有人生效，含群主/管理员）
	if memberInfo.IsMute == model.Yes {
		return errors.New("已被群主或管理员禁言！")
	}

	if !opt.IsVerifyGroupMute {
		return nil
	}

	// force_status=1：强制禁言，所有人不可发言（含群主、管理员）
	if groupInfo.ForceStatus == model.GroupForceStatusMute {
		return errors.New("该群已被强制禁言")
	}

	// is_mute 全员禁言：仅普通成员不可发言，群主和管理员仍可发言
	isManager := memberInfo.Leader == model.GroupMemberLeaderOwner ||
		memberInfo.Leader == model.GroupMemberLeaderAdmin
	if groupInfo.IsMute == model.Yes && !isManager {
		return errors.New("此群聊已开启全员禁言！")
	}

	return nil
}
