package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/external/hdpay"
	"github.com/gzydong/go-chat/external/hmpay"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

const merchantThirdPayURLExpire = 2 * time.Minute

func merchantOrderToListProto(o *model.MerchantOrder, merchantNickname string) *web.MerchantOrderListItem {
	if o == nil {
		return nil
	}
	return &web.MerchantOrderListItem{
		Id:               int32(o.Id),
		OrderId:          o.OrderId,
		BuyerId:          int32(o.BuyerId),
		SalerId:          int32(o.SalerId),
		Amount:           o.Amount,
		TaskId:           int32(o.TaskId),
		Counts:           o.Counts,
		PayTypeInfo:      o.PayTypeInfo,
		PayTypeId:        int32(o.PayTypeId),
		BuyType:          int32(o.BuyType),
		Status:           int32(o.Status),
		IsCancel:         int32(o.IsCancel),
		IsAppeal:         int32(o.IsAppeal),
		CreatedAt:        timeutil.FormatDatetime(o.CreatedAt),
		Wronger:          int32(o.Wronger),
		Judge:            o.Judge,
		JudgeTime:        timeutil.FormatUnixSecond(o.JudgeTime),
		MerchantNickname: merchantNickname,
		AppealMaterials:  o.AppealMaterials,
		CancelReason:     o.CancelReason,
		Remark:           o.Remark,
	}
}

func merchantOrderSalerIDs(rows []model.MerchantOrder) []int {
	seen := make(map[int]struct{}, len(rows))
	ids := make([]int, 0, len(rows))
	for i := range rows {
		uid := rows[i].SalerId
		if uid <= 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		ids = append(ids, uid)
	}
	return ids
}

type merchantOrderUserExtras struct {
	SalerUserNickname     string
	SalerUserAvatar       string
	SalerMerchantNickname string
	BuyerUserNickname     string
	BuyerUserAvatar       string
	BuyerMerchantNickname string
}

func (u *User) loadMerchantOrderUserExtras(ctx context.Context, buyerID, salerID int) merchantOrderUserExtras {
	var out merchantOrderUserExtras
	userIDs := make([]int, 0, 2)
	if buyerID > 0 {
		userIDs = append(userIDs, buyerID)
	}
	if salerID > 0 {
		userIDs = append(userIDs, salerID)
	}
	if len(userIDs) == 0 {
		return out
	}
	merchantNickMap, _ := u.merchantNicknamesByUserIDs(ctx, userIDs)
	if buyerID > 0 {
		if buyer, err := u.UsersRepo.FindByIdWithCache(ctx, buyerID); err == nil && buyer != nil {
			out.BuyerUserNickname = buyer.Nickname
			out.BuyerUserAvatar = buyer.Avatar
		}
		out.BuyerMerchantNickname = merchantNickMap[buyerID]
	}
	if salerID > 0 {
		if saler, err := u.UsersRepo.FindByIdWithCache(ctx, salerID); err == nil && saler != nil {
			out.SalerUserNickname = saler.Nickname
			out.SalerUserAvatar = saler.Avatar
		}
		out.SalerMerchantNickname = merchantNickMap[salerID]
	}
	return out
}

