package v1

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
)

type Wallet struct {
	WalletService service.IWalletService
}

// GetBalance 获取余额
//
//	@Summary		获取余额
//	@Description	获取用户钱包余额
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	WalletBalanceResponse
//	@Router			/api/v1/wallet/balance [post]
func (w *Wallet) GetBalance(ctx context.Context, req *WalletBalanceRequest) (*WalletBalanceResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	balance, err := w.WalletService.GetBalance(ctx, int(userId))
	if err != nil {
		return nil, err
	}

	return &WalletBalanceResponse{
		Balance: balance,
	}, nil
}

// Recharge 充值
//
//	@Summary		充值
//	@Description	钱包充值
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletRechargeRequest	true	"充值请求"
//	@Success		200		{object}	WalletRechargeResponse
//	@Router			/api/v1/wallet/recharge [post]
func (w *Wallet) Recharge(ctx context.Context, req *WalletRechargeRequest) (*WalletRechargeResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Amount <= 0 {
		return nil, errorx.New(400, "充值金额必须大于0")
	}

	result, err := w.WalletService.Recharge(ctx, int(userId), req.Amount, req.PayMethod)
	if err != nil {
		return nil, err
	}

	return &WalletRechargeResponse{
		OrderId:   result.OrderId,
		Amount:    result.Amount,
		Balance:   result.Balance,
		Status:    result.Status,
		CreatedAt: result.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// VerifyPaymentPassword 验证支付密码
//
//	@Summary		验证支付密码
//	@Description	验证用户支付密码
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletVerifyPasswordRequest	true	"验证密码请求"
//	@Success		200		{object}	WalletVerifyPasswordResponse
//	@Router			/api/v1/wallet/verify-password [post]
func (w *Wallet) VerifyPaymentPassword(ctx context.Context, req *WalletVerifyPasswordRequest) (*WalletVerifyPasswordResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Password == "" {
		return nil, errorx.New(400, "支付密码不能为空")
	}

	valid, err := w.WalletService.VerifyPaymentPassword(ctx, int(userId), req.Password)
	if err != nil {
		return nil, err
	}

	return &WalletVerifyPasswordResponse{
		Valid: valid,
	}, nil
}

// Transfer 转账
//
//	@Summary		转账
//	@Description	转账给其他用户
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletTransferRequest	true	"转账请求"
//	@Success		200		{object}	WalletTransferResponse
//	@Router			/api/v1/wallet/transfer [post]
func (w *Wallet) Transfer(ctx context.Context, req *WalletTransferRequest) (*WalletTransferResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Amount <= 0 {
		return nil, errorx.New(400, "转账金额必须大于0")
	}

	if req.ToUserId <= 0 {
		return nil, errorx.New(400, "收款用户ID无效")
	}

	if req.Password == "" {
		return nil, errorx.New(400, "支付密码不能为空")
	}

	result, err := w.WalletService.Transfer(ctx, int(userId), int(req.ToUserId), req.Amount, req.Remark, req.Password)
	if err != nil {
		return nil, err
	}

	return &WalletTransferResponse{
		TransferId: result.TransferId,
		Amount:     result.Amount,
		Fee:        result.Fee,
		Status:     result.Status,
		CreatedAt:  result.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetTransactionHistory 获取交易记录
//
//	@Summary		交易记录
//	@Description	获取钱包交易记录
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletHistoryRequest	true	"交易记录请求"
//	@Success		200		{object}	WalletHistoryResponse
//	@Router			/api/v1/wallet/history [post]
func (w *Wallet) GetTransactionHistory(ctx context.Context, req *WalletHistoryRequest) (*WalletHistoryResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var startDate, endDate time.Time
	if req.StartDate != "" {
		startDate, _ = time.Parse("2006-01-02", req.StartDate)
	}
	if req.EndDate != "" {
		endDate, _ = time.Parse("2006-01-02", req.EndDate)
	}

	result, err := w.WalletService.GetTransactionHistory(ctx, int(userId), startDate, endDate, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}

	items := make([]*WalletTransactionItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &WalletTransactionItem{
			Id:          item.Id,
			Type:        item.Type,
			Amount:      item.Amount,
			Balance:     item.Balance,
			Description: item.Description,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &WalletHistoryResponse{
		Items:      items,
		Total:      int32(result.Total),
		Page:       int32(result.Page),
		PageSize:   int32(result.PageSize),
		TotalPages: int32(result.TotalPages),
	}, nil
}

// SendRedEnvelope 发送红包
//
//	@Summary		发送红包
//	@Description	发送红包
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletSendRedEnvelopeRequest	true	"发送红包请求"
//	@Success		200		{object}	WalletSendRedEnvelopeResponse
//	@Router			/api/v1/wallet/red-envelope/send [post]
func (w *Wallet) SendRedEnvelope(ctx context.Context, req *WalletSendRedEnvelopeRequest) (*WalletSendRedEnvelopeResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Amount <= 0 {
		return nil, errorx.New(400, "红包金额必须大于0")
	}

	if req.Count <= 0 {
		return nil, errorx.New(400, "红包个数必须大于0")
	}

	result, err := w.WalletService.SendRedEnvelope(ctx, &service.SendRedEnvelopeRequest{
		SenderId: int(userId),
		ChatType: int(req.ChatType),
		ChatId:   int(req.ChatId),
		Amount:   req.Amount,
		Count:    int(req.Count),
		Type:     req.Type,
		Greeting: req.Greeting,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}

	return &WalletSendRedEnvelopeResponse{
		EnvelopeId: result.EnvelopeId,
		Amount:     result.Amount,
		Count:      int32(result.Count),
		Type:       result.Type,
		Status:     result.Status,
		CreatedAt:  result.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// ReceiveRedEnvelope 领取红包
//
//	@Summary		领取红包
//	@Description	领取红包
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletReceiveRedEnvelopeRequest	true	"领取红包请求"
//	@Success		200		{object}	WalletReceiveRedEnvelopeResponse
//	@Router			/api/v1/wallet/red-envelope/receive [post]
func (w *Wallet) ReceiveRedEnvelope(ctx context.Context, req *WalletReceiveRedEnvelopeRequest) (*WalletReceiveRedEnvelopeResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.EnvelopeId == "" {
		return nil, errorx.New(400, "红包ID不能为空")
	}

	result, err := w.WalletService.ReceiveRedEnvelope(ctx, req.EnvelopeId, int(userId))
	if err != nil {
		return nil, err
	}

	return &WalletReceiveRedEnvelopeResponse{
		EnvelopeId: result.EnvelopeId,
		Amount:     result.Amount,
		Status:     result.Status,
		ReceivedAt: result.ReceivedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetRedEnvelopeDetail 获取红包详情
//
//	@Summary		红包详情
//	@Description	获取红包详情
//	@Tags			钱包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		WalletRedEnvelopeDetailRequest	true	"红包详情请求"
//	@Success		200		{object}	WalletRedEnvelopeDetailResponse
//	@Router			/api/v1/wallet/red-envelope/detail [post]
func (w *Wallet) GetRedEnvelopeDetail(ctx context.Context, req *WalletRedEnvelopeDetailRequest) (*WalletRedEnvelopeDetailResponse, error) {
	if req.EnvelopeId == "" {
		return nil, errorx.New(400, "红包ID不能为空")
	}

	detail, err := w.WalletService.GetRedEnvelopeDetail(ctx, req.EnvelopeId)
	if err != nil {
		return nil, err
	}

	receivers := make([]*WalletRedEnvelopeReceiver, 0, len(detail.ReceivedList))
	for _, r := range detail.ReceivedList {
		receivers = append(receivers, &WalletRedEnvelopeReceiver{
			UserId:     int32(r.UserId),
			UserName:   r.UserName,
			Amount:     r.Amount,
			ReceivedAt: r.ReceivedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &WalletRedEnvelopeDetailResponse{
		EnvelopeId:    detail.EnvelopeId,
		SenderId:      int32(detail.SenderId),
		SenderName:    detail.SenderName,
		Amount:        detail.Amount,
		Count:         int32(detail.Count),
		Type:          detail.Type,
		Greeting:      detail.Greeting,
		Status:        detail.Status,
		ReceivedCount: int32(detail.ReceivedCount),
		ReceivedList:  receivers,
		CreatedAt:     detail.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// Request and Response types

type WalletBalanceRequest struct{}

type WalletBalanceResponse struct {
	Balance float64 `json:"balance"`
}

type WalletRechargeRequest struct {
	Amount    float64 `json:"amount"`
	PayMethod string  `json:"pay_method"` // alipay, wechat, bank
}

type WalletRechargeResponse struct {
	OrderId   string  `json:"order_id"`
	Amount    float64 `json:"amount"`
	Balance   float64 `json:"balance"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type WalletVerifyPasswordRequest struct {
	Password string `json:"password"`
}

type WalletVerifyPasswordResponse struct {
	Valid bool `json:"valid"`
}

type WalletTransferRequest struct {
	ToUserId int32   `json:"to_user_id"`
	Amount   float64 `json:"amount"`
	Remark   string  `json:"remark"`
	Password string  `json:"password"`
}

type WalletTransferResponse struct {
	TransferId string  `json:"transfer_id"`
	Amount     float64 `json:"amount"`
	Fee        float64 `json:"fee"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
}

type WalletHistoryRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Page      int32  `json:"page"`
	PageSize  int32  `json:"page_size"`
}

type WalletHistoryResponse struct {
	Items      []*WalletTransactionItem `json:"items"`
	Total      int32                    `json:"total"`
	Page       int32                    `json:"page"`
	PageSize   int32                    `json:"page_size"`
	TotalPages int32                    `json:"total_pages"`
}

type WalletTransactionItem struct {
	Id          string  `json:"id"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Balance     float64 `json:"balance"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}

type WalletSendRedEnvelopeRequest struct {
	ChatType int32   `json:"chat_type"` // 1:私聊 2:群聊
	ChatId   int32   `json:"chat_id"`
	Amount   float64 `json:"amount"`
	Count    int32   `json:"count"`
	Type     string  `json:"type"` // normal, lucky
	Greeting string  `json:"greeting"`
	Password string  `json:"password"`
}

type WalletSendRedEnvelopeResponse struct {
	EnvelopeId string  `json:"envelope_id"`
	Amount     float64 `json:"amount"`
	Count      int32   `json:"count"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
}

type WalletReceiveRedEnvelopeRequest struct {
	EnvelopeId string `json:"envelope_id"`
}

type WalletReceiveRedEnvelopeResponse struct {
	EnvelopeId string  `json:"envelope_id"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status"`
	ReceivedAt string  `json:"received_at"`
}

type WalletRedEnvelopeDetailRequest struct {
	EnvelopeId string `json:"envelope_id"`
}

type WalletRedEnvelopeDetailResponse struct {
	EnvelopeId    string                       `json:"envelope_id"`
	SenderId      int32                        `json:"sender_id"`
	SenderName    string                       `json:"sender_name"`
	Amount        float64                      `json:"amount"`
	Count         int32                        `json:"count"`
	Type          string                       `json:"type"`
	Greeting      string                       `json:"greeting"`
	Status        string                       `json:"status"`
	ReceivedCount int32                        `json:"received_count"`
	ReceivedList  []*WalletRedEnvelopeReceiver `json:"received_list"`
	CreatedAt     string                       `json:"created_at"`
}

type WalletRedEnvelopeReceiver struct {
	UserId     int32   `json:"user_id"`
	UserName   string  `json:"user_name"`
	Amount     float64 `json:"amount"`
	ReceivedAt string  `json:"received_at"`
}
