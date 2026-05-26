package v1

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
)

func merchantChatPeerUserID(sess *model.MerchantSession, uid int) int {
	if sess.InviterId == uid {
		return sess.FriendId
	}
	return sess.InviterId
}

func merchantChatExtraPreview(extra string) string {
	s := strings.TrimSpace(extra)
	if s == "" {
		return ""
	}
	const maxRune = 80
	if utf8.RuneCountInString(s) <= maxRune {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxRune {
		return string(runes[:maxRune]) + "…"
	}
	return s
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

func (u *User) assertMerchantSessionParticipant(sess *model.MerchantSession, uid int) error {
	if sess == nil {
		return errorx.New(404, "会话不存在")
	}
	if sess.Status != 1 {
		return errorx.New(400, "会话不可用")
	}
	if sess.InviterId != uid && sess.FriendId != uid {
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

// MerchantChatSend 发送商户 C2C 消息（首条自动创建 merchant_session）
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

	sess, err := u.MerchantSessionRepo.FindByOrderTag(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		sess = &model.MerchantSession{
			InviterId:  ord.BuyerId,
			FriendId:   ord.SalerId,
			IsTop:      2,
			Status:     1,
			Tags:       model.MerchantSessionOrderTag(orderID),
			DeleteTime: 0,
		}
		if err := u.MerchantSessionRepo.Create(ctx, sess); err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				sess, err = u.MerchantSessionRepo.FindByOrderTag(ctx, orderID)
				if err != nil || sess == nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	if err := u.assertMerchantSessionParticipant(sess, uid); err != nil {
		return nil, err
	}

	peer := merchantChatPeerUserID(sess, uid)
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
		SessionId:  sess.Id,
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
	_ = u.MerchantSessionRepo.TouchUpdatedAt(ctx, sess.Id)

	msgJSON, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	sub := entity.SubEventImMessageMerchantPayload{
		OrderId: orderID,
		Message: string(msgJSON),
	}
	sub.InboxUserId = uid
	p1 := jsonutil.Encode(sub)
	sub.InboxUserId = peer
	p2 := jsonutil.Encode(sub)
	_ = u.PushMessage.MultiPush(ctx, entity.ImTopicChat, []*entity.SubscribeMessage{
		{Event: entity.SubEventImMessageMerchant, Payload: p1},
		{Event: entity.SubEventImMessageMerchant, Payload: p2},
	})
	u.UnreadStorage.Incr(ctx, peer, entity.ChatMerchantMode, sess.Id)

	return &web.MerchantChatSendResponse{
		MessageDbId: row.Id,
		MsgId:       row.MsgId,
		SessionId:   int32(sess.Id),
	}, nil
}

// MerchantChatUnread 指定订单（order_no）对应商户会话的未读数；会话按订单主键 id 与 merchant_session.tags 关联，买卖双方即买家与任务发布人（卖方）
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
	sess, err := u.MerchantSessionRepo.FindByOrderTag(ctx, ord.Id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return &web.MerchantChatUnreadResponse{TotalUnread: 0}, nil
	}
	n := u.UnreadStorage.Get(ctx, uid, entity.ChatMerchantMode, sess.Id)
	return &web.MerchantChatUnreadResponse{TotalUnread: int32(n)}, nil
}

// MerchantChatSessionList 商户对话列表（无分页，按更新时间倒序）。
// 数据来自 merchant_session：inviter_id 或 friend_id 为当前用户且 status=展示。
func (u *User) MerchantChatSessionList(ctx context.Context, _ *web.MerchantChatSessionListRequest) (*web.MerchantChatSessionListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	rows, err := u.MerchantSessionRepo.ListVisibleByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := &web.MerchantChatSessionListResponse{Items: make([]*web.MerchantChatSessionItem, 0, len(rows))}
	for i := range rows {
		oid, _ := strconv.Atoi(strings.TrimSpace(rows[i].Tags))
		peerID := merchantChatPeerUserID(&rows[i], uid)
		peer, err := u.UsersRepo.FindByIdWithCache(ctx, peerID)
		nick, ava := "", ""
		if err == nil && peer != nil {
			nick = peer.Nickname
			ava = peer.Avatar
		}
		preview := ""
		if lm, err := u.MerchantMessageRepo.FindLatestBySession(ctx, rows[i].Id); err == nil && lm != nil {
			preview = merchantChatExtraPreview(lm.Extra)
		}
		out.Items = append(out.Items, &web.MerchantChatSessionItem{
			SessionId:    int32(rows[i].Id),
			OrderId:      int32(oid),
			PeerUserId:   int32(peerID),
			PeerNickname: nick,
			PeerAvatar:   ava,
			LastPreview:  preview,
			UpdatedAt:    timeutil.FormatDatetime(rows[i].UpdatedAt),
			UnreadNum:    int32(u.merchantChatSessionUnread(ctx, uid, rows[i].Id)),
		})
	}
	return out, nil
}

// MerchantChatMessageList 会话内消息分页（id 倒序，最新在前）
func (u *User) MerchantChatMessageList(ctx context.Context, in *web.MerchantChatMessageListRequest) (*web.MerchantChatMessageListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := int(session.UserId)
	sid := int(in.GetSessionId())
	sess, err := u.MerchantSessionRepo.FindByID(ctx, sid)
	if err != nil {
		return nil, err
	}
	if err := u.assertMerchantSessionParticipant(sess, uid); err != nil {
		return nil, err
	}
	page, pageSize := normMerchantTaskPage(int(in.GetPage()), int(in.GetPageSize()))
	rows, total, err := u.MerchantMessageRepo.ListBySessionDesc(ctx, sid, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*web.MerchantChatMessageItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		items = append(items, &web.MerchantChatMessageItem{
			Id:         r.Id,
			MsgId:      r.MsgId,
			OrgMsgId:   r.OrgMsgId,
			SessionId:  int32(r.SessionId),
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
	tot := int32(total)
	if total > 1<<31-1 {
		tot = 1<<31 - 1
	}
	return &web.MerchantChatMessageListResponse{Items: items, Total: tot}, nil
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
	if err := u.assertMerchantSessionParticipant(sess, uid); err != nil {
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
