package v1

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service/message"
)

func merchantChatPeerUserID(sess *model.MerchantSession) int {
	return sess.FriendId
}

func (u *User) setMerchantChatLastMessage(ctx context.Context, uid, inboxSessionID int, msgType int, extra string, sendTime time.Time) {
	if u.MessageStorage == nil || inboxSessionID <= 0 {
		return
	}
	preview := message.PreviewText(msgType, extra)
	msgTime := sendTime.Format(time.DateTime)
	if sendTime.IsZero() {
		msgTime = timeutil.FormatDatetime(time.Now())
	}
	last := &cache.LastCacheMessage{Content: preview, Datetime: msgTime}
	_ = u.MessageStorage.Set(ctx, entity.ChatMerchantMode, uid, inboxSessionID, last)
}

func (u *User) merchantChatLastPreview(ctx context.Context, uid, sessionID int, fallback time.Time) (preview, updatedAt string) {
	preview = "..."
	updatedAt = timeutil.FormatDatetime(fallback)
	if u.MessageStorage == nil || sessionID <= 0 {
		return preview, updatedAt
	}
	if msg, err := u.MessageStorage.Get(ctx, entity.ChatMerchantMode, uid, sessionID); err == nil && msg != nil {
		if msg.Content != "" {
			preview = msg.Content
		}
		if strings.TrimSpace(msg.Datetime) != "" {
			updatedAt = msg.Datetime
		}
	}
	return preview, updatedAt
}

func (u *User) assertMerchantChatOrder(ctx context.Context, orderID int, uid int) (*model.MerchantOrder, error) {
	o, err := u.MerchantOrderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if o.BuyerId != uid && o.SalerId != uid {
		return nil, errorx.New(403, "无权参与该订单对话")
	}
	return o, nil
}

// assertMerchantChatOrderByOrderNo 按订单号校验：须为买卖双方之一；任务发布人须与卖方一致（与下单逻辑一致）
func (u *User) assertMerchantChatOrderByOrderNo(ctx context.Context, orderNo string, uid int) (*model.MerchantOrder, error) {
	o, err := u.MerchantOrderRepo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errorx.New(404, "订单不存在")
	}
	if o.BuyerId != uid && o.SalerId != uid {
		return nil, errorx.New(403, "无权查看该订单")
	}
	task, err := u.MerchantTaskRepo.FindByID(ctx, o.TaskId)
	if err != nil {
		return nil, err
	}
	if task != nil && task.UserId != o.SalerId {
		return nil, errorx.New(400, "订单与任务发布人不一致")
	}
	return o, nil
}

func (u *User) assertMerchantSessionOwner(sess *model.MerchantSession, uid int) error {
	if sess == nil {
		return errorx.New(404, "会话不存在")
	}
	if sess.Status != 1 {
		return errorx.New(400, "会话不可用")
	}
	if sess.DeleteTime != 0 {
		return errorx.New(400, "会话已删除")
	}
	if sess.InviterId != uid {
		return errorx.New(403, "无权访问该会话")
	}
	return nil
}

func (u *User) merchantChatSessionUnread(ctx context.Context, uid, sessionID int) int {
	if u.UnreadStorage == nil {
		return 0
	}
	return u.UnreadStorage.Get(ctx, uid, entity.ChatMerchantMode, sessionID)
}

func merchantChatPushPayload(orderID int, inboxUID, inboxSessionID int, msgJSON string) string {
	return jsonutil.Encode(entity.SubEventImMessageMerchantPayload{
		InboxUserId:    inboxUID,
		InboxSessionId: inboxSessionID,
		OrderId:        orderID,
		Message:        msgJSON,
	})
}

