package v1

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

const suretyCountEpsilon = 1e-6

func isMerchantEffectiveForTask(m *model.Merchant, now int64) bool {
	if m == nil || m.Status != model.MerchantStatusApproved {
		return false
	}
	if m.IsClose != 0 {
		return false
	}
	if m.IsFrozen != 0 && int64(m.FrozenTime) > now {
		return false
	}
	if m.IsLimit != 0 && int64(m.LimitTime) > now {
		return false
	}
	return true
}

func (u *User) requireEffectiveMerchant(ctx context.Context, userId int, actionHint ...string) (*model.Merchant, error) {
	m, err := u.MerchantRepo.FindLatestApprovedByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if !isMerchantEffectiveForTask(m, now) {
		hint := "无法执行该操作"
		if len(actionHint) > 0 && actionHint[0] != "" {
			hint = actionHint[0]
		}
		return nil, errorx.New(403, "商户资质未通过或已被限制/封禁/关闭，"+hint)
	}
	return m, nil
}

func (u *User) assertListedSellCountWithinSurety(ctx context.Context, userId int, excludeTaskID int, addCount float64, surety float64) error {
	sum, err := u.MerchantTaskRepo.SumListedActiveCount(ctx, userId, excludeTaskID)
	if err != nil {
		return err
	}
	if sum+addCount > surety+suretyCountEpsilon {
		return errorx.New(400, "已上架且待交易/交易中的任务出售数量总和不能超过商户保证金")
	}
	return nil
}

func merchantTaskToProto(t *model.MerchantTask) *web.UserMerchantTaskItem {
	if t == nil {
		return nil
	}
	return &web.UserMerchantTaskItem{
		Id:           int32(t.Id),
		UserId:       int32(t.UserId),
		CurrencyType: int32(t.CurrencyType),
		Price:        t.Price,
		SellCount:    t.Count,
		Paytype:      t.Paytype,
		Status:       int32(t.Status),
		IsUp:         int32(t.IsUp),
		UpTime:       int32(t.UpTime),
		IsDeleted:    int32(t.IsDeleted),
		CreatedAt:    timeutil.FormatDatetime(t.CreatedAt),
		UpdatedAt:    timeutil.FormatDatetime(t.UpdatedAt),
	}
}

func normMerchantTaskPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// MerchantTaskCreate 发布挂售任务（默认未上架）
func (u *User) MerchantTaskCreate(ctx context.Context, in *web.UserMerchantTaskCreateRequest) (*web.UserMerchantTaskCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法操作挂售任务"); err != nil {
		return nil, err
	}
	row := &model.MerchantTask{
		UserId:       uid,
		CurrencyType: int(in.GetCurrencyType()),
		Price:        in.GetPrice(),
		Count:        in.GetSellCount(),
		Paytype:      strings.TrimSpace(in.GetPaytype()),
		Status:       model.MerchantTaskStatusPending,
		IsUp:         0,
		UpTime:       0,
		IsDeleted:    0,
	}
	if err := u.MerchantTaskRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &web.UserMerchantTaskCreateResponse{Id: int32(row.Id)}, nil
}

// MerchantTaskMyList 本人任务列表
func (u *User) MerchantTaskMyList(ctx context.Context, in *web.UserMerchantTaskMyListRequest) (*web.UserMerchantTaskListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.MerchantTaskRepo.ListByUserID(ctx, int(session.UserId), page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.UserMerchantTaskItem, 0, len(rows))
	for i := range rows {
		items = append(items, merchantTaskToProto(&rows[i]))
	}
	tot := int32(total)
	if total > math.MaxInt32 {
		tot = math.MaxInt32
	}
	return &web.UserMerchantTaskListResponse{Items: items, Total: tot}, nil
}

// MerchantTaskMarketList 全平台已上架且待交易
func (u *User) MerchantTaskMarketList(ctx context.Context, in *web.UserMerchantTaskMarketListRequest) (*web.UserMerchantTaskListResponse, error) {
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.MerchantTaskRepo.ListMarketPending(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.UserMerchantTaskItem, 0, len(rows))
	for i := range rows {
		items = append(items, merchantTaskToProto(&rows[i]))
	}
	tot := int32(total)
	if total > math.MaxInt32 {
		tot = math.MaxInt32
	}
	return &web.UserMerchantTaskListResponse{Items: items, Total: tot}, nil
}

