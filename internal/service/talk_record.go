package service

import (
	"context"
	"errors"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/sliceutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"gorm.io/gorm"
)

var _ ITalkRecordService = (*TalkRecordService)(nil)

type ITalkRecordService interface {
	FindAllPrivateRecordByOriMsgId(ctx context.Context, msgId string) ([]*model.TalkUserMessage, error)
	FindPrivateRecordByMsgId(ctx context.Context, msgId string) (*model.TalkUserMessage, error)
	FindTalkPrivateRecord(ctx context.Context, uid int, msgId string) (*model.TalkMessageRecord, error)
	FindTalkGroupRecord(ctx context.Context, msgId string) (*model.TalkMessageRecord, error)
	FindAllTalkRecords(ctx context.Context, opt *FindAllTalkRecordsOpt) ([]*model.TalkMessageRecord, error)
	FindForwardRecords(ctx context.Context, uid int, msgIds []string, talkType int) ([]*model.TalkMessageRecord, error)
}

type TalkRecordService struct {
	*repo.Source
	TalkVoteCache         *cache.Vote
	TalkRecordsVoteRepo   *repo.GroupVote
	GroupMemberRepo       *repo.GroupMember
	TalkRecordFriendRepo  *repo.TalkUserMessage
	TalkRecordGroupRepo   *repo.TalkGroupMessage
	TalkRecordsDeleteRepo *repo.TalkGroupMessageDel
	TalkSessionRepo       *repo.TalkSession
}

func (s *TalkRecordService) FindPrivateRecordByMsgId(ctx context.Context, msgId string) (*model.TalkUserMessage, error) {
	return s.TalkRecordFriendRepo.FindByMsgId(ctx, msgId)
}

func (s *TalkRecordService) FindAllPrivateRecordByOriMsgId(ctx context.Context, msgId string) ([]*model.TalkUserMessage, error) {
	return s.TalkRecordFriendRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Where("org_msg_id = ?", msgId)
	})
}

type FindAllTalkRecordsOpt struct {
	TalkType      int    // 对话类型
	UserId        int    // 获取消息的用户
	ReceiverId    int    // 接收者ID
	MsgType       []int  // 消息类型
	Keyword       string // 搜索关键字（可选，按内容模糊匹配）
	Cursor        int64  // 上次查询的游标
	Limit         int    // 数据行数
	IncludeHidden bool   // 是否包含 is_revoked=0 的混合子段（history-records 用）
}

func (s *TalkRecordService) FindTalkPrivateRecord(ctx context.Context, uid int, msgId string) (*model.TalkMessageRecord, error) {
	// 只需要根据 msg_id 查询即可，不再需要 user_id
	talkRecordFriendInfo, err := s.TalkRecordFriendRepo.FindByWhere(ctx, "msg_id = ?", msgId)
	if err != nil {
		return nil, err
	}

	record := &model.TalkMessageRecord{
		TalkMode:   entity.ChatPrivateMode,
		FromId:     talkRecordFriendInfo.FromId,
		ReceiverId: talkRecordFriendInfo.ReceiverId,
		MsgId:      talkRecordFriendInfo.MsgId,
		MsgType:    talkRecordFriendInfo.MsgType,
		Nickname:   "",
		Avatar:     "",
		IsRevoked:  talkRecordFriendInfo.IsRevoked,
		IsRead:     talkRecordFriendInfo.IsRead,
		SendTime:   talkRecordFriendInfo.SendTime,
		Extra:      talkRecordFriendInfo.Extra,
		Quote:      talkRecordFriendInfo.Quote,
	}

	list, err := s.handleTalkRecords(ctx, []*model.TalkMessageRecord{record})
	if err != nil {
		return nil, err
	}

	return list[0], nil
}

