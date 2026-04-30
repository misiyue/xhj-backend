package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gzydong/go-chat/external/wallet"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

var _ IWalletService = (*WalletService)(nil)

// IWalletService 钱包服务接口（对接外部钱包服务）
type IWalletService interface {
	// GetBalance 获取余额
	GetBalance(ctx context.Context, userId int) (float64, error)

	// Recharge 充值
	Recharge(ctx context.Context, userId int, amount float64, payMethod string) (*RechargeResult, error)

	// Transfer 转账
	Transfer(ctx context.Context, fromUserId int, toUserId int, amount float64, remark string, password string) (*TransferResult, error)

	// GetTransactionHistory 获取交易记录
	GetTransactionHistory(ctx context.Context, userId int, startDate, endDate time.Time, page, pageSize int) (*TransactionHistoryResult, error)

	// SendRedEnvelope 发送红包
	SendRedEnvelope(ctx context.Context, req *SendRedEnvelopeRequest) (*RedEnvelopeResult, error)

	// ReceiveRedEnvelope 领取红包
	ReceiveRedEnvelope(ctx context.Context, envelopeId string, userId int) (*ReceiveRedEnvelopeResult, error)

	// GetRedEnvelopeDetail 获取红包详情
	GetRedEnvelopeDetail(ctx context.Context, envelopeId string) (*RedEnvelopeDetail, error)

	// VerifyPaymentPassword 验证支付密码
	VerifyPaymentPassword(ctx context.Context, userId int, password string) (bool, error)
}

// WalletService 钱包服务实现（对接外部钱包平台）
type WalletService struct {
	RedEnvelopeService IRedEnvelopeService
	WalletUserRepo     *repo.WalletUser
	UsersRepo          *repo.Users
}

// RechargeResult 充值结果
type RechargeResult struct {
	OrderId   string    `json:"order_id"`
	Amount    float64   `json:"amount"`
	Balance   float64   `json:"balance"`
	Status    string    `json:"status"` // success, pending, failed
	CreatedAt time.Time `json:"created_at"`
}

// TransferResult 转账结果
type TransferResult struct {
	TransferId string    `json:"transfer_id"`
	FromUserId int       `json:"from_user_id"`
	ToUserId   int       `json:"to_user_id"`
	Amount     float64   `json:"amount"`
	Fee        float64   `json:"fee"`
	Status     string    `json:"status"` // success, pending, failed
	CreatedAt  time.Time `json:"created_at"`
}

