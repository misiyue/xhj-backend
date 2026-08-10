package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gzydong/go-chat/config"
)

const marzbanTokenTTL = 10 * time.Minute

var ErrMarzbanUserNotFound = errors.New("Marzban 用户不存在")
var marzbanUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

type IMarzbanService interface {
	CreateByID(ctx context.Context, id int, dataLimit int64, expireDays int) (*MarzbanUserInfo, error)
	GetByID(ctx context.Context, id int) (*MarzbanUserInfo, error)
}

type MarzbanService struct {
	Config     *config.Config
	HTTPClient *http.Client

	tokenMu        sync.Mutex `wire:"-"`
	accessToken    string     `wire:"-"`
	tokenExpiresAt time.Time  `wire:"-"`
}

type MarzbanUserInfo struct {
	ID               int
	Username         string
	Status           string
	DataLimit        int64
	UsedTraffic      int64
	RemainingTraffic int64
	UnlimitedTraffic bool
	Expire           int64
	ExpireAt         string
	UnlimitedExpire  bool
	SubscriptionURL  string
	Created          bool
}

type marzbanTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type marzbanUserResponse struct {
	Username        string `json:"username"`
	Status          string `json:"status"`
	DataLimit       int64  `json:"data_limit"`
	UsedTraffic     int64  `json:"used_traffic"`
	Expire          int64  `json:"expire"`
	SubscriptionURL string `json:"subscription_url"`
}

func (s *MarzbanService) CreateByID(ctx context.Context, id int, dataLimit int64, expireDays int) (*MarzbanUserInfo, error) {
	if id <= 0 {
		return nil, errors.New("用户ID无效")
	}
	if dataLimit <= 0 {
		return nil, errors.New("流量额度必须大于0")
	}
	if expireDays <= 0 {
		return nil, errors.New("有效天数必须大于0")
	}

	username, err := s.username(id)
	if err != nil {
		return nil, err
	}

	existing, status, err := s.getUser(ctx, username)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK {
		return s.toUserInfo(id, existing, false), nil
	}
	if status != http.StatusNotFound {
		return nil, fmt.Errorf("查询 Marzban 用户失败，状态码：%d", status)
	}

	conf, err := s.marzbanConfig()
	if err != nil {
		return nil, err
	}
	protocols := conf.DefaultProtocols
	if len(protocols) == 0 {
		protocols = []string{"vless"}
	}
	proxies := make(map[string]map[string]any, len(protocols))
	for _, protocol := range protocols {
		protocol = strings.TrimSpace(protocol)
		if protocol != "" {
			proxies[protocol] = map[string]any{}
		}
	}
	if len(proxies) == 0 {
		return nil, errors.New("Marzban 默认协议配置不能为空")
	}

	payload := map[string]any{
		"username":                  username,
		"status":                    "active",
		"data_limit":                dataLimit,
		"expire":                    time.Now().AddDate(0, 0, expireDays).Unix(),
		"data_limit_reset_strategy": "no_reset",
		"proxies":                   proxies,
		"note":                      fmt.Sprintf("系统用户ID：%d", id),
	}
	if len(conf.DefaultInbounds) > 0 {
		payload["inbounds"] = conf.DefaultInbounds
	}

	var created marzbanUserResponse
	status, err = s.request(ctx, http.MethodPost, "/api/user", payload, &created, false)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict {
		existing, status, err = s.getUser(ctx, username)
		if err != nil {
			return nil, err
		}
		if status == http.StatusOK {
			return s.toUserInfo(id, existing, false), nil
		}
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, fmt.Errorf("创建 Marzban 用户失败，状态码：%d", status)
	}

	return s.toUserInfo(id, &created, true), nil
}

func (s *MarzbanService) GetByID(ctx context.Context, id int) (*MarzbanUserInfo, error) {
	if id <= 0 {
		return nil, errors.New("用户ID无效")
	}
	username, err := s.username(id)
	if err != nil {
		return nil, err
	}
	user, status, err := s.getUser(ctx, username)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrMarzbanUserNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("查询 Marzban 用户失败，状态码：%d", status)
	}
	return s.toUserInfo(id, user, false), nil
}

