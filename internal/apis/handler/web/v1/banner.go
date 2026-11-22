package v1

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ web.IBannerHandler = (*Banner)(nil)

type Banner struct {
	BannerAdApplyRepo *repo.BannerAdApply
}

// AdApply 轮播广告投放申请
func (b *Banner) AdApply(ctx context.Context, in *web.BannerAdApplyRequest) (*web.BannerAdApplyResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if uid <= 0 {
		return nil, entity.ErrPermissionDenied
	}
	if b == nil || b.BannerAdApplyRepo == nil {
		return nil, errorx.New(500, "服务未就绪")
	}

	banner := strings.TrimSpace(in.GetBanner())
	title := strings.TrimSpace(in.GetTitle())
	if banner == "" || title == "" {
		return nil, entity.ErrInvalidParams
	}
	if utf8.RuneCountInString(banner) > 255 || utf8.RuneCountInString(title) > 255 {
		return nil, errorx.New(400, "参数长度超出限制")
	}

	pending, err := b.BannerAdApplyRepo.FindByWhere(ctx, "user_id = ? and status = ?", uid, model.BannerAdApplyStatusWait)
	if err != nil {
		return nil, err
	}
	if pending != nil && pending.Id > 0 {
		return nil, errorx.New(400, "你有申请在审核中")
	}

	if err := b.BannerAdApplyRepo.Create(ctx, &model.BannerAdApply{
		UserId: uid,
		Banner: banner,
		Title:  title,
		Status: model.BannerAdApplyStatusWait,
	}); err != nil {
		return nil, err
	}
	return &web.BannerAdApplyResponse{}, nil
}
