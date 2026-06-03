package v1

import (
	"context"
	"math"
	"strings"

	pb "github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/repo"
)

type Invite struct {
	UsersRepo *repo.Users
}

// GetMyInviteCode 获取当前用户邀请码（users.invite_code，为空则生成并保存）
func (i *Invite) GetMyInviteCode(ctx context.Context, _ *pb.InviteCodeGetRequest) (*pb.InviteCodeGetResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	code, err := i.UsersRepo.EnsureInviteCode(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}
	return &pb.InviteCodeGetResponse{Code: code}, nil
}

// GetInviteStats 获取邀请统计
func (i *Invite) GetInviteStats(ctx context.Context, _ *pb.InviteStatsRequest) (*pb.InviteStatsResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	n, err := i.UsersRepo.CountByInviteUserId(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}
	total := int32(n)
	if n > math.MaxInt32 {
		total = math.MaxInt32
	}
	return &pb.InviteStatsResponse{TotalInvitations: total}, nil
}

// ListInviteFriends 我邀请注册的好友列表（users.invite_user_id = 当前用户），支持分页
func (i *Invite) ListInviteFriends(ctx context.Context, req *pb.InviteFriendListRequest) (*pb.InviteFriendListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, users, err := i.UsersRepo.PaginationByInviteUserId(ctx, int(session.UserId), page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.InviteFriendItem, 0, len(users))
	for _, u := range users {
		items = append(items, &pb.InviteFriendItem{
			UserId:    int32(u.Id),
			Nickname:  u.Nickname,
			Avatar:    u.Avatar,
			Mobile:    maskInviteMobile(u.Mobile),
			Email:     maskInviteEmail(u.Email),
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	total32 := int32(total)
	if total > math.MaxInt32 {
		total32 = math.MaxInt32
	}

	return &pb.InviteFriendListResponse{Items: items, Total: total32}, nil
}

func maskInviteMobile(m *string) string {
	if m == nil || *m == "" {
		return ""
	}
	s := *m
	if len(s) < 7 {
		return "****"
	}
	return s[:3] + "****" + s[len(s)-4:]
}

func maskInviteEmail(e string) string {
	if e == "" {
		return ""
	}
	parts := strings.Split(e, "@")
	if len(parts) != 2 || parts[0] == "" {
		return "****"
	}
	name := parts[0]
	if len(name) <= 2 {
		return "**@" + parts[1]
	}
	return name[:2] + "****@" + parts[1]
}
