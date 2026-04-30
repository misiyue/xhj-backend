package filesystem

import (
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	// conf := testutil.GetConfig()
	//
	// conf.Filesystem.Local.SSL = false
	// conf.Filesystem.Local.Root = "./data"
	// conf.Filesystem.Local.Endpoint = "127.0.0.1:9000"
	// conf.Filesystem.Local.BucketPublic = "im-static"
	// conf.Filesystem.Local.BucketPrivate = "im-private"
	//
	// conf.Filesystem.Minio.SSL = false
	// conf.Filesystem.Minio.Endpoint = "127.0.0.1:9000"
	// conf.Filesystem.Minio.SecretId = "Q3AM3UQ867SPQQA43P2F"
	// conf.Filesystem.Minio.SecretKey = "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
	// conf.Filesystem.Minio.BucketPublic = "im-static"
	// conf.Filesystem.Minio.BucketPrivate = "im-private"
	//
	// client := NewMinioFilesystem(conf)
	// client := NewLocalFilesystem(conf)

	// err := client.Write("im-private", []byte("hello world"), "filesystem.txt")
	// fmt.Println(err)

	// bt, err := client.GetObject("im-private", "filesystem.txt")
	// fmt.Println(err)
	// fmt.Println(string(bt))
	//
	// err := client.WriteLocal("im-private", "./util.go", "filesystem.txt")
	// fmt.Println(err)

	// err := client.Copy("im-private", "filesystem.txt", "filesystem2.txt")
	// fmt.Println(err)

	// bt, err := client.Stat("im-private", "filesystem2.txt")
	// fmt.Println(err)
	// fmt.Println(bt)

	// value := client.PublicUrl("im-private", "filesystem2.txt")
	// fmt.Println(value)

	// // Make a buffer with 6MB of data
	// buf := bytes.Repeat([]byte("abcdef"), 1024*1024)
	//
	// // Open the file.
	// file, err := os.Open("./node-v18.15.0-linux-x64.tar.xz")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	//
	// defer file.Close()
	//
	// items := make([]ObjectPart, 0)
	//
	// upload, err := client.InitiateMultipartUpload("im-private", "node-v18.15.0-linux-x64.txt")
	// fmt.Println(upload, err)
	//
	// if err != nil {
	// 	return
	// }
	//
	// obj, err := client.PutObjectPart("im-private", "node-v18.15.0-linux-x64.txt", upload, 1, bytes.NewReader(buf[:5*1024*1024]), 5*1024*1024)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	//
	// items = append(items, obj)
	//
	// obj, err = client.PutObjectPart("im-private", "node-v18.15.0-linux-x64.txt", upload, 2, bytes.NewReader(buf[5*1024*1024:]), 1024*1024)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	//
	// items = append(items, obj)
	//
	// // Close the file.
	// err = client.CompleteMultipartUpload("im-private", "node-v18.15.0-linux-x64.txt", upload, items)
	// fmt.Println(err)
}

func TestOssFilesystemInterface(t *testing.T) {
	conf := OssSystemConfig{
		SSL:           true,
		AccessKeyId:   "test-key-id",
		AccessSecret:  "test-key-secret",
		BucketPublic:  "im-static",
		BucketPrivate: "im-private",
		Endpoint:      "oss-cn-hangzhou.aliyuncs.com",
	}

	fs := NewOssFilesystem(conf)

	// Verify it implements IFilesystem
	var _ IFilesystem = fs

	// Test Driver
	if fs.Driver() != OssDriver {
		t.Errorf("expected driver %q, got %q", OssDriver, fs.Driver())
	}

	// Test BucketPublicName
	if fs.BucketPublicName() != "im-static" {
		t.Errorf("expected bucket public %q, got %q", "im-static", fs.BucketPublicName())
	}

	// Test BucketPrivateName
	if fs.BucketPrivateName() != "im-private" {
		t.Errorf("expected bucket private %q, got %q", "im-private", fs.BucketPrivateName())
	}

	// Test PublicUrl
	url := fs.PublicUrl("im-static", "media/test.jpg")
	expected := "https://im-static.oss-cn-hangzhou.aliyuncs.com/media/test.jpg"
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}

	// Test GetOssObjectURL
	objUrl := fs.GetOssObjectURL("im-static", "media/test.jpg")
	if objUrl != expected {
		t.Errorf("expected object URL %q, got %q", expected, objUrl)
	}

	// Test GetOssObjectURL with leading slash
	objUrl2 := fs.GetOssObjectURL("im-static", "/media/test.jpg")
	if objUrl2 != expected {
		t.Errorf("expected object URL %q, got %q", expected, objUrl2)
	}
}

