package message

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type ForwardMessageOpt struct {
	MsgIds       []string `json:"msg_ids"`
	TalkMode     int      `json:"talk_mode"`
	ReceiverId   int      `json:"receiver_id"`
	UserId       int      `json:"user_id"`
	ToUserId     int      `json:"to_user_id"`
	ToUserIdType int      `json:"to_user_id_type"` // 1:用户ID 2:群ID
}

type forwardSourceItem struct {
	MsgId    string
	MsgType  int
	Extra    string
	FromId   int
	TalkType int // 消息实际所在会话类型（展平嵌套转发后使用）
}

// SplitForward 分拆转发
func (s *Service) toSplitForward(ctx context.Context, req ForwardMessageOpt) error {
	var (
		now = time.Now()
		db  = s.Source.Db().WithContext(ctx)
	)

	sources, err := s.loadForwardSources(ctx, req)
	if err != nil {
		return err
	}
	sources, err = s.flattenForwardSources(ctx, sources, 0)
	if err != nil {
		return err
	}

	messageItems := make([]model.TalkRecord, 0, len(sources))
	for _, v := range sources {
		messageItems = append(messageItems, model.TalkRecord{
			MsgType: v.MsgType,
			Extra:   v.Extra,
		})
	}

	// 向群发送消息
	if req.ToUserIdType == entity.ChatGroupMode {

		items := make([]model.TalkGroupMessage, 0)
		for _, v := range messageItems {
			items = append(items, model.TalkGroupMessage{
				MsgId:     strutil.NewMsgId(),
				MsgType:   v.MsgType,
				GroupId:   req.ToUserId,
				FromId:    req.UserId,
				IsRevoked: model.No,
				IsDeleted: model.No,
				Extra:     v.Extra,
				Quote:     "{}",
				SendTime:  now,
			})
		}

		if err := db.Create(items).Error; err == nil {
			// 更新该群在会话列表中的「最后一条消息」预览，与普通群消息一致
			recs := make([]model.TalkRecordExtraForwardRecord, 0, len(messageItems))
			for _, v := range messageItems {
				recs = append(recs, model.TalkRecordExtraForwardRecord{Content: PreviewText(v.MsgType, v.Extra)})
			}
			msgPreview := forwardPreviewFromRecords(recs, 80)
			msgTime := now.Format(time.DateTime)
			_ = s.MessageStorage.Set(ctx, entity.ChatGroupMode, req.UserId, req.ToUserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})

			err = s.PushMessage.MultiPush(ctx, entity.ImTopicChat,
				lo.Map(items, func(item model.TalkGroupMessage, index int) *entity.SubscribeMessage {
					return &entity.SubscribeMessage{
						Event: entity.SubEventImMessage,
						Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
							TalkMode: entity.ChatGroupMode,
							Message:  jsonutil.Encode(item),
						}),
					}
				}),
			)

			if err != nil {
				logger.Errorf("split forward message failed :%s", err.Error())
			} else {
				// Update unread count for all group members except the sender
				members := s.GroupMemberRepo.GetMembers(ctx, req.ToUserId)
				for _, member := range members {
					if member.UserId != req.UserId {
						s.UnreadStorage.Incr(ctx, member.UserId, entity.ChatGroupMode, req.ToUserId)
					}
				}
			}
		}
	} else {
		// 单条转发 → 私聊：对每条被转发的消息，只在 talk_user_message 中插入 1 条记录（UserId=0），
		// 再通过 WebSocket 给双方各推送一份视图，保证 storage 与普通私聊保持一致。

		// 为当前会话获取 session_id，确保历史消息接口能按 session_id 查询到这些转发消息
		sessionId := 0
		if s.TalkSessionRepo != nil {
			if sess, err := s.TalkSessionRepo.FindByWhere(ctx,
				"user_id = ? and receiver_id = ? and talk_mode = ?",
				req.UserId, req.ToUserId, entity.ChatPrivateMode,
			); err == nil && sess.Id > 0 {
				if sess.SessionId > 0 {
					sessionId = sess.SessionId
				} else {
					sessionId = sess.Id
				}
			}
		}

		s.ensurePrivateSession(ctx, req.UserId, req.ToUserId)

		// 构造最近一条转发摘要用于会话列表 msg_text
		recs := make([]model.TalkRecordExtraForwardRecord, 0, len(messageItems))
		for _, v := range messageItems {
			recs = append(recs, model.TalkRecordExtraForwardRecord{Content: PreviewText(v.MsgType, v.Extra)})
		}
		msgPreview := forwardPreviewFromRecords(recs, 80)
		msgTime := now.Format(time.DateTime)
		_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, req.UserId, req.ToUserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})
		_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, req.ToUserId, req.UserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})
		// 接收方未读 +1
		s.UnreadStorage.Incr(ctx, req.ToUserId, entity.ChatPrivateMode, req.UserId)

		// 逐条落库并推送：每条消息只存一条记录（UserId=0），但推送给双方各一份视图
		list := make([]*entity.SubscribeMessage, 0, len(messageItems)*2)
		for _, v := range messageItems {
			msgId := strutil.NewMsgId()

			record := &model.TalkUserMessage{
				MsgId:      strutil.NewMsgId(),
				OrgMsgId:   msgId,
				MsgType:    v.MsgType,
				UserId:     0, // 与普通私聊保持一致，逻辑所有者通过 session / 关系表处理
				ReceiverId: req.ToUserId,
				FromId:     req.UserId,
				SessionId:  sessionId,
				IsRevoked:  model.No,
				IsDeleted:  model.No,
				Extra:      v.Extra,
				Quote:      "{}",
				SendTime:   now,
			}

			if err := db.Create(record).Error; err != nil {
				logger.Errorf("split forward message save failed :%s", err.Error())
				continue
			}

			// 发送方视图
			sender := *record
			sender.UserId = req.UserId

			// 接收方视图
			receiver := *record
			receiver.UserId = req.ToUserId

			list = append(list,
				&entity.SubscribeMessage{
					Event: entity.SubEventImMessage,
					Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
						TalkMode: entity.ChatPrivateMode,
						Message:  jsonutil.Encode(sender),
					}),
				},
				&entity.SubscribeMessage{
					Event: entity.SubEventImMessage,
					Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
						TalkMode: entity.ChatPrivateMode,
						Message:  jsonutil.Encode(receiver),
					}),
				},
			)
		}

		if err := s.PushMessage.MultiPush(ctx, entity.ImTopicChat, list); err != nil {
			logger.Errorf("split forward message failed :%s", err.Error())
		}
	}

	return nil
}