func merchantOrderToProto(o *model.MerchantOrder, extras merchantOrderUserExtras) *web.MerchantOrderItem {
	if o == nil {
		return nil
	}
	return &web.MerchantOrderItem{
		Id:                    int32(o.Id),
		OrderId:               o.OrderId,
		BuyerId:               int32(o.BuyerId),
		SalerId:               int32(o.SalerId),
		SalerUserNickname:     extras.SalerUserNickname,
		SalerUserAvatar:       extras.SalerUserAvatar,
		SalerMerchantNickname: extras.SalerMerchantNickname,
		BuyerUserNickname:     extras.BuyerUserNickname,
		BuyerUserAvatar:       extras.BuyerUserAvatar,
		BuyerMerchantNickname: extras.BuyerMerchantNickname,
		Amount:                o.Amount,
		TaskId:                int32(o.TaskId),
		Counts:                o.Counts,
		PayTypeInfo:           o.PayTypeInfo,
		PayTypeId:             int32(o.PayTypeId),
		BuyType:               int32(o.BuyType),
		Status:                int32(o.Status),
		PayImg:                o.PayImg,
		IsCancel:              int32(o.IsCancel),
		IsAppeal:              int32(o.IsAppeal),
		AppealId:              int32(o.AppealId),
		AppealTime:            int32(o.AppealTime),
		AppealReason:          o.AppealReason,
		AppealMaterials:       o.AppealMaterials,
		CancelId:              int32(o.CancelId),
		CancelReason:          o.CancelReason,
		Remark:                o.Remark,
		PayTime:               int32(o.PayTime),
		CancelTime:            int32(o.CancelTime),
		Wronger:               int32(o.Wronger),
		Judge:                 o.Judge,
		JudgeTime:             timeutil.FormatUnixSecond(o.JudgeTime),
		CreatedAt:             timeutil.FormatDatetime(o.CreatedAt),
		UpdatedAt:             timeutil.FormatDatetime(o.UpdatedAt),
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
func (u *User) MerchantOrderCreate(ctx context.Context, in *web.MerchantOrderCreateRequest) (*web.MerchantOrderCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	buyerID := int(session.UserId)
	taskID := int(in.GetTaskId())
	payTypeID := int(in.GetPayTypeId())
	counts := in.GetCounts()

	task, err := u.MerchantTaskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errorx.New(404, "任务不存在")
	}
	if platformKey, needLimit := model.MerchantOrderPayTypePlatformKey(payTypeID); needLimit {
		m, err := u.MerchantRepo.FindLatestApprovedByUserId(ctx, task.UserId)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, errorx.New(400, "卖家商户未通过审核")
		}
		if err := u.validateMerchantOrderPayAmount(ctx, task, counts, platformKey, m.PayTypes); err != nil {
			return nil, err
		}
	}

	row, err := u.MerchantOrderRepo.CreateFromTask(ctx, buyerID, taskID, counts, strings.TrimSpace(in.GetPayTypeInfo()), payTypeID, int(in.GetBuyType()), strings.TrimSpace(in.GetRemark()))
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
	return &web.MerchantOrderCreateResponse{Id: int32(row.Id), OrderId: row.OrderId}, nil
}

// MerchantOrderDetail 订单详情（买卖双方）
func (u *User) MerchantOrderDetail(ctx context.Context, in *web.MerchantOrderDetailRequest) (*web.MerchantOrderItem, error) {
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
	extras := u.loadMerchantOrderUserExtras(ctx, o.BuyerId, o.SalerId)
	return merchantOrderToProto(o, extras), nil
}

// MerchantOrderCancel 取消订单：仅买家可取消待支付/已支付订单
func (u *User) MerchantOrderCancel(ctx context.Context, in *web.MerchantOrderCancelRequest) (*web.MerchantOrderActionResponse, error) {
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
		return nil, errorx.New(403, "仅买家可取消订单")
	}
	err = u.MerchantOrderRepo.CancelOrderTx(ctx, o.Id, int(in.GetCancelId()), strings.TrimSpace(in.GetCancelReason()))
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrMerchantOrderAlreadyCancelled):
			return nil, errorx.New(400, "订单已取消")
		case errors.Is(err, repo.ErrMerchantOrderNotCancellable):
			return nil, errorx.New(400, "当前订单状态不可取消")
		default:
			return nil, err
		}
	}
	return &web.MerchantOrderActionResponse{}, nil
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

// MerchantOrderList 订单分页列表，direct：1 买家（默认），2 卖家；status 逗号分隔筛选；is_appeal=1 筛选申诉中未裁定
func (u *User) MerchantOrderList(ctx context.Context, in *web.MerchantOrderListRequest) (*web.MerchantOrderListResponse, error) {
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
	appealPending := in.GetIsAppeal() == 1
	asBuyer := direct == 1
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.MerchantOrderRepo.ListByParticipant(ctx, uid, asBuyer, statuses, appealPending, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.MerchantOrderListItem, 0, len(rows))
	nickMap, _ := u.merchantNicknamesByUserIDs(ctx, merchantOrderSalerIDs(rows))
	for i := range rows {
		items = append(items, merchantOrderToListProto(&rows[i], nickMap[rows[i].SalerId]))
	}
	tot := int32(total)
	if total > math.MaxInt32 {
		tot = math.MaxInt32
	}
	return &web.MerchantOrderListResponse{Items: items, Total: tot}, nil
}