func (s *MarzbanService) getUser(ctx context.Context, username string) (*marzbanUserResponse, int, error) {
	var user marzbanUserResponse
	status, err := s.request(ctx, http.MethodGet, "/api/user/"+url.PathEscape(username), nil, &user, false)
	return &user, status, err
}

func (s *MarzbanService) request(ctx context.Context, method, path string, body, out any, retried bool) (int, error) {
	conf, err := s.marzbanConfig()
	if err != nil {
		return 0, err
	}
	token, err := s.token(ctx, retried)
	if err != nil {
		return 0, err
	}

	var reader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return 0, fmt.Errorf("序列化 Marzban 请求失败：%w", marshalErr)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(conf.BaseURL, "/")+path, reader)
	if err != nil {
		return 0, fmt.Errorf("创建 Marzban 请求失败：%w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求 Marzban 服务失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && !retried {
		return s.request(ctx, method, path, body, out, true)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, fmt.Errorf("读取 Marzban 响应失败：%w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return 0, fmt.Errorf("解析 Marzban 响应失败：%w", err)
		}
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusConflict {
		return resp.StatusCode, fmt.Errorf("Marzban 接口请求失败，状态码：%d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (s *MarzbanService) token(ctx context.Context, forceRefresh bool) (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if !forceRefresh && s.accessToken != "" && time.Now().Before(s.tokenExpiresAt) {
		return s.accessToken, nil
	}
	conf, err := s.marzbanConfig()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("username", conf.AdminUsername)
	form.Set("password", conf.AdminPassword)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(conf.BaseURL, "/")+"/api/admin/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建 Marzban 登录请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("登录 Marzban 失败：%w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取 Marzban 登录响应失败：%w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Marzban 管理员认证失败，状态码：%d", resp.StatusCode)
	}
	var result marzbanTokenResponse
	if err := json.Unmarshal(data, &result); err != nil || result.AccessToken == "" {
		return "", errors.New("Marzban 登录响应中缺少访问令牌")
	}
	s.accessToken = result.AccessToken
	s.tokenExpiresAt = time.Now().Add(marzbanTokenTTL)
	return s.accessToken, nil
}

func (s *MarzbanService) username(id int) (string, error) {
	conf, err := s.marzbanConfig()
	if err != nil {
		return "", err
	}
	prefix := strings.TrimSpace(conf.UserPrefix)
	if prefix == "" {
		prefix = "xhj_"
	}
	username := prefix + strconv.Itoa(id)
	if len(username) < 3 || len(username) > 32 {
		return "", errors.New("根据用户ID生成的 Marzban 用户名长度必须在3到32个字符之间")
	}
	if !marzbanUsernamePattern.MatchString(username) {
		return "", errors.New("Marzban 用户名前缀只能包含字母、数字、下划线、横线、点和@符号")
	}
	return username, nil
}

func (s *MarzbanService) marzbanConfig() (*config.Marzban, error) {
	if s.Config == nil || s.Config.Marzban == nil {
		return nil, errors.New("Marzban 服务未配置")
	}
	conf := s.Config.Marzban
	if strings.TrimSpace(conf.BaseURL) == "" || strings.TrimSpace(conf.AdminUsername) == "" || conf.AdminPassword == "" {
		return nil, errors.New("Marzban 地址或管理员账号配置不完整")
	}
	return conf, nil
}

func (s *MarzbanService) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *MarzbanService) toUserInfo(id int, user *marzbanUserResponse, created bool) *MarzbanUserInfo {
	remaining := user.DataLimit - user.UsedTraffic
	if remaining < 0 {
		remaining = 0
	}
	expireAt := ""
	if user.Expire > 0 {
		expireAt = time.Unix(user.Expire, 0).Format(time.RFC3339)
	}
	return &MarzbanUserInfo{
		ID:               id,
		Username:         user.Username,
		Status:           user.Status,
		DataLimit:        user.DataLimit,
		UsedTraffic:      user.UsedTraffic,
		RemainingTraffic: remaining,
		UnlimitedTraffic: user.DataLimit == 0,
		Expire:           user.Expire,
		ExpireAt:         expireAt,
		UnlimitedExpire:  user.Expire == 0,
		SubscriptionURL:  user.SubscriptionURL,
		Created:          created,
	}
}
