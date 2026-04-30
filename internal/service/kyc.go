package service

import (
	"context"
	"errors"
	"time"
)

var _ IKYCService = (*MockKYCService)(nil)

// IKYCService KYC服务接口（对接外部实名认证服务）
type IKYCService interface {
	// GetKYCStatus 获取KYC状态
	GetKYCStatus(ctx context.Context, userId int) (*KYCStatus, error)

	// SubmitKYC 提交KYC认证
	SubmitKYC(ctx context.Context, req *KYCSubmitRequest) (*KYCSubmitResult, error)

	// GetKYCDetail 获取KYC详情
	GetKYCDetail(ctx context.Context, userId int) (*KYCDetail, error)

	// UploadIDCard 上传身份证照片
	UploadIDCard(ctx context.Context, userId int, side string, imageUrl string) error

	// UploadFaceImage 上传人脸照片
	UploadFaceImage(ctx context.Context, userId int, imageUrl string) error
}

// MockKYCService KYC服务的Mock实现
type MockKYCService struct{}

// KYC状态常量
const (
	KYCStatusNotSubmitted = 0 // 未提交
	KYCStatusPending      = 1 // 审核中
	KYCStatusApproved     = 2 // 已通过
	KYCStatusRejected     = 3 // 已拒绝
)

// KYCStatus KYC状态
type KYCStatus struct {
	UserId      int       `json:"user_id"`
	Status      int       `json:"status"` // 0:未提交 1:审核中 2:已通过 3:已拒绝
	RealName    string    `json:"real_name"`
	IDCard      string    `json:"id_card"`
	RejectReason string   `json:"reject_reason"`
	SubmittedAt time.Time `json:"submitted_at"`
	ApprovedAt  time.Time `json:"approved_at"`
}

// KYCSubmitRequest KYC提交请求
type KYCSubmitRequest struct {
	UserId         int    `json:"user_id"`
	RealName       string `json:"real_name"`
	IDCardNumber   string `json:"id_card_number"`
	IDCardFrontUrl string `json:"id_card_front_url"` // 身份证正面
	IDCardBackUrl  string `json:"id_card_back_url"`  // 身份证反面
	FaceImageUrl   string `json:"face_image_url"`    // 人脸照片
}

// KYCSubmitResult KYC提交结果
type KYCSubmitResult struct {
	ApplicationId string    `json:"application_id"`
	Status        int       `json:"status"`
	SubmittedAt   time.Time `json:"submitted_at"`
}

// KYCDetail KYC详情
type KYCDetail struct {
	UserId           int       `json:"user_id"`
	Status           int       `json:"status"`
	RealName         string    `json:"real_name"`
	IDCardNumber     string    `json:"id_card_number"`
	IDCardFrontUrl   string    `json:"id_card_front_url"`
	IDCardBackUrl    string    `json:"id_card_back_url"`
	FaceImageUrl     string    `json:"face_image_url"`
	RejectReason     string    `json:"reject_reason"`
	SubmittedAt      time.Time `json:"submitted_at"`
	ReviewedAt       time.Time `json:"reviewed_at"`
	ApplicationId    string    `json:"application_id"`
}

// Mock implementation

func (m *MockKYCService) GetKYCStatus(ctx context.Context, userId int) (*KYCStatus, error) {
	// Mock data - 实际应调用外部KYC服务
	return &KYCStatus{
		UserId:      userId,
		Status:      KYCStatusNotSubmitted,
		RealName:    "",
		IDCard:      "",
		RejectReason: "",
		SubmittedAt: time.Time{},
		ApprovedAt:  time.Time{},
	}, nil
}

func (m *MockKYCService) SubmitKYC(ctx context.Context, req *KYCSubmitRequest) (*KYCSubmitResult, error) {
	// 验证必填字段
	if req.RealName == "" {
		return nil, errors.New("真实姓名不能为空")
	}
	if req.IDCardNumber == "" {
		return nil, errors.New("身份证号不能为空")
	}
	if req.IDCardFrontUrl == "" {
		return nil, errors.New("身份证正面照片不能为空")
	}
	if req.IDCardBackUrl == "" {
		return nil, errors.New("身份证反面照片不能为空")
	}
	if req.FaceImageUrl == "" {
		return nil, errors.New("人脸照片不能为空")
	}

	// Mock data - 实际应调用外部KYC服务API
	return &KYCSubmitResult{
		ApplicationId: "KYC" + time.Now().Format("20060102150405"),
		Status:        KYCStatusPending,
		SubmittedAt:   time.Now(),
	}, nil
}

func (m *MockKYCService) GetKYCDetail(ctx context.Context, userId int) (*KYCDetail, error) {
	// Mock data
	return &KYCDetail{
		UserId:           userId,
		Status:           KYCStatusNotSubmitted,
		RealName:         "",
		IDCardNumber:     "",
		IDCardFrontUrl:   "",
		IDCardBackUrl:    "",
		FaceImageUrl:     "",
		RejectReason:     "",
		SubmittedAt:      time.Time{},
		ReviewedAt:       time.Time{},
		ApplicationId:    "",
	}, nil
}

func (m *MockKYCService) UploadIDCard(ctx context.Context, userId int, side string, imageUrl string) error {
	// Mock implementation
	if side != "front" && side != "back" {
		return errors.New("side must be 'front' or 'back'")
	}
	if imageUrl == "" {
		return errors.New("imageUrl cannot be empty")
	}
	return nil
}

func (m *MockKYCService) UploadFaceImage(ctx context.Context, userId int, imageUrl string) error {
	// Mock implementation
	if imageUrl == "" {
		return errors.New("imageUrl cannot be empty")
	}
	return nil
}
