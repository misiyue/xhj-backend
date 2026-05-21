package v1

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

func merchantOrderToListProto(o *model.MerchantOrder) *web.UserMerchantOrderListItem {
	if o == nil {
		return nil
	}
	return &web.UserMerchantOrderListItem{
		Id:        int32(o.Id),
		OrderId:   o.OrderId,
		BuyerId:   int32(o.BuyerId),
		SalerId:   int32(o.SalerId),
		Amount:    o.Amount,
		TaskId:    int32(o.TaskId),
		Counts:    o.Counts,
		PayType:   int32(o.PayType),
		BuyType:   int32(o.BuyType),
		Status:    int32(o.Status),
		IsCancel:  int32(o.IsCancel),
		IsAppeal:  int32(o.IsAppeal),
		CreatedAt: timeutil.FormatDatetime(o.CreatedAt),
	}
}

func merchantOrderToProto(o *model.MerchantOrder) *web.UserMerchantOrderItem {
	if o == nil {
		return nil
	}
	return &web.UserMerchantOrderItem{
		Id:           int32(o.Id),
		OrderId:      o.OrderId,
		BuyerId:      int32(o.BuyerId),
		SalerId:      int32(o.SalerId),
		Amount:       o.Amount,
		TaskId:       int32(o.TaskId),
		Counts:       o.Counts,
		PayType:      int32(o.PayType),
		BuyType:      int32(o.BuyType),
		Status:       int32(o.Status),
		PayImg:       o.PayImg,
		IsCancel:     int32(o.IsCancel),
		IsAppeal:     int32(o.IsAppeal),
		AppealId:     int32(o.AppealId),
		AppealTime:   int32(o.AppealTime),
		AppealReason: o.AppealReason,
		CancelId:     int32(o.CancelId),
		Remark:       o.Remark,
		PayTime:      int32(o.PayTime),
		CancelTime:   int32(o.CancelTime),
		Wronger:      int32(o.Wronger),
		Judge:        o.Judge,
		JudgeTime:    int32(o.JudgeTime),
		CreatedAt:    timeutil.FormatDatetime(o.CreatedAt),
		UpdatedAt:    timeutil.FormatDatetime(o.UpdatedAt),
	}
}

func (u *User) assertOrderParticipant(o *model.MerchantOrder, uid int) error {
	if o == nil {
		return errorx.New(404, "订单不存在")
	}
	if o.BuyerId != uid && o.SalerId != uid {
		return errorx.New(403, "无权查看该订单")
	}
	return nil
}

// MerchantOrderCreate 创建订单（可购买不超过挂单剩余 count 的任意数量，事务内扣减 count）
func (u *User) MerchantOrderCreate(ctx context.Context, in *web.UserMerchantOrderCreateRequest) (*web.UserMerchantOrderCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	buyerID := int(session.UserId)
	row, err := u.MerchantOrderRepo.CreateFromTask(ctx, buyerID, int(in.GetTaskId()), in.GetCounts(), int(in.GetPayType()), int(in.GetBuyType()))
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrMerchantOrderSelfBuy):
			return nil, errorx.New(400, "不能购买自己的挂单")
		case errors.Is(err, repo.ErrMerchantOrderTaskUnavailable):
			return nil, errorx.New(400, "挂单不可购买（未上架、已售完、已完成或已删除）")
		case errors.Is(err, repo.ErrMerchantOrderInsufficientCount):
			return nil, errorx.New(400, "购买数量须大于 0 且不超过挂单剩余数量")
		default:
			if err.Error() == "任务不存在" {
				return nil, errorx.New(404, "任务不存在")
			}
			return nil, err
		}
	}
	return &web.UserMerchantOrderCreateResponse{Id: int32(row.Id), OrderId: row.OrderId}, nil
}

// MerchantOrderDetail 订单详情（买卖双方）
func (u *User) MerchantOrderDetail(ctx context.Context, in *web.UserMerchantOrderDetailRequest) (*web.UserMerchantOrderItem, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	o, err := u.MerchantOrderRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if err := u.assertOrderParticipant(o, uid); err != nil {
		return nil, err
	}
	return merchantOrderToProto(o), nil
}

// MerchantOrderCancel 取消订单（买卖双方在待支付/已支付阶段均可发起；挂单恢复待交易）
func (u *User) MerchantOrderCancel(ctx context.Context, in *web.UserMerchantOrderCancelRequest) (*web.UserMerchantOrderActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	o, err := u.MerchantOrderRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if err := u.assertOrderParticipant(o, uid); err != nil {
		return nil, err
	}
	err = u.MerchantOrderRepo.CancelOrderTx(ctx, o.Id, int(in.GetCancelId()), strings.TrimSpace(in.GetRemark()))
	if err != nil {
		if strings.Contains(err.Error(), "已取消") {
			return nil, errorx.New(400, "订单已取消")
		}
		if strings.Contains(err.Error(), "不可取消") {
			return nil, errorx.New(400, "当前订单状态不可取消")
		}
		return nil, err
	}
	return &web.UserMerchantOrderActionResponse{}, nil
}

// parseMerchantOrderStatusFilter 解析逗号分隔的订单状态，如 "0,1,3"
func parseMerchantOrderStatusFilter(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	seen := make(map[int]struct{})
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, errorx.New(400, "status 格式错误，请使用 0,1,2,3 逗号分隔")
		}
		if v < model.MerchantOrderStatusPendingPay || v > model.MerchantOrderStatusCancelled {
			return nil, errorx.New(400, "status 取值范围为 0-3")
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errorx.New(400, "status 格式错误，请使用 0,1,2,3 逗号分隔")
	}
	return out, nil
}

