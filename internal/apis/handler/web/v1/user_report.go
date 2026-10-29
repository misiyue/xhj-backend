package v1

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
)

type UserReportRequest struct {
	DefeId   int    `json:"defe_id" binding:"required,gte=1"`
	DefeType int    `json:"defe_type" binding:"required,oneof=1 2"`
	Reason   string `json:"reason" binding:"required"`
}

type UserReportResponse struct{}

// Report 用户举报
//
//	@Summary		用户举报
//	@Description	举报用户或群；用户需为好友，群需为群成员；同一对象 30 分钟内不可重复举报
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			request	body		UserReportRequest	true	"举报请求"
//	@Success		200		{object}	UserReportResponse
//	@Router			/api/v1/user/report [post]
//	@Security		Bearer
func (u *User) Report(ctx context.Context, in *UserReportRequest) (*UserReportResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if in == nil {
		return nil, errorx.New(400, "参数错误")
	}

	reason := in.Reason
	if utf8.RuneCountInString(reason) == 0 {
		return nil, errorx.New(400, "请填写举报原因")
	}
	if utf8.RuneCountInString(reason) > 255 {
		return nil, errorx.New(400, "举报原因过长")
	}

	if in.DefeType == model.UserReportDefeTypeUser {
		if in.DefeId == uid {
			return nil, errorx.New(400, "不能举报自己")
		}
		if u.ContactRepo == nil || !u.ContactRepo.IsFriend(ctx, uid, in.DefeId, false) {
			return nil, errorx.New(400, "对方还不是你的好友！")
		}
	} else if in.DefeType == model.UserReportDefeTypeGroup {
		if u.GroupMemberRepo == nil || !u.GroupMemberRepo.IsMember(ctx, in.DefeId, uid, false) {
			return nil, errorx.New(400, "你不是该群成员")
		}
	} else {
		return nil, errorx.New(400, "被诉类型错误")
	}

	if u.UserReportRepo == nil {
		return nil, errorx.New(500, "系统繁忙，请稍后再试")
	}

	exists, err := u.UserReportRepo.ExistsRecent(ctx, uid, in.DefeId, in.DefeType, 30*time.Minute)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errorx.New(400, "举报操作频繁")
	}

	now := time.Now()
	row := &model.UserReport{
		UserId:    uid,
		DefeId:    in.DefeId,
		DefeType:  in.DefeType,
		Reason:    reason,
		Status:    model.UserReportStatusPending,
		Result:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := u.UserReportRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &UserReportResponse{}, nil
}
