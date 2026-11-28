package group

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
)

// AdApply 群投放申请（仅群创建人）
func (g Group) AdApply(ctx context.Context, in *web.GroupAdApplyRequest) (*web.GroupAdApplyResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	groupID := int(in.GetGroupId())
	if groupID <= 0 {
		return nil, entity.ErrInvalidParams
	}

	intro := strings.TrimSpace(in.GetIntro())
	if utf8.RuneCountInString(intro) > 255 {
		return nil, errorx.New(400, "简介最多 255 个字符")
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
	if group.CreatorId != uid {
		return nil, entity.ErrPermissionDenied
	}

	pending, err := g.GroupAdApplyRepo.FindByWhere(ctx, "user_id = ? and status = ?", uid, model.GroupAdApplyStatusWait)
	if err != nil {
		return nil, err
	}
	if pending != nil && pending.Id > 0 {
		return nil, errorx.New(400, "你有申请在审核中")
	}

	if err := g.GroupAdApplyRepo.Create(ctx, &model.GroupAdApply{
		UserId:  uid,
		GroupId: groupID,
		Intro:   intro,
		Status:  model.GroupAdApplyStatusWait,
	}); err != nil {
		return nil, err
	}
	return &web.GroupAdApplyResponse{}, nil
}