// MerchantOrderConfirmPay 买方确认已支付（上传凭证）
func (u *User) MerchantOrderConfirmPay(ctx context.Context, in *web.MerchantOrderConfirmPayRequest) (*web.MerchantOrderActionResponse, error) {
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
	u.notifyMerchantOrderPaid(ctx, o)
	return &web.MerchantOrderActionResponse{}, nil
}

func (u *User) notifyMerchantOrderPaid(ctx context.Context, o *model.MerchantOrder) {
	if u.SysNotice == nil || o == nil || o.SalerId <= 0 {
		return
	}
	orderNo := strings.TrimSpace(o.OrderId)
	if orderNo == "" {
		return
	}
	if err := u.SysNotice.PublishFromTemplate(ctx, o.SalerId, model.NoticeTemplateFlagC2cPaid, map[string]string{
		"order": orderNo,
	}, ""); err != nil {
		logger.Errorf("merchant_order paid notice err: order_id=%s saler_id=%d %s", orderNo, o.SalerId, err.Error())
	}
}

func (u *User) notifyMerchantOrderPaidByOrderNo(ctx context.Context, orderNo string) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return
	}
	o, err := u.MerchantOrderRepo.FindByOrderNo(ctx, orderNo)
	if err != nil || o == nil {
		if err != nil {
			logger.Errorf("merchant_order paid notice load err: order_no=%s %s", orderNo, err.Error())
		}
		return
	}
	u.notifyMerchantOrderPaid(ctx, o)
}

// MerchantOrderUrge 催单（买卖家均可；仅记录日志，不改变订单状态）
func (u *User) MerchantOrderUrge(ctx context.Context, in *web.MerchantOrderIdRequest) (*web.MerchantOrderActionResponse, error) {
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
	return &web.MerchantOrderActionResponse{}, nil
}

func urgeRoleLabel(o *model.MerchantOrder, uid int) string {
	if o.BuyerId == uid {
		return "buyer"
	}
	return "seller"
}

// MerchantOrderAppealSeller 卖家（商户）发起申诉
func (u *User) MerchantOrderAppealSeller(ctx context.Context, in *web.MerchantOrderAppealRequest) (*web.MerchantOrderActionResponse, error) {
	return u.merchantOrderAppeal(ctx, in, model.MerchantOrderAppealSideSeller)
}

// MerchantOrderAppealBuyer 买家发起申诉
func (u *User) MerchantOrderAppealBuyer(ctx context.Context, in *web.MerchantOrderAppealRequest) (*web.MerchantOrderActionResponse, error) {
	return u.merchantOrderAppeal(ctx, in, model.MerchantOrderAppealSideBuyer)
}

func (u *User) merchantOrderAppeal(ctx context.Context, in *web.MerchantOrderAppealRequest, side int) (*web.MerchantOrderActionResponse, error) {
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
	err = u.MerchantOrderRepo.AppealOrderTx(ctx, o.Id, side, strings.TrimSpace(in.GetAppealReason()), strings.TrimSpace(in.GetAppealMaterials()))
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrMerchantOrderAlreadyAppeal):
			return nil, errorx.New(400, "该订单已被申诉，不可重复申诉")
		case errors.Is(err, repo.ErrMerchantOrderNotAppealable):
			return nil, errorx.New(400, "仅已支付且未申诉的订单可发起申诉")
		default:
			return nil, err
		}
	}
	return &web.MerchantOrderActionResponse{}, nil
}

// MerchantOrderPay 第三方统一下单：按订单 pay_type_id 分发；宏达支付未过期 pay_url 则直接返回
func (u *User) MerchantOrderPay(ctx context.Context, in *web.MerchantOrderPayRequest) (*web.MerchantOrderPayResponse, error) {
	orderNo := strings.TrimSpace(in.GetOrderId())
	if orderNo == "" {
		return nil, errorx.New(400, "order_id 不能为空")
	}
	o, err := u.MerchantOrderRepo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if o.IsCancel != 0 {
		return nil, errorx.New(400, "订单已取消")
	}
	if o.Status != model.MerchantOrderStatusPendingPay {
		return nil, errorx.New(400, "当前订单状态不可支付")
	}

	mch, err := u.MerchantRepo.FindLatestApprovedByUserId(ctx, o.SalerId)
	if err != nil {
		return nil, err
	}
	if mch == nil {
		return nil, errorx.New(400, "卖家商户未通过审核")
	}

	switch o.PayTypeId {
	case model.MerchantPayTypeHd:
		return u.merchantOrderPayHd(ctx, in, o, mch)
	case model.MerchantPayTypeHm:
		return u.merchantOrderPayHm(ctx, in, o, mch)
	default:
		return nil, errorx.New(400, "该订单支付方式不支持在线支付")
	}
}

