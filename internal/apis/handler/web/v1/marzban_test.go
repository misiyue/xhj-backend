package v1

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/stretchr/testify/require"
)

type marzbanServiceStub struct {
	createdID int
	queriedID int
	checkinID int
}

func (s *marzbanServiceStub) CreateByID(_ context.Context, id int, dataLimit int64, _ int) (*service.MarzbanUserInfo, error) {
	s.createdID = id
	return &service.MarzbanUserInfo{
		ID:        id,
		Username:  "xhj_0",
		DataLimit: dataLimit,
		Created:   true,
	}, nil
}

func (s *marzbanServiceStub) GetByID(_ context.Context, id int) (*service.MarzbanUserInfo, error) {
	s.queriedID = id
	return &service.MarzbanUserInfo{ID: id, Username: "xhj_0"}, nil
}

func (s *marzbanServiceStub) Checkin(_ context.Context, id int) (*service.MarzbanUserInfo, error) {
	s.checkinID = id
	return &service.MarzbanUserInfo{ID: id, Username: "xhj_0"}, nil
}

func (s *marzbanServiceStub) CheckinStatus(_ context.Context, _ int) (bool, string, error) {
	return false, "", nil
}

func TestMarzbanCheckinUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &marzbanServiceStub{}
	handler := &Marzban{MarzbanService: stub}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/v1/marzban/check-in", strings.NewReader(`{"id": 99}`))
	ctx.Request = req.WithContext(context.WithValue(req.Context(), middleware.AuthClaimsKey{}, entity.WebClaims{UserId: 42}))
	response, err := handler.Checkin(ctx)
	require.NoError(t, err)
	require.Equal(t, 42, stub.checkinID)
	require.Equal(t, 42, response.ID)
}

func TestCreateMarzbanUserAllowsZeroIDWithoutLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &marzbanServiceStub{}
	handler := &Marzban{MarzbanService: stub}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/marzban/user", strings.NewReader(`{
		"id": 0,
		"data_limit_gb": 1,
		"expire_days": 30
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	result, err := handler.CreateUser(ctx)

	require.NoError(t, err)
	require.Equal(t, 0, stub.createdID)
	require.Equal(t, 0, result.ID)
}

func TestGetMarzbanUserAllowsZeroIDWithoutLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &marzbanServiceStub{}
	handler := &Marzban{MarzbanService: stub}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "0"}}
	ctx.Request = httptest.NewRequest("GET", "/api/v1/marzban/user/0", nil)

	result, err := handler.GetUser(ctx)

	require.NoError(t, err)
	require.Equal(t, 0, stub.queriedID)
	require.Equal(t, 0, result.ID)
}

func TestDataLimitGBToBytes(t *testing.T) {
	bytes, err := dataLimitGBToBytes(100)
	require.NoError(t, err)
	require.Equal(t, int64(100*(1<<30)), bytes)

	bytes, err = dataLimitGBToBytes(1.5)
	require.NoError(t, err)
	require.Equal(t, int64(1536*(1<<20)), bytes)
}

func TestDataLimitGBToBytesRejectsInvalidValue(t *testing.T) {
	_, err := dataLimitGBToBytes(0)
	require.Error(t, err)
}