// CombineForward 合并转发
func (s *Service) toCombineForward(ctx context.Context, req ForwardMessageOpt) error {
	var (
		now = time.Now()
		db  = s.Source.Db().WithContext(ctx)

		extra = model.TalkRecordExtraForward{
			TalkType:   req.TalkMode,
			UserId:     req.UserId,
			ReceiverId: req.ReceiverId,
			MsgIds:     req.MsgIds,
			Records:    make([]model.TalkRecordExtraForwardRecord, 0),
		}
		pushMessageItems = make([]entity.SubEventImMessagePayload, 0)
	)

	sources, err := s.loadForwardSources(ctx, req)
	if err != nil {
		return err
	}
	sources, err = s.flattenForwardSources(ctx, sources, 0)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("没有可转发的消息")
	}

	// 展平后指向真实内容消息，再次打开/再次转发都能命中
	leafMsgIds := make([]string, 0, len(sources))
	for _, v := range sources {
		leafMsgIds = append(leafMsgIds, v.MsgId)
	}
	extra.MsgIds = leafMsgIds
	extra.TalkType = sources[0].TalkType
	if extra.TalkType == 0 {
		extra.TalkType = req.TalkMode
	}

	uids := make([]int, 0, len(sources))
	for _, v := range sources {
		uids = append(uids, v.FromId)
	}
	userNameItems, err := s.findUserNameList(ctx, uids)
	if err != nil {
		return err
	}
	limit := 3
	if len(sources) < limit {
		limit = len(sources)
	}
	for _, v := range sources[:limit] {
		extra.Records = append(extra.Records, model.TalkRecordExtraForwardRecord{
			Nickname: userNameItems[v.FromId],
			Content:  PreviewText(v.MsgType, v.Extra),
		})
	}

	switch req.ToUserIdType {
	case entity.ChatPrivateMode: // 好友发送消息
		// 私聊合并转发：与普通私聊、单条转发保持一致，只在 talk_user_message 中插入 1 条记录（UserId=0），
		// 再通过 WebSocket 推送给双方各一份视图。

		// 为目标会话获取 session_id，便于历史消息接口按 session_id 查询到这条合并转发消息
		sessionId := 0
		if s.TalkSessionRepo != nil {
			if sess, err := s.TalkSessionRepo.FindByWhere(ctx,
				"user_id = ? and receiver_id = ? and talk_mode = ?",
				req.UserId, req.ToUserId, entity.ChatPrivateMode,
			); err == nil && sess.Id > 0 {
				if sess.SessionId > 0 {
					sessionId = sess.SessionId
				} else {
					sessionId = sess.Id
				}
			}
		}

		s.ensurePrivateSession(ctx, req.UserId, req.ToUserId)

		// 更新会话「最后一条消息」缓存，会话列表 msg_text 显示合并转发摘要
		msgPreview := forwardPreviewFromRecords(extra.Records, 80)
		msgTime := now.Format(time.DateTime)
		_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, req.UserId, req.ToUserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})
		_ = s.MessageStorage.Set(ctx, entity.ChatPrivateMode, req.ToUserId, req.UserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})
		// 接收方未读数 +1
		s.UnreadStorage.Incr(ctx, req.ToUserId, entity.ChatPrivateMode, req.UserId)

		// 实际落库：只存一条合并转发记录（UserId=0）
		msgId := strutil.NewMsgId()
		record := &model.TalkUserMessage{
			MsgId:      strutil.NewMsgId(),
			OrgMsgId:   msgId,
			MsgType:    entity.ChatMsgTypeForward,
			UserId:     0, // 逻辑所有权通过 session/关系控制
			ReceiverId: req.ToUserId,
			FromId:     req.UserId,
			SessionId:  sessionId,
			IsRevoked:  model.No,
			IsDeleted:  model.No,
			Extra:      jsonutil.Encode(extra),
			Quote:      "{}",
			SendTime:   now,
		}

		if err := db.Create(record).Error; err != nil {
			return err
		}

		// 构造发送方/接收方视图并推送
		sender := *record
		sender.UserId = req.UserId

		receiver := *record
		receiver.UserId = req.ToUserId

		pushMessageItems = append(pushMessageItems,
			entity.SubEventImMessagePayload{
				TalkMode: entity.ChatPrivateMode,
				Message:  jsonutil.Encode(sender),
			},
			entity.SubEventImMessagePayload{
				TalkMode: entity.ChatPrivateMode,
				Message:  jsonutil.Encode(receiver),
			},
		)

	case entity.ChatGroupMode: // 向群发送消息
		record := model.TalkGroupMessage{
			MsgId:     strutil.NewMsgId(),
			MsgType:   entity.ChatMsgTypeForward,
			GroupId:   req.ToUserId,
			FromId:    req.UserId,
			IsRevoked: model.No,
			IsDeleted: model.No,
			Extra:     jsonutil.Encode(extra),
			Quote:     "{}",
			SendTime:  now,
		}

		if err := db.Create(&record).Error; err != nil {
			return err
		}

		// 更新该群在会话列表中的「最后一条消息」预览（合并转发摘要）
		msgPreview := forwardPreviewFromRecords(extra.Records, 80)
		msgTime := now.Format(time.DateTime)
		_ = s.MessageStorage.Set(ctx, entity.ChatGroupMode, req.UserId, req.ToUserId, &cache.LastCacheMessage{Content: msgPreview, Datetime: msgTime})

		pushMessageItems = append(pushMessageItems, entity.SubEventImMessagePayload{
			TalkMode: entity.ChatGroupMode,
			Message:  jsonutil.Encode(record),
		})
	}

	if len(pushMessageItems) > 0 {
		err := s.PushMessage.MultiPush(ctx,
			entity.ImTopicChat,
			lo.Map(pushMessageItems, func(item entity.SubEventImMessagePayload, index int) *entity.SubscribeMessage {
				return &entity.SubscribeMessage{
					Event: entity.SubEventImMessage,
					Payload: jsonutil.Encode(entity.SubEventImMessagePayload{
						TalkMode: item.TalkMode,
						Message:  item.Message,
					}),
				}
			}),
		)

		if err != nil {
			logger.Errorf("forward message failed :%s", err.Error())
		}
	}

	return nil
}

