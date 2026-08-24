package v1

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

const yunxinDefaultServerAPIBaseURL = "https://api.netease.im/nimserver"

type Yunxin struct {
	Config     *config.Config
	UsersRepo  *repo.Users
	TokenCache *cache.YunxinTokenStorage
	HTTPClient *http.Client
}

func NewYunxin(conf *config.Config, usersRepo *repo.Users, tokenCache *cache.YunxinTokenStorage) *Yunxin {
	return &Yunxin{
		Config:     conf,
		UsersRepo:  usersRepo,
		TokenCache: tokenCache,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type YunxinCredentialsResponse struct {
	AppKey string `json:"app_key"`
	Accid  string `json:"accid"`
	Token  string `json:"token"`
}

// Credentials returns only the authenticated user's Yunxin credentials.  The
// account is provisioned lazily so deploying CallKit never needs a bulk user
// migration. Token is cached in Redis for 2 hours.
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

	accid, token, err := h.ensureToken(ctx.Request.Context(), user)
	if err != nil {
		return nil, err
	}

	return &YunxinCredentialsResponse{
		AppKey: h.Config.Yunxin.AppKey,
		Accid:  accid,
		Token:  token,
	}, nil
}

func (h *Yunxin) validateConfig() error {
	if h == nil || h.Config == nil || h.Config.App == nil || h.Config.Yunxin == nil || !h.Config.Yunxin.Enabled {
		return errorx.New(503, "云信通话未启用")
	}
	if strings.TrimSpace(h.Config.Yunxin.AppKey) == "" || strings.TrimSpace(h.Config.Yunxin.AppSecret) == "" {
		return errorx.New(500, "云信通话配置不完整")
	}
	return nil
}

func (h *Yunxin) ensureToken(ctx context.Context, user *model.Users) (accid, token string, err error) {
	accid = strconv.Itoa(user.Id)

	if h.TokenCache != nil {
		token, err = h.TokenCache.Get(ctx, user.Id)
		if err != nil {
			return "", "", errorx.New(500, "读取云信凭证缓存失败")
		}
		if token != "" {
			_ = h.updateProfile(ctx, accid, user.Nickname, user.Avatar)
			return accid, token, nil
		}
	}

	token, err = newYunxinToken()
	if err != nil {
		return "", "", errorx.New(500, "生成云信凭证失败")
	}
	if err := h.createOrRepairAccount(ctx, accid, token, user.Nickname, user.Avatar); err != nil {
		return "", "", err
	}
	if h.TokenCache != nil {
		if err := h.TokenCache.Set(ctx, user.Id, token); err != nil {
			return "", "", errorx.New(500, "写入云信凭证缓存失败")
		}
	}

	return accid, token, nil
}

// EnsureUserAccount 确保用户已在云信开户且 Redis 中有 token；缓存命中则仅 best-effort 同步资料。
func (h *Yunxin) EnsureUserAccount(ctx context.Context, userID int) error {
	if h == nil || h.Config == nil || h.Config.Yunxin == nil || !h.Config.Yunxin.Enabled {
		return nil
	}
	if err := h.validateConfig(); err != nil {
		return err
	}
	user, err := h.UsersRepo.FindByIdWithCache(ctx, userID)
	if err != nil {
		return errorx.New(500, "读取用户信息失败")
	}
	if user == nil || user.IsUnavailable() {
		return errorx.New(403, "对方账号不可用于通话")
	}
	_, _, err = h.ensureToken(ctx, user)
	return err
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
