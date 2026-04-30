package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	sts "github.com/tencentyun/qcloud-cos-sts-sdk/go"
)

type STS struct {
	Config     *config.Config
	Filesystem filesystem.IFilesystem
}

// GetUploadPresignedUrlRequest 获取上传预签名URL请求参数
type GetUploadPresignedUrlRequest struct {
	FileName string `json:"file_name" binding:"required"` // 文件名
	FileType string `json:"file_type"`                    // 文件类型 (image, video, file)
	// ContentType should match the PUT request Content-Type header
	ContentType string `json:"content_type"`
}

// GetUploadPresignedUrlResponse 获取上传预签名URL响应参数
type GetUploadPresignedUrlResponse struct {
	Platform     string `json:"platform"`      // 存储平台 (cos, oss)
	PresignedUrl string `json:"presigned_url"` // 预签名PUT URL
	ObjectUrl    string `json:"object_url"`    // 上传完成后的文件访问URL
	Key          string `json:"key"`           // 对象路径
	Bucket       string `json:"bucket"`        // Bucket名称
	StartTime    int64  `json:"start_time"`    // 生效时间(Unix时间戳)
	ExpiredTime  int64  `json:"expired_time"`  // 过期时间(Unix时间戳)
}

// GetCosSTSCredentials 获取COS临时密钥
//
//	@Summary		获取COS临时密钥
//	@Description	获取用于客户端直接上传文件的临时访问凭证
//	@Tags			上传
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.GetCosSTSRequest	true	"请求参数"
//	@Success		200		{object}	web.GetCosSTSResponse
//	@Router			/api/v1/upload/sts [post]
//	@Security		Bearer
func (s *STS) GetCosSTSCredentials(ctx *gin.Context) (*web.GetCosSTSResponse, error) {
	in := &web.GetCosSTSRequest{}
	if err := ctx.ShouldBindJSON(in); err != nil {
		return nil, errorx.New(400, err.Error())
	}

	cosConfig := s.Config.Filesystem.Cos

	// 生成对象路径（GenMediaObjectName已经包含了media/{type}/{date}的路径结构）
	objectKey := strutil.GenMediaObjectName(in.FileName, 0, 0)

	// 创建STS客户端
	client := sts.NewClient(
		cosConfig.SecretId,
		cosConfig.SecretKey,
		nil,
	)

	// 设置策略（允许上传到指定路径）
	bucket := cosConfig.BucketPublic
	// 使用配置中的主账号UIN（腾讯云账号ID，10位数字）
	cosUid := cosConfig.AppId
	if cosUid == "" {
		// 如果未配置，尝试从bucket名称中提取作为回退（不推荐）
		parts := strings.Split(bucket, "-")
		cosUid = parts[len(parts)-1]
		logger.Infof("[STS Warning] cos.app_id 未配置，从bucket名称提取: %s", cosUid)
	}

	policy := &sts.CredentialPolicy{
		Statement: []sts.CredentialPolicyStatement{
			{
				Action: []string{
					// 简单上传
					"name/cos:PostObject",
					"name/cos:PutObject",
					// 分片上传
					"name/cos:InitiateMultipartUpload",
					"name/cos:ListMultipartUploads",
					"name/cos:ListParts",
					"name/cos:UploadPart",
					"name/cos:CompleteMultipartUpload",
				},
				Effect: "allow",
				Resource: []string{
					fmt.Sprintf("qcs::cos:%s:uid/%s:%s/%s", cosConfig.Region, cosUid, bucket, objectKey),
				},
			},
		},
	}

	opt := &sts.CredentialOptions{
		DurationSeconds: int64(time.Hour.Seconds()),
		Region:          cosConfig.Region,
		Policy:          policy,
	}

	res, err := client.GetCredential(opt)
	if err != nil {
		return nil, errorx.New(500, "获取临时密钥失败: "+err.Error())
	}

	if res == nil || res.Credentials == nil {
		return nil, errorx.New(500, "获取临时密钥失败: 返回凭证为空")
	}

	tmpBytes, _ := json.Marshal(res)
	logger.Infof("[STS] 获取COS临时密钥成功: %s", string(tmpBytes))

	credentials := res.Credentials
	startTime := int64(res.StartTime)
	expiredTime := int64(res.ExpiredTime)
	if startTime == 0 || expiredTime == 0 {
		now := time.Now().Unix()
		startTime = now
		expiredTime = now + opt.DurationSeconds
	}

	// 生成预签名PUT URL
	contentType := resolveContentType(in.FileName, "")
	presignedURL, err := s.generatePresignedPutURL(
		cosConfig,
		bucket,
		objectKey,
		time.Duration(opt.DurationSeconds)*time.Second,
		contentType,
	)
	if err != nil {
		return nil, errorx.New(500, "生成预签名URL失败: "+err.Error())
	}

	return &web.GetCosSTSResponse{
		TmpSecretId:  credentials.TmpSecretID,
		TmpSecretKey: credentials.TmpSecretKey,
		SessionToken: credentials.SessionToken,
		Region:       cosConfig.Region,
		Bucket:       bucket,
		Key:          objectKey,
		StartTime:    startTime,
		ExpiredTime:  expiredTime,
		PresignedUrl: presignedURL,
	}, nil
}