// loadForwardSources 从源会话加载待转发消息
func (s *Service) loadForwardSources(ctx context.Context, req ForwardMessageOpt) ([]forwardSourceItem, error) {
	db := s.Source.Db().WithContext(ctx)
	out := make([]forwardSourceItem, 0, len(req.MsgIds))

	if req.TalkMode == entity.ChatGroupMode {
		records := make([]model.TalkGroupMessage, 0)
		err := db.Model(&model.TalkGroupMessage{}).
			Where("group_id = ? and msg_id in ?", req.ReceiverId, req.MsgIds).
			Order("id asc").
			Scan(&records).Error
		if err != nil {
			return nil, err
		}
		for _, v := range records {
			out = append(out, forwardSourceItem{
				MsgId:    v.MsgId,
				MsgType:  v.MsgType,
				Extra:    v.Extra,
				FromId:   v.FromId,
				TalkType: entity.ChatGroupMode,
			})
		}
		return out, nil
	}

	records := make([]model.TalkUserMessage, 0)
	var err error
	if s.TalkSessionRepo != nil {
		session, e := s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", req.UserId, req.ReceiverId, entity.ChatPrivateMode)
		if e != nil {
			return nil, e
		}
		err = db.Model(&model.TalkUserMessage{}).
			Where("session_id = ? and msg_id in ?", session.SessionId, req.MsgIds).
			Order("id asc").
			Scan(&records).Error
	} else {
		err = db.Model(&model.TalkUserMessage{}).
			Where("user_id = ? and receiver_id = ? and msg_id in ?", req.UserId, req.ReceiverId, req.MsgIds).
			Order("id asc").
			Scan(&records).Error
	}
	if err != nil {
		return nil, err
	}
	for _, v := range records {
		out = append(out, forwardSourceItem{
			MsgId:    v.MsgId,
			MsgType:  v.MsgType,
			Extra:    v.Extra,
			FromId:   v.FromId,
			TalkType: entity.ChatPrivateMode,
		})
	}
	return out, nil
}

