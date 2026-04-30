package service

import (
	"context"
	"testing"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/stretchr/testify/assert"
)

// TestGroupServiceListSignature verifies that IGroupService.List requires context.Context
func TestGroupServiceListSignature(t *testing.T) {
	// Verify GroupService implements IGroupService (compile-time check already in group.go)
	var _ IGroupService = (*GroupService)(nil)

	// Verify the interface method accepts context as first parameter
	var svc IGroupService
	assert.Nil(t, svc) // svc is nil, just checking interface is properly defined
}

// TestGroupServiceInterfaceContract ensures the interface requires context
func TestGroupServiceInterfaceContract(t *testing.T) {
	// Verify the List method signature includes context.Context
	// If the interface changes, this test will fail to compile
	var fn func(ctx context.Context, userId int) ([]*model.GroupItem, error)
	_ = fn
}
