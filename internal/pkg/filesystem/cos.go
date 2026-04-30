package filesystem

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

var _ IFilesystem = (*CosFilesystem)(nil)

type CosFilesystem struct {
	config     CosSystemConfig
	httpClient *http.Client
}

func NewCosFilesystem(config CosSystemConfig) *CosFilesystem {
	return &CosFilesystem{
		config: config,
		httpClient: &http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:  config.SecretId,
				SecretKey: config.SecretKey,
			},
		},
	}
}

func (m CosFilesystem) getHost4Download(bucketName string) string {
	if m.config.CosDomain != "" {
		return fmt.Sprintf("%s", m.config.CosDomain)
	}

	return fmt.Sprintf("%s.cos.%s.myqcloud.com", bucketName, m.config.Region)
}

func (m CosFilesystem) getHost4Upload(bucketName string) string {
	if m.config.CosDomain != "" {
		return fmt.Sprintf("%s", m.config.CosDomain)
	}

	return fmt.Sprintf("%s.cos.%s.myqcloud.com", bucketName, m.config.Region)
}

func (m CosFilesystem) getClient4Download(bucketName string) *cos.Client {
	scheme := "https"
	if m.config.CosDomain != "" {
		if !m.config.CosDomainSSL {
			scheme = "http"
		}
	} else if !m.config.SSL {
		scheme = "http"
	}

	bucketUrlStr := fmt.Sprintf("%s://%s", scheme, m.getHost4Download(bucketName))
	u, _ := url.Parse(bucketUrlStr)
	b := &cos.BaseURL{BucketURL: u}

	// Create client
	return cos.NewClient(b, m.httpClient)
}

func (m CosFilesystem) getClient4Upload(bucketName string) *cos.Client {
	scheme := "https"
	if m.config.CosDomain != "" {
		if !m.config.CosDomainSSL {
			scheme = "http"
		}
	} else if !m.config.SSL {
		scheme = "http"
	}

	bucketUrlStr := fmt.Sprintf("%s://%s", scheme, m.getHost4Upload(bucketName))
	u, _ := url.Parse(bucketUrlStr)
	b := &cos.BaseURL{BucketURL: u}

	// Create client
	return cos.NewClient(b, m.httpClient)
}

func (m CosFilesystem) Driver() string {
	return CosDriver
}

func (m CosFilesystem) BucketPublicName() string {
	return m.config.BucketPublic
}

func (m CosFilesystem) BucketPrivateName() string {
	return m.config.BucketPrivate
}

func (m CosFilesystem) Stat(bucketName string, objectName string) (*FileStatInfo, error) {
	client := m.getClient4Download(bucketName)
	resp, err := client.Object.Head(context.Background(), objectName, nil)
	if err != nil {
		return nil, err
	}

	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	lastModified, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))

	return &FileStatInfo{
		Name:        objectName,
		Size:        contentLength,
		Ext:         path.Ext(objectName),
		MimeType:    resp.Header.Get("Content-Type"),
		LastModTime: lastModified,
	}, nil
}

func (m CosFilesystem) Write(bucketName string, objectName string, stream []byte) error {
	client := m.getClient4Upload(bucketName)
	_, err := client.Object.Put(context.Background(), objectName, strings.NewReader(string(stream)), nil)
	return err
}

func (m CosFilesystem) Copy(bucketName string, srcObjectName, objectName string) error {
	return m.CopyObject(bucketName, srcObjectName, bucketName, objectName)
}

func (m CosFilesystem) CopyObject(srcBucketName string, srcObjectName, dstBucketName string, dstObjectName string) error {
	client := m.getClient4Upload(dstBucketName)

	// Source URL for COS Copy
	// source URL format: <bucket-name>.cos.<region>.myqcloud.com/<key>
	srcUrl := fmt.Sprintf("%s/%s", m.getHost4Download(srcBucketName), srcObjectName)

	_, _, err := client.Object.Copy(context.Background(), dstObjectName, srcUrl, nil)
	return err
}

func (m CosFilesystem) Delete(bucketName string, objectName string) error {
	client := m.getClient4Upload(bucketName)
	_, err := client.Object.Delete(context.Background(), objectName)
	return err
}

func (m CosFilesystem) GetObject(bucketName string, objectName string) ([]byte, error) {
	client := m.getClient4Download(bucketName)
	resp, err := client.Object.Get(context.Background(), objectName, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (m CosFilesystem) PublicUrl(bucketName, objectName string) string {
	client := m.getClient4Download(bucketName)
	// For public bucket, we can generate a presigned URL or just the direct URL.
	// We'll use presigned generic method as typical for consistency, or ObjectUrl.
	// But usually PublicUrl implies no signature.
	// The Minio implementation returns a presigned URL even for PublicUrl (likely with long expiry or no auth if handled differently).
	// Actually Minio checks if it is public bucket, but still calls PresignedGetObject.

	// For COS, ObjectURL gives the direct URL.
	u := client.Object.GetObjectURL(objectName)
	return u.String()
}

func (m CosFilesystem) PrivateUrl(bucketName, objectName string, filename string, expire time.Duration) string {
	client := m.getClient4Download(bucketName)

	opt := &cos.PresignedURLOptions{
		Query:  &url.Values{},
		Header: &http.Header{},
	}
	opt.Query.Add("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	u, err := client.Object.GetPresignedURL(context.Background(), http.MethodGet, objectName, m.config.SecretId, m.config.SecretKey, expire, opt)
	if err != nil {
		panic(err)
	}

	return u.String()
}

func (m CosFilesystem) InitiateMultipartUpload(bucketName, objectName string) (string, error) {
	client := m.getClient4Upload(bucketName)
	v, _, err := client.Object.InitiateMultipartUpload(context.Background(), objectName, nil)
	if err != nil {
		return "", err
	}
	return v.UploadID, nil
}

func (m CosFilesystem) PutObjectPart(bucketName, objectName string, uploadID string, index int, data io.Reader, size int64) (ObjectPart, error) {
	client := m.getClient4Upload(bucketName)
	// COS part number 1-10000
	partNumber := index

	// The SDK expects data as io.Reader.
	// Optimization: The SDK doesn't strictly require Seekable reader but uses it if available for retries/signing.

	resp, err := client.Object.UploadPart(context.Background(), objectName, uploadID, partNumber, data, nil)
	if err != nil {
		return ObjectPart{}, err
	}

	return ObjectPart{
		PartNumber:     partNumber,
		ETag:           resp.Header.Get("ETag"),
		PartObjectName: objectName,
	}, nil
}

func (m CosFilesystem) CompleteMultipartUpload(bucketName, objectName, uploadID string, parts []ObjectPart) error {
	client := m.getClient4Upload(bucketName)

	opt := &cos.CompleteMultipartUploadOptions{}
	for _, part := range parts {
		opt.Parts = append(opt.Parts, cos.Object{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}

	_, _, err := client.Object.CompleteMultipartUpload(context.Background(), objectName, uploadID, opt)
	return err
}

func (m CosFilesystem) AbortMultipartUpload(bucketName, objectName, uploadID string) error {
	client := m.getClient4Upload(bucketName)
	_, err := client.Object.AbortMultipartUpload(context.Background(), objectName, uploadID)
	return err
}