// flattenForwardSources 将嵌套的转发气泡展开为实际内容消息，保证再次转发后仍可打开
func (s *Service) flattenForwardSources(ctx context.Context, items []forwardSourceItem, depth int) ([]forwardSourceItem, error) {
	if depth >= 5 || len(items) == 0 {
		return items, nil
	}

	out := make([]forwardSourceItem, 0, len(items))
	for _, item := range items {
		if item.MsgType != entity.ChatMsgTypeForward {
			out = append(out, item)
			continue
		}

		var extra model.TalkRecordExtraForward
		if err := jsonutil.Unmarshal(item.Extra, &extra); err != nil || len(extra.MsgIds) == 0 {
			out = append(out, item)
			continue
		}

		nested, err := s.loadMessagesByMsgIds(ctx, extra.TalkType, extra.MsgIds)
		if err != nil {
			return nil, err
		}
		if len(nested) == 0 {
			out = append(out, item)
			continue
		}
		nested, err = s.flattenForwardSources(ctx, nested, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, nested...)
	}
	return out, nil
}

// loadMessagesByMsgIds 按 msg_id 加载消息（优先 talkType，缺的再查另一张表）
func (s *Service) loadMessagesByMsgIds(ctx context.Context, talkType int, msgIds []string) ([]forwardSourceItem, error) {
	if len(msgIds) == 0 {
		return nil, nil
	}
	db := s.Source.Db().WithContext(ctx)
	out := make([]forwardSourceItem, 0, len(msgIds))
	found := make(map[string]struct{})

	appendPrivate := func(ids []string) error {
		rows := make([]model.TalkUserMessage, 0)
		if err := db.Model(&model.TalkUserMessage{}).
			Where("msg_id in ? and is_deleted = ? and is_revoked <> ?", ids, model.No, model.TalkMsgRevokedHidden).
			Order("id asc").
			Scan(&rows).Error; err != nil {
			return err
		}
		for _, v := range rows {
			if _, ok := found[v.MsgId]; ok {
				continue
			}
			found[v.MsgId] = struct{}{}
			out = append(out, forwardSourceItem{
				MsgId:    v.MsgId,
				MsgType:  v.MsgType,
				Extra:    v.Extra,
				FromId:   v.FromId,
				TalkType: entity.ChatPrivateMode,
			})
		}
		return nil
	}
	appendGroup := func(ids []string) error {
		rows := make([]model.TalkGroupMessage, 0)
		if err := db.Model(&model.TalkGroupMessage{}).
			Where("msg_id in ? and is_deleted = ? and is_revoked <> ?", ids, model.No, model.TalkMsgRevokedHidden).
			Order("id asc").
			Scan(&rows).Error; err != nil {
			return err
		}
		for _, v := range rows {
			if _, ok := found[v.MsgId]; ok {
				continue
			}
			found[v.MsgId] = struct{}{}
			out = append(out, forwardSourceItem{
				MsgId:    v.MsgId,
				MsgType:  v.MsgType,
				Extra:    v.Extra,
				FromId:   v.FromId,
				TalkType: entity.ChatGroupMode,
			})
		}
		return nil
	}

	var err error
	if talkType == entity.ChatGroupMode {
		err = appendGroup(msgIds)
	} else {
		err = appendPrivate(msgIds)
	}
	if err != nil {
		return nil, err
	}

	if len(out) < len(msgIds) {
		missing := make([]string, 0)
		for _, id := range msgIds {
			if _, ok := found[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			if talkType == entity.ChatGroupMode {
				err = appendPrivate(missing)
			} else {
				err = appendGroup(missing)
			}
			if err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// ensurePrivateSession 确保私聊双方在 talk_session 中都有会话记录，否则对方会话栏不显示
func (s *Service) ensurePrivateSession(ctx context.Context, uidA, uidB int) {
	if s.TalkSessionRepo == nil {
		return
	}
	for _, pair := range []struct{ uid, rid int }{{uidA, uidB}, {uidB, uidA}} {
		_, err := s.TalkSessionRepo.FindByWhere(ctx, "user_id = ? and receiver_id = ? and talk_mode = ?", pair.uid, pair.rid, entity.ChatPrivateMode)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.TalkSessionRepo.Create(ctx, &model.TalkSession{
				UserId:     pair.uid,
				ReceiverId: pair.rid,
				TalkMode:   entity.ChatPrivateMode,
				IsTop:      model.No,
				IsDelete:   model.No,
				IsDisturb:  model.No,
				IsRobot:    model.No,
			})
		}
	}
}

func (s *Service) findUserNameList(ctx context.Context, uids []int) (map[int]string, error) {
	users := make([]model.Users, 0)

	err := s.Source.Db().WithContext(ctx).Find(&users, "id in ?", uids).Error
	if err != nil {
		return nil, err
	}

	items := make(map[int]string)
	for _, v := range users {
		items[v.Id] = v.Nickname
	}

	return items, nil
}

// PreviewText 会话列表最后一条消息摘要（与 talk session-list 的 MsgText 一致）
func PreviewText(msgType int, extra string) string {
	switch msgType {
	case entity.ChatMsgTypeText:
		data := model.TalkRecordExtraText{}
		if err := jsonutil.Unmarshal(extra, &data); err != nil {
			return ""
		}

		return strutil.MtSubstr(data.Content, 0, 200)
	case entity.ChatMsgSysRetainDaysSet:
		var data struct {
			RetainDays int `json:"retain_days"`
		}
		if err := jsonutil.Unmarshal(extra, &data); err != nil {
			return entity.ChatMsgSysRetainDaysSetPreview(0)
		}
		return entity.ChatMsgSysRetainDaysSetPreview(data.RetainDays)
	case entity.ChatMsgTypeCmd:
		data := model.TalkRecordExtraCmd{}
		if err := jsonutil.Unmarshal(extra, &data); err != nil {
			return entity.ChatMsgTypeMapping[entity.ChatMsgTypeCmd]
		}
		if strings.TrimSpace(data.Intro) != "" {
			return strutil.MtSubstr(strings.TrimSpace(data.Intro), 0, 200)
		}
		return entity.ChatMsgTypeMapping[entity.ChatMsgTypeCmd]
	case entity.ChatMsgTypeGroupNotice:
		prefix := entity.ChatMsgTypeMapping[entity.ChatMsgTypeGroupNotice]
		data := model.TalkRecordExtraGroupNotice{}
		if err := jsonutil.Unmarshal(extra, &data); err != nil {
			return prefix
		}
		content := strings.TrimSpace(data.Content)
		if content == "" {
			return prefix
		}
		// 去掉换行，会话列表单行展示
		content = strings.ReplaceAll(content, "\r\n", " ")
		content = strings.ReplaceAll(content, "\n", " ")
		content = strings.ReplaceAll(content, "\r", " ")
		return prefix + strutil.MtSubstr(content, 0, 50)
	default:
		if value, ok := entity.ChatMsgTypeMapping[msgType]; ok {
			return value
		}
	}

	return "未知消息"
}

// forwardPreviewFromRecords 根据转发条目的 Records 生成会话列表展示的摘要（替换纯"[转发消息]"）
func forwardPreviewFromRecords(records []model.TalkRecordExtraForwardRecord, maxLen int) string {
	if len(records) == 0 {
		return "[转发消息]"
	}
	parts := make([]string, 0, len(records))
	for _, r := range records {
		c := strings.TrimSpace(r.Content)
		if c != "" {
			parts = append(parts, c)
		}
	}
	if len(parts) == 0 {
		return "[转发消息]"
	}
	preview := "转发：" + strings.Join(parts, "、")
	return strutil.MtSubstr(preview, 0, maxLen)
}
