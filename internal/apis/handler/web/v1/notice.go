package v1

import (
	"context"
	"errors"
	"math"
	"strings"

	pb "github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

const noticePageSize = 15

type Notice struct {
	NoticeLetterRepo  *repo.NoticeLetter
	NoticeArticleRepo *repo.NoticeArticle
}

func (n *Notice) ensureRepo() error {
	if n == nil || n.NoticeLetterRepo == nil {
		return errors.New("NoticeLetterRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}
	return nil
}

// ListNotice 系统通知列表，按时间倒序，每页 15 条
func (n *Notice) ListNotice(ctx context.Context, req *pb.NoticeListRequest) (*pb.NoticeListResponse, error) {
	if err := n.ensureRepo(); err != nil {
		return nil, err
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}

	rows, total, err := n.NoticeLetterRepo.ListByUserDesc(ctx, int(session.UserId), page, noticePageSize)
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

// GetUnreadCount 未读通知数，并返回最新一条通知摘要
func (n *Notice) GetUnreadCount(ctx context.Context, _ *pb.NoticeUnreadCountRequest) (*pb.NoticeUnreadCountResponse, error) {
	if err := n.ensureRepo(); err != nil {
		return nil, err
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	userID := int(session.UserId)

	nUnread, err := n.NoticeLetterRepo.CountUnread(ctx, userID)
	if err != nil {
		return nil, err
	}

	count := int32(nUnread)
	if nUnread > math.MaxInt32 {
		count = math.MaxInt32
	}

	resp := &pb.NoticeUnreadCountResponse{Count: count}
	latest, err := n.NoticeLetterRepo.FindLatestByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		resp.Title = latest.Title
		resp.Content = latest.Content
		resp.CreatedAt = timeutil.FormatDatetime(latest.CreatedAt)
	}

	return resp, nil
}

// ClearUnread 清除未读（全部标记为已读）
func (n *Notice) ClearUnread(ctx context.Context, _ *pb.NoticeClearUnreadRequest) (*pb.NoticeClearUnreadResponse, error) {
	if err := n.ensureRepo(); err != nil {
		return nil, err
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	if err := n.NoticeLetterRepo.MarkAllRead(ctx, int(session.UserId)); err != nil {
		return nil, err
	}

	return &pb.NoticeClearUnreadResponse{}, nil
}

// GetNoticeArticle 获取通知文章（notice_article，仅 status=开启；传 code 时按 code 查，否则按 id 查）
func (n *Notice) GetNoticeArticle(ctx context.Context, req *pb.NoticeArticleGetRequest) (*pb.NoticeArticleGetResponse, error) {
	if n == nil || n.NoticeArticleRepo == nil {
		return nil, errors.New("NoticeArticleRepo 未注入，请执行 go generate 更新 wire_gen.go")
	}

	code := strings.TrimSpace(req.GetCode())
	var (
		row *model.NoticeArticle
		err error
	)
	if code != "" {
		row, err = n.NoticeArticleRepo.FindEnabledByCode(ctx, code)
	} else if req.GetId() > 0 {
		row, err = n.NoticeArticleRepo.FindEnabledById(ctx, int(req.GetId()))
	} else {
		return nil, errorx.New(400, "请提供 id 或 code")
	}
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, entity.ErrDataNotFound
	}
	return &pb.NoticeArticleGetResponse{
		Id:        int32(row.Id),
		Title:     row.Title,
		Content:   row.Content,
		Status:    int32(row.Status),
		CreatedAt: timeutil.FormatDatetime(row.CreatedAt),
		UpdatedAt: timeutil.FormatDatetime(row.UpdatedAt),
	}, nil
}
