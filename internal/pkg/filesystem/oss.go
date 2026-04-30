package filesystem

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var _ IFilesystem = (*OssFilesystem)(nil)

type OssFilesystem struct {
	config OssSystemConfig
	client *oss.Client
}

func NewOssFilesystem(config OssSystemConfig) *OssFilesystem {
	opts := []oss.ClientOption{}

	endpoint := config.Endpoint
	usesCname := false

	// 根据ReplaceDomainEnabled开关决定使用哪个域名
	if config.ReplaceDomainEnabled && config.OssDomain != "" {
		// 使用自定义CNAME域名
		endpoint = config.OssDomain
		usesCname = true
		opts = append(opts, oss.UseCname(true))
	}

	// 移除endpoint中可能包含的协议前缀（如https://或http://）
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")

	scheme := "https"
	if !config.SSL {
		if usesCname && !config.OssDomainSSL {
			scheme = "http"
		} else if !usesCname {
			scheme = "http"
		}
	}

	client, err := oss.New(
		fmt.Sprintf("%s://%s", scheme, endpoint),
		config.AccessKeyId,
		config.AccessSecret,
		opts...,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create OSS client: %v", err))
	}

	return &OssFilesystem{
		config: config,
		client: client,
	}
}

func (m OssFilesystem) getBucket(bucketName string) (*oss.Bucket, error) {
	return m.client.Bucket(bucketName)
}

func (m OssFilesystem) getPublicHost(bucketName string) string {
	if m.config.OssDomain != "" {
		return m.config.OssDomain
	}
	// 移除endpoint中可能包含的协议前缀，确保只保留主机名
	endpoint := m.config.Endpoint
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	return fmt.Sprintf("%s.%s", bucketName, endpoint)
}

func (m OssFilesystem) Driver() string {
	return OssDriver
}

func (m OssFilesystem) BucketPublicName() string {
	return m.config.BucketPublic
}

func (m OssFilesystem) BucketPrivateName() string {
	return m.config.BucketPrivate
}

func (m OssFilesystem) Stat(bucketName string, objectName string) (*FileStatInfo, error) {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return nil, err
	}

	header, err := bucket.GetObjectDetailedMeta(objectName)
	if err != nil {
		return nil, err
	}

	contentLength, _ := strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	lastModified, _ := time.Parse(http.TimeFormat, header.Get("Last-Modified"))

	return &FileStatInfo{
		Name:        objectName,
		Size:        contentLength,
		Ext:         path.Ext(objectName),
		MimeType:    header.Get("Content-Type"),
		LastModTime: lastModified,
	}, nil
}

func (m OssFilesystem) Write(bucketName string, objectName string, stream []byte) error {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return err
	}

	return bucket.PutObject(objectName, bytes.NewReader(stream))
}

func (m OssFilesystem) Copy(bucketName string, srcObjectName, objectName string) error {
	return m.CopyObject(bucketName, srcObjectName, bucketName, objectName)
}

func (m OssFilesystem) CopyObject(srcBucketName string, srcObjectName, dstBucketName string, dstObjectName string) error {
	bucket, err := m.getBucket(dstBucketName)
	if err != nil {
		return err
	}

	_, err = bucket.CopyObjectFrom(srcBucketName, srcObjectName, dstObjectName)
	return err
}

func (m OssFilesystem) Delete(bucketName string, objectName string) error {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return err
	}

	return bucket.DeleteObject(objectName)
}

func (m OssFilesystem) GetObject(bucketName string, objectName string) ([]byte, error) {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return nil, err
	}

	body, err := bucket.GetObject(objectName)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	return io.ReadAll(body)
}

func (m OssFilesystem) PublicUrl(bucketName, objectName string) string {
	scheme := "https"
	if !m.config.SSL && (m.config.OssDomain == "" || !m.config.OssDomainSSL) {
		scheme = "http"
	}

	host := m.getPublicHost(bucketName)
	return fmt.Sprintf("%s://%s/%s", scheme, host, objectName)
}

func (m OssFilesystem) PrivateUrl(bucketName, objectName string, filename string, expire time.Duration) string {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		panic(err)
	}

	signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, int64(expire.Seconds()),
		oss.ResponseContentDisposition(fmt.Sprintf("attachment; filename=\"%s\"", filename)),
	)
	if err != nil {
		panic(err)
	}

	return signedURL
}

func (m OssFilesystem) InitiateMultipartUpload(bucketName, objectName string) (string, error) {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return "", err
	}

	result, err := bucket.InitiateMultipartUpload(objectName)
	if err != nil {
		return "", err
	}

	return result.UploadID, nil
}

