package v1

import (
	"context"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
)

type KYC struct {
	KYCService service.IKYCService
}

// GetKYCStatus 获取KYC状态
//
//	@Summary		获取KYC状态
//	@Description	获取用户实名认证状态
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	KYCStatusResponse
//	@Router			/api/v1/kyc/status [post]
func (k *KYC) GetKYCStatus(ctx context.Context, req *KYCStatusRequest) (*KYCStatusResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	status, err := k.KYCService.GetKYCStatus(ctx, int(userId))
	if err != nil {
		return nil, err
	}

	return &KYCStatusResponse{
		UserId:       int32(status.UserId),
		Status:       int32(status.Status),
		RealName:     status.RealName,
		IDCard:       maskIDCard(status.IDCard),
		RejectReason: status.RejectReason,
		SubmittedAt:  formatTime(status.SubmittedAt),
		ApprovedAt:   formatTime(status.ApprovedAt),
	}, nil
}

// SubmitKYC 提交KYC认证
//
//	@Summary		提交KYC认证
//	@Description	提交实名认证信息
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Param			request	body		KYCSubmitRequestData	true	"KYC提交请求"
//	@Success		200		{object}	KYCSubmitResponse
//	@Router			/api/v1/kyc/submit [post]
func (k *KYC) SubmitKYC(ctx context.Context, req *KYCSubmitRequestData) (*KYCSubmitResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	// 验证必填字段
	if req.RealName == "" {
		return nil, errorx.New(400, "真实姓名不能为空")
	}
	if req.IDCardNumber == "" {
		return nil, errorx.New(400, "身份证号不能为空")
	}
	if req.IDCardFrontUrl == "" {
		return nil, errorx.New(400, "身份证正面照片不能为空")
	}
	if req.IDCardBackUrl == "" {
		return nil, errorx.New(400, "身份证反面照片不能为空")
	}
	if req.FaceImageUrl == "" {
		return nil, errorx.New(400, "人脸照片不能为空")
	}

	result, err := k.KYCService.SubmitKYC(ctx, &service.KYCSubmitRequest{
		UserId:         int(userId),
		RealName:       req.RealName,
		IDCardNumber:   req.IDCardNumber,
		IDCardFrontUrl: req.IDCardFrontUrl,
		IDCardBackUrl:  req.IDCardBackUrl,
		FaceImageUrl:   req.FaceImageUrl,
	})
	if err != nil {
		return nil, err
	}

	return &KYCSubmitResponse{
		ApplicationId: result.ApplicationId,
		Status:        int32(result.Status),
		SubmittedAt:   result.SubmittedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetKYCDetail 获取KYC详情
//
//	@Summary		获取KYC详情
//	@Description	获取实名认证详细信息
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	KYCDetailResponse
//	@Router			/api/v1/kyc/detail [post]
func (k *KYC) GetKYCDetail(ctx context.Context, req *KYCDetailRequest) (*KYCDetailResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	detail, err := k.KYCService.GetKYCDetail(ctx, int(userId))
	if err != nil {
		return nil, err
	}

	return &KYCDetailResponse{
		UserId:         int32(detail.UserId),
		Status:         int32(detail.Status),
		RealName:       detail.RealName,
		IDCardNumber:   maskIDCard(detail.IDCardNumber),
		IDCardFrontUrl: detail.IDCardFrontUrl,
		IDCardBackUrl:  detail.IDCardBackUrl,
		FaceImageUrl:   detail.FaceImageUrl,
		RejectReason:   detail.RejectReason,
		SubmittedAt:    formatTime(detail.SubmittedAt),
		ReviewedAt:     formatTime(detail.ReviewedAt),
		ApplicationId:  detail.ApplicationId,
	}, nil
}

// UploadIDCard 上传身份证照片
//
//	@Summary		上传身份证
//	@Description	上传身份证正反面照片
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Param			request	body		KYCUploadIDCardRequest	true	"上传身份证请求"
//	@Success		200		{object}	KYCUploadIDCardResponse
//	@Router			/api/v1/kyc/upload-idcard [post]
func (k *KYC) UploadIDCard(ctx context.Context, req *KYCUploadIDCardRequest) (*KYCUploadIDCardResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.Side != "front" && req.Side != "back" {
		return nil, errorx.New(400, "side必须为front或back")
	}

	if req.ImageUrl == "" {
		return nil, errorx.New(400, "图片URL不能为空")
	}

	if err := k.KYCService.UploadIDCard(ctx, int(userId), req.Side, req.ImageUrl); err != nil {
		return nil, err
	}

	return &KYCUploadIDCardResponse{
		Success: true,
	}, nil
}

// UploadFaceImage 上传人脸照片
//
//	@Summary		上传人脸照片
//	@Description	上传人脸识别照片
//	@Tags			KYC
//	@Accept			json
//	@Produce		json
//	@Param			request	body		KYCUploadFaceRequest	true	"上传人脸照片请求"
//	@Success		200		{object}	KYCUploadFaceResponse
//	@Router			/api/v1/kyc/upload-face [post]
func (k *KYC) UploadFaceImage(ctx context.Context, req *KYCUploadFaceRequest) (*KYCUploadFaceResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userId := session.UserId

	if req.ImageUrl == "" {
		return nil, errorx.New(400, "图片URL不能为空")
	}

	if err := k.KYCService.UploadFaceImage(ctx, int(userId), req.ImageUrl); err != nil {
		return nil, err
	}

	return &KYCUploadFaceResponse{
		Success: true,
	}, nil
}

// Helper functions

func maskIDCard(idCard string) string {
	if len(idCard) < 8 {
		return idCard
	}
	// 隐藏中间部分: 330***********1234
	return idCard[:3] + "***********" + idCard[len(idCard)-4:]
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// Request and Response types

type KYCStatusRequest struct{}

type KYCStatusResponse struct {
	UserId       int32  `json:"user_id"`
	Status       int32  `json:"status"` // 0:未提交 1:审核中 2:已通过 3:已拒绝
	RealName     string `json:"real_name"`
	IDCard       string `json:"id_card"`
	RejectReason string `json:"reject_reason"`
	SubmittedAt  string `json:"submitted_at"`
	ApprovedAt   string `json:"approved_at"`
}

type KYCSubmitRequestData struct {
	RealName       string `json:"real_name"`
	IDCardNumber   string `json:"id_card_number"`
	IDCardFrontUrl string `json:"id_card_front_url"`
	IDCardBackUrl  string `json:"id_card_back_url"`
	FaceImageUrl   string `json:"face_image_url"`
}

type KYCSubmitResponse struct {
	ApplicationId string `json:"application_id"`
	Status        int32  `json:"status"`
	SubmittedAt   string `json:"submitted_at"`
}

type KYCDetailRequest struct{}

type KYCDetailResponse struct {
	UserId         int32  `json:"user_id"`
	Status         int32  `json:"status"`
	RealName       string `json:"real_name"`
	IDCardNumber   string `json:"id_card_number"`
	IDCardFrontUrl string `json:"id_card_front_url"`
	IDCardBackUrl  string `json:"id_card_back_url"`
	FaceImageUrl   string `json:"face_image_url"`
	RejectReason   string `json:"reject_reason"`
	SubmittedAt    string `json:"submitted_at"`
	ReviewedAt     string `json:"reviewed_at"`
	ApplicationId  string `json:"application_id"`
}

type KYCUploadIDCardRequest struct {
	Side     string `json:"side"` // front or back
	ImageUrl string `json:"image_url"`
}

type KYCUploadIDCardResponse struct {
	Success bool `json:"success"`
}

type KYCUploadFaceRequest struct {
	ImageUrl string `json:"image_url"`
}

type KYCUploadFaceResponse struct {
	Success bool `json:"success"`
}
