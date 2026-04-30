package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
)

// IRedEnvelopeService 红包服务接口
// 抽象红包核心逻辑，以便后续对接第三方钱包服务
type IRedEnvelopeService interface {
	// Create 创建红包
	Create(ctx context.Context, req *SendRedEnvelopeRequest) (*RedEnvelopeInfo, error)

	// Receive 领取红包
	Receive(ctx context.Context, envelopeId string, userId int) (*RedEnvelopeReceiveInfo, error)

	// GetDetail 获取红包详情
	GetDetail(ctx context.Context, envelopeId string) (*RedEnvelopeInfo, error)

	// GetStatus 获取红包状态（快速查询，用于消息列表展示）
	GetStatus(ctx context.Context, envelopeId string, userId int) (*RedEnvelopeStatusInfo, error)

	// ExpireOverdue 过期处理：将超时红包标记为已过期并退款
	ExpireOverdue(ctx context.Context) (int, error)
}

// RedEnvelopeInfo 红包详情DTO
type RedEnvelopeInfo struct {
	EnvelopeId    string                      `json:"envelope_id"`
	SenderId      int                         `json:"sender_id"`
	SenderName    string                      `json:"sender_name"`
	ChatType      int                         `json:"chat_type"`
	ChatId        int                         `json:"chat_id"`
	Type          string                      `json:"type"`           // normal / lucky
	Amount        float64                     `json:"amount"`
	RemainAmount  float64                     `json:"remain_amount"`
	Count         int                         `json:"count"`
	RemainCount   int                         `json:"remain_count"`
	Greeting      string                      `json:"greeting"`
	Status        string                      `json:"status"`         // available / finished / expired
	StatusText    string                      `json:"status_text"`    // 待领取 / 已领完 / 已过期
	BestUserId    int                         `json:"best_user_id"`   // 手气最佳用户ID（拼手气红包）
	BestUserName  string                      `json:"best_user_name"` // 手气最佳用户名
	BestAmount    float64                     `json:"best_amount"`    // 手气最佳金额
	ReceivedCount int                         `json:"received_count"`
	ReceivedList  []*RedEnvelopeReceiverInfo  `json:"received_list"`
	RefundAmount  float64                     `json:"refund_amount"`
	CreatedAt     time.Time                   `json:"created_at"`
	ExpiredAt     time.Time                   `json:"expired_at"`
}

// RedEnvelopeReceiverInfo 红包领取记录DTO
type RedEnvelopeReceiverInfo struct {
	UserId     int       `json:"user_id"`
	UserName   string    `json:"user_name"`
	Amount     float64   `json:"amount"`
	IsBest     bool      `json:"is_best"`     // 是否手气最佳
	ReceivedAt time.Time `json:"received_at"`
}

// RedEnvelopeReceiveInfo 领取红包结果DTO
type RedEnvelopeReceiveInfo struct {
	EnvelopeId string    `json:"envelope_id"`
	UserId     int       `json:"user_id"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"`      // success / finished / expired / repeated
	IsBest     bool      `json:"is_best"`     // 是否手气最佳（拼手气红包，领完后才确定）
	ReceivedAt time.Time `json:"received_at"`
}

// RedEnvelopeStatusInfo 红包状态简要信息（用于消息展示）
type RedEnvelopeStatusInfo struct {
	EnvelopeId   string  `json:"envelope_id"`
	Status       string  `json:"status"`       // available / finished / expired
	StatusText   string  `json:"status_text"`  // 待领取 / 已领完 / 已过期
	Type         string  `json:"type"`         // normal / lucky
	HasReceived  bool    `json:"has_received"` // 当前用户是否已领取
	ReceivedAmt  float64 `json:"received_amt"` // 当前用户领取金额
	IsBest       bool    `json:"is_best"`      // 当前用户是否手气最佳
	BestUserId   int     `json:"best_user_id"`
	BestUserName string  `json:"best_user_name"`
	BestAmount   float64 `json:"best_amount"`
}

// CreateRedEnvelopeRequest is defined in wallet.go and reused here

// ------------------------------------------------
// InMemoryRedEnvelopeService 红包服务的内存实现（用于开发测试）
// 可直接替换为对接第三方钱包的实现
// ------------------------------------------------

var _ IRedEnvelopeService = (*InMemoryRedEnvelopeService)(nil)

type inMemoryEnvelope struct {
	Info      RedEnvelopeInfo
	Receivers []*RedEnvelopeReceiverInfo
}

