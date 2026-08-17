package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/samber/lo"

	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service/message"
	"gorm.io/gorm/clause"
)

var _ ITalkSessionService = (*TalkSessionService)(nil)

type ITalkSessionService interface {
	List(ctx context.Context, uid int) ([]*model.TalkSessionDisplay, error)
	Create(ctx context.Context, opt *TalkSessionCreateOpt) (*model.TalkSession, error)
	Delete(ctx context.Context, uid int, talkMode int, receiverId int) error
	Top(ctx context.Context, opt *TalkSessionTopOpt) (int, error)
	Disturb(ctx context.Context, opt *TalkSessionDisturbOpt) (int, error)
	SessionDetail(ctx context.Context, uid int, talkMode int, receiverId int) (*model.TalkSession, error)
	SetRetainDays(ctx context.Context, opt *TalkSessionSetRetainDaysOpt) (int, error)
	BatchAddList(ctx context.Context, uid int, values map[string]int)
}

type TalkSessionService struct {
	*repo.Source
	TalkSessionRepo *repo.TalkSession
	GroupMemberRepo *repo.GroupMember
	GroupRepo       *repo.Group
	Message         message.IService
}

func (s *TalkSessionService) List(ctx context.Context, uid int) ([]*model.TalkSessionDisplay, error) {
	var items []*model.TalkSessionDisplay

	// Query explicit talk sessions using GORM
	err := s.Source.Db().WithContext(ctx).
		Table("talk_session").
		Select([]string{
			"talk_session.id", "talk_session.session_id", "talk_session.talk_mode", "talk_session.receiver_id", "talk_session.updated_at",
			"talk_session.is_disturb", "talk_session.is_top", "talk_session.is_robot", "talk_session.is_delete", "talk_session.retain_days",
			"`users`.avatar", "`users`.nickname",
			"`group`.name as group_name", "`group`.avatar as group_avatar",
		}).
		Joins("left join `users` ON talk_session.receiver_id = `users`.id AND talk_session.talk_mode = ?", entity.ChatPrivateMode).
		Joins("left join `group` ON talk_session.receiver_id = `group`.id AND talk_session.talk_mode = ?", entity.ChatGroupMode).
		// 兼容历史数据：is_delete=0 也视为“未删除”，避免老数据查不到只能退回虚拟会话
		Where("talk_session.user_id = ? and (talk_session.is_delete = ? or talk_session.is_delete = 0)", uid, model.No).
		Order("talk_session.updated_at desc").
		Find(&items).Error

	if err != nil {
		return nil, err
	}

	// 按 (talk_mode, receiver_id) 去重，避免表中有重复会话时返回同一联系人/群多条。
	// 如果同一个会话既有真实记录（id>0），又有异常/重复记录，优先保留真实记录；否则保留 updated_at 最新的一条。
	seen := make(map[string]*model.TalkSessionDisplay)
	for _, item := range items {
		key := fmt.Sprintf("%d_%d", item.TalkMode, item.ReceiverId)
		existing, ok := seen[key]
		if !ok {
			seen[key] = item
			continue
		}

		// 优先保留真实会话（id>0）
		if existing.Id == 0 && item.Id > 0 {
			seen[key] = item
			continue
		}
		if existing.Id > 0 && item.Id == 0 {
			continue
		}

		// 否则保留更新时间更晚的一条
		if item.UpdatedAt.After(existing.UpdatedAt) {
			seen[key] = item
		}
	}

	deduped := make([]*model.TalkSessionDisplay, 0, len(seen))
	for _, v := range seen {
		deduped = append(deduped, v)
	}
	items = deduped

	// Get all groups user is a member of
	userGroupIds := s.GroupMemberRepo.GetUserGroupIds(ctx, uid)
	if len(userGroupIds) > 0 {
		// Get group IDs that already have explicit sessions (is_delete=0)
		sessionGroupIds := make([]int, 0)
		for _, item := range items {
			if item.TalkMode == entity.ChatGroupMode {
				sessionGroupIds = append(sessionGroupIds, item.ReceiverId)
			}
		}

		// 用户已删除的群会话（is_delete=1）不再补回列表
		var deletedGroupIds []int
		_ = s.Source.Db().WithContext(ctx).Table("talk_session").
			Where("user_id = ? and talk_mode = ? and is_delete = ?", uid, entity.ChatGroupMode, model.Yes).
			Distinct("receiver_id").Pluck("receiver_id", &deletedGroupIds).Error

		// Find groups without explicit sessions, and exclude explicitly deleted ones
		missingGroupIds := lo.Filter(userGroupIds, func(gid int, _ int) bool {
			if lo.Contains(sessionGroupIds, gid) {
				return false
			}
			if lo.Contains(deletedGroupIds, gid) {
				return false
			}
			return true
		})

		// If there are groups without sessions, fetch their info and add them
		if len(missingGroupIds) > 0 {
			var groups []model.Group
			if err := s.Source.Db().WithContext(ctx).
				Where("id IN ?", missingGroupIds).
				Select("id", "name", "avatar", "created_at").
				Find(&groups).Error; err == nil {
				// Add groups without explicit sessions to the list
				for _, group := range groups {
					items = append(items, &model.TalkSessionDisplay{
						Id:          0,
						TalkMode:    entity.ChatGroupMode,
						ReceiverId:  group.Id,
						IsDelete:    model.No,
						IsTop:       model.No,
						IsRobot:     model.No,
						IsDisturb:   model.No,
						GroupName:   group.Name,
						GroupAvatar: group.Avatar,
						UpdatedAt:   group.CreatedAt,
					})
				}
			}
		}
	}

	// Re-sort all items by updated_at desc
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return items, nil
}

