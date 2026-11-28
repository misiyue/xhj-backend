package talk

import (
	"context"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/cache"
)

// clearGroupAtMeMentions 清空用户在某群的 @ 未读，并推送 count=0 给客户端
func clearGroupAtMeMentions(ctx context.Context, mention *cache.MentionStorage, push *logic.PushMessage, uid, groupId int) bool {
	if mention == nil || uid <= 0 || groupId <= 0 {
		return false
	}
	if err := mention.ClearMentions(ctx, uid, groupId); err != nil {
		logger.Errorf("clear mentions err: user_id=%d group_id=%d %s", uid, groupId, err.Error())
		return false
	}
	if push != nil {
		_ = push.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventImMessageMention,
			Payload: jsonutil.Encode(entity.SubEventImMessageMentionPayload{
				UserId:  uid,
				GroupId: groupId,
				MsgIds:  []string{},
				Count:   0,
			}),
		})
	}
	return true
}
