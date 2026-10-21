package entity

import "strings"

type EmailSendChannel string

const (
	EmailLoginChannel         = "email_login"
	EmailRegisterChannel      = "email_register"
	EmailForgetAccountChannel = "email_forget_account"
	EmailChangeAccountChannel = "email_change_account"
	EmailUpdateChannel        = "email_update" // 前端更换邮箱渠道别名，等同 email_change_account
	EmailVerifyChannel        = "email_verify"
)

// NormalizeEmailChannel 将前端/历史渠道名归一为存储用 channel
func NormalizeEmailChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	switch channel {
	case "", EmailVerifyChannel:
		if channel == "" {
			return EmailVerifyChannel
		}
		return channel
	case EmailUpdateChannel, SmsChangeAccountChannel, "change", "email_change":
		return EmailChangeAccountChannel
	default:
		return channel
	}
}

// EmailChangeVerifyChannels 更换邮箱校验时依次尝试的 channel（含历史别名）
func EmailChangeVerifyChannels() []string {
	return []string{
		EmailChangeAccountChannel,
		EmailUpdateChannel,
		EmailVerifyChannel,
	}
}
