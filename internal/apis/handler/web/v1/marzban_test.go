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
