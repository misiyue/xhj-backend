package consume

import (
	"context"
	"encoding/json"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/logic"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/pkg/longnet"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/gzydong/go-chat/internal/service/message"
)

var handlers map[string]func(ctx context.Context, data []byte)

type Handler struct {
	Config             *config.Config
	OrganizeRepo       *repo.Organize
	UserRepo           *repo.Users
	Source             *repo.Source
	TalkRecordsService service.ITalkRecordService
	ContactService     service.IContactService
	MessageService     message.IService
	TalkService        service.ITalkService
	PushMessage        *logic.PushMessage
	serv               longnet.IServer `wire:"-"`
	GroupMemberRepo    *repo.GroupMember
}

func (h *Handler) init() {
	handlers = make(map[string]func(ctx context.Context, data []byte))

	handlers[entity.SubEventImMessage] = h.onConsumeTalk
	handlers[entity.SubEventImMessageMerchant] = h.onConsumeMerchantMessage
	handlers[entity.SubEventImMessageKeyboard] = h.onConsumeTalkKeyboard
	handlers[entity.SubEventImMessageRevoke] = h.onConsumeTalkRevoke
	handlers[entity.SubEventImMessageRead] = h.onConsumeMessageRead
	handlers[entity.SubEventImSessionUnreadCleared] = h.onConsumeSessionUnreadCleared
	handlers[entity.SubEventImMessageMention] = h.onConsumeMention
	handlers[entity.SubEventContactStatus] = h.onConsumeContactStatus
	handlers[entity.SubEventContactApply] = h.onConsumeContactApply
	handlers[entity.SubEventContactApplyResult] = h.onConsumeContactApplyResult
	handlers[entity.SubEventGroupJoin] = h.onConsumeGroupJoin
	handlers[entity.SubEventGroupApply] = h.onConsumeGroupApply
	handlers[entity.SubEventSysNotice] = h.onConsumeSysNotice

	// Call Signaling
	handlers[entity.SubEventImCallInvite] = func(ctx context.Context, data []byte) {
		h.onConsumeTalkCall(ctx, data, entity.SubEventImCallInvite)
	}
	handlers[entity.SubEventImCallAccept] = func(ctx context.Context, data []byte) {
		h.onConsumeTalkCall(ctx, data, entity.SubEventImCallAccept)
	}
	handlers[entity.SubEventImCallReject] = func(ctx context.Context, data []byte) {
		h.onConsumeTalkCall(ctx, data, entity.SubEventImCallReject)
	}
	handlers[entity.SubEventImCallHangup] = func(ctx context.Context, data []byte) {
		h.onConsumeTalkCall(ctx, data, entity.SubEventImCallHangup)
	}
	handlers[entity.SubEventImCallCancel] = func(ctx context.Context, data []byte) {
		h.onConsumeTalkCall(ctx, data, entity.SubEventImCallCancel)
	}
}

func (h *Handler) SetServ(serv longnet.IServer) {
	h.serv = serv
	patchConsumeHandlerDeps(h)
}

func patchConsumeHandlerDeps(h *Handler) {
	if h == nil {
		return
	}
	ts, ok := h.TalkService.(*service.TalkService)
	if !ok || ts == nil {
		return
	}
	if ts.PushMessage == nil && h.PushMessage != nil {
		ts.PushMessage = h.PushMessage
	} else if h.PushMessage == nil && ts.PushMessage != nil {
		h.PushMessage = ts.PushMessage
	}
}

func (h *Handler) Call(ctx context.Context, event string, data []byte) {
	if handlers == nil {
		h.init()
	}

	//logger.Infof("consume chat event: [%s]", event)
	if call, ok := handlers[event]; ok {
		call(ctx, data)
	} else {
		logger.Infof("consume chat event: [%s]未注册回调事件", event)
	}
}

func Message(cmd string, body any) []byte {
	msg := map[string]any{
		"event":   cmd,
		"payload": body,
	}

	data, _ := json.Marshal(msg)
	return data
}

func buildMessageReadWS(talkMode, readerId, receiverId int, msgIds []string) []byte {
	return Message(entity.PushEventImMessageRead, entity.ImMessageReadPayload{
		TalkMode:   talkMode,
		FromId:     readerId,
		ReceiverId: receiverId,
		MsgIds:     msgIds,
	})
}
