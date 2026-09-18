package group

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
)

// Totops 群置顶推广列表（按 sort 倒序）
func (g Group) Totops(ctx context.Context, in *web.GroupTotopsRequest) (*web.GroupTotopsResponse, error) {
	if g.GroupTotopRepo == nil {
		return nil, errors.New("GroupTotopRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	groupID := int(in.GetGroupId())

	group, err := g.GroupRepo.FindById(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group != nil && group.IsDismiss == model.Yes {
		return &web.GroupTotopsResponse{Items: []*web.GroupTotopsResponse_Item{}}, nil
	}
	if !g.GroupMemberRepo.IsMember(ctx, groupID, uid, false) {
		return nil, entity.ErrPermissionDenied
	}

	rows, err := g.GroupTotopRepo.ListByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	items := make([]*web.GroupTotopsResponse_Item, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		items = append(items, &web.GroupTotopsResponse_Item{
			Id:    int32(row.Id),
			Cover: row.Cover,
			Intro: row.Intro,
			Btn:   row.Btn,
			Url:   row.Url,
		})
	}
	return &web.GroupTotopsResponse{Items: items}, nil
}
