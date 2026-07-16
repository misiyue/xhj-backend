package entity

const (
	SubEventImMessage              = "sub.im.message"                // 对话消息通知
	SubEventImMessageMerchant      = "sub.im.message.merchant"       // 商户订单 C2C 对话消息通知
	SubEventImMessageKeyboard      = "sub.im.message.keyboard"       // 键盘输入事件通知
	SubEventImMessageRevoke        = "sub.im.message.revoke"         // 聊天消息撤销通知
	SubEventImMessageRead          = "sub.im.message.read"         // 私聊消息已读通知
	SubEventImSessionUnreadCleared = "sub.im.session.unread.cleared" // 会话未读清零通知
	SubEventImMessageMention       = "sub.im.message.mention"        // @提及消息通知
	SubEventContactStatus          = "sub.im.contact.status"         // 用户在线状态通知
	SubEventContactApply           = "sub.im.contact.apply"          // 好友申请消息通知
	SubEventContactApplyResult     = "sub.im.contact.apply_result"   // 好友申请结果通知
	SubEventGroupJoin              = "sub.im.group.join"             // 邀请加入群聊通知
	SubEventGroupApply             = "sub.im.group.apply"            // 入群申请通知
	SubEventImCallInvite           = "sub.im.call.invite"            // 通话邀请通知
	SubEventImCallAccept           = "sub.im.call.accept"            // 接受通话通知
	SubEventImCallReject           = "sub.im.call.reject"            // 拒绝通话通知
	SubEventImCallHangup           = "sub.im.call.hangup"            // 挂断通话通知
	SubEventImCallCancel           = "sub.im.call.cancel"            // 取消通话通知
	SubEventSysNotice              = "sub.im.sys.notice"             // 系统通知
)

type SubEventImCallPayload struct {
	FromId         int    `json:"from_id"`
	ToId           int    `json:"to_id"`
	RoomId         int    `json:"room_id"`
	CallType       int    `json:"call_type"`        // 1: voice, 2: video
	FromUserName   string `json:"from_user_name"`   // For frontend display
	FromUserAvatar string `json:"from_user_avatar"` // For frontend display
}

type SubscribeMessage struct {
	Event   string `json:"event"`   // 事件
	Payload string `json:"payload"` // json 字符串
}

type SubEventImMessagePayload struct {
	TalkMode int    `json:"talk_mode"` // 1 单聊 2 群聊
	Message  string `json:"message"`   // json 字符串
}

// SubEventImMessageMerchantPayload 商户 C2C 消息订阅 payload
type SubEventImMessageMerchantPayload struct {
	InboxUserId    int    `json:"inbox_user_id"`              // 该条推送应对哪一方用户投递
	InboxSessionId int    `json:"inbox_session_id,omitempty"` // 接收方会话栏 merchant_session.id
	OrderId        int    `json:"order_id"`                   // merchant_order.id
	Message        string `json:"message"`                    // model.MerchantMessage JSON
}

// SubEventImMessageMentionPayload @提及消息订阅事件payload
type SubEventImMessageMentionPayload struct {
	UserId    int      `json:"user_id"`    // 被@的用户ID
	GroupId   int      `json:"group_id"`   // 群组ID
	GroupName string   `json:"group_name"` // 群组名称
	FromId    int      `json:"from_id"`    // 发送者ID
	Nickname  string   `json:"nickname"`   // 发送者昵称
	Avatar    string   `json:"avatar"`     // 发送者头像
	MsgIds    []string `json:"msg_ids"`    // 提及消息ID列表
	Count     int      `json:"count"`      // 未读提及消息数量
	AtAll     bool     `json:"at_all"`     // 是否@所有人
}

type SubEventGroupJoinPayload struct {
	Type    int   `json:"type"` // 1 加入 2 退出
	GroupId int   `json:"group_id"`
	Uids    []int `json:"uids"`
}

type SubEventGroupApplyPayload struct {
	GroupId int `json:"group_id"`
	UserId  int `json:"user_id"`
	ApplyId int `json:"apply_id"`
}

type SubEventContactApplyPayload struct {
	ApplyId int `json:"apply_id"` // 申请ID
	Type    int `json:"type"`     // 1: 新申请  2: 拒绝  3: 同意
}

type SubEventContactApplyResultPayload struct {
	ApplierId int    `json:"applier_id"` // 申请人的用户ID (需要接收通知的人)
	UserId    int    `json:"user_id"`    // 操作人的用户ID (接受或拒绝的人)
	Nickname  string `json:"nickname"`   // 操作人的昵称
	Result    int    `json:"result"`     // 1: 同意  2: 拒绝
}

type SubEventImMessageKeyboardPayload struct {
	FromId     int `json:"from_id"`
	ReceiverId int `json:"receiver_id"`
}

type SubEventContactStatusPayload struct {
	Status int `json:"status"` // 1:上线 2:下线
	UserId int `json:"user_id"`
}

type SubEventTalkRevokePayload struct {
	TalkMode int    `json:"talk_mode"` // 1单聊 2群聊
	MsgId    string `json:"msg_id"`    // 消息ID
	Remark   string `json:"remark"`
}

type SubEventImMessageReadPayload struct {
	TalkMode   int      `json:"talk_mode"`   // 1 私聊
	FromId     int      `json:"from_id"`     // 消息发送方（需收到已读通知）
	ReceiverId int      `json:"receiver_id"` // 阅读者
	MsgIds     []string `json:"msg_ids"`     // 已读消息 ID 列表
}

type SubEventImSessionUnreadClearedPayload struct {
	UserId     int `json:"user_id"`
	TalkMode   int `json:"talk_mode"`
	ReceiverId int `json:"receiver_id"`
	UnreadNum  int `json:"unread_num"`
}

// SubEventSysNoticePayload 系统通知订阅 payload（queue -> comet）
type SubEventSysNoticePayload struct {
	UserId    int    `json:"user_id"`
	Id        int    `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Url       string `json:"url"`
	CreatedAt string `json:"created_at"`
}
