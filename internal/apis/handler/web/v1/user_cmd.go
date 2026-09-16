package v1

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

func userCmdToProto(row *model.UserCmd) *web.UserCmdItem {
	if row == nil {
		return nil
	}
	return &web.UserCmdItem{
		Id:        int32(row.Id),
		UserId:    int32(row.UserId),
		Title:     row.Title,
		Intro:     row.Intro,
		File:      row.File,
		Btns:      row.Btns,
		CreatedAt: timeutil.FormatDatetime(row.CreatedAt),
		UpdatedAt: timeutil.FormatDatetime(row.UpdatedAt),
	}
}

func validateUserCmdBtns(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return errorx.New(400, "btns 须为 JSON 数组")
	}
	return nil
}

// UserCmdList 当前用户指令列表（按 id 倒序）
func (u *User) UserCmdList(ctx context.Context, in *web.UserCmdListRequest) (*web.UserCmdListResponse, error) {
	if u.UserCmdRepo == nil {
		return nil, errorx.New(500, "UserCmdRepo 未注入")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.UserCmdRepo.ListByUserID(ctx, int(session.UserId), page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.UserCmdItem, 0, len(rows))
	for i := range rows {
		items = append(items, userCmdToProto(&rows[i]))
	}
	tot := int32(total)
	if total > math.MaxInt32 {
		tot = math.MaxInt32
	}
	return &web.UserCmdListResponse{Items: items, Total: tot}, nil
}

// UserCmdSave 保存用户指令（id=0 新增，id>0 编辑本人记录）
func (u *User) UserCmdSave(ctx context.Context, in *web.UserCmdSaveRequest) (*web.UserCmdSaveResponse, error) {
	if u.UserCmdRepo == nil {
		return nil, errorx.New(500, "UserCmdRepo 未注入")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	title := strings.TrimSpace(in.GetTitle())
	if title == "" {
		return nil, errorx.New(400, "标题不能为空")
	}
	btns := strings.TrimSpace(in.GetBtns())
	if err := validateUserCmdBtns(btns); err != nil {
		return nil, err
	}
	intro := strings.TrimSpace(in.GetIntro())
	file := strings.TrimSpace(in.GetFile())

	id := int(in.GetId())
	if id > 0 {
		row, err := u.UserCmdRepo.FindOwned(ctx, id, uid)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, errorx.New(404, "指令不存在")
		}
		updates := map[string]any{
			"title": title,
			"intro": intro,
			"file":  file,
			"btns":  btns,
		}
		if err := u.UserCmdRepo.UpdateByID(ctx, id, updates); err != nil {
			return nil, err
		}
		return &web.UserCmdSaveResponse{Id: int32(id)}, nil
	}

	row := &model.UserCmd{
		UserId: uid,
		Title:  title,
		Intro:  intro,
		File:   file,
		Btns:   btns,
	}
	if err := u.UserCmdRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &web.UserCmdSaveResponse{Id: int32(row.Id)}, nil
}

// UserCmdDel 删除本人用户指令
func (u *User) UserCmdDel(ctx context.Context, in *web.UserCmdDelRequest) (*web.UserCmdDelResponse, error) {
	if u.UserCmdRepo == nil {
		return nil, errorx.New(500, "UserCmdRepo 未注入")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	id := int(in.GetId())

	row, err := u.UserCmdRepo.FindOwned(ctx, id, uid)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errorx.New(404, "指令不存在")
	}
	if err := u.UserCmdRepo.DeleteOwned(ctx, id, uid); err != nil {
		return nil, err
	}
	return &web.UserCmdDelResponse{}, nil
}