// MerchantTaskDetail 任务详情
func (u *User) MerchantTaskDetail(ctx context.Context, in *web.UserMerchantTaskDetailRequest) (*web.UserMerchantTaskItem, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	t, err := u.MerchantTaskRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if t.IsDeleted != 0 && int(session.UserId) != t.UserId {
		return nil, errorx.New(404, "任务不存在")
	}
	return merchantTaskToProto(t), nil
}

// MerchantTaskUpdate 修改自己的任务
func (u *User) MerchantTaskUpdate(ctx context.Context, in *web.UserMerchantTaskUpdateRequest) (*web.UserMerchantTaskUpdateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法操作挂售任务"); err != nil {
		return nil, err
	}
	t, err := u.MerchantTaskRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if t.IsDeleted != 0 {
		return nil, errorx.New(400, "任务已结束，无法修改")
	}
	if t.Status != model.MerchantTaskStatusPending {
		return nil, errorx.New(400, "仅待交易中的任务可修改")
	}
	if t.IsUp != 0 {
		return nil, errorx.New(400, "仅下架状态的任务可修改，请先下架后再编辑")
	}
	newCount := in.GetSellCount()
	updates := map[string]any{
		"currency_type": int(in.GetCurrencyType()),
		"price":         in.GetPrice(),
		"count":         newCount,
		"paytype": strings.TrimSpace(in.GetPaytype()),
	}
	if err := u.MerchantTaskRepo.UpdateByID(ctx, t.Id, updates); err != nil {
		return nil, err
	}
	return &web.UserMerchantTaskUpdateResponse{}, nil
}

// MerchantTaskUp 上架
func (u *User) MerchantTaskUp(ctx context.Context, in *web.UserMerchantTaskIdRequest) (*web.UserMerchantTaskActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	m, err := u.requireEffectiveMerchant(ctx, uid, "无法操作挂售任务")
	if err != nil {
		return nil, err
	}
	t, err := u.MerchantTaskRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if t.IsDeleted != 0 {
		return nil, errorx.New(400, "任务已结束，无法上架")
	}
	if t.Status != model.MerchantTaskStatusPending {
		return nil, errorx.New(400, "仅待交易任务可上架")
	}
	if t.IsUp != 0 {
		return nil, errorx.New(400, "任务已处于上架状态")
	}
	if err := u.assertListedSellCountWithinSurety(ctx, uid, t.Id, t.Count, m.Surety); err != nil {
		return nil, err
	}
	now := int(time.Now().Unix())
	if err := u.MerchantTaskRepo.UpdateByID(ctx, t.Id, map[string]any{"is_up": 1, "up_time": now}); err != nil {
		return nil, err
	}
	return &web.UserMerchantTaskActionResponse{}, nil
}

// MerchantTaskDown 下架
func (u *User) MerchantTaskDown(ctx context.Context, in *web.UserMerchantTaskIdRequest) (*web.UserMerchantTaskActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法操作挂售任务"); err != nil {
		return nil, err
	}
	t, err := u.MerchantTaskRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if t.IsDeleted != 0 {
		return nil, errorx.New(400, "任务已结束")
	}
	if t.IsUp == 0 {
		return nil, errorx.New(400, "任务未上架")
	}
	if err := u.MerchantTaskRepo.UpdateByID(ctx, t.Id, map[string]any{"is_up": 0}); err != nil {
		return nil, err
	}
	return &web.UserMerchantTaskActionResponse{}, nil
}

// MerchantTaskFinish 结束任务（软删除）
func (u *User) MerchantTaskFinish(ctx context.Context, in *web.UserMerchantTaskIdRequest) (*web.UserMerchantTaskActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	if _, err := u.requireEffectiveMerchant(ctx, uid, "无法操作挂售任务"); err != nil {
		return nil, err
	}
	t, err := u.MerchantTaskRepo.FindOwned(ctx, int(in.GetId()), uid)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if t.IsDeleted != 0 {
		return nil, errorx.New(400, "任务已结束")
	}
	if err := u.MerchantTaskRepo.UpdateByID(ctx, t.Id, map[string]any{"is_deleted": 1, "is_up": 0}); err != nil {
		return nil, err
	}
	return &web.UserMerchantTaskActionResponse{}, nil
}
