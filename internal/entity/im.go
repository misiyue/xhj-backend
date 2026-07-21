package entity

// IM 渠道分组(用于业务划分，业务间相互隔离)
const (
	// ImChannelChat 默认分组
	ImChannelChat    = "chat"    // im.Sessions.Chat.Name()
	ImChannelExample = "example" // im.Sessions.Example.Name()
)

const (
	// ImTopicChat 默认渠道消息订阅
	ImTopicChat        = "im:message:chat:all"
	ImTopicChatPrivate = "im:message:chat:%s"

	// ImTopicExample Example渠道消息订阅
	ImTopicExample        = "im:message:example:all"
	ImTopicExamplePrivate = "im:message:example:%s"
)

type ImMessagePayload struct {
	TalkMode   int `json:"talk_mode"`            // 对话类型[1:私信;2:群聊;3:商户C2C]
	FromId     int `json:"from_id"`              // 发送者用户ID
	ReceiverId int `json:"receiver_id"`          // 接收者ID[好友ID或者群ID]
	Body       any `json:"body"`                 // 私信消息或群聊消息
	SessionId  int `json:"session_id,omitempty"` // 商户会话 merchant_session.id（talk_mode=3）
	OrderId    int `json:"order_id,omitempty"`   // merchant_order.id（talk_mode=3）
}

type ImMessagePayloadBody struct {
	MsgId     string `json:"msg_id"`
	MsgType   int    `json:"msg_type"`
	FromId    int    `json:"from_id"` // 发送者ID
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	IsRevoked int    `json:"is_revoked"`
	IsRead    int    `json:"is_read,omitempty"` // 私聊：0-未读 1-已读
	SendTime  string `json:"send_time"`
	Extra     any    `json:"extra"` // 额外参数
	Quote     any    `json:"quote"` // 额外参数
}

// ImContactApplyPayload
// im.contact.apply
type ImContactApplyPayload struct {
	UserId    int    `json:"user_id"`
	Nickname  string `json:"nickname"`
	Remark    string `json:"remark"`
	ApplyTime string `json:"apply_time"`
}

// ImContactApplyResultPayload
// im.contact.apply_result
type ImContactApplyResultPayload struct {
	// 同意添加好友的用户ID
	UserId int `json:"user_id"`
	// 用户昵称
	Nickname string `json:"nickname"`
	// 申请备注
	ApplyResult string `json:"apply_result"`
	// 操作时间
	OperateTime string `json:"operate_time"`
}

// ImGroupApplyPayload im.group.apply
type ImGroupApplyPayload struct {
	GroupId   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	UserId    int    `json:"user_id"`
	Nickname  string `json:"nickname"`
	Remark    string `json:"remark"`
	ApplyTime string `json:"apply_time"`
}

// ImMessageKeyboardPayload im.message.keyboard
type ImMessageKeyboardPayload struct {
	FromId     int `json:"from_id"`
	ReceiverId int `json:"receiver_id"`
}

// ImMessageRevokePayload im.message.revoke
type ImMessageRevokePayload struct {
	TalkMode   int    `json:"talk_mode"`
	FromId     int    `json:"from_id"`
	ReceiverId int    `json:"receiver_id"`
	MsgId      string `json:"msg_id"`
	Remark     string `json:"remark"`
}

// ImMessageReadPayload im.message.read - 消息已读（私聊/群聊）
type ImMessageReadPayload struct {
	TalkMode   int      `json:"talk_mode"`
	FromId     int      `json:"from_id"`
	ReceiverId int      `json:"receiver_id"`
	MsgIds     []string `json:"msg_ids"`
}

// ImSessionUnreadClearedPayload im.session.unread.cleared
type ImSessionUnreadClearedPayload struct {
	UserId     int `json:"user_id"`
	TalkMode   int `json:"talk_mode"`
	ReceiverId int `json:"receiver_id"`
	UnreadNum  int `json:"unread_num"`
}

// ImCallPayload 通话事件
type ImCallPayload struct {
	FromUserId     int    `json:"from_user_id"`     // Match frontend expectation
	ToUserId       int    `json:"to_user_id"`       // Match frontend expectation
	RoomId         int    `json:"room_id"`          // Match frontend expectation
	CallType       int    `json:"call_type"`        // Match frontend expectation (1: voice, 2: video)
	FromUserName   string `json:"from_user_name"`   // Match frontend expectation
	FromUserAvatar string `json:"from_user_avatar"` // Match frontend expectation
}

// ImMessageMentionPayload im.message.mention - @提及通知
type ImMessageMentionPayload struct {
	GroupId   int      `json:"group_id"`   // 群组ID
	GroupName string   `json:"group_name"` // 群组名称
	FromId    int      `json:"from_id"`    // 发送者ID
	Nickname  string   `json:"nickname"`   // 发送者昵称
	Avatar    string   `json:"avatar"`     // 发送者头像
	MsgIds    []string `json:"msg_ids"`    // 提及消息ID列表（按时间倒序，最新的在前）
	Count     int      `json:"count"`      // 未读提及消息数量
	AtAll     bool     `json:"at_all"`     // 是否@所有人
}

// ImSysNoticePayload im.sys.notice - 系统通知推送
type ImSysNoticePayload struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Url       string `json:"url"`
	CreatedAt string `json:"created_at"`
}