type TalkSessionCreateOpt struct {
	UserId     int
	TalkType   int
	ReceiverId int
	IsBoot     bool
}

// Create 创建会话列表
func (s *TalkSessionService) Create(ctx context.Context, opt *TalkSessionCreateOpt) (*model.TalkSession, error) {

	result, err := s.TalkSessionRepo.FindByWhere(ctx, "talk_mode = ? and user_id = ? and receiver_id = ?", opt.TalkType, opt.UserId, opt.ReceiverId)
	if err != nil {
		return nil, err
	}

	if result == nil || result.Id == 0 {
		result = &model.TalkSession{
			TalkMode:   opt.TalkType,
			UserId:     opt.UserId,
			ReceiverId: opt.ReceiverId,
			IsTop:      model.No,
			IsDelete:   model.No,
			IsDisturb:  model.No,
			IsRobot:    model.No,
		}

		if opt.IsBoot {
			result.IsRobot = model.Yes
		}

		// 对于私聊模式，需要关联成对的会话
		if opt.TalkType == entity.ChatPrivateMode {
			// 检查是否已有反向会话
			reverseSession, err := s.TalkSessionRepo.FindByWhere(ctx, "talk_mode = ? and user_id = ? and receiver_id = ?", opt.TalkType, opt.ReceiverId, opt.UserId)
			if err == nil && reverseSession != nil && reverseSession.Id > 0 {
				// 如果反向会话已存在，使用较小的 ID 作为 session_id
				result.SessionId = lo.Min([]int{reverseSession.Id, result.Id + 1}) // result.Id + 1 是新记录预计的ID
			}
		}

		if err := s.Source.Db().WithContext(ctx).Create(result).Error; err != nil {
			return nil, err
		}

		// 创建后，如果是私聊且没有反向会话，则将 session_id 设为自己的 ID
		if opt.TalkType == entity.ChatPrivateMode && result.SessionId == 0 {
			result.SessionId = result.Id
			s.Source.Db().WithContext(ctx).Model(result).Update("session_id", result.SessionId)
		}

		// 对于私聊，自动创建反向会话并与这个会话关联
		if opt.TalkType == entity.ChatPrivateMode {
			reverseResult, err := s.TalkSessionRepo.FindByWhere(ctx, "talk_mode = ? and user_id = ? and receiver_id = ?", opt.TalkType, opt.ReceiverId, opt.UserId)
			if err == nil && (reverseResult == nil || reverseResult.Id == 0) {
				// 反向会话不存在，需要创建
				reverseSession := &model.TalkSession{
					TalkMode:   opt.TalkType,
					UserId:     opt.ReceiverId,
					ReceiverId: opt.UserId,
					SessionId:  result.Id,
					IsTop:      model.No,
					IsDelete:   model.No,
					IsDisturb:  model.No,
					IsRobot:    model.No,
				}
				s.Source.Db().WithContext(ctx).Create(reverseSession)
			} else if err == nil && reverseResult != nil && reverseResult.SessionId == 0 {
				// 反向会话存在但没有 session_id，更新它
				s.Source.Db().WithContext(ctx).Model(reverseResult).Update("session_id", result.SessionId)
			}
		}
	} else {
		// 已有会话：仅恢复未删除状态，保留 is_top / is_disturb 等用户设置
		updates := map[string]any{
			"is_delete":  model.No,
			"updated_at": time.Now(),
		}
		if opt.IsBoot {
			updates["is_robot"] = model.Yes
			result.IsRobot = model.Yes
		}
		if _, err := s.TalkSessionRepo.UpdateByWhere(ctx, updates, "id = ?", result.Id); err != nil {
			return nil, err
		}
		result.IsDelete = model.No
	}

	return result, nil
}

