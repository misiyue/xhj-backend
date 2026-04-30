package strutil

import (
	"crypto/md5"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// GenImageName 随机生成指定后缀的图片名
func GenImageName(ext string, width, height int) string {
	return fmt.Sprintf("%s_%dx%d.%s", uuid.New().String(), width, height, ext)
}

func GenFileName(ext string) string {
	return fmt.Sprintf("%s.%s", uuid.New().String(), ext)
}

// sanitizeFileName 清理文件名，去除特殊字符，限制长度
// 保留字母、数字、下划线、中划线和 Unicode 字母（支持中文等）
func sanitizeFileName(originalName string, maxLength int) string {
	// 获取不带扩展名的文件名
	lastDotIndex := strings.LastIndex(originalName, ".")
	var baseName, ext string
	if lastDotIndex > 0 {
		baseName = originalName[:lastDotIndex]
		ext = originalName[lastDotIndex:]
	} else {
		baseName = originalName
		ext = ""
	}

	// 清理基础名称：保留字母、数字、下划线、中划线和 Unicode 字母（中文等）
	var result strings.Builder
	for _, r := range baseName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || unicode.IsLetter(r) {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	sanitized := result.String()

	// 移除连续的 "-"
	re := regexp.MustCompile(`-+`)
	sanitized = re.ReplaceAllString(sanitized, "-")

	// 移除前后的 "-"
	sanitized = strings.Trim(sanitized, "-")

	// 限制长度（按字符计算，而非字节）
	fullName := sanitized + ext
	fullNameRunes := []rune(fullName)

	if len(fullNameRunes) > maxLength {
		extRunes := []rune(ext)
		availableLen := maxLength - len(extRunes)

		if availableLen > 0 {
			sanitizedRunes := []rune(sanitized)
			if len(sanitizedRunes) > availableLen {
				sanitizedRunes = sanitizedRunes[:availableLen]
			}
			sanitized = string(sanitizedRunes)
		}
	}

	return sanitized + ext
}

// GenMediaObjectName 生成媒体对象路径
// 新设计：使用哈希值作为uuid路径，原始文件名作为下载文件名
// 参数：originalFileName - 原始文件名（例如 "photo.jpg"）
// width, height - 仅用于图片类型的元数据（将来可能使用）
func GenMediaObjectName(originalFileName string, width, height int) string {
	// 获取文件扩展名
	lastDotIndex := strings.LastIndex(originalFileName, ".")
	var ext string
	if lastDotIndex > 0 {
		ext = strings.ToLower(originalFileName[lastDotIndex+1:])
	} else {
		ext = "bin"
	}

	// 判断媒体类型
	var mediaType = "common"
	switch ext {
	case "png", "jpeg", "jpg", "gif", "webp", "svg", "ico":
		mediaType = "image"
	case "mp3", "wav", "aac", "ogg", "flac":
		mediaType = "audio"
	case "mp4", "avi", "mov", "wmv", "mkv":
		mediaType = "video"
	}

	// 生成哈希值用于路径（基于原始文件名和时间戳，确保不同上传的同名文件也能区分）
	hash := md5.New()
	io.WriteString(hash, originalFileName+time.Now().String())
	hashStr := fmt.Sprintf("%x", hash.Sum(nil))[:8] // 使用前 8 位 MD5 哈希

	// 清理文件名（去除特殊字符，限制长度 < 32）
	cleanedName := sanitizeFileName(originalFileName, 32)

	// 路径格式：media/{mediaType}/{YYYYMM}/{hash}/{cleanedFileName}
	return fmt.Sprintf("media/%s/%s/%s/%s", mediaType, time.Now().Format("200601"), hashStr, cleanedName)
}