type InMemoryRedEnvelopeService struct {
	mu        sync.RWMutex
	envelopes map[string]*inMemoryEnvelope
	counter   int
}

func NewInMemoryRedEnvelopeService() *InMemoryRedEnvelopeService {
	return &InMemoryRedEnvelopeService{
		envelopes: make(map[string]*inMemoryEnvelope),
	}
}

func (s *InMemoryRedEnvelopeService) Create(ctx context.Context, req *SendRedEnvelopeRequest) (*RedEnvelopeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// counter is safe here - protected by mu.Lock above
	s.counter++
	now := time.Now()
	envelopeId := fmt.Sprintf("RED%s%04d", now.Format("20060102150405"), s.counter)
	expiredAt := now.Add(time.Duration(entity.RedEnvelopeExpireHours) * time.Hour)

	info := RedEnvelopeInfo{
		EnvelopeId:    envelopeId,
		SenderId:      req.SenderId,
		SenderName:    fmt.Sprintf("用户%d", req.SenderId),
		ChatType:      req.ChatType,
		ChatId:        req.ChatId,
		Type:          req.Type,
		Amount:        req.Amount,
		RemainAmount:  req.Amount,
		Count:         req.Count,
		RemainCount:   req.Count,
		Greeting:      req.Greeting,
		Status:        entity.RedEnvelopeStatusAvailable,
		StatusText:    entity.RedEnvelopeStatusText[entity.RedEnvelopeStatusAvailable],
		ReceivedCount: 0,
		ReceivedList:  make([]*RedEnvelopeReceiverInfo, 0),
		CreatedAt:     now,
		ExpiredAt:     expiredAt,
	}

	s.envelopes[envelopeId] = &inMemoryEnvelope{
		Info:      info,
		Receivers: make([]*RedEnvelopeReceiverInfo, 0),
	}

	return &info, nil
}

func (s *InMemoryRedEnvelopeService) Receive(ctx context.Context, envelopeId string, userId int) (*RedEnvelopeReceiveInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	env, ok := s.envelopes[envelopeId]
	if !ok {
		return nil, fmt.Errorf("红包不存在")
	}

	// 检查是否过期
	if time.Now().After(env.Info.ExpiredAt) {
		if env.Info.Status == entity.RedEnvelopeStatusAvailable {
			s.markExpired(env)
		}
		return &RedEnvelopeReceiveInfo{
			EnvelopeId: envelopeId,
			UserId:     userId,
			Amount:     0,
			Status:     entity.RedEnvelopeReceiveStatusExpired,
			ReceivedAt: time.Now(),
		}, nil
	}

	// 检查红包是否已领完
	if env.Info.Status == entity.RedEnvelopeStatusFinished {
		return &RedEnvelopeReceiveInfo{
			EnvelopeId: envelopeId,
			UserId:     userId,
			Amount:     0,
			Status:     entity.RedEnvelopeReceiveStatusFinished,
			ReceivedAt: time.Now(),
		}, nil
	}

	// 检查是否已领取过
	for _, r := range env.Receivers {
		if r.UserId == userId {
			return &RedEnvelopeReceiveInfo{
				EnvelopeId: envelopeId,
				UserId:     userId,
				Amount:     r.Amount,
				Status:     entity.RedEnvelopeReceiveStatusRepeated,
				IsBest:     r.IsBest,
				ReceivedAt: r.ReceivedAt,
			}, nil
		}
	}

	// 计算领取金额
	var amount float64
	if env.Info.Type == entity.RedEnvelopeTypeLucky {
		amount = s.calcLuckyAmount(env.Info.RemainAmount, env.Info.RemainCount)
	} else {
		// 普通红包：平均分配
		amount = math.Round(env.Info.Amount/float64(env.Info.Count)*100) / 100
		if env.Info.RemainCount == 1 {
			amount = math.Round(env.Info.RemainAmount*100) / 100
		}
	}

	now := time.Now()
	receiver := &RedEnvelopeReceiverInfo{
		UserId:     userId,
		UserName:   fmt.Sprintf("用户%d", userId),
		Amount:     amount,
		ReceivedAt: now,
	}

	env.Receivers = append(env.Receivers, receiver)
	env.Info.RemainAmount = math.Round((env.Info.RemainAmount-amount)*100) / 100
	env.Info.RemainCount--
	env.Info.ReceivedCount++
	env.Info.ReceivedList = env.Receivers

	// 拼手气红包：追踪手气最佳
	if env.Info.Type == entity.RedEnvelopeTypeLucky {
		if amount > env.Info.BestAmount {
			env.Info.BestUserId = userId
			env.Info.BestUserName = receiver.UserName
			env.Info.BestAmount = amount
		}
		// 所有人领完后标记手气最佳
		if env.Info.RemainCount == 0 {
			for _, r := range env.Receivers {
				r.IsBest = r.UserId == env.Info.BestUserId
			}
		}
	}

	// 检查是否已领完
	if env.Info.RemainCount == 0 {
		env.Info.Status = entity.RedEnvelopeStatusFinished
		env.Info.StatusText = entity.RedEnvelopeStatusText[entity.RedEnvelopeStatusFinished]
	}

	isBest := false
	if env.Info.Type == entity.RedEnvelopeTypeLucky && env.Info.RemainCount == 0 {
		isBest = receiver.IsBest
	}

	return &RedEnvelopeReceiveInfo{
		EnvelopeId: envelopeId,
		UserId:     userId,
		Amount:     amount,
		Status:     entity.RedEnvelopeReceiveStatusSuccess,
		IsBest:     isBest,
		ReceivedAt: now,
	}, nil
}