// GetUploadPresignedUrl 获取上传预签名URL（统一接口，支持COS和OSS）
//
//	@Summary		获取上传预签名URL
//	@Description	根据当前配置的存储驱动生成预签名上传URL，支持COS和OSS
//	@Tags			上传
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GetUploadPresignedUrlRequest	true	"请求参数"
//	@Success		200		{object}	GetUploadPresignedUrlResponse
//	@Router			/api/v1/upload/presigned-url [post]
//	@Security		Bearer
func (s *STS) GetUploadPresignedUrl(ctx *gin.Context) (*GetUploadPresignedUrlResponse, error) {
	in := &GetUploadPresignedUrlRequest{}
	if err := ctx.ShouldBindJSON(in); err != nil {
		return nil, errorx.New(400, err.Error())
	}

	objectKey := strutil.GenMediaObjectName(in.FileName, 0, 0)
	driver := s.Config.Filesystem.Default
	duration := time.Hour
	// 预签名上传统一使用 application/octet-stream，前端 PUT 时也传该类型即可支持任意文件（zip/wav/apk 等）
	contentType := "application/octet-stream"
	if strings.TrimSpace(in.ContentType) != "" {
		contentType = resolveContentType(in.FileName, in.ContentType)
	}

	switch driver {
	case filesystem.OssDriver:
		return s.getOssPresignedUrl(objectKey, duration, contentType)
	case filesystem.CosDriver:
		return s.getCosPresignedUrl(objectKey, duration, contentType)
	default:
		return nil, errorx.New(400, "当前存储驱动不支持预签名上传: "+driver)
	}
}

// getOssPresignedUrl 生成阿里云OSS预签名PUT URL
func (s *STS) getOssPresignedUrl(objectKey string, duration time.Duration, contentType string) (*GetUploadPresignedUrlResponse, error) {
	ossConfig := s.Config.Filesystem.Oss

	ossFs := filesystem.NewOssFilesystem(ossConfig)

	bucket := ossConfig.BucketPublic
	presignedURL, err := ossFs.GeneratePresignedPutURL(bucket, objectKey, duration, contentType)
	if err != nil {
		return nil, errorx.New(500, "生成OSS预签名URL失败: "+err.Error())
	}

	objectURL := ossFs.GetOssObjectURL(bucket, objectKey)

	now := time.Now().Unix()

	logger.Infof("[OSS] 生成预签名URL成功, presignedURL: %s, key: %s, url: %s", presignedURL, objectKey, objectURL)

	return &GetUploadPresignedUrlResponse{
		Platform:     "oss",
		PresignedUrl: presignedURL,
		ObjectUrl:    objectURL,
		Key:          objectKey,
		Bucket:       bucket,
		StartTime:    now,
		ExpiredTime:  now + int64(duration.Seconds()),
	}, nil
}

// getCosPresignedUrl 生成腾讯云COS预签名PUT URL
func (s *STS) getCosPresignedUrl(objectKey string, duration time.Duration, contentType string) (*GetUploadPresignedUrlResponse, error) {
	cosConfig := s.Config.Filesystem.Cos
	bucket := cosConfig.BucketPublic

	presignedURL, err := s.generatePresignedPutURL(cosConfig, bucket, objectKey, duration, contentType)
	if err != nil {
		return nil, errorx.New(500, "生成COS预签名URL失败: "+err.Error())
	}

	objectURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", bucket, cosConfig.Region, objectKey)

	now := time.Now().Unix()

	return &GetUploadPresignedUrlResponse{
		Platform:     "cos",
		PresignedUrl: presignedURL,
		ObjectUrl:    objectURL,
		Key:          objectKey,
		Bucket:       bucket,
		StartTime:    now,
		ExpiredTime:  now + int64(duration.Seconds()),
	}, nil
}

// generatePresignedPutURL 使用主密钥生成预签名PUT URL
func (s *STS) generatePresignedPutURL(
	cosConfig filesystem.CosSystemConfig,
	bucket string,
	objectKey string,
	duration time.Duration,
	contentType string,
) (string, error) {
	// 构建COS URL
	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, cosConfig.Region)
	u, err := url.Parse(bucketURL)
	if err != nil {
		return "", fmt.Errorf("解析bucket URL失败: %w", err)
	}

	// 创建COS客户端（使用主密钥）
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cosConfig.SecretId,
			SecretKey: cosConfig.SecretKey,
		},
	})

	// 生成预签名URL（用于PUT上传）
	var opt *cos.PresignedURLOptions
	if contentType != "" {
		opt = &cos.PresignedURLOptions{Header: &http.Header{}}
		opt.Header.Set("Content-Type", contentType)
	}
	presignedURL, err := client.Object.GetPresignedURL(
		context.Background(),
		http.MethodPut,
		objectKey,
		cosConfig.SecretId,
		cosConfig.SecretKey,
		duration,
		opt,
	)
	if err != nil {
		return "", fmt.Errorf("生成预签名URL失败: %w", err)
	}

	logger.Infof("[STS] 生成预签名URL成功: %s", presignedURL.String())
	return presignedURL.String(), nil
}

func resolveContentType(fileName string, override string) string {
	contentType := strings.TrimSpace(override)
	if contentType != "" {
		return contentType
	}

	ext := strings.ToLower(path.Ext(fileName))
	if ext != "" {
		contentType = mime.TypeByExtension(ext)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return contentType
}