// merchantOrderPayHd 宏达支付统一下单
func (u *User) merchantOrderPayHd(ctx context.Context, in *web.MerchantOrderPayRequest, o *model.MerchantOrder, mch *model.Merchant) (*web.MerchantOrderPayResponse, error) {
	hdc := u.ensureHdPayReady()
	if hdc == nil {
		return nil, errorx.New(500, "宏达支付未配置")
	}
	client := hdpay.GetClient()
	orderNo := o.OrderId

	channelPayType, err := u.merchantHdChannelPayType(ctx, mch.PayTypes)
	if err != nil {
		return nil, err
	}
	if channelPayType == "" {
		return nil, errorx.New(400, "商户未开通宏达支付")
	}

	existing, err := u.MerchantHdOrderRepo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if existing != nil && strings.TrimSpace(existing.PayURL) != "" {
		if time.Since(existing.CreatedAt) < merchantThirdPayURLExpire {
			return &web.MerchantOrderPayResponse{PayUrl: existing.PayURL, OrderId: orderNo}, nil
		}
	}
	if existing != nil {
		newOrderNo, err := u.MerchantOrderRepo.RefreshOrderNoForHdPay(ctx, o.Id, orderNo)
		if err != nil {
			return nil, err
		}
		orderNo = newOrderNo
		o.OrderId = newOrderNo
	}

	submitAmount := formatPayAmount(o.Amount)
	now := time.Now().Unix()
	userIP := middleware.ClientIPFromContext(ctx)
	req := &hdpay.CreateOrderRequest{
		SubmitAmount: submitAmount,
		OrderNo:      orderNo,
		NotifyURL:    hdc.NotifyURL,
		ReturnURL:    strings.TrimSpace(in.GetReturnUrl()),
		AppID:        hdc.AppID,
		Time:         now,
		PayType:      channelPayType,
		UserIP:       userIP,
	}
	data, err := client.CreateOrder(req)
	if err != nil {
		return nil, errorx.New(400, err.Error())
	}

	amt, _ := strconv.ParseFloat(submitAmount, 64)
	row := &model.MerchantHdOrder{
		OrderNo:      orderNo,
		LocalNo:      data.LocalNo,
		PayType:      channelPayType,
		PayURL:       data.PayURL,
		SubmitAmount: amt,
	}
	if existing != nil {
		err = u.MerchantHdOrderRepo.UpdateByOrderNo(ctx, orderNo, map[string]any{
			"local_no":      data.LocalNo,
			"pay_type":      channelPayType,
			"pay_url":       data.PayURL,
			"submit_amount": amt,
			"created_at":    time.Now(),
		})
	} else {
		err = u.MerchantHdOrderRepo.Create(ctx, row)
	}
	if err != nil {
		return nil, err
	}
	return &web.MerchantOrderPayResponse{PayUrl: data.PayURL, OrderId: orderNo}, nil
}

