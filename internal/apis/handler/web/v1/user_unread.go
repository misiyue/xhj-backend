package v1

import (
	"context"
	"math"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
)

// UnreadSummary 当前用户各类未读数汇总
func (u *User) UnreadSummary(ctx context.Context, _ *web.UserUnreadSummaryRequest) (*web.UserUnreadSummaryResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)

	talkUnread := 0
	if u.TalkSessionService != nil && u.UnreadStorage != nil {
		sessions, err := u.TalkSessionService.List(ctx, uid)
		if err != nil {
			return nil, err
		}
		for _, item := range sessions {
			if item.TalkMode != entity.ChatPrivateMode && item.TalkMode != entity.ChatGroupMode {
				continue
			}
			talkUnread += u.UnreadStorage.Get(ctx, uid, item.TalkMode, item.ReceiverId)
		}
	}

	merchantUnread := 0
	if u.MerchantSessionRepo != nil && u.UnreadStorage != nil {
		rows, err := u.MerchantSessionRepo.ListVisibleByUser(ctx, uid)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			merchantUnread += u.UnreadStorage.Get(ctx, uid, entity.ChatMerchantMode, rows[i].Id)
		}
	}

	contactApplyUnread := 0
	if u.ContactApplyService != nil {
		contactApplyUnread = u.ContactApplyService.GetApplyUnreadNum(ctx, uid)
	}

	groupApplyUnread := 0
	if u.GroupApplyStorage != nil {
		groupApplyUnread = u.GroupApplyStorage.Get(ctx, uid)
	}

	var noticeUnread int64
	if u.NoticeLetterRepo != nil {
		n, err := u.NoticeLetterRepo.CountUnread(ctx, uid)
		if err != nil {
			return nil, err
		}
		noticeUnread = n
	}

	total := talkUnread + merchantUnread + contactApplyUnread + groupApplyUnread + int(noticeUnread)

	return &web.UserUnreadSummaryResponse{
		TalkUnread:         clampInt32(talkUnread),
		MerchantUnread:     clampInt32(merchantUnread),
		ContactApplyUnread: clampInt32(contactApplyUnread),
		GroupApplyUnread:   clampInt32(groupApplyUnread),
		NoticeUnread:       clampInt32(int(noticeUnread)),
		TotalUnread:        clampInt32(total),
	}, nil
}

func clampInt32(n int) int32 {
	if n <= 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}