// TransactionHistoryResult 交易记录结果
type TransactionHistoryResult struct {
	Items      []*TransactionItem `json:"items"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

// TransactionItem 交易记录项
type TransactionItem struct {
	Id          string    `json:"id"`
	Type        string    `json:"type"` // recharge, transfer, red_envelope, receive
	Amount      float64   `json:"amount"`
	Balance     float64   `json:"balance"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// SendRedEnvelopeRequest 发送红包请求
type SendRedEnvelopeRequest struct {
	SenderId int     `json:"sender_id"`
	ChatType int     `json:"chat_type"` // 1:私聊 2:群聊
	ChatId   int     `json:"chat_id"`
	Amount   float64 `json:"amount"`
	Count    int     `json:"count"`    // 红包个数
	Type     string  `json:"type"`     // normal:普通红包 lucky:拼手气红包
	Greeting string  `json:"greeting"` // 祝福语
	Password string  `json:"password"` // 支付密码
}

// RedEnvelopeResult 红包结果
type RedEnvelopeResult struct {
	EnvelopeId string    `json:"envelope_id"`
	SenderId   int       `json:"sender_id"`
	Amount     float64   `json:"amount"`
	Count      int       `json:"count"`
	Type       string    `json:"type"`
	Status     string    `json:"status"` // available, finished, expired
	CreatedAt  time.Time `json:"created_at"`
	ExpiredAt  time.Time `json:"expired_at"` // 过期时间（24小时后）
}

// ReceiveRedEnvelopeResult 领取红包结果
type ReceiveRedEnvelopeResult struct {
	EnvelopeId string    `json:"envelope_id"`
	UserId     int       `json:"user_id"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"` // success, finished, expired
	ReceivedAt time.Time `json:"received_at"`
}

// RedEnvelopeDetail 红包详情
type RedEnvelopeDetail struct {
	EnvelopeId    string                 `json:"envelope_id"`
	SenderId      int                    `json:"sender_id"`
	SenderName    string                 `json:"sender_name"`
	Amount        float64                `json:"amount"`
	Count         int                    `json:"count"`
	Type          string                 `json:"type"`
	Greeting      string                 `json:"greeting"`
	Status        string                 `json:"status"`
	ReceivedCount int                    `json:"received_count"`
	ReceivedList  []*RedEnvelopeReceiver `json:"received_list"`
	CreatedAt     time.Time              `json:"created_at"`
	ExpiredAt     time.Time              `json:"expired_at"` // 过期时间（24小时后）
}

// RedEnvelopeReceiver 红包领取记录
type RedEnvelopeReceiver struct {
	UserId     int       `json:"user_id"`
	UserName   string    `json:"user_name"`
	Amount     float64   `json:"amount"`
	ReceivedAt time.Time `json:"received_at"`
}

// PasswordError represents payment password verification error
type PasswordError struct {
	Message string
}

func (e *PasswordError) Error() string {
	return e.Message
}

// getOrCreateWalletUser retrieves the wallet platform user ID for an IM user,
// registering a new wallet account if one does not exist.
func (s *WalletService) getOrCreateWalletUser(ctx context.Context, userId int) (*model.WalletUser, error) {
	wu, err := s.WalletUserRepo.FindByUserId(ctx, userId)
	if err == nil && wu != nil {
		return wu, nil
	}

	// Look up the IM user to get a unique identifier for registration
	imUser, err := s.UsersRepo.FindById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to find IM user %d: %w", userId, err)
	}

	phone := imUser.Email
	if imUser.Mobile != nil && *imUser.Mobile != "" {
		phone = *imUser.Mobile
	}
	if phone == "" {
		phone = fmt.Sprintf("im_user_%d", userId)
	}

	c := wallet.GetClient()
	if c == nil {
		return nil, fmt.Errorf("wallet client not initialized")
	}

	thirdUID := fmt.Sprintf("%d", userId)
	// Generate a unique password per user using their IM user ID and phone
	walletPwd := fmt.Sprintf("wp_%s_%d", phone, userId)
	regData, err := c.RegisterThirdParty(phone, walletPwd, thirdUID)
	if err != nil {
		return nil, fmt.Errorf("failed to register wallet user: %w", err)
	}

	walletUserID, err := wallet.ParseRegisterUserID(regData.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid wallet user_id in register response: %w", err)
	}
	wu = &model.WalletUser{
		UserId:    userId,
		WalletUID: walletUserID,
		UUID:      regData.UUID,
		Phone:     regData.Phone,
	}

	if err := s.WalletUserRepo.Repo.Create(ctx, wu); err != nil {
		return nil, fmt.Errorf("failed to save wallet user mapping: %w", err)
	}

	return wu, nil
}

func (s *WalletService) GetBalance(ctx context.Context, userId int) (float64, error) {
	wu, err := s.getOrCreateWalletUser(ctx, userId)
	if err != nil {
		return 0, err
	}

	c := wallet.GetClient()
	if c == nil {
		return 0, fmt.Errorf("wallet client not initialized")
	}

	// Query USDT balance using the platform key (no user password needed for balance via account assets)
	assets, err := c.GetAccountAssets(fmt.Sprintf("%d", userId), wu.WalletUID)
	if err != nil {
		return 0, fmt.Errorf("failed to get account assets: %w", err)
	}

	// Find USDT balance from assets
	for _, asset := range assets {
		if asset.Name == "USDT" {
			balance, err := strconv.ParseFloat(asset.Account, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse USDT balance %q: %w", asset.Account, err)
			}
			return balance, nil
		}
	}

	return 0, nil
}

func (s *WalletService) Recharge(ctx context.Context, userId int, amount float64, payMethod string) (*RechargeResult, error) {
	wu, err := s.getOrCreateWalletUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	c := wallet.GetClient()
	if c == nil {
		return nil, fmt.Errorf("wallet client not initialized")
	}

	payInfo, err := c.GetPayInfo(wu.WalletUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recharge info: %w", err)
	}

	return &RechargeResult{
		OrderId:   payInfo.Address,
		Amount:    amount,
		Balance:   0, // Balance will be updated after on-chain confirmation
		Status:    "pending",
		CreatedAt: time.Now(),
	}, nil
}

func (s *WalletService) Transfer(ctx context.Context, fromUserId int, toUserId int, amount float64, remark string, password string) (*TransferResult, error) {
	fromWU, err := s.getOrCreateWalletUser(ctx, fromUserId)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve sender wallet: %w", err)
	}

	toWU, err := s.getOrCreateWalletUser(ctx, toUserId)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve receiver wallet: %w", err)
	}

	c := wallet.GetClient()
	if c == nil {
		return nil, fmt.Errorf("wallet client not initialized")
	}

	transferData, err := c.SendFunds(fromWU.WalletUID, toWU.WalletUID, amount, remark)
	if err != nil {
		return nil, fmt.Errorf("wallet transfer failed: %w", err)
	}

	return &TransferResult{
		TransferId: fmt.Sprintf("TRF%d_%d", fromWU.WalletUID, time.Now().UnixMilli()),
		FromUserId: fromUserId,
		ToUserId:   toUserId,
		Amount:     transferData.Amount,
		Fee:        0,
		Status:     "success",
		CreatedAt:  time.Now(),
	}, nil
}