// MerchantChatSend 发送商户 C2C 消息（参考私聊：双方各一条会话栏，共享 session_id 关联消息）
func (u *User) MerchantChatSend(ctx context.Context, in *web.MerchantChatSendRequest) (*web.MerchantChatSendResponse, error) {
	if u.PushMessage == nil || u.UnreadStorage == nil {
		return nil, errorx.New(500, "消息推送或未读组件未初始化")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	orderID := int(in.GetOrderId())
	ord, err := u.assertMerchantChatOrder(ctx, orderID, uid)
	if err != nil {
		return nil, err
	}

	peer := ord.SalerId
	if uid == ord.SalerId {
		peer = ord.BuyerId
	}

	mine, peerSess, chatSessionID, err := u.MerchantSessionRepo.EnsurePair(ctx, uid, peer, orderID)
	if err != nil {
		return nil, err
	}
	if err := u.assertMerchantSessionOwner(mine, uid); err != nil {
		return nil, err
	}

	msgID := strings.TrimSpace(in.GetMsgId())
	if msgID == "" {
		msgID = strutil.NewMsgId()
	}
	quote := strings.TrimSpace(in.GetQuote())
	if quote == "" {
		quote = "{}"
	}
	now := time.Now()
	row := &model.MerchantMessage{
		MsgId:      msgID,
		OrgMsgId:   msgID,
		SessionId:  chatSessionID,
		MsgType:    int(in.GetMsgType()),
		UserId:     uid,
		ReceiverId: peer,
		FromId:     uid,
		IsRevoked:  model.No,
		IsDeleted:  model.No,
		Extra:      in.GetExtra(),
		Quote:      quote,
		SendTime:   now,
	}
	if err := u.MerchantMessageRepo.Create(ctx, row); err != nil {
		return nil, err
	}

	message.TryOneSignalChatPush(ctx, u.UsersRepo, u.NoticeTemplateRepo, nil, peer, 0, 0, model.NoticeTemplateFlagC2cChat)

	msgJSON, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	_ = u.PushMessage.MultiPush(ctx, entity.ImTopicChat, []*entity.SubscribeMessage{
		{Event: entity.SubEventImMessageMerchant, Payload: merchantChatPushPayload(orderID, uid, mine.Id, string(msgJSON))},
		{Event: entity.SubEventImMessageMerchant, Payload: merchantChatPushPayload(orderID, peer, peerSess.Id, string(msgJSON))},
	})
	u.UnreadStorage.Incr(ctx, peer, entity.ChatMerchantMode, peerSess.Id)
	u.setMerchantChatLastMessage(ctx, uid, mine.Id, row.MsgType, row.Extra, row.SendTime)
	u.setMerchantChatLastMessage(ctx, peer, peerSess.Id, row.MsgType, row.Extra, row.SendTime)

	return &web.MerchantChatSendResponse{
		MessageDbId: row.Id,
		MsgId:       row.MsgId,
		SessionId:   int32(mine.Id),
	}, nil
}

// MerchantChatUnread 当前用户与订单对手方唯一活跃会话的未读数
func (u *User) MerchantChatUnread(ctx context.Context, in *web.MerchantChatUnreadRequest) (*web.MerchantChatUnreadResponse, error) {
	if u.UnreadStorage == nil {
		return nil, errorx.New(500, "未读组件未初始化")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	orderNo := strings.TrimSpace(in.GetOrderNo())
	if orderNo == "" {
		return nil, errorx.New(400, "请传入订单号 order_no")
	}
	ord, err := u.assertMerchantChatOrderByOrderNo(ctx, orderNo, uid)
	if err != nil {
		return nil, err
	}
	peer := ord.SalerId
	if uid == ord.SalerId {
		peer = ord.BuyerId
	}
	sess, err := u.MerchantSessionRepo.FindActiveByOwnerPeer(ctx, uid, peer)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return &web.MerchantChatUnreadResponse{TotalUnread: 0}, nil
	}
	n := u.UnreadStorage.Get(ctx, uid, entity.ChatMerchantMode, sess.Id)
	return &web.MerchantChatUnreadResponse{TotalUnread: int32(n)}, nil
}

// MerchantChatSessionList 商户对话列表（inviter_id=当前用户 且 delete_time=0）
func (u *User) MerchantChatSessionList(ctx context.Context, _ *web.MerchantChatSessionListRequest) (*web.MerchantChatSessionListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	rows, err := u.MerchantSessionRepo.ListVisibleByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := &web.MerchantChatSessionListResponse{Items: make([]*web.MerchantChatSessionItem, 0, len(rows))}
	peerIDs := make([]int, 0, len(rows))
	for i := range rows {
		peerIDs = append(peerIDs, merchantChatPeerUserID(&rows[i]))
	}
	merchantNickMap, _ := u.merchantNicknamesByUserIDs(ctx, peerIDs)
	for i := range rows {
		peerID := merchantChatPeerUserID(&rows[i])
		oid, _ := strconv.Atoi(strings.TrimSpace(rows[i].Tags))
		peer, err := u.UsersRepo.FindByIdWithCache(ctx, peerID)
		nick, ava := "", ""
		if err == nil && peer != nil {
			nick = peer.Nickname
			ava = peer.Avatar
		}
		preview, updatedAt := u.merchantChatLastPreview(ctx, uid, rows[i].Id, rows[i].UpdatedAt)
		out.Items = append(out.Items, &web.MerchantChatSessionItem{
			SessionId:        int32(rows[i].Id),
			OrderId:          int32(oid),
			PeerUserId:       int32(peerID),
			PeerNickname:     nick,
			PeerAvatar:       ava,
			MerchantNickname: merchantNickMap[peerID],
			LastPreview:      preview,
			UpdatedAt:        updatedAt,
			UnreadNum:        int32(u.merchantChatSessionUnread(ctx, uid, rows[i].Id)),
		})
	}
	return out, nil
}

// MerchantChatMessageList 会话内消息（参考 /api/v1/message/records：receiver_id + cursor + limit）
func (u *User) MerchantChatMessageList(ctx context.Context, in *web.MerchantChatMessageListRequest) (*web.MerchantChatMessageListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	receiverID := int(in.GetReceiverId())
	sess, err := u.MerchantSessionRepo.FindActiveByOwnerPeer(ctx, uid, receiverID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return &web.MerchantChatMessageListResponse{Items: []*web.MerchantChatMessageItem{}}, nil
	}
	if err := u.assertMerchantSessionOwner(sess, uid); err != nil {
		return nil, err
	}
	chatSID := repo.ChatSessionID(sess)
	limit := int(in.GetLimit())
	if limit <= 0 {
		limit = 30
	}
	rows, err := u.MerchantMessageRepo.ListBySessionBeforeCursor(ctx, chatSID, int64(in.GetCursor()), limit)
	if err != nil {
		return nil, err
	}
	inboxSID := sess.Id
	items := make([]*web.MerchantChatMessageItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		items = append(items, &web.MerchantChatMessageItem{
			Id:         r.Id,
			MsgId:      r.MsgId,
			OrgMsgId:   r.OrgMsgId,
			SessionId:  int32(inboxSID),
			MsgType:    int32(r.MsgType),
			UserId:     int32(r.UserId),
			ReceiverId: int32(r.ReceiverId),
			FromId:     int32(r.FromId),
			IsRevoked:  int32(r.IsRevoked),
			IsDeleted:  int32(r.IsDeleted),
			Extra:      r.Extra,
			Quote:      r.Quote,
			SendTime:   timeutil.FormatDatetime(r.SendTime),
			CreatedAt:  timeutil.FormatDatetime(r.CreatedAt),
		})
	}
	var nextCursor int32
	if n := len(rows); n > 0 {
		if n >= limit {
			nextCursor = int32(rows[n-1].Id)
		}
	}
	return &web.MerchantChatMessageListResponse{Items: items, Cursor: nextCursor}, nil
}

// MerchantChatClearUnread 清除当前用户在指定商户会话下的未读计数（Redis），并推送 im.session.unread.cleared（talk_mode=3，receiver_id 为 session_id）
func (u *User) MerchantChatClearUnread(ctx context.Context, in *web.MerchantChatClearUnreadRequest) (*web.MerchantChatClearUnreadResponse, error) {
	if u.UnreadStorage == nil {
		return nil, errorx.New(500, "未读组件未初始化")
	}
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	sid := int(in.GetSessionId())
	sess, err := u.MerchantSessionRepo.FindByID(ctx, sid)
	if err != nil {
		return nil, err
	}
	if err := u.assertMerchantSessionOwner(sess, uid); err != nil {
		return nil, err
	}
	u.UnreadStorage.Reset(ctx, uid, entity.ChatMerchantMode, sid)
	if u.PushMessage != nil {
		_ = u.PushMessage.Push(ctx, entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventImSessionUnreadCleared,
			Payload: jsonutil.Encode(entity.SubEventImSessionUnreadClearedPayload{
				UserId:     uid,
				TalkMode:   entity.ChatMerchantMode,
				ReceiverId: sid,
				UnreadNum:  0,
			}),
		})
	}
	return &web.MerchantChatClearUnreadResponse{}, nil
}
