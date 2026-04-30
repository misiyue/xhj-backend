package comet

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/jsonutil"
	"github.com/gzydong/go-chat/internal/pkg/longnet"
	"github.com/gzydong/go-chat/internal/pkg/server"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/tidwall/gjson"
)

var _ longnet.IHandler = (*Handler)(nil)

type Handler struct {
	UserClient  *cache.UserClient
	PushMessage *logic.PushMessage
}

// OnOpen 链接建立成功
func (h *Handler) OnOpen(smg longnet.ISessionManager, s longnet.ISession) {
	if err := h.UserClient.Bind(context.Background(), server.ID(), s.ConnId(), s.UserId()); err != nil {
		_ = s.Close()
		return
	}

	_ = s.Write([]byte(fmt.Sprintf(`{"event":"connect","payload":{"ping_interval":%d,"ping_timeout":%d}}`, smg.Options().PingInterval, smg.Options().PingTimeout)))
}

// OnMessage 接收到消息
func (h *Handler) OnMessage(smg longnet.ISessionManager, c longnet.ISession, message []byte) {
	event := gjson.GetBytes(message, "event").String()
	if event == "" {
		event = gjson.GetBytes(message, "type").String()
	}
	if event == "" {
		event = gjson.GetBytes(message, "cmd").String()
	}

	switch event {
	case "ping":
		_ = h.UserClient.Bind(context.Background(), server.ID(), c.ConnId(), c.UserId())
		_ = c.Write([]byte(`{"event":"pong"}`))

	case "im.message.keyboard":
		_ = h.PushMessage.Push(context.Background(), entity.ImTopicChat, &entity.SubscribeMessage{
			Event: entity.SubEventImMessageKeyboard,
			Payload: jsonutil.Encode(entity.SubEventImMessageKeyboardPayload{
				FromId:     int(c.UserId()),
				ReceiverId: int(gjson.GetBytes(message, "payload.receiver_id").Int()),
			}),
		})

	case "im.call.invite", "im.call.accept", "im.call.reject", "im.call.hangup", "im.call.cancel":
		subEvent := ""
		switch event {
		case "im.call.invite":
			subEvent = entity.SubEventImCallInvite
		case "im.call.accept":
			subEvent = entity.SubEventImCallAccept
		case "im.call.reject":
			subEvent = entity.SubEventImCallReject
		case "im.call.hangup":
			subEvent = entity.SubEventImCallHangup
		case "im.call.cancel":
			subEvent = entity.SubEventImCallCancel
		}

		// Parse fields from frontend payload (uses to_user_id and call_type)
		// Note: accept, reject, and hangup events may not include call_type, from_user_name, from_user_avatar
		toId := int(gjson.GetBytes(message, "payload.to_user_id").Int())
		roomId := int(gjson.GetBytes(message, "payload.room_id").Int())
		callType := int(gjson.GetBytes(message, "payload.call_type").Int())            // Optional: only in invite
		fromUserName := gjson.GetBytes(message, "payload.from_user_name").String()     // Optional: only in invite
		fromUserAvatar := gjson.GetBytes(message, "payload.from_user_avatar").String() // Optional: only in invite

		slog.Info("[CallEvent] Received from WebSocket", "event", event, "from_user_id", c.UserId(), "to_user_id", toId, "room_id", roomId, "call_type", callType)

		err := h.PushMessage.Push(context.Background(), entity.ImTopicChat, &entity.SubscribeMessage{
			Event: subEvent,
			Payload: jsonutil.Encode(entity.SubEventImCallPayload{
				FromId:         int(c.UserId()),
				ToId:           toId,
				RoomId:         roomId,
				CallType:       callType,
				FromUserName:   fromUserName,
				FromUserAvatar: fromUserAvatar,
			}),
		})

		if err != nil {
			slog.Error("[CallEvent] Failed to push to Redis", "error", err, "event", event, "from_user_id", c.UserId(), "to_user_id", toId)
		} else {
			slog.Info("[CallEvent] Successfully pushed to Redis", "event", event, "from_user_id", c.UserId(), "to_user_id", toId)
		}
	}
}

// OnClose 链接关闭
func (h *Handler) OnClose(cid int64, uid int64) {
	if err := h.UserClient.UnBind(context.Background(), server.ID(), cid, uid); err != nil {
		slog.Error("unbind error", "error", err)
	}
}