// Delete 删除会话
func (s *TalkSessionService) Delete(ctx context.Context, uid int, talkMode int, receiverId int) error {
	_, err := s.TalkSessionRepo.UpdateByWhere(ctx, map[string]any{
		"is_delete":  model.Yes,
		"updated_at": time.Now(),
	}, "user_id = ? and receiver_id = ? and talk_mode = ?", uid, receiverId, talkMode)
	return err
}

type TalkSessionTopOpt struct {
	UserId     int // 用户id
	TalkMode   int // 1:私聊 2:群聊
	ReceiverId int // 对方id
	Action     int // 1:置顶 2:取消置顶
}

// Top 会话置顶
//
// 优化点：
//   - 先尝试更新已有会话记录；
//   - 如果 RowsAffected = 0，说明当前用户还没有这条会话，则自动创建一条默认会话并写入 is_top；
//   - 这样即使用户从未在该群/会话中发过消息，直接点置顶也会生成真实会话行，SessionList 中不再是 id=0 的虚拟会话。
func (s *TalkSessionService) Top(ctx context.Context, opt *TalkSessionTopOpt) (int, error) {
	isTop := lo.Ternary(opt.Action == 1, model.Yes, model.No)
	now := time.Now()

	affected, err := s.TalkSessionRepo.UpdateByWhere(ctx, map[string]any{
		"is_top":     isTop,
		"updated_at": now,
	}, "user_id = ? and talk_mode = ? and receiver_id = ?", opt.UserId, opt.TalkMode, opt.ReceiverId)
	if err != nil {
		return isTop, err
	}

	// 没有任何记录被更新，则为该会话自动创建一条记录
	if affected == 0 {
		session := &model.TalkSession{
			TalkMode:   opt.TalkMode,
			UserId:     opt.UserId,
			ReceiverId: opt.ReceiverId,
			IsTop:      isTop,
			IsDisturb:  model.No,
			IsDelete:   model.No,
			IsRobot:    model.No,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if err := s.TalkSessionRepo.Create(ctx, session); err != nil {
			return isTop, err
		}
	}

	return isTop, nil
}

type TalkSessionDisturbOpt struct {
	UserId     int
	TalkMode   int
	ReceiverId int
	Action     int
}

// Disturb 会话免打扰
//
// 与 Top 类似：如果当前会话不存在，则自动创建一条记录并写入 is_disturb，避免 SessionList 中只有 id=0 的虚拟会话。
func (s *TalkSessionService) Disturb(ctx context.Context, opt *TalkSessionDisturbOpt) (int, error) {
	isDisturb := lo.Ternary(opt.Action == 1, model.Yes, model.No)
	now := time.Now()

	affected, err := s.TalkSessionRepo.UpdateByWhere(ctx, map[string]any{
		"is_disturb": isDisturb,
		"updated_at": now,
	}, "user_id = ? and talk_mode = ? and receiver_id = ?", opt.UserId, opt.TalkMode, opt.ReceiverId)
	if err != nil {
		return isDisturb, err
	}

	if affected == 0 {
		session := &model.TalkSession{
			TalkMode:   opt.TalkMode,
			UserId:     opt.UserId,
			ReceiverId: opt.ReceiverId,
			IsTop:      model.No,
			IsDisturb:  isDisturb,
			IsDelete:   model.No,
			IsRobot:    model.No,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if err := s.TalkSessionRepo.Create(ctx, session); err != nil {
			return isDisturb, err
		}
	}

	return isDisturb, nil
}

// SessionDetail 会话详情
func (s *TalkSessionService) SessionDetail(ctx context.Context, uid int, talkMode int, receiverId int) (*model.TalkSession, error) {
	return s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and talk_mode = ? and receiver_id = ?", uid, talkMode, receiverId)
}

type TalkSessionSetRetainDaysOpt struct {
	UserId        int
	LinkSessionId int // talk_session.session_id（私聊成对关联 ID）
	RetainDays    int
}

// SetRetainDays 设置私聊消息保留天数：校验己方与对方 talk_session 后，按 session_id 同步更新
func (s *TalkSessionService) SetRetainDays(ctx context.Context, opt *TalkSessionSetRetainDaysOpt) (int, error) {
	if opt == nil || opt.UserId <= 0 || opt.LinkSessionId <= 0 {
		return 0, entity.ErrPermissionDenied
	}
	if opt.RetainDays < 0 || opt.RetainDays > 3650 {
		return 0, entity.ErrPermissionDenied
	}

	rows, err := s.TalkSessionRepo.FindPrivatePairByLinkSessionId(ctx, opt.LinkSessionId)
	if err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, entity.ErrDataNotFound
	}

	var mine, peer *model.TalkSession
	for _, row := range rows {
		if row.UserId == opt.UserId {
			mine = row
			continue
		}
		if peer == nil {
			peer = row
		}
	}
	if mine == nil {
		return 0, entity.ErrPermissionDenied
	}
	if peer == nil {
		return 0, entity.ErrDataNotFound
	}
	if mine.ReceiverId != peer.UserId || peer.ReceiverId != mine.UserId {
		return 0, entity.ErrPermissionDenied
	}

	if err := s.TalkSessionRepo.UpdateRetainDaysByLinkSessionId(ctx, opt.LinkSessionId, opt.RetainDays); err != nil {
		return 0, err
	}

	if s.Message != nil {
		if err := s.Message.CreatePrivateRetainDaysSetMessage(ctx, opt.UserId, mine.ReceiverId, opt.RetainDays); err != nil {
			return 0, err
		}
	}

	return opt.RetainDays, nil
}

// BatchAddList 批量添加会话列表
func (s *TalkSessionService) BatchAddList(ctx context.Context, uid int, values map[string]int) {

	ctime := time.Now()

	sessions := make([]*model.TalkSession, 0)
	for k, v := range values {
		if v == 0 {
			continue
		}

		value := strings.Split(k, "_")
		if len(value) != 2 {
			continue
		}

		talkMode := 0
		receiverId := 0
		fmt.Sscanf(value[0], "%d", &talkMode)
		fmt.Sscanf(value[1], "%d", &receiverId)

		if talkMode == 0 || receiverId == 0 {
			continue
		}

		sessions = append(sessions, &model.TalkSession{
			TalkMode:   talkMode,
			UserId:     uid,
			ReceiverId: receiverId,
			CreatedAt:  ctime,
			UpdatedAt:  ctime,
		})
	}

	if len(sessions) == 0 {
		return
	}

	// 创建会话后，自动同步 session_id
	s.Source.Db().WithContext(ctx).Clauses(clause.OnConflict{
		DoUpdates: clause.Assignments(map[string]interface{}{
			"is_delete":  model.No,
			"updated_at": ctime,
		}),
	}).Create(&sessions)

	// 为新创建的会话设置 session_id
	for _, session := range sessions {
		fetched, err := s.TalkSessionRepo.FindByWhere(ctx, "talk_mode = ? and user_id = ? and receiver_id = ?", session.TalkMode, session.UserId, session.ReceiverId)
		if err != nil {
			continue
		}

		if fetched.SessionId == 0 {
			// 设置 session_id 为当前记录的 ID
			s.Source.Db().WithContext(ctx).Model(fetched).Update("session_id", fetched.Id)

			// 如果是私聊，为对方也设置相同的 session_id
			if fetched.TalkMode == entity.ChatPrivateMode {
				reverseSession, err := s.TalkSessionRepo.FindByWhere(ctx, "talk_mode = ? and user_id = ? and receiver_id = ?", fetched.TalkMode, fetched.ReceiverId, fetched.UserId)
				if err == nil && reverseSession.SessionId == 0 {
					s.Source.Db().WithContext(ctx).Model(reverseSession).Update("session_id", fetched.Id)
				}
			}
		}
	}
}