func (m OssFilesystem) PutObjectPart(bucketName, objectName string, uploadID string, index int, data io.Reader, size int64) (ObjectPart, error) {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return ObjectPart{}, err
	}

	imur := oss.InitiateMultipartUploadResult{
		Bucket:   bucketName,
		Key:      objectName,
		UploadID: uploadID,
	}

	part, err := bucket.UploadPart(imur, data, size, index)
	if err != nil {
		return ObjectPart{}, err
	}

	return ObjectPart{
		PartNumber:     part.PartNumber,
		ETag:           part.ETag,
		PartObjectName: objectName,
	}, nil
}

func (m OssFilesystem) CompleteMultipartUpload(bucketName, objectName, uploadID string, parts []ObjectPart) error {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return err
	}

	imur := oss.InitiateMultipartUploadResult{
		Bucket:   bucketName,
		Key:      objectName,
		UploadID: uploadID,
	}

	var ossParts []oss.UploadPart
	for _, part := range parts {
		ossParts = append(ossParts, oss.UploadPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}

	_, err = bucket.CompleteMultipartUpload(imur, ossParts)
	return err
}

func (m OssFilesystem) AbortMultipartUpload(bucketName, objectName, uploadID string) error {
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return err
	}

	imur := oss.InitiateMultipartUploadResult{
		Bucket:   bucketName,
		Key:      objectName,
		UploadID: uploadID,
	}

	return bucket.AbortMultipartUpload(imur)
}

// GeneratePresignedPutURL generates a presigned PUT URL for direct client upload.
// The URL is valid for the specified duration. Clients must send the same Content-Type
// header when PUTting as was used when signing (use application/octet-stream for any file type).
func (m OssFilesystem) GeneratePresignedPutURL(bucketName, objectName string, expire time.Duration, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	options := []oss.Option{oss.ContentType(contentType)}
	bucket, err := m.getBucket(bucketName)
	if err != nil {
		return "", err
	}

	signedURL, err := bucket.SignURL(objectName, oss.HTTPPut, int64(expire.Seconds()), options...)
	if err != nil {
		return "", fmt.Errorf("generate presigned URL failed: %w", err)
	}

	signedURL = m.fixOSSPresignedURL(signedURL)
	signedURL = m.normalizeOSSPresignedURL(signedURL)

	return signedURL, nil
}

// fixOSSPresignedURL fixes the presigned URL by ensuring OSSAccessKeyId has a value
func (m OssFilesystem) fixOSSPresignedURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	query := u.Query()

	// Check if OSSAccessKeyId exists but has no value
	// This can happen with certain SDK versions
	if _, exists := query["OSSAccessKeyId"]; exists && query.Get("OSSAccessKeyId") == "" {
		query.Set("OSSAccessKeyId", m.config.AccessKeyId)
		u.RawQuery = query.Encode()
	}

	return u.String()
}

// normalizeOSSPresignedURL ensures path separators are not percent-encoded.
func (m OssFilesystem) normalizeOSSPresignedURL(urlStr string) string {
	queryIndex := strings.Index(urlStr, "?")
	if queryIndex == -1 {
		return strings.ReplaceAll(strings.ReplaceAll(urlStr, "%2F", "/"), "%2f", "/")
	}

	pathPart := urlStr[:queryIndex]
	queryPart := urlStr[queryIndex:]
	pathPart = strings.ReplaceAll(strings.ReplaceAll(pathPart, "%2F", "/"), "%2f", "/")

	return pathPart + queryPart
}

// GetOssObjectURL returns the full object URL for an uploaded file.
func (m OssFilesystem) GetOssObjectURL(bucketName, objectName string) string {
	scheme := "https"
	if !m.config.SSL && (m.config.OssDomain == "" || !m.config.OssDomainSSL) {
		scheme = "http"
	}

	host := m.getPublicHost(bucketName)

	// Ensure objectName doesn't start with /
	objectName = strings.TrimPrefix(objectName, "/")

	return fmt.Sprintf("%s://%s/%s", scheme, host, objectName)
}

// replaceOSSUrlDomain 替换签名URL中的OSS域名为自定义域名，以避免CORS问题
// 当配置了OssDomain且ReplaceDomainEnabled为true时，将URL中的OSS域名替换为自定义域名
func (m OssFilesystem) replaceOSSUrlDomain(urlStr string) string {
	// 如果没有启用域名替换或没有配置自定义域名，直接返回原URL
	if !m.config.ReplaceDomainEnabled || m.config.OssDomain == "" {
		return urlStr
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	// 获取OSS的原始endpoint（去掉协议前缀）
	endpoint := strings.TrimPrefix(strings.TrimPrefix(m.config.Endpoint, "https://"), "http://")

	// OSS签名URL的host格式为 bucket.endpoint
	// 检查host是否以 bucket.endpoint 开头
	expectedHostPrefix := fmt.Sprintf("%s.%s", m.config.BucketPublic, endpoint)

	// 如果当前host匹配OSS的格式，直接替换为自定义域名
	if u.Host == expectedHostPrefix {
		u.Host = m.config.OssDomain
	}

	return u.String()
}