func (s *TalkRecordService) FindTalkGroupRecord(ctx context.Context, msgId string) (*model.TalkMessageRecord, error) {
	talkRecordGroupInfo, err := s.TalkRecordGroupRepo.FindByMsgId(ctx, msgId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if talkRecordGroupInfo == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if talkRecordGroupInfo.IsDeleted == model.Yes {
		return nil, gorm.ErrRecordNotFound
	}

	record := &model.TalkMessageRecord{
		TalkMode:   entity.ChatGroupMode,
		FromId:     talkRecordGroupInfo.FromId,
		ReceiverId: talkRecordGroupInfo.GroupId,
		MsgId:      talkRecordGroupInfo.MsgId,
		MsgType:    talkRecordGroupInfo.MsgType,
		Nickname:   "",
		Avatar:     "",
		IsRevoked:  talkRecordGroupInfo.IsRevoked,
		SendTime:   talkRecordGroupInfo.SendTime,
		Extra:      talkRecordGroupInfo.Extra,
		Quote:      talkRecordGroupInfo.Quote,
	}

	list, err := s.handleTalkRecords(ctx, []*model.TalkMessageRecord{record})
	if err != nil {
		return nil, err
	}

	return list[0], nil
}

// FindAllTalkRecords 获取所有对话消息
func (s *TalkRecordService) FindAllTalkRecords(ctx context.Context, opt *FindAllTalkRecordsOpt) ([]*model.TalkMessageRecord, error) {
	var (
		items  = make([]*model.TalkMessageRecord, 0, opt.Limit)
		cursor = opt.Cursor
	)

	for {
		// 这里查询数据放弃了关联查询，所以这里需要查询多次，防止查询中存在用户已删除的数据需要过滤掉
		list, err := s.findAllRecords(ctx, &FindAllTalkRecordsOpt{
			TalkType:      opt.TalkType,
			UserId:        opt.UserId,
			ReceiverId:    opt.ReceiverId,
			MsgType:       opt.MsgType,
			Keyword:       opt.Keyword,
			Cursor:        cursor,
			Limit:         opt.Limit + 10, // 多查几条数据
			IncludeHidden: opt.IncludeHidden,
		})

		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			break
		}

		if opt.TalkType == entity.ChatGroupMode {
			tmpMsgIds := make([]string, 0, len(list))
			for _, v := range list {
				tmpMsgIds = append(tmpMsgIds, v.MsgId)
			}

			msgIds, err := s.TalkRecordsDeleteRepo.FindAllMsgIds(ctx, opt.UserId, tmpMsgIds)
			if err != nil {
				return nil, err
			}

			hashIds := make(map[string]struct{}, len(msgIds))
			for _, msgId := range msgIds {
				hashIds[msgId] = struct{}{}
			}

			for _, v := range list {
				if _, ok := hashIds[v.MsgId]; ok {
					continue
				}

				items = append(items, v)
			}
		} else {
			items = append(items, list...)
		}

		if len(items) >= opt.Limit || len(list) < opt.Limit {
			break
		}

		// 设置游标继续往下执行
		cursor = list[len(list)-1].Id
	}

	if len(items) > opt.Limit {
		items = items[:opt.Limit]
	}

	return s.handleTalkRecords(ctx, items)
}

