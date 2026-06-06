package v1

import (
	"context"
	"math"

	pb "github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

const noticePageSize = 15

type Notice struct {
	SysNoticeRepo *repo.SysNotice
}

// ListNotice 系统通知列表，按时间倒序，每页 15 条
func (n *Notice) ListNotice(ctx context.Context, req *pb.NoticeListRequest) (*pb.NoticeListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}

	rows, total, err := n.SysNoticeRepo.ListByUserDesc(ctx, int(session.UserId), page, noticePageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.NoticeItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &pb.NoticeItem{
			Id:        int32(row.Id),
			Title:     row.Title,
			Content:   row.Content,
			Url:       row.Url,
			IsRead:    int32(row.IsRead),
			CreatedAt: timeutil.FormatDatetime(row.CreatedAt),
		})
	}

	total32 := int32(total)
	if total > math.MaxInt32 {
		total32 = math.MaxInt32
	}

	return &pb.NoticeListResponse{Items: items, Total: total32}, nil
}

// GetUnreadCount 未读通知数
func (n *Notice) GetUnreadCount(ctx context.Context, _ *pb.NoticeUnreadCountRequest) (*pb.NoticeUnreadCountResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	nUnread, err := n.SysNoticeRepo.CountUnread(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	count := int32(nUnread)
	if nUnread > math.MaxInt32 {
		count = math.MaxInt32
	}

	return &pb.NoticeUnreadCountResponse{Count: count}, nil
}

// ClearUnread 清除未读（全部标记为已读）
func (n *Notice) ClearUnread(ctx context.Context, _ *pb.NoticeClearUnreadRequest) (*pb.NoticeClearUnreadResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	if err := n.SysNoticeRepo.MarkAllRead(ctx, int(session.UserId)); err != nil {
		return nil, err
	}

	return &pb.NoticeClearUnreadResponse{}, nil
}