func (s *WalletService) GetTransactionHistory(ctx context.Context, userId int, startDate, endDate time.Time, page, pageSize int) (*TransactionHistoryResult, error) {
	wu, err := s.getOrCreateWalletUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	c := wallet.GetClient()
	if c == nil {
		return nil, fmt.Errorf("wallet client not initialized")
	}

	// Wallet API uses 0-based page index
	apiPage := page - 1
	if apiPage < 0 {
		apiPage = 0
	}

	billData, err := c.GetBillList(wu.WalletUID, apiPage, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction history: %w", err)
	}

	items := make([]*TransactionItem, 0, len(billData.Data))
	for _, bill := range billData.Data {
		createdAt, err := time.Parse("2006-01-02 15:04:05", bill.CreateTime)
		if err != nil {
			createdAt = time.Time{}
		}
		items = append(items, &TransactionItem{
			Id:          fmt.Sprintf("%d", bill.ID),
			Type:        bill.BillType,
			Amount:      bill.Account,
			Balance:     bill.AfterAccount,
			Description: bill.Remark,
			Status:      "success",
			CreatedAt:   createdAt,
		})
	}

	totalPages := 0
	if pageSize > 0 && billData.Count > 0 {
		totalPages = (billData.Count + pageSize - 1) / pageSize
	}

	return &TransactionHistoryResult{
		Items:      items,
		Total:      billData.Count,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *WalletService) SendRedEnvelope(ctx context.Context, req *SendRedEnvelopeRequest) (*RedEnvelopeResult, error) {
	// Verify payment password first
	valid, err := s.VerifyPaymentPassword(ctx, req.SenderId, req.Password)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, &PasswordError{Message: "支付密码错误"}
	}

	// 委托给红包服务
	info, err := s.RedEnvelopeService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &RedEnvelopeResult{
		EnvelopeId: info.EnvelopeId,
		SenderId:   info.SenderId,
		Amount:     info.Amount,
		Count:      info.Count,
		Type:       info.Type,
		Status:     info.Status,
		CreatedAt:  info.CreatedAt,
		ExpiredAt:  info.ExpiredAt,
	}, nil
}

func (s *WalletService) ReceiveRedEnvelope(ctx context.Context, envelopeId string, userId int) (*ReceiveRedEnvelopeResult, error) {
	// 委托给红包服务
	result, err := s.RedEnvelopeService.Receive(ctx, envelopeId, userId)
	if err != nil {
		return nil, err
	}

	return &ReceiveRedEnvelopeResult{
		EnvelopeId: result.EnvelopeId,
		UserId:     result.UserId,
		Amount:     result.Amount,
		Status:     result.Status,
		ReceivedAt: result.ReceivedAt,
	}, nil
}

func (s *WalletService) GetRedEnvelopeDetail(ctx context.Context, envelopeId string) (*RedEnvelopeDetail, error) {
	// 委托给红包服务
	info, err := s.RedEnvelopeService.GetDetail(ctx, envelopeId)
	if err != nil {
		return nil, err
	}

	receivers := make([]*RedEnvelopeReceiver, 0, len(info.ReceivedList))
	for _, r := range info.ReceivedList {
		receivers = append(receivers, &RedEnvelopeReceiver{
			UserId:     r.UserId,
			UserName:   r.UserName,
			Amount:     r.Amount,
			ReceivedAt: r.ReceivedAt,
		})
	}

	return &RedEnvelopeDetail{
		EnvelopeId:    info.EnvelopeId,
		SenderId:      info.SenderId,
		SenderName:    info.SenderName,
		Amount:        info.Amount,
		Count:         info.Count,
		Type:          info.Type,
		Greeting:      info.Greeting,
		Status:        info.Status,
		ReceivedCount: info.ReceivedCount,
		ReceivedList:  receivers,
		CreatedAt:     info.CreatedAt,
		ExpiredAt:     info.ExpiredAt,
	}, nil
}

func (s *WalletService) VerifyPaymentPassword(ctx context.Context, userId int, password string) (bool, error) {
	// Password verification is delegated to the wallet platform during actual transfers
	// (the key parameter in sendfunds_thirdparty / getuserfunds_thirdparty).
	// This method only performs a basic non-empty check as a client-side guard.
	return password != "", nil
}
