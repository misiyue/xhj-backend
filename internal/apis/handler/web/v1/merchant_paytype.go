package v1

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

func merchantPaytypeToProto(p *model.MerchantPaytype) *web.UserMerchantPaytypeItem {
	if p == nil {
		return nil
	}
	return &web.UserMerchantPaytypeItem{
		Id:        int32(p.Id),
		UserId:    int32(p.UserId),
		TypeId:    int32(p.TypeId),
		Account:   p.Account,
		Nickname:  p.Nickname,
		OpenBank:  p.OpenBank,
		IsDelete:  int32(p.IsDelete),
		CreatedAt: timeutil.FormatDatetime(p.CreatedAt),
		UpdatedAt: timeutil.FormatDatetime(p.UpdatedAt),
	}
}

// MerchantPaytypeCreate 创建收款方式
func (u *User) MerchantPaytypeCreate(ctx context.Context, in *web.UserMerchantPaytypeCreateRequest) (*web.UserMerchantPaytypeCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法管理收款方式"); err != nil {
		return nil, err
	}
	row := &model.MerchantPaytype{
		UserId:   uid,
		TypeId:   int(in.GetTypeId()),
		Account:  strings.TrimSpace(in.GetAccount()),
		Nickname: strings.TrimSpace(in.GetNickname()),
		OpenBank: strings.TrimSpace(in.GetOpenBank()),
		IsDelete: 0,
	}
	if err := u.MerchantPaytypeRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &web.UserMerchantPaytypeCreateResponse{Id: int32(row.Id)}, nil
}

// MerchantPaytypeUpdate 编辑收款方式
func (u *User) MerchantPaytypeUpdate(ctx context.Context, in *web.UserMerchantPaytypeUpdateRequest) (*web.UserMerchantPaytypeUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法管理收款方式"); err != nil {
		return nil, err
	}
	row, err := u.MerchantPaytypeRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errorx.New(404, "收款方式不存在")
	}
	if row.IsDelete != 0 {
		return nil, errorx.New(400, "该收款方式已作废，无法编辑")
	}
	updates := map[string]any{
		"type_id":   int(in.GetTypeId()),
		"account":   strings.TrimSpace(in.GetAccount()),
		"nickname":  strings.TrimSpace(in.GetNickname()),
		"open_bank": strings.TrimSpace(in.GetOpenBank()),
	}
	if err := u.MerchantPaytypeRepo.UpdateByID(ctx, row.Id, updates); err != nil {
		return nil, err
	}
	return &web.UserMerchantPaytypeUpdateResponse{}, nil
}

// MerchantPaytypeList 本人全部收款方式（含已作废），不分页
func (u *User) MerchantPaytypeList(ctx context.Context, _ *web.UserMerchantPaytypeListRequest) (*web.UserMerchantPaytypeListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法管理收款方式"); err != nil {
		return nil, err
	}
	rows, err := u.MerchantPaytypeRepo.ListAllByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	items := make([]*web.UserMerchantPaytypeItem, 0, len(rows))
	for i := range rows {
		items = append(items, merchantPaytypeToProto(&rows[i]))
	}
	return &web.UserMerchantPaytypeListResponse{Items: items}, nil
}

// MerchantPaytypeInvalidate 作废收款方式
func (u *User) MerchantPaytypeInvalidate(ctx context.Context, in *web.UserMerchantPaytypeInvalidateRequest) (*web.UserMerchantPaytypeInvalidateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法管理收款方式"); err != nil {
		return nil, err
	}
	row, err := u.MerchantPaytypeRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errorx.New(404, "收款方式不存在")
	}
	if row.IsDelete != 0 {
		return nil, errorx.New(400, "该收款方式已作废")
	}
	if err := u.MerchantPaytypeRepo.UpdateByID(ctx, row.Id, map[string]any{"is_delete": 1}); err != nil {
		return nil, err
	}
	return &web.UserMerchantPaytypeInvalidateResponse{}, nil
}
