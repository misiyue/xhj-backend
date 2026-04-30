package encrypt

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"math/rand"
)

func Md5(str string) string {
	h := md5.New()
	h.Write([]byte(str))

	return hex.EncodeToString(h.Sum(nil))
}

// GenerateSalt 生成4位随机盐值（与xhj系统一致）
func GenerateSalt() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// HashPassword 使用md5(salt + password + salt)加密密码（与xhj系统一致）
func HashPassword(password, salt string) string {
	return Md5(salt + password + salt)
}

// VerifyPassword 验证加密的文本是否与纯文本相同
func VerifyPassword(hash, password, salt string) bool {
	return hash == Md5(salt+password+salt)
}

func Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Base64Decode(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}