// merchantOrderPayHm 汇美支付统一下单
func (u *User) merchantOrderPayHm(ctx context.Context, in *web.MerchantOrderPayRequest, o *model.MerchantOrder, mch *model.Merchant) (*web.MerchantOrderPayResponse, error) {
	hmc := u.ensureHmPayReady()
	if hmc == nil {
		return nil, errorx.New(500, "汇美支付未配置")
	}
	orderNo := o.OrderId

	channelPayType, err := u.merchantHmChannelPayType(ctx, mch.PayTypes)
	if err != nil {
		return nil, err
	}
	if channelPayType == "" {
		return nil, errorx.New(400, "商户未开通汇美支付")
	}

	existing, err := u.MerchantHmOrderRepo.FindByOrderId(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if existing != nil && strings.TrimSpace(existing.PayURL) != "" {
		if time.Since(existing.CreatedAt) < merchantThirdPayURLExpire {
			return &web.MerchantOrderPayResponse{PayUrl: existing.PayURL, OrderId: orderNo}, nil
		}
	}
	if existing != nil {
		newOrderNo, err := u.MerchantOrderRepo.RefreshOrderNoForHmPay(ctx, o.Id, orderNo)
		if err != nil {
			return nil, err
		}
		orderNo = newOrderNo
		o.OrderId = newOrderNo
	}

	submitAmount := formatPayAmount(o.Amount)
	payURL, err := hmpay.GetClient().CreateOrder(&hmpay.CreateOrderRequest{
		OrderID:     orderNo,
		Value:       submitAmount,
		PayType:     channelPayType,
		CallbackURL: hmc.CallbackURL,
		HrefBackURL: strings.TrimSpace(in.GetReturnUrl()),
	})
	if err != nil {
		return nil, errorx.New(400, err.Error())
	}

	row := &model.MerchantHmOrder{
		OrderId: orderNo,
		PayURL:  payURL,
	}
	if existing != nil {
		err = u.MerchantHmOrderRepo.UpdateByOrderId(ctx, orderNo, map[string]any{
			"pay_url":    payURL,
			"created_at": time.Now(),
		})
	} else {
		err = u.MerchantHmOrderRepo.Create(ctx, row)
	}
	if err != nil {
		return nil, err
	}
	return &web.MerchantOrderPayResponse{PayUrl: payURL, OrderId: orderNo}, nil
}

// MerchantOrderHdpayNotify 宏达支付回调（返回纯文本 success / fail）
func (u *User) MerchantOrderHdpayNotify(c *gin.Context) {
	mc := u.hdpayConfig()
	if mc == nil {
		c.String(200, "fail")
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(200, "fail")
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		c.String(200, "fail")
		return
	}
	sign, _ := raw["sign"].(string)
	params := hdpay.ParamsFromMap(raw)
	if !hdpay.VerifySign(params, mc.AppSecret, sign) {
		c.String(200, "fail")
		return
	}

	orderNo, _ := raw["order_no"].(string)
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		c.String(200, "fail")
		return
	}

	status, _ := raw["status"].(string)
	statusText, _ := raw["status_text"].(string)
	localNo, _ := raw["local_no"].(string)
	payedAtStr := ""
	if v, ok := raw["payed_at"].(string); ok {
		payedAtStr = v
	}

	var submitAmount float64
	switch v := raw["submit_amount"].(type) {
	case float64:
		submitAmount = v
	case string:
		submitAmount, _ = strconv.ParseFloat(v, 64)
	}

	hdUpdates := map[string]any{
		"local_no":      strings.TrimSpace(localNo),
		"status":        strings.TrimSpace(status),
		"status_text":   strings.TrimSpace(statusText),
		"submit_amount": submitAmount,
	}
	if t := repo.ParseHdPayedAt(payedAtStr); t != nil {
		hdUpdates["payed_at"] = t
	}

	payTimeUnix := int(time.Now().Unix())
	if t := repo.ParseHdPayedAt(payedAtStr); t != nil {
		payTimeUnix = int(t.Unix())
	}

	ctx := c.Request.Context()
	marked, err := u.MerchantHdOrderRepo.ApplyNotifyAndMarkOrderPaid(ctx, orderNo, hdUpdates, payTimeUnix)
	if err != nil {
		c.String(200, "fail")
		return
	}
	if marked {
		u.notifyMerchantOrderPaidByOrderNo(ctx, orderNo)
	}
	c.String(200, "success")
}

// MerchantOrderHmpayNotify 汇美支付回调（返回纯文本 success / fail）
func (u *User) MerchantOrderHmpayNotify(c *gin.Context) {
	mc := u.hmpayConfig()
	if mc == nil {
		c.String(200, "fail")
		return
	}
	orderID := notifyParam(c, "orderid")
	restateStr := notifyParam(c, "restate")
	ovalue := notifyParam(c, "ovalue")
	sign := notifyParam(c, "sign")
	if orderID == "" || restateStr == "" || ovalue == "" {
		c.String(200, "fail")
		return
	}
	if !hmpay.VerifyNotifySign(orderID, restateStr, ovalue, mc.Key, sign) {
		c.String(200, "fail")
		return
	}
	restate, err := strconv.Atoi(restateStr)
	if err != nil {
		c.String(200, "fail")
		return
	}
	ctx := c.Request.Context()
	marked, err := u.MerchantHmOrderRepo.ApplyNotifyAndMarkOrderPaid(ctx, orderID, restate, int(time.Now().Unix()))
	if err != nil {
		c.String(200, "fail")
		return
	}
	if marked {
		u.notifyMerchantOrderPaidByOrderNo(ctx, orderID)
	}
	c.String(200, "Ok")
}

