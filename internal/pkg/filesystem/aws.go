package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
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

	// Create an Amazon S3 service client
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // Force path style if needed, usually cleaner for compatibility
		if conf.Endpoint != "" {
			o.BaseEndpoint = aws.String(conf.Endpoint)
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
	request, err := m.presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}, func(o *s3.PresignOptions) {
		o.Expires = 30 * time.Minute
	})
	if err != nil {
		panic(err)
	}
	// For public bucket, we might want to return the direct URL if it's configured as public access,
	// but using presigned URL with long expiry or just URL construction is common.
	// The Minio implementation returns a presigned URL.

	return request.URL
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
