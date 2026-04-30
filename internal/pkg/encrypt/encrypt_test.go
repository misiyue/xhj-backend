package encrypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase64Decode(t *testing.T) {

}

func TestBase64Encode(t *testing.T) {

}

func TestHashPassword(t *testing.T) {
	salt := "abcd"
	pwd := HashPassword("admin123", salt)

	// md5 produces 32-char hex string
	assert.Equal(t, 32, len(pwd))
	// Verify it matches md5(salt + password + salt)
	assert.Equal(t, Md5(salt+"admin123"+salt), pwd)
}

func TestMd5(t *testing.T) {
	assert.Equal(t, "c069d1fbc7bf8e994de8299110e68bc5", Md5("s6hqzp6j0kdfzh4n_cjq6b180000gn"))
}

func TestVerifyPassword(t *testing.T) {
	salt := GenerateSalt()
	pwd := HashPassword("admin123", salt)

	assert.Equal(t, true, VerifyPassword(pwd, "admin123", salt))
	assert.Equal(t, false, VerifyPassword(pwd, "admin1234453", salt))
}

func TestGenerateSalt(t *testing.T) {
	salt := GenerateSalt()
	assert.Equal(t, 4, len(salt))
}