func notifyParam(c *gin.Context, key string) string {
	if v := strings.TrimSpace(c.Query(key)); v != "" {
		return v
	}
	return strings.TrimSpace(c.PostForm(key))
}

func (u *User) hdpayConfig() *config.Hdpay {
	if u.Config == nil || u.Config.Hdpay == nil {
		return nil
	}
	m := u.Config.Hdpay
	if strings.TrimSpace(m.OrderURL) == "" || strings.TrimSpace(m.AppID) == "" || strings.TrimSpace(m.AppSecret) == "" {
		return nil
	}
	return m
}

// ensureHdPayReady 读取配置并在客户端未初始化时补调 Init（与 main.initHdPay 一致）
func (u *User) ensureHdPayReady() *config.Hdpay {
	mc := u.hdpayConfig()
	if mc == nil {
		return nil
	}
	if hdpay.GetClient() == nil {
		hdpay.Init(mc.OrderURL, mc.AppID, mc.AppSecret)
	}
	if hdpay.GetClient() == nil {
		return nil
	}
	return mc
}

func (u *User) hmpayConfig() *config.Hmpay {
	if u.Config == nil || u.Config.Hmpay == nil {
		return nil
	}
	m := u.Config.Hmpay
	if !m.Valid() {
		return nil
	}
	return m
}

func (u *User) ensureHmPayReady() *config.Hmpay {
	mc := u.hmpayConfig()
	if mc == nil {
		return nil
	}
	if hmpay.GetClient() == nil {
		hmpay.Init(mc.OrderURL, mc.Parter, mc.ReqType, mc.Key)
	}
	if hmpay.GetClient() == nil {
		return nil
	}
	return mc
}

func (u *User) merchantPaymentByPayTypes(ctx context.Context, payTypesJSON, platformKey string) (*model.MerchantPayment, error) {
	paymentID, ok := model.MerchantPayTypePaymentID(payTypesJSON, platformKey)
	if !ok {
		return nil, nil
	}
	payment, err := u.MerchantPaymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if payment == nil || payment.Platform != platformKey {
		return nil, nil
	}
	return payment, nil
}

func (u *User) merchantHdChannelPayType(ctx context.Context, payTypesJSON string) (string, error) {
	payment, err := u.merchantPaymentByPayTypes(ctx, payTypesJSON, model.MerchantPayTypeKeyHd)
	if err != nil {
		return "", err
	}
	if payment == nil {
		return "", nil
	}
	return strings.TrimSpace(payment.Code), nil
}

func (u *User) merchantHmChannelPayType(ctx context.Context, payTypesJSON string) (string, error) {
	payment, err := u.merchantPaymentByPayTypes(ctx, payTypesJSON, model.MerchantPayTypeKeyHm)
	if err != nil {
		return "", err
	}
	if payment == nil {
		return "", nil
	}
	return strings.TrimSpace(payment.Code), nil
}

func (u *User) validateMerchantOrderPayAmount(ctx context.Context, task *model.MerchantTask, counts float64, platformKey, payTypesJSON string) error {
	payment, err := u.merchantPaymentByPayTypes(ctx, payTypesJSON, platformKey)
	if err != nil {
		return err
	}
	if payment == nil {
		return errorx.New(400, "卖家未开通该支付方式")
	}
	amount := math.Round(task.Price*counts*100) / 100
	if payment.Min != nil && amount < float64(*payment.Min) {
		return errorx.New(400, fmt.Sprintf("订单金额不能低于 %d 元", *payment.Min))
	}
	if payment.Max != nil && amount > float64(*payment.Max) {
		return errorx.New(400, fmt.Sprintf("订单金额不能超过 %d 元", *payment.Max))
	}
	return nil
}

func formatPayAmount(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}
