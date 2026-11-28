package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMarzbanCheckinAllowsRewardAfterSixHours(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	firstCheckin := now
	oldExpire := now.Add(24 * time.Hour).Unix()
	currentExpire := oldExpire
	putCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/xhj_42":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "status": "active", "expire": currentExpire})
		case r.Method == http.MethodPut && r.URL.Path == "/api/user/xhj_42":
			putCalls++
			var body map[string]int64
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, currentExpire+6*3600, body["expire"])
			currentExpire = body["expire"]
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "status": "active", "expire": body["expire"]})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	svc := newTestMarzbanService(server.URL)
	svc.DB = newTestMarzbanCheckinDB(t)
	svc.checkinClock = func() time.Time { return now }
	user, err := svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, oldExpire+6*3600, user.Expire)
	signed, nextReset, err := svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, signed)
	require.Equal(t, "2026-10-12T16:00:00+08:00", nextReset)
	_, err = svc.Checkin(context.Background(), 42)
	require.ErrorIs(t, err, ErrMarzbanAlreadyCheckedIn)
	now = firstCheckin.Add(6*time.Hour - time.Second)
	_, err = svc.Checkin(context.Background(), 42)
	require.ErrorIs(t, err, ErrMarzbanAlreadyCheckedIn)
	require.Equal(t, 1, putCalls)
	now = firstCheckin.Add(6 * time.Hour)
	signed, _, err = svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, signed)
	user, err = svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, oldExpire+12*3600, user.Expire)
	require.Equal(t, 2, putCalls)
	signed, nextReset, err = svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, signed)
	require.Equal(t, "2026-10-12T22:00:00+08:00", nextReset)
	var records []model.MarzbanCheckin
	require.NoError(t, svc.DB.Find(&records).Error)
	require.Len(t, records, 1)
	require.Equal(t, now.Unix(), records[0].CreatedAt.Unix())
	require.Equal(t, user.Expire, records[0].Expire)
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
	svc := newTestMarzbanService(server.URL)
	svc.DB = newTestMarzbanCheckinDB(t)
	_, err := svc.Checkin(context.Background(), 55)
	require.Error(t, err)
	signed, nextReset, err := svc.CheckinStatus(context.Background(), 55)
	require.NoError(t, err)
	require.False(t, signed)
	require.Empty(t, nextReset)
	_, err = svc.Checkin(context.Background(), 55)
	require.Error(t, err)
	require.Equal(t, 2, putCalls)
	var count int64
	require.NoError(t, svc.DB.Model(&model.MarzbanCheckinState{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, svc.DB.Model(&model.MarzbanCheckin{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestMarzbanCheckinLegacyCooldownCrossesMidnight(t *testing.T) {
	lastCheckin := time.Date(2026, 10, 12, 23, 59, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	now := lastCheckin.Add(2 * time.Minute)
	putCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet:
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "expire": lastCheckin.Unix()})
		case r.Method == http.MethodPut:
			putCalls++
			var body map[string]int64
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, now.Add(6*time.Hour).Unix(), body["expire"])
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "expire": body["expire"]})
		}
	}))
	defer server.Close()
	svc := newTestMarzbanService(server.URL)
	svc.DB = newTestMarzbanCheckinDB(t)
	svc.checkinClock = func() time.Time { return now }
	// These records predate the new cooldown table and must remain effective.
	require.NoError(t, svc.DB.Create(&model.MarzbanCheckin{
		UserID: 42, CheckinDate: "2026-10-12", CreatedAt: lastCheckin,
	}).Error)
	require.NoError(t, svc.DB.Create(&model.MarzbanCheckin{
		UserID: 42, CheckinDate: "2026-10-11", CreatedAt: lastCheckin.Add(-24 * time.Hour),
	}).Error)
	signed, nextReset, err := svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, signed)
	require.Equal(t, "2026-10-13T05:59:00+08:00", nextReset)
	_, err = svc.Checkin(context.Background(), 42)
	require.ErrorIs(t, err, ErrMarzbanAlreadyCheckedIn)
	require.Zero(t, putCalls)
	now = lastCheckin.Add(6 * time.Hour)
	_, err = svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, putCalls)
	signed, nextReset, err = svc.CheckinStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, signed)
	require.Equal(t, "2026-10-13T11:59:00+08:00", nextReset)
}

func TestMarzbanCheckinConcurrentRequestsRewardOnce(t *testing.T) {
	now := time.Now()
	putCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/admin/token":
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"access_token": "token"})
		case r.Method == http.MethodGet:
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "expire": now.Add(time.Hour).Unix()})
		case r.Method == http.MethodPut:
			putCalls++
			var body map[string]int64
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			writeMarzbanJSON(t, w, http.StatusOK, map[string]any{"username": "xhj_42", "expire": body["expire"]})
		}
	}))
	defer server.Close()
	svc := newTestMarzbanService(server.URL)
	svc.DB = newTestMarzbanCheckinDB(t)
	svc.checkinClock = func() time.Time { return now }
	const requests = 8
	results := make(chan error, requests)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < requests; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := svc.Checkin(context.Background(), 42)
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrMarzbanAlreadyCheckedIn)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, putCalls)
}

func newTestMarzbanCheckinDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	// SQLite has no row locks; one connection serializes these test transactions.
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&model.MarzbanCheckin{}, &model.MarzbanCheckinState{}))
	return db
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
