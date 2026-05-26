package config

// Hmpay 汇美支付配置
type Hmpay struct {
	OrderURL  string `yaml:"order_url"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	NotifyURL string `yaml:"notify_url"`
	PayType   string `yaml:"pay_type"` // 通道编号，默认 106
}
