package entity

type EmailSendChannel string

const (
	EmailLoginChannel         = "email_login"
	EmailRegisterChannel      = "email_register"
	EmailForgetAccountChannel = "email_forget_account"
	EmailChangeAccountChannel = "email_change_account"
	EmailVerifyChannel        = "email_verify"
)
