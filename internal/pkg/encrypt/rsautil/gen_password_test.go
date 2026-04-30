package rsautil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestGenLoginPassword(t *testing.T) {
	// 1. 明文密码（你想用来登录的密码）
	plain := "123456"

	// 2. 把 config.yaml 里的 app.public_key 粘进来（只需要公钥即可生成登录用的密文）
	// 注意：不要带行首缩进空格，否则 pem.Decode 会失败。
	publicKeyPem := normalizePEM(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApSUDGukqMaT8KmG1u85+
LeZin0BACqie6GWg6wDHh+PxFESkBvvJISqTRtQ0C+jcBQMV3SKbFtdirJPtmRyB
sqJxNLqk1oimtGwVy4y/vJFFbQerKcliJG2AhrXywQdM9by7yZFEWAex8iH0LLPA
UwLzQ8byRMFevaIXu4zMsGj1nwGTjsPa+Mz0UHNrvBUwP1Zy+L4tRwQBAQfleIBh
2yNtwrDFqwAHHyp+bZv8rg0olTuG4ai/HEg6RCC1oSxJa7Da1myNwB7mMmfuGyC+
iEjI/xhIsbGfGnK9II905sGTGMDFykWQiqUy7X0nyLDC2DTKwv0ETHxX9mPpcIDI
nQIDAQAB
-----END PUBLIC KEY-----`)

	pub, err := parsePublicKey([]byte(publicKeyPem))
	if err != nil {
		t.Fatalf("parse public key error: %v", err)
	}

	cipherBytes, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	cipher := base64.StdEncoding.EncodeToString(cipherBytes)

	fmt.Println("加密后的密码（用于 Postman 的 password 字段）:")
	fmt.Println(cipher)
}

func normalizePEM(pemStr string) string {
	lines := strings.Split(pemStr, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}
