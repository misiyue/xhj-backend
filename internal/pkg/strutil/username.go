package strutil

import (
	"fmt"
	"strings"
	"time"
)

const (
	// UsernamePrefix 自动生成用户名前缀
	UsernamePrefix = "xhj"
	// usernameBodyLen 后缀总长度（字母 + -_+=）
	usernameBodyLen = 32
	// usernameIDDigits 用于双射编码 user_id 的位数；56^11 < 2^64 < 56^12，故取 11
	// 不同 user_id → 这 11 位一定不同 → 整串 username 唯一
	usernameIDDigits = 11
)

// usernameAlphabet 可用字符：大小写字母与 -_+=（共 56 个）
var usernameAlphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_+=")

// usernameMod11 = 56^11，可放入 uint64，用于对 user_id 做可逆仿射变换
var usernameMod11 = func() uint64 {
	m := uint64(1)
	base := uint64(len(usernameAlphabet))
	for i := 0; i < usernameIDDigits; i++ {
		m *= base
	}
	return m
}()

// EncodeUserIDUsername 由自增 user_id 生成唯一登录名：xhj + 32 位。
// 唯一性：前 usernameIDDigits 位是 user_id 在 56 进制下的双射编码，不同 id 必不同。
func EncodeUserIDUsername(userID int) string {
	return EncodeUserIDUsernameWithSalt(userID, 0)
}

// EncodeUserIDUsernameWithSalt 生成 username。
// salt 只影响外观填充位，不改变 id 双射段；用于与历史脏数据冲突时换皮重试。
func EncodeUserIDUsernameWithSalt(userID, salt int) string {
	if userID < 0 {
		userID = -userID
	}
	if salt < 0 {
		salt = -salt
	}

	base := uint64(len(usernameAlphabet))
	mod := usernameMod11

	// 仿射变换 x = (id*mul+add) mod 56^11
	// mul 与 mod 互素（mod=2^33*7^11，mul 取奇数且非 7 倍数）⇒ 对 id 可逆 ⇒ 不同 id 不撞车
	const mul uint64 = 7919
	const add uint64 = 104729
	x := (uint64(userID)%mod*mul + add) % mod

	buf := make([]byte, usernameBodyLen)

	// 1) 双射段：把 x 写成 11 位 base56（高位在前）
	for i := usernameIDDigits - 1; i >= 0; i-- {
		buf[i] = usernameAlphabet[x%base]
		x /= base
	}

	// 2) 填充段：由 user_id + salt 派生，只影响观感；salt 变化可避开历史同名
	stream := uint64(userID)*11400714819323198485 + uint64(salt)*14029467366897019727 + 0x9E3779B97F4A7C15
	for i := usernameIDDigits; i < usernameBodyLen; i++ {
		stream = stream*6364136223846793005 + 1
		buf[i] = usernameAlphabet[stream%base]
	}

	// 3) 固定位置置换，打散「前段像流水号」的观感；置换可逆，不破坏唯一性
	return UsernamePrefix + string(permuteUsernameBody(buf))
}

// permuteUsernameBody 固定置换 32 个位置（与 id 无关），使外观更均匀
func permuteUsernameBody(in []byte) []byte {
	// 质数步长置换：out[i] = in[(i*prime)%n]，prime 与 n=32 互素
	const prime = 7
	n := len(in)
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = in[(i*prime)%n]
	}
	return out
}

// IsAutoUsername 是否为系统自动生成的 xhj 用户名格式
func IsAutoUsername(username string) bool {
	if !strings.HasPrefix(username, UsernamePrefix) {
		return false
	}
	body := username[len(UsernamePrefix):]
	if len(body) != usernameBodyLen {
		return false
	}
	for i := 0; i < len(body); i++ {
		if !strings.ContainsRune(string(usernameAlphabet), rune(body[i])) {
			return false
		}
	}
	return true
}

// TempRegisterUsername 注册落库前的临时占位（插入拿到自增 id 后再替换为正式 username）
func TempRegisterUsername() string {
	return fmt.Sprintf("_reg_%d", time.Now().UnixNano())
}
