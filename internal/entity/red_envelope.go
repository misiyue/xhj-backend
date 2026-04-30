package entity

// 红包类型
const (
	RedEnvelopeTypeNormal = "normal" // 普通红包
	RedEnvelopeTypeLucky  = "lucky"  // 拼手气红包
)

// 红包状态
const (
	RedEnvelopeStatusAvailable = "available" // 待领取
	RedEnvelopeStatusFinished  = "finished"  // 已领完
	RedEnvelopeStatusExpired   = "expired"   // 已过期
)

// 领取状态
const (
	RedEnvelopeReceiveStatusSuccess  = "success"  // 领取成功
	RedEnvelopeReceiveStatusFinished = "finished" // 已抢完
	RedEnvelopeReceiveStatusExpired  = "expired"  // 已过期
	RedEnvelopeReceiveStatusRepeated = "repeated" // 已领取过
)

// RedEnvelopeStatusText 红包状态对应文本
var RedEnvelopeStatusText = map[string]string{
	RedEnvelopeStatusAvailable: "待领取",
	RedEnvelopeStatusFinished:  "已领完",
	RedEnvelopeStatusExpired:   "已过期",
}

// 红包过期时间（24小时）
const RedEnvelopeExpireHours = 24
