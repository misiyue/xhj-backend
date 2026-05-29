package config

import "strings"

const HmpayDefaultPayType = "106"

// Hmpay 汇美支付配置
type Hmpay struct {
	OrderURL  string `yaml:"order_url"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	NotifyURL string `yaml:"notify_url"`
	PayType   string `yaml:"pay_type"` // 通道编号，见 HmpayDefaultPayType
}

// ChannelPayType 统一下单 pay_type 参数（config.yaml hmpay.pay_type）
func (h *Hmpay) ChannelPayType() string {
	if h == nil {
		return HmpayDefaultPayType
	}
	if v := strings.TrimSpace(h.PayType); v != "" {
		return v
	}
	return HmpayDefaultPayType
}