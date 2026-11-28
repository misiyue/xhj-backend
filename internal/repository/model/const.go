package model

type State int

const (
	Yes = 1 // 是
	No  = 2 // 否
)

const (
	// TalkMsgRevokedHidden 混合消息拆出的冗余子段：会话列表隐藏，history-records 可查
	TalkMsgRevokedHidden = 0
)
