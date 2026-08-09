package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gzydong/go-chat/config"
	"github.com/stretchr/testify/require"
)

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
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/xhj_42":
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			getCalls++
			writeJSON(t, w, http.StatusNotFound, map[string]any{"detail": "not found"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/user":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "xhj_42", body["username"])
			require.Equal(t, float64(dataLimit), body["data_limit"])
			require.Equal(t, "active", body["status"])
			writeJSON(t, w, http.StatusOK, map[string]any{
				"username":         "xhj_42",
				"status":           "active",
				"data_limit":       dataLimit,
				"used_traffic":     used,
				"expire":           expire,
				"subscription_url": "https://example.com/sub/token",
			})
		default:
			t.Fatalf("收到未预期的请求：%s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	svc := newTestMarzbanService(server.URL)
	result, err := svc.CreateByID(context.Background(), userID, dataLimit, 30)

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
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case "/api/user/xhj_7":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"username":     "xhj_7",
				"status":       "active",
				"data_limit":   1000,
				"used_traffic": 300,
				"expire":       0,
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
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"username":     "xhj_9",
			"status":       "active",
			"data_limit":   0,
			"used_traffic": 1234,
			"expire":       0,
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

func newTestMarzbanService(baseURL string) *MarzbanService {
	return &MarzbanService{
		Config: &config.Config{Marzban: &config.Marzban{
			BaseURL:          baseURL,
			AdminUsername:    "admin",
			AdminPassword:    "secret",
			UserPrefix:       "xhj_",
			DefaultProtocols: []string{"vless"},
		}},
		HTTPClient: &http.Client{Timeout: time.Second},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(value))
}
