package model

import "time"

// RedEnvelope 红包记录
type RedEnvelope struct {
	Id            int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	EnvelopeId    string    `gorm:"column:envelope_id;uniqueIndex" json:"envelope_id"`     // 红包唯一ID
	SenderId      int       `gorm:"column:sender_id;index" json:"sender_id"`               // 发送者用户ID
	ChatType      int       `gorm:"column:chat_type" json:"chat_type"`                     // 聊天类型 1:私聊 2:群聊
	ChatId        int       `gorm:"column:chat_id" json:"chat_id"`                         // 聊天对象ID（好友/群组）
	Type          string    `gorm:"column:type" json:"type"`                               // 红包类型 normal:普通 lucky:拼手气
	Amount        float64   `gorm:"column:amount" json:"amount"`                           // 总金额
	RemainAmount  float64   `gorm:"column:remain_amount" json:"remain_amount"`             // 剩余金额
	Count         int       `gorm:"column:count" json:"count"`                             // 红包总数
	RemainCount   int       `gorm:"column:remain_count" json:"remain_count"`               // 剩余个数
	Greeting      string    `gorm:"column:greeting" json:"greeting"`                       // 祝福语
	Status        string    `gorm:"column:status;index" json:"status"`                     // 状态 available/finished/expired
	BestUserId    int       `gorm:"column:best_user_id" json:"best_user_id"`               // 手气最佳用户ID（拼手气红包）
	BestAmount    float64   `gorm:"column:best_amount" json:"best_amount"`                 // 手气最佳金额（拼手气红包）
	RefundAmount  float64   `gorm:"column:refund_amount" json:"refund_amount"`             // 退款金额（过期退回）
	ExpiredAt     time.Time `gorm:"column:expired_at;index" json:"expired_at"`             // 过期时间
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RedEnvelope) TableName() string {
	return "red_envelope"
}

// RedEnvelopeReceiver 红包领取记录
type RedEnvelopeReceiver struct {
	Id         int64     `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"`
	EnvelopeId string    `gorm:"column:envelope_id;index" json:"envelope_id"` // 红包ID
	UserId     int       `gorm:"column:user_id;index" json:"user_id"`         // 领取者用户ID
	Amount     float64   `gorm:"column:amount" json:"amount"`                 // 领取金额
	IsBest     bool      `gorm:"column:is_best" json:"is_best"`              // 是否手气最佳（拼手气红包）
	ReceivedAt time.Time `gorm:"column:received_at" json:"received_at"`       // 领取时间
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (RedEnvelopeReceiver) TableName() string {
	return "red_envelope_receiver"
}