func (s *InMemoryRedEnvelopeService) GetDetail(ctx context.Context, envelopeId string) (*RedEnvelopeInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envelopes[envelopeId]
	if !ok {
		return nil, fmt.Errorf("红包不存在")
	}

	// 动态检查过期
	info := env.Info
	if info.Status == entity.RedEnvelopeStatusAvailable && time.Now().After(info.ExpiredAt) {
		info.Status = entity.RedEnvelopeStatusExpired
		info.StatusText = entity.RedEnvelopeStatusText[entity.RedEnvelopeStatusExpired]
	}

	return &info, nil
}

func (s *InMemoryRedEnvelopeService) GetStatus(ctx context.Context, envelopeId string, userId int) (*RedEnvelopeStatusInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envelopes[envelopeId]
	if !ok {
		return nil, fmt.Errorf("红包不存在")
	}

	status := env.Info.Status
	if status == entity.RedEnvelopeStatusAvailable && time.Now().After(env.Info.ExpiredAt) {
		status = entity.RedEnvelopeStatusExpired
	}

	result := &RedEnvelopeStatusInfo{
		EnvelopeId:   envelopeId,
		Status:       status,
		StatusText:   entity.RedEnvelopeStatusText[status],
		Type:         env.Info.Type,
		BestUserId:   env.Info.BestUserId,
		BestUserName: env.Info.BestUserName,
		BestAmount:   env.Info.BestAmount,
	}

	// 查找当前用户领取情况
	for _, r := range env.Receivers {
		if r.UserId == userId {
			result.HasReceived = true
			result.ReceivedAmt = r.Amount
			result.IsBest = r.IsBest
			break
		}
	}

	return result, nil
}

func (s *InMemoryRedEnvelopeService) ExpireOverdue(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	now := time.Now()
	for _, env := range s.envelopes {
		if env.Info.Status == entity.RedEnvelopeStatusAvailable && now.After(env.Info.ExpiredAt) {
			s.markExpired(env)
			count++
		}
	}

	return count, nil
}

func (s *InMemoryRedEnvelopeService) markExpired(env *inMemoryEnvelope) {
	env.Info.Status = entity.RedEnvelopeStatusExpired
	env.Info.StatusText = entity.RedEnvelopeStatusText[entity.RedEnvelopeStatusExpired]
	env.Info.RefundAmount = env.Info.RemainAmount
}

// calcLuckyAmount 拼手气红包金额计算（二倍均值法）
func (s *InMemoryRedEnvelopeService) calcLuckyAmount(remainAmount float64, remainCount int) float64 {
	if remainCount == 1 {
		return math.Round(remainAmount*100) / 100
	}
	// 最小金额 0.01
	min := 0.01
	max := (remainAmount / float64(remainCount)) * 2
	// 使用 crypto/rand 生成安全随机数
	var b [8]byte
	_, _ = rand.Read(b[:])
	randFloat := float64(binary.LittleEndian.Uint64(b[:])) / float64(^uint64(0))
	amount := min + randFloat*(max-min)
	amount = math.Round(amount*100) / 100
	if amount < min {
		amount = min
	}
	// 确保剩余金额够分
	if remainAmount-amount < float64(remainCount-1)*min {
		amount = remainAmount - float64(remainCount-1)*min
		amount = math.Round(amount*100) / 100
	}
	return amount
}
