package v1

import (
	"context"
	"math"
	"strings"

	pb "github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
)

type Invite struct {
	InviteCodeService service.IInviteCodeService
	UsersRepo         *repo.Users
}

// GenerateInviteCode 生成邀请码
func (i *Invite) GenerateInviteCode(ctx context.Context, req *pb.InviteGenerateRequest) (*pb.InviteGenerateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	expireDays := req.ExpireDays
	if expireDays <= 0 {
		expireDays = 36500 // 默认100年（长期）
	}

	// maxUsage := req.MaxUsage
	// if maxUsage <= 0 {
	// 	maxUsage = 1 // 默认1次
	// }
	maxUsage := 0

	inviteCode, err := i.InviteCodeService.GenerateInviteCode(ctx, int(session.UserId), int(expireDays), int(maxUsage))
	if err != nil {
		return nil, err
	}

	return &pb.InviteGenerateResponse{
		Code:          inviteCode.Code,
		ExpireAt:      inviteCode.ExpireAt.Format("2006-01-02 15:04:05"),
		MaxUsageCount: int32(inviteCode.MaxUsageCount),
	}, nil
}

// GetMyInviteCodes 获取我的邀请码列表
func (i *Invite) GetMyInviteCodes(ctx context.Context, req *pb.InviteListRequest) (*pb.InviteListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	codes, err := i.InviteCodeService.GetUserInviteCodes(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	items := make([]*pb.InviteCodeItem, 0, len(codes))
	for _, code := range codes {
		items = append(items, &pb.InviteCodeItem{
			Id:            int32(code.Id),
			Code:          code.Code,
			Status:        int32(code.Status),
			ExpireAt:      code.ExpireAt.Format("2006-01-02 15:04:05"),
			MaxUsageCount: int32(code.MaxUsageCount),
			UsageCount:    int32(code.UsageCount),
			CreatedAt:     code.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &pb.InviteListResponse{
		Items: items,
	}, nil
}

// GetInviteStats 获取邀请统计
func (i *Invite) GetInviteStats(ctx context.Context, req *pb.InviteStatsRequest) (*pb.InviteStatsResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	stats, err := i.InviteCodeService.GetInviteStats(ctx, int(session.UserId))
	if err != nil {
		return nil, err
	}

	return &pb.InviteStatsResponse{
		TotalCodes:       int32(stats["total_codes"]),
		AvailableCodes:   int32(stats["available_codes"]),
		UsedCodes:        int32(stats["used_codes"]),
		TotalInvitations: int32(stats["total_invitations"]),
	}, nil
}

// DisableInviteCode 禁用邀请码
func (i *Invite) DisableInviteCode(ctx context.Context, req *pb.InviteDisableRequest) (*pb.InviteDisableResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	if err := i.InviteCodeService.DisableInviteCode(ctx, req.Code, int(session.UserId)); err != nil {
		return nil, err
	}

	return &pb.InviteDisableResponse{}, nil
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
