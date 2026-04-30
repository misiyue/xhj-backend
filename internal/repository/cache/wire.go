package cache

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewCaptchaStorage,
	NewContactRemark,
	NewRedisLock,
	NewMessageStorage,
	NewRelation,
	NewJwtTokenStorage,
	NewSidStorage,
	NewSmsStorage,
	NewEmailStorage,
	NewVote,
	NewUnreadStorage,
	NewGroupApplyStorage,
	NewUserClient,
	NewMentionStorage,
)
