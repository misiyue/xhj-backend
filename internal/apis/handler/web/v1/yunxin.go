package v1

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

const yunxinDefaultServerAPIBaseURL = "https://api.netease.im/nimserver"

type Yunxin struct {
	Config         *config.Config
	UsersRepo      *repo.Users
	CredentialRepo *repo.YunxinCredential
	HTTPClient     *http.Client
}

func NewYunxin(conf *config.Config, usersRepo *repo.Users, credentialRepo *repo.YunxinCredential) *Yunxin {
	return &Yunxin{
		Config:         conf,
		UsersRepo:      usersRepo,
		CredentialRepo: credentialRepo,
		HTTPClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

type YunxinCredentialsResponse struct {
	AppKey string `json:"app_key"`
	Accid  string `json:"accid"`
	Token  string `json:"token"`
}

// Credentials returns only the authenticated user's Yunxin credentials.  The
// account is provisioned lazily so deploying CallKit never needs a bulk user
// migration.
//
// @Summary      获取云信通话凭证
// @Description  懒创建当前用户的云信 IM 账号并返回 CallKit 登录凭证
// @Tags         Yunxin
// @Produce      json
// @Success      200 {object} YunxinCredentialsResponse
// @Router       /api/v1/yunxin/credentials [get]
func (h *Yunxin) Credentials(ctx *gin.Context) (any, error) {
	session, err := middleware.FormContext[entity.WebClaims](ctx.Request.Context())
	if err != nil || session.UserId <= 0 {
		return nil, errorx.New(401, "未登录")
	}
	if err := h.validateConfig(); err != nil {
		return nil, err
	}

	user, err := h.UsersRepo.FindByIdWithCache(ctx.Request.Context(), int(session.UserId))
	if err != nil || user == nil || user.IsUnavailable() {
		return nil, errorx.New(403, "当前账号不可用于通话")
	}

	credential, err := h.ensureCredential(ctx.Request.Context(), user)
	if err != nil {
		return nil, err
	}
	token, err := decryptYunxinToken(h.Config.App.AesKey, credential.TokenCiphertext)
	if err != nil {
		return nil, errorx.New(500, "云信凭证解密失败")
	}

	return &YunxinCredentialsResponse{
		AppKey: h.Config.Yunxin.AppKey,
		Accid:  credential.Accid,
		Token:  token,
	}, nil
}

func (h *Yunxin) validateConfig() error {
	if h == nil || h.Config == nil || h.Config.App == nil || h.Config.Yunxin == nil || !h.Config.Yunxin.Enabled {
		return errorx.New(503, "云信通话未启用")
	}
	if strings.TrimSpace(h.Config.Yunxin.AppKey) == "" || strings.TrimSpace(h.Config.Yunxin.AppSecret) == "" || strings.TrimSpace(h.Config.App.AesKey) == "" {
		return errorx.New(500, "云信通话配置不完整")
	}
	return nil
}

func (h *Yunxin) ensureCredential(ctx context.Context, user *model.Users) (*model.YunxinCredential, error) {
	credential, err := h.CredentialRepo.FindByUserId(ctx, user.Id)
	if err != nil {
		return nil, errorx.New(500, "读取云信凭证失败")
	}
	if credential != nil {
		// Profile updates are best-effort: a transient provider failure must not
		// sign an otherwise valid user out of CallKit.
		_ = h.updateProfile(ctx, credential.Accid, user.Nickname, user.Avatar)
		return credential, nil
	}

	accid := strconv.Itoa(user.Id)
	token, err := newYunxinToken()
	if err != nil {
		return nil, errorx.New(500, "生成云信凭证失败")
	}
	if err := h.createOrRepairAccount(ctx, accid, token, user.Nickname, user.Avatar); err != nil {
		return nil, err
	}
	ciphertext, err := encryptYunxinToken(h.Config.App.AesKey, token)
	if err != nil {
		return nil, errorx.New(500, "加密云信凭证失败")
	}
	credential = &model.YunxinCredential{
		UserId:          user.Id,
		Accid:           accid,
		TokenCiphertext: ciphertext,
	}
	if err := h.CredentialRepo.Create(ctx, credential); err != nil {
		// A concurrent first login may have completed the same work. Re-read the
		// winner instead of issuing a different credential.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return h.CredentialRepo.FindByUserId(ctx, user.Id)
		}
		return nil, errorx.New(500, "保存云信凭证失败")
	}
	return credential, nil
}

func (h *Yunxin) createOrRepairAccount(ctx context.Context, accid, token, nickname, avatar string) error {
	code, err := h.callAPI(ctx, "/user/create.action", url.Values{
		"accid": {accid}, "token": {token}, "name": {nickname}, "icon": {avatar},
	})
	if err != nil {
		return errorx.New(502, "创建云信账号失败")
	}
	if code == 200 {
		return nil
	}
	// 414 is Yunxin's "account already exists" response. This only occurs
	// after an interrupted deployment/database recovery; reset its token once
	// and preserve the local ownership mapping.
	if code != 414 {
		return errorx.New(502, "创建云信账号失败")
	}
	code, err = h.callAPI(ctx, "/user/update.action", url.Values{
		"accid": {accid}, "token": {token}, "name": {nickname}, "icon": {avatar},
	})
	if err != nil || code != 200 {
		return errorx.New(502, "修复云信账号失败")
	}
	return nil
}

func (h *Yunxin) updateProfile(ctx context.Context, accid, nickname, avatar string) error {
	code, err := h.callAPI(ctx, "/user/updateUinfo.action", url.Values{
		"accid": {accid}, "name": {nickname}, "icon": {avatar},
	})
	if err != nil || code != 200 {
		return fmt.Errorf("yunxin update profile failed")
	}
	return nil
}

func (h *Yunxin) callAPI(ctx context.Context, path string, form url.Values) (int, error) {
	nonce, err := newYunxinToken()
	if err != nil {
		return 0, err
	}
	currentTime := strconv.FormatInt(time.Now().Unix(), 10)
	checksum := sha1.Sum([]byte(h.Config.Yunxin.AppSecret + nonce + currentTime))
	baseURL := strings.TrimRight(h.Config.Yunxin.ServerAPIBaseURL, "/")
	if baseURL == "" {
		baseURL = yunxinDefaultServerAPIBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("AppKey", h.Config.Yunxin.AppKey)
	req.Header.Set("Nonce", nonce)
	req.Header.Set("CurTime", currentTime)
	req.Header.Set("CheckSum", hex.EncodeToString(checksum[:]))

	client := h.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("yunxin api status=%d", resp.StatusCode)
	}
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	return result.Code, nil
}

func newYunxinToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func encryptYunxinToken(keySource, token string) (string, error) {
	key := sha256.Sum256([]byte(keySource))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, []byte(token), nil)...)), nil
}

func decryptYunxinToken(keySource, ciphertext string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(keySource))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