func TestOssFilesystemWithCustomDomain(t *testing.T) {
	conf := OssSystemConfig{
		SSL:           true,
		AccessKeyId:   "test-key-id",
		AccessSecret:  "test-key-secret",
		BucketPublic:  "im-static",
		BucketPrivate: "im-private",
		Endpoint:      "oss-cn-hangzhou.aliyuncs.com",
		OssDomain:     "cdn.example.com",
		OssDomainSSL:  true,
	}

	fs := NewOssFilesystem(conf)

	// Test PublicUrl with custom domain
	url := fs.PublicUrl("im-static", "media/test.jpg")
	expected := "https://cdn.example.com/media/test.jpg"
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}

func TestOssFilesystemHttpUrl(t *testing.T) {
	conf := OssSystemConfig{
		SSL:           false,
		AccessKeyId:   "test-key-id",
		AccessSecret:  "test-key-secret",
		BucketPublic:  "im-static",
		BucketPrivate: "im-private",
		Endpoint:      "oss-cn-hangzhou.aliyuncs.com",
	}

	fs := NewOssFilesystem(conf)

	url := fs.PublicUrl("im-static", "media/test.jpg")
	expected := "http://im-static.oss-cn-hangzhou.aliyuncs.com/media/test.jpg"
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}

// TestOssPresignedUrlDomainReplacement tests that presigned URLs have OSS domain replaced with custom domain
func TestOssPresignedUrlDomainReplacement(t *testing.T) {
	conf := OssSystemConfig{
		SSL:                  true,
		AccessKeyId:          "LTAI5tPn1vL8BLLAm3R2vs61",
		AccessSecret:         "test-key-secret",
		BucketPublic:         "xhjobj",
		BucketPrivate:        "im-private",
		Endpoint:             "oss-ap-southeast-1.aliyuncs.com",
		OssDomain:            "cdn.example.com",
		OssDomainSSL:         true,
		ReplaceDomainEnabled: true,
	}

	fs := NewOssFilesystem(conf)

	// Test replaceOSSUrlDomain with a typical signed URL
	signedURL := "https://xhjobj.oss-ap-southeast-1.aliyuncs.com/media%2Fimage%2F202602%2F745868c7%2F6c771051-8580-401e-a059-813.jpeg?Expires=1770885499&OSSAccessKeyId=LTAI5tPn1vL8BLLAm3R2vs61&Signature=jtLccknYN7WZZuiyJDZ3limVVag%3D"

	result := fs.replaceOSSUrlDomain(signedURL)

	// The domain should be replaced with custom domain
	if !strings.Contains(result, "cdn.example.com") {
		t.Errorf("expected custom domain in URL, got %q", result)
	}

	// The original OSS domain should not be in the result
	if strings.Contains(result, "oss-ap-southeast-1.aliyuncs.com") {
		t.Errorf("OSS domain should be replaced, got %q", result)
	}

	// Query parameters should be preserved
	if !strings.Contains(result, "Expires=1770885499") {
		t.Errorf("query parameters should be preserved, got %q", result)
	}
}

// TestOssPresignedUrlWithoutCustomDomain tests that presigned URLs are unchanged without custom domain
func TestOssPresignedUrlWithoutCustomDomain(t *testing.T) {
	conf := OssSystemConfig{
		SSL:                  true,
		AccessKeyId:          "test-key-id",
		AccessSecret:         "test-key-secret",
		BucketPublic:         "xhjobj",
		BucketPrivate:        "im-private",
		Endpoint:             "oss-ap-southeast-1.aliyuncs.com",
		OssDomain:            "", // No custom domain
		OssDomainSSL:         true,
		ReplaceDomainEnabled: true,
	}

	fs := NewOssFilesystem(conf)

	signedURL := "https://xhjobj.oss-ap-southeast-1.aliyuncs.com/media/test.jpg?Expires=123456"

	result := fs.replaceOSSUrlDomain(signedURL)

	// URL should remain unchanged
	if result != signedURL {
		t.Errorf("URL should remain unchanged without custom domain, got %q", result)
	}
}

// TestOssPresignedUrlReplacementDisabled tests that URL replacement is skipped when disabled
func TestOssPresignedUrlReplacementDisabled(t *testing.T) {
	conf := OssSystemConfig{
		SSL:                  true,
		AccessKeyId:          "LTAI5tPn1vL8BLLAm3R2vs61",
		AccessSecret:         "test-key-secret",
		BucketPublic:         "xhjobj",
		BucketPrivate:        "im-private",
		Endpoint:             "oss-ap-southeast-1.aliyuncs.com",
		OssDomain:            "cdn.example.com",
		OssDomainSSL:         true,
		ReplaceDomainEnabled: false, // Disabled
	}

	fs := NewOssFilesystem(conf)

	signedURL := "https://xhjobj.oss-ap-southeast-1.aliyuncs.com/media/test.jpg?Expires=123456"

	result := fs.replaceOSSUrlDomain(signedURL)

	// URL should remain unchanged when disabled
	if result != signedURL {
		t.Errorf("URL should remain unchanged when replacement disabled, got %q", result)
	}
}
