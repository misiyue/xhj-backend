package group

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/service"
)

// Secret 开启/关闭进群密码（仅群主/管理员）
func (g Group) Secret(ctx context.Context, in *web.GroupSecretRequest) (*web.GroupSecretResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	group, err := g.loadManageableGroup(ctx, int(in.GetGroupId()), uid)
	if err != nil {
		return nil, err
	}

	isSecret := int(in.GetIsSecret())
	if isSecret == model.GroupIsSecretYes {
		var secretCode string
		if err := g.GroupRepo.Model(ctx).
			Select("IFNULL(secret_code, '')").
			Where("id = ?", group.Id).
			Scan(&secretCode).Error; err != nil {
			return nil, err
		}
		if strings.TrimSpace(secretCode) == "" {
			return nil, errorx.New(400, "请先设置入群密码")
		}
	}

	_, err = g.GroupRepo.UpdateByWhere(ctx, map[string]any{
		"is_secret":  isSecret,
		"updated_at": time.Now(),
	}, "id = ?", group.Id)
	if err != nil {
		return nil, err
	}
	return &web.GroupSecretResponse{}, nil
}

// SecretCode 设置入群密码（仅群主/管理员）
func (g Group) SecretCode(ctx context.Context, in *web.GroupSecretCodeRequest) (*web.GroupSecretCodeResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	group, err := g.loadManageableGroup(ctx, int(in.GetGroupId()), uid)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(in.GetSecretCode())
	if code == "" || utf8.RuneCountInString(code) > 8 {
		return nil, errorx.New(400, "入群密码长度为 1-8 个字符")
	}

	_, err = g.GroupRepo.UpdateByWhere(ctx, map[string]any{
		"secret_code": code,
		"updated_at":  time.Now(),
	}, "id = ?", group.Id)
	if err != nil {
		return nil, err
	}
	return &web.GroupSecretCodeResponse{}, nil
}

// SecretEnter 群外用户凭密码加入群聊
func (g Group) SecretEnter(ctx context.Context, in *web.GroupSecretEnterRequest) (*web.GroupSecretEnterResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	groupID := int(in.GetGroupId())
	if groupID <= 0 {
		return nil, errorx.New(400, "参数错误")
	}

	group, err := g.GroupRepo.FindById(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Id == 0 {
		return nil, entity.ErrGroupNotExist
	}
	if group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}
	if !group.SecretEnabled() {
		return nil, errorx.New(400, "该群未开启密码验证")
	}
	if strings.TrimSpace(group.SecretCode) == "" || strings.TrimSpace(in.GetSecretCode()) != strings.TrimSpace(group.SecretCode) {
		return nil, errorx.New(400, "入群密码错误")
	}

	if g.GroupMemberRepo.IsMember(ctx, groupID, uid, false) {
		return &web.GroupSecretEnterResponse{}, nil
	}

	key := fmt.Sprintf("group_join:%d", groupID)
	if !g.RedisLock.Lock(ctx, key, 20) {
		return nil, entity.ErrTooFrequentOperation
	}
	defer g.RedisLock.UnLock(ctx, key)

	if g.GroupMemberRepo.IsMember(ctx, groupID, uid, false) {
		return &web.GroupSecretEnterResponse{}, nil
	}

	count, err := g.GroupMemberRepo.FindCount(ctx, "group_id = ? and is_quit = ?", groupID, model.No)
	if err != nil {
		return nil, err
	}
	if int(count)+1 >= model.GroupMemberMaxNum {
		return nil, entity.ErrGroupMemberLimit
	}

	if err := g.GroupService.Invite(ctx, &service.GroupInviteOpt{
		UserId:    uid,
		GroupId:   groupID,
		MemberIds: []int{uid},
	}); err != nil {
		return nil, err
	}
	return &web.GroupSecretEnterResponse{}, nil
}

func (g Group) loadManageableGroup(ctx context.Context, groupID, uid int) (*model.Group, error) {
	if groupID <= 0 {
		return nil, errorx.New(400, "参数错误")
	}
	group, err := g.GroupRepo.FindById(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil || group.Id == 0 {
		return nil, entity.ErrGroupNotExist
	}
	if group.IsDismiss == model.Yes {
		return nil, entity.ErrGroupDismissed
	}
	if !g.GroupMemberRepo.IsLeader(ctx, groupID, uid) {
		return nil, entity.ErrPermissionDenied
	}
	return group, nil
}