// findAllRecords 查询所有对话消息记录
// 对于私聊，先查询 TalkSession 获取 SessionId，然后使用 SessionId 查询消息
// 对于群聊，直接使用 GroupId 查询消息
func (s *TalkRecordService) findAllRecords(ctx context.Context, opt *FindAllTalkRecordsOpt) ([]*model.TalkMessageRecord, error) {
	query := s.Source.Db().WithContext(ctx)

	fields := []string{
		"id",
		"msg_id",
		"msg_type",
		"is_revoked",
		"extra",
		"quote",
		"send_time",
		"from_id",
	}
	if opt.TalkType == entity.ChatPrivateMode {
		fields = append(fields, "is_read")
	}

	if opt.TalkType == 1 {
		// 私聊：先查询 TalkSession 获取 SessionId
		query = query.Model(&model.TalkUserMessage{})

		if s.TalkSessionRepo != nil {
			// 优化方案：使用 SessionId 查询
			session, err := s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", opt.UserId, opt.ReceiverId, opt.TalkType)
			if err != nil {
				return nil, err
			}
			query.Where("session_id = ?", session.SessionId)
		} else {
			// 降级方案：使用传统方式查询（兼容未注入 TalkSessionRepo 的情况）
			query.Where("user_id = ? and receiver_id = ?", opt.UserId, opt.ReceiverId)
		}

		query.Where("is_deleted = ?", model.No)
	} else {
		// 群聊：直接使用 GroupId 查询；过滤已删除（is_deleted=1）
		// msg_type=1201（保留天数提示）仅对 from_id=当前用户 可见
		query = query.Model(&model.TalkGroupMessage{})
		query.Where("group_id = ?", opt.ReceiverId)
		query.Where("is_deleted = ?", model.No)
		query.Where("(msg_type <> ? OR from_id = ?)", entity.ChatMsgSysRetainDaysSet, opt.UserId)
	}

	// 会话消息默认隐藏混合子段（is_revoked=0）；history-records 设 IncludeHidden 后可按类型索引取出
	if !opt.IncludeHidden {
		query.Where("is_revoked <> ?", model.TalkMsgRevokedHidden)
	}

	// 关键字搜索：仅对文本/混合/代码等内容型消息按内容做模糊匹配。
	// 为简单起见，这里直接对 extra 字段做 LIKE；如果后续抽出独立 content 字段，可改为针对 content。
	if opt.Keyword != "" {
		kw := "%" + opt.Keyword + "%"
		query = query.Where("extra LIKE ?", kw)
	}

	query.Select(fields)

	if opt.Cursor > 0 {
		query.Where("id < ?", opt.Cursor)
	}

	if len(opt.MsgType) > 0 {
		query.Where("msg_type in ?", opt.MsgType)
	}

	query.Order("id desc").Limit(opt.Limit)

	var items []*model.TalkMessageRecord
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}

	for i := 0; i < len(items); i++ {
		items[i].TalkMode = opt.TalkType
		items[i].ReceiverId = opt.ReceiverId
		if opt.TalkType == entity.ChatGroupMode {
			items[i].IsRead = 0
		}
	}

	return items, nil
}

// FindForwardRecords 获取转发消息记录（支持再次转发后的嵌套展开）
func (s *TalkRecordService) FindForwardRecords(ctx context.Context, uid int, msgIds []string, talkType int) ([]*model.TalkMessageRecord, error) {
	items, err := s.lookupForwardMsgIds(ctx, uid, msgIds, talkType)
	if err != nil {
		return nil, err
	}
	items, err = s.expandNestedForwardRecords(ctx, uid, items, 0)
	if err != nil {
		return nil, err
	}
	return s.handleTalkRecords(ctx, items)
}

