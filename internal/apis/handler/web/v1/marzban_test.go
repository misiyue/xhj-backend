package v1

import (
	"context"
	"testing"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/stretchr/testify/require"
)

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
