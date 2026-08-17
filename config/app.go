package config

type App struct {
	Env                    string   `yaml:"env"`
	Debug                  bool     `yaml:"debug"`
	PublicKey              string   `yaml:"public_key"`
	PrivateKey             string   `yaml:"private_key"`
	AesKey                 string   `yaml:"aes_key"`
	AdminEmail             []string `yaml:"admin_email"`
	AllowPhoneRegistration bool `yaml:"allow_phone_registration"` // 是否允许手机号注册，默认 false
	RequireInviteCode      bool `yaml:"require_invite_code"`      // 是否需要邀请码注册，默认 false
	// DisableLoginCaptcha 为 true 时跳过登录图形验证码校验（兼容旧客户端/本地调试；默认 false，即启用验证码）
	DisableLoginCaptcha bool `yaml:"disable_login_captcha"`
	// RegisterIPLimit 同一 IP 在 24 小时内的最大成功注册次数（Redis 计数）；0 表示不限制
	RegisterIPLimit int `yaml:"register_ip_limit"`
	// RegisterDeviceLimit 同一设备码（device_code）累计最大成功注册次数，无过期；0 表示不限制
	RegisterDeviceLimit int `yaml:"register_device_limit"`
}
