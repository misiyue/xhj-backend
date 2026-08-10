package v1

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/stretchr/testify/require"
)

type marzbanServiceStub struct {
	createdID int
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

func (s *marzbanServiceStub) GetByID(_ context.Context, _ int) (*service.MarzbanUserInfo, error) {
	return nil, service.ErrMarzbanUserNotFound
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

func TestValidateMarzbanOwner(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.AuthClaimsKey{}, entity.WebClaims{UserId: 12})
	require.NoError(t, validateMarzbanOwner(ctx, 12))

	err := validateMarzbanOwner(ctx, 13)
	require.Error(t, err)
	var businessErr *errorx.Error
	require.ErrorAs(t, err, &businessErr)
	require.Equal(t, 403, businessErr.Code)
}

func TestValidateMarzbanOwnerRequiresLogin(t *testing.T) {
	err := validateMarzbanOwner(context.Background(), 12)
	require.Error(t, err)
	var businessErr *errorx.Error
	require.ErrorAs(t, err, &businessErr)
	require.Equal(t, 401, businessErr.Code)
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
