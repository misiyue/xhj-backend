package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMarzbanCheckinAddsSixHoursOncePerDay(t *testing.T) {
	now := time.Now().Unix()
	oldExpire := now + 3600
	putCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/xhj_42":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "status": "active", "expire": oldExpire})
		case r.Method == http.MethodPut && r.URL.Path == "/api/user/xhj_42":
			putCalls++
			var body map[string]int64
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, oldExpire+6*3600, body["expire"])
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "status": "active", "expire": body["expire"]})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open("file:marzban_checkin_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MarzbanCheckin{}))
	svc := newTestMarzbanService(server.URL)
	svc.DB = db
	user, err := svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, oldExpire+6*3600, user.Expire)
	signed, nextReset, err := svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, signed)
	require.NotEmpty(t, nextReset)
	_, err = svc.Checkin(context.Background(), 42)
	require.ErrorIs(t, err, ErrMarzbanAlreadyCheckedIn)
	require.Equal(t, 1, putCalls)
}

func TestMarzbanCheckinFailureDoesNotConsumeReward(t *testing.T) {
	putCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet:
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_55", "status": "expired", "expire": time.Now().Add(-time.Hour).Unix()})
		case r.Method == http.MethodPut:
			putCalls++
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open("file:marzban_checkin_failure_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MarzbanCheckin{}))
	svc := newTestMarzbanService(server.URL)
	svc.DB = db
	_, err = svc.Checkin(context.Background(), 55)
	require.Error(t, err)
	signed, _, err := svc.CheckinStatus(context.Background(), 55)
	require.NoError(t, err)
	require.False(t, signed)
	_, err = svc.Checkin(context.Background(), 55)
	require.Error(t, err)
	require.Equal(t, 2, putCalls)
}

func TestMarzbanCreateByID(t *testing.T) {
	const (
		userID    = 42
		dataLimit = int64(100 * (1 << 30))
		used      = int64(10 * (1 << 30))
	)
	expire := time.Now().Add(30 * 24 * time.Hour).Unix()
	getCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/admin/token":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "admin", r.Form.Get("username"))
			require.Equal(t, "secret", r.Form.Get("password"))
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/xhj_42":
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			getCalls++
			writeMarzbanJSON(t, w, http.StatusNotFound, map[string]any{"detail": "not found"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/user":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "xhj_42", body["username"])
			require.Equal(t, float64(dataLimit), body["data_limit"])
			require.Equal(t, "active", body["status"])
			proxies, ok := body["proxies"].(map[string]any)
			require.True(t, ok)
			vless, ok := proxies["vless"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "xtls-rprx-vision", vless["flow"])
			inbounds, ok := body["inbounds"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, []any{"VLESS TCP REALITY TEST"}, inbounds["vless"])
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{
				"username": "xhj_42", "status": "active", "data_limit": dataLimit,
				"used_traffic": used, "expire": expire,
				"subscription_url": "https://example.com/sub/token",
			})
		default:
			t.Fatalf("收到未预期的请求：%s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := newTestMarzbanService(server.URL).CreateByID(context.Background(), userID, dataLimit, 30)
	require.NoError(t, err)
	require.Equal(t, 1, getCalls)
	require.True(t, result.Created)
	require.Equal(t, dataLimit-used, result.RemainingTraffic)
	require.Equal(t, expire, result.Expire)
	require.NotEmpty(t, result.ExpireAt)
}

func TestMarzbanCreateByIDReturnsExistingUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case "/api/user/xhj_7":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{
				"username": "xhj_7", "status": "active", "data_limit": 1000,
				"used_traffic": 300, "expire": 0,
			})
		default:
			t.Fatalf("收到未预期的请求：%s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := newTestMarzbanService(server.URL).CreateByID(context.Background(), 7, 5000, 10)
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Equal(t, int64(700), result.RemainingTraffic)
	require.True(t, result.UnlimitedExpire)
}

func TestMarzbanGetByIDUnlimitedTraffic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/token" {
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
			return
		}
		writeMarzbanJSON(t, w, http.StatusOK, map[string]any{
			"username": "xhj_9", "status": "active", "data_limit": 0,
			"used_traffic": 1234, "expire": 0,
		})
	}))
	defer server.Close()

	result, err := newTestMarzbanService(server.URL).GetByID(context.Background(), 9)
	require.NoError(t, err)
	require.True(t, result.UnlimitedTraffic)
	require.Equal(t, int64(0), result.RemainingTraffic)
}

func TestMarzbanCreateByIDRequiresPlan(t *testing.T) {
	svc := newTestMarzbanService("http://127.0.0.1")
	_, err := svc.CreateByID(context.Background(), 9, 0, 30)
	require.EqualError(t, err, "流量额度必须大于0")
	_, err = svc.CreateByID(context.Background(), 9, 1024, 0)
	require.EqualError(t, err, "有效天数必须大于0")
}

func TestMarzbanUsernameAllowsZeroID(t *testing.T) {
	username, err := newTestMarzbanService("http://127.0.0.1").username(0)
	require.NoError(t, err)
	require.Equal(t, "xhj_0", username)
}

func newTestMarzbanService(baseURL string) *MarzbanService {
	return &MarzbanService{
		Config: &config.Config{Marzban: &config.Marzban{
			BaseURL: baseURL, AdminUsername: "admin", AdminPassword: "secret",
			UserPrefix: "xhj_", DefaultProtocols: []string{"vless"},
			DefaultProxies: map[string]map[string]any{
				"vless": {"flow": "xtls-rprx-vision"},
			},
			DefaultInbounds: map[string][]string{
				"vless": {"VLESS TCP REALITY TEST"},
			},
		}},
		HTTPClient: &http.Client{Timeout: time.Second},
	}
}

func writeMarzbanJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(value))
}