func (s *TalkRecordService) lookupForwardMsgIds(ctx context.Context, uid int, msgIds []string, talkType int) ([]*model.TalkMessageRecord, error) {
	if len(msgIds) == 0 {
		return []*model.TalkMessageRecord{}, nil
	}

	privateFields := []string{"id", "msg_id", "msg_type", "is_revoked", "is_read", "extra", "quote", "send_time", "from_id"}
	groupFields := []string{"id", "msg_id", "msg_type", "is_revoked", "extra", "quote", "send_time", "from_id"}

	loadPrivate := func(ids []string) ([]*model.TalkMessageRecord, error) {
		var rows []*model.TalkMessageRecord
		err := s.Source.Db().WithContext(ctx).Model(&model.TalkUserMessage{}).
			Select(privateFields).
			Where("msg_id in ? and is_deleted = ? and is_revoked <> ?", ids, model.No, model.TalkMsgRevokedHidden).
			Order("id asc").
			Scan(&rows).Error
		return rows, err
	}
	loadGroup := func(ids []string) ([]*model.TalkMessageRecord, error) {
		var rows []*model.TalkMessageRecord
		err := s.Source.Db().WithContext(ctx).Model(&model.TalkGroupMessage{}).
			Select(groupFields).
			Where("msg_id in ?", ids).
			Where("is_deleted = ?", model.No).
			Where("is_revoked <> ?", model.TalkMsgRevokedHidden).
			Where("(msg_type <> ? OR from_id = ?)", entity.ChatMsgSysRetainDaysSet, uid).
			Order("id asc").
			Scan(&rows).Error
		if err != nil {
			return nil, err
		}
		for i := range rows {
			rows[i].IsRead = 0
		}
		return rows, nil
	}

	var (
		items []*model.TalkMessageRecord
		err   error
	)
	if talkType == entity.ChatGroupMode {
		items, err = loadGroup(msgIds)
	} else {
		items, err = loadPrivate(msgIds)
	}
	if err != nil {
		return nil, err
	}

	// 再次转发后 talk_mode 可能与原始消息表不一致，缺的再查另一张表
	if len(items) < len(msgIds) {
		found := make(map[string]struct{}, len(items))
		for _, it := range items {
			found[it.MsgId] = struct{}{}
		}
		missing := make([]string, 0)
		for _, id := range msgIds {
			if _, ok := found[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			var more []*model.TalkMessageRecord
			if talkType == entity.ChatGroupMode {
				more, err = loadPrivate(missing)
			} else {
				more, err = loadGroup(missing)
			}
			if err != nil {
				return nil, err
			}
			items = append(items, more...)
		}
	}

	return orderTalkRecordsByMsgIds(items, msgIds), nil
}

// expandNestedForwardRecords 打开合并转发时，若条目本身仍是转发气泡则递归展开为实际消息
func (s *TalkRecordService) expandNestedForwardRecords(ctx context.Context, uid int, items []*model.TalkMessageRecord, depth int) ([]*model.TalkMessageRecord, error) {
	if depth >= 5 || len(items) == 0 {
		return items, nil
	}

	out := make([]*model.TalkMessageRecord, 0, len(items))
	for _, item := range items {
		if item == nil || item.MsgType != entity.ChatMsgTypeForward {
			out = append(out, item)
			continue
		}

		var extra model.TalkRecordExtraForward
		if err := jsonutil.Unmarshal(item.Extra, &extra); err != nil || len(extra.MsgIds) == 0 {
			out = append(out, item)
			continue
		}

		nested, err := s.lookupForwardMsgIds(ctx, uid, extra.MsgIds, extra.TalkType)
		if err != nil {
			return nil, err
		}
		if len(nested) == 0 {
			out = append(out, item)
			continue
		}
		nested, err = s.expandNestedForwardRecords(ctx, uid, nested, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, nested...)
	}
	return out, nil
}

func orderTalkRecordsByMsgIds(items []*model.TalkMessageRecord, msgIds []string) []*model.TalkMessageRecord {
	if len(items) <= 1 || len(msgIds) == 0 {
		return items
	}
	byId := make(map[string]*model.TalkMessageRecord, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		if _, ok := byId[it.MsgId]; !ok {
			byId[it.MsgId] = it
		}
	}
	ordered := make([]*model.TalkMessageRecord, 0, len(items))
	used := make(map[string]struct{}, len(items))
	for _, id := range msgIds {
		if it, ok := byId[id]; ok {
			ordered = append(ordered, it)
			used[id] = struct{}{}
		}
	}
	for _, it := range items {
		if it == nil {
			continue
		}
		if _, ok := used[it.MsgId]; ok {
			continue
		}
		ordered = append(ordered, it)
	}
	return ordered
}

// HandleTalkRecords 处理消息
func (s *TalkRecordService) handleTalkRecords(ctx context.Context, items []*model.TalkMessageRecord) ([]*model.TalkMessageRecord, error) {
	if len(items) == 0 {
		return make([]*model.TalkMessageRecord, 0), nil
	}

	uids := make([]int, 0, len(items))
	for _, item := range items {
		uids = append(uids, item.FromId)
	}

	var usersItems []*model.Users
	err := s.Source.Db().WithContext(ctx).Model(&model.Users{}).
		Select("id,nickname,avatar").
		Where("id in ?", sliceutil.Unique(uids)).Scan(&usersItems).Error
	if err != nil {
		return nil, err
	}

	hashUser := make(map[int]*model.Users)
	for _, user := range usersItems {
		hashUser[user.Id] = user
	}

	for i := 0; i < len(items); i++ {
		if user, ok := hashUser[items[i].FromId]; ok {
			items[i].Nickname = user.Nickname
			items[i].Avatar = user.Avatar
		}

		//if err = jsonutil.Unmarshal(items[i].Extra, &items[i].Extra); err != nil {
		//	fmt.Println("ERR===>", items[i].Msgid, items[i].Extra, items[i].Extra)
		//}
		//
		//if err = jsonutil.Unmarshal(items[i].Quote, &items[i].Quote); err != nil {
		//	fmt.Println("ERR===>", items[i].Msgid, items[i].Quote, items[i].Quote)
		//}
	}

	return items, nil
}
