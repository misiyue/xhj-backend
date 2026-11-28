package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var _ IFilesystem = (*AwsFilesystem)(nil)

type AwsFilesystem struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	config        AwsSystemConfig
}

func NewAwsFilesystem(conf AwsSystemConfig) *AwsFilesystem {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(conf.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(conf.SecretId, conf.SecretKey, "")),
	)
	if err != nil {
		panic(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	usePathStyle := conf.UsePathStyle
	// 自定义 endpoint（如 MinIO）未显式配置时默认 path-style，兼容性更好
	if conf.Endpoint != "" && !conf.UsePathStyle {
		usePathStyle = true
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
		if conf.Endpoint != "" {
			endpoint := strings.TrimSpace(conf.Endpoint)
			endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
			scheme := "https"
			if !conf.SSL {
				scheme = "http"
			}
			o.BaseEndpoint = aws.String(fmt.Sprintf("%s://%s", scheme, endpoint))
		}
	})

	presignClient := s3.NewPresignClient(client)

	return &AwsFilesystem{
		client:        client,
		presignClient: presignClient,
		config:        conf,
	}
}

func (m AwsFilesystem) Driver() string {
	return AwsDriver
}

func (m AwsFilesystem) BucketPublicName() string {
	return m.config.BucketPublic
}

func (m AwsFilesystem) BucketPrivateName() string {
	return m.config.BucketPrivate
}

func (m AwsFilesystem) Stat(bucketName string, objectName string) (*FileStatInfo, error) {
	output, err := m.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, err
	}

	return &FileStatInfo{
		Name:        objectName,
		Size:        aws.ToInt64(output.ContentLength),
		Ext:         path.Ext(objectName),
		MimeType:    aws.ToString(output.ContentType),
		LastModTime: aws.ToTime(output.LastModified),
	}, nil
}

func (m AwsFilesystem) Write(bucketName string, objectName string, stream []byte) error {
	_, err := m.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   bytes.NewReader(stream),
	})
	return err
}

func (m AwsFilesystem) Copy(bucketName string, srcObjectName, objectName string) error {
	return m.CopyObject(bucketName, srcObjectName, bucketName, objectName)
}

func (m AwsFilesystem) CopyObject(srcBucketName string, srcObjectName, dstBucketName string, dstObjectName string) error {
	// Source must be URL encoded "bucket/key"
	source := fmt.Sprintf("%s/%s", srcBucketName, srcObjectName)
	_, err := m.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucketName),
		Key:        aws.String(dstObjectName),
		CopySource: aws.String(source),
	})
	return err
}

func (m AwsFilesystem) Delete(bucketName string, objectName string) error {
	_, err := m.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	return err
}

func (m AwsFilesystem) GetObject(bucketName string, objectName string) ([]byte, error) {
	output, err := m.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

func (m AwsFilesystem) PublicUrl(bucketName, objectName string) string {
	// 公开桶优先返回直链；无自定义域名时退回短期 GET 预签名
	if m.config.S3Domain != "" || m.config.BucketPublic == bucketName {
		return m.GetAwsObjectURL(bucketName, objectName)
	}
	request, err := m.presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}, func(o *s3.PresignOptions) {
		o.Expires = 30 * time.Minute
	})
	if err != nil {
		panic(err)
	}
	return request.URL
}

// GeneratePresignedPutURL 生成 Amazon S3 预签名 PUT URL，供前端直传。
// 前端 PUT 时 Content-Type 必须与签名时一致。
func (m AwsFilesystem) GeneratePresignedPutURL(bucketName, objectName string, expire time.Duration, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if expire <= 0 {
		expire = time.Hour
	}

	request, err := m.presignClient.PresignPutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectName),
		ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = expire
	})
	if err != nil {
		return "", fmt.Errorf("generate s3 presigned put url failed: %w", err)
	}
	return request.URL, nil
}

// GetAwsObjectURL 返回上传完成后的对象访问地址
func (m AwsFilesystem) GetAwsObjectURL(bucketName, objectName string) string {
	objectName = strings.TrimPrefix(objectName, "/")
	scheme := "https"
	if !m.config.SSL {
		scheme = "http"
	}

	if domain := strings.TrimSpace(m.config.S3Domain); domain != "" {
		domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
		if m.config.S3DomainSSL {
			scheme = "https"
		} else if !m.config.SSL {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s/%s", scheme, domain, objectName)
	}

	if endpoint := strings.TrimSpace(m.config.Endpoint); endpoint != "" {
		endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
		// 自定义 endpoint 使用 path-style: https://endpoint/bucket/key
		return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, bucketName, objectName)
	}

	// 标准 AWS virtual-hosted-style
	region := m.config.Region
	if region == "" {
		region = "us-east-1"
	}
	return fmt.Sprintf("%s://%s.s3.%s.amazonaws.com/%s", scheme, bucketName, region, objectName)
}

func (m AwsFilesystem) PrivateUrl(bucketName, objectName string, filename string, expire time.Duration) string {
	request, err := m.presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket:                     aws.String(bucketName),
		Key:                        aws.String(objectName),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", filename)),
	}, func(o *s3.PresignOptions) {
		o.Expires = expire
	})
	if err != nil {
		panic(err)
	}

	return request.URL
}

func (m AwsFilesystem) InitiateMultipartUpload(bucketName, objectName string) (string, error) {
	output, err := m.client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(output.UploadId), nil
}

func (m AwsFilesystem) PutObjectPart(bucketName, objectName string, uploadID string, index int, data io.Reader, size int64) (ObjectPart, error) {
	// AWS PartNumber starts at 1
	partNumber := int32(index)

	// We need to read the data into a ReadSeeker if possible, but the interface says io.Reader.
	// AWS SDK v2 PutObject needs Body as io.Reader.
	// UploadPartInput Body is io.Reader.

	output, err := m.client.UploadPart(context.TODO(), &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(objectName),
		PartNumber: aws.Int32(partNumber),
		UploadId:   aws.String(uploadID),
		Body:       data,
	})
	if err != nil {
		return ObjectPart{}, err
	}

	return ObjectPart{
		PartNumber:     int(partNumber),
		ETag:           aws.ToString(output.ETag),
		PartObjectName: objectName,
	}, nil
}

func (m AwsFilesystem) CompleteMultipartUpload(bucketName, objectName, uploadID string, parts []ObjectPart) error {
	var completedParts []types.CompletedPart
	for _, part := range parts {
		completedParts = append(completedParts, types.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int32(int32(part.PartNumber)),
		})
	}

	_, err := m.client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(objectName),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	return err
}

func (m AwsFilesystem) AbortMultipartUpload(bucketName, objectName, uploadID string) error {
	_, err := m.client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(objectName),
		UploadId: aws.String(uploadID),
	})
	return err
}