// MerchantOrderList 订单分页列表，direct：1 买家（默认），2 卖家；status 逗号分隔筛选
func (u *User) MerchantOrderList(ctx context.Context, in *web.UserMerchantOrderListRequest) (*web.UserMerchantOrderListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	direct := int(in.GetDirect())
	if direct == 0 {
		direct = 1
	}
	if direct != 1 && direct != 2 {
		return nil, errorx.New(400, "direct 须为 1（买家订单）或 2（卖家订单）")
	}
	statuses, err := parseMerchantOrderStatusFilter(in.GetStatus())
	if err != nil {
		return nil, err
	}
	asBuyer := direct == 1
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.MerchantOrderRepo.ListByParticipant(ctx, uid, asBuyer, statuses, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.UserMerchantOrderListItem, 0, len(rows))
	for i := range rows {
		items = append(items, merchantOrderToListProto(&rows[i]))
	}
	tot := int32(total)
	if total > math.MaxInt32 {
		tot = math.MaxInt32
	}
	return &web.UserMerchantOrderListResponse{Items: items, Total: tot}, nil
}

// MerchantOrderConfirmPay 买方确认已支付（上传凭证）
func (u *User) MerchantOrderConfirmPay(ctx context.Context, in *web.UserMerchantOrderConfirmPayRequest) (*web.UserMerchantOrderActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	o, err := u.MerchantOrderRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if o.BuyerId != uid {
		return nil, errorx.New(403, "仅买家可确认支付")
	}
	if o.IsCancel != 0 {
		return nil, errorx.New(400, "订单已取消")
	}
	if o.Status != model.MerchantOrderStatusPendingPay {
		return nil, errorx.New(400, "当前状态不可确认支付")
	}
	now := int(time.Now().Unix())
	err = u.MerchantOrderRepo.UpdateByID(ctx, o.Id, map[string]any{
		"status":   model.MerchantOrderStatusPaid,
		"pay_img":  strings.TrimSpace(in.GetPayImg()),
		"pay_time": now,
	})
	if err != nil {
		return nil, err
	}
	return &web.UserMerchantOrderActionResponse{}, nil
}

// MerchantOrderUrge 催单（买卖家均可；仅记录日志，不改变订单状态）
func (u *User) MerchantOrderUrge(ctx context.Context, in *web.UserMerchantOrderIdRequest) (*web.UserMerchantOrderActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	o, err := u.MerchantOrderRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if err := u.assertOrderParticipant(o, uid); err != nil {
		return nil, err
	}
	if o.IsCancel != 0 {
		return nil, errorx.New(400, "订单已取消")
	}
	if o.Status != model.MerchantOrderStatusPendingPay && o.Status != model.MerchantOrderStatusPaid {
		return nil, errorx.New(400, "当前状态无需催单")
	}
	logger.Infof("merchant_order urge: order_id=%d user_id=%d role=%s", o.Id, uid, urgeRoleLabel(o, uid))
	return &web.UserMerchantOrderActionResponse{}, nil
}

func urgeRoleLabel(o *model.MerchantOrder, uid int) string {
	if o.BuyerId == uid {
		return "buyer"
	}
	return "seller"
}

// MerchantOrderAppealSeller 卖家（商户）发起申诉
func (u *User) MerchantOrderAppealSeller(ctx context.Context, in *web.UserMerchantOrderAppealRequest) (*web.UserMerchantOrderActionResponse, error) {
	return u.merchantOrderAppeal(ctx, in, model.MerchantOrderAppealSideSeller)
}

// MerchantOrderAppealBuyer 买家发起申诉
func (u *User) MerchantOrderAppealBuyer(ctx context.Context, in *web.UserMerchantOrderAppealRequest) (*web.UserMerchantOrderActionResponse, error) {
	return u.merchantOrderAppeal(ctx, in, model.MerchantOrderAppealSideBuyer)
}

func (u *User) merchantOrderAppeal(ctx context.Context, in *web.UserMerchantOrderAppealRequest, side int) (*web.UserMerchantOrderActionResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	o, err := u.MerchantOrderRepo.FindByID(ctx, int(in.GetId()))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if side == model.MerchantOrderAppealSideBuyer && o.BuyerId != uid {
		return nil, errorx.New(403, "仅买家可发起买家申诉")
	}
	if side == model.MerchantOrderAppealSideSeller && o.SalerId != uid {
		return nil, errorx.New(403, "仅卖家可发起卖家申诉")
	}
	if o.IsCancel != 0 {
		return nil, errorx.New(400, "订单已取消，不可申诉")
	}
	if o.Status != model.MerchantOrderStatusPendingPay && o.Status != model.MerchantOrderStatusPaid {
		return nil, errorx.New(400, "当前订单状态不可申诉")
	}
	if o.IsAppeal != 0 {
		return nil, errorx.New(400, "订单已处于申诉中")
	}
	now := int(time.Now().Unix())
	err = u.MerchantOrderRepo.UpdateByID(ctx, o.Id, map[string]any{
		"is_appeal":      1,
		"appeal_id":      side,
		"appeal_time":    now,
		"appeal_reason":  strings.TrimSpace(in.GetAppealReason()),
	})
	if err != nil {
		return nil, err
	}
	return &web.UserMerchantOrderActionResponse{}, nil
}
