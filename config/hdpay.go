package config

import "strings"

// 宏达支付通道：801-小额，802-中额，803-大额
const (
	HdpayChannelSmall  = "801"
	HdpayChannelMedium = "802"
	HdpayChannelLarge  = "803"
)

// Hdpay 宏达支付配置（config.yaml hdpay 节点）
type Hdpay struct {
	OrderURL  string `yaml:"order_url"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	NotifyURL string `yaml:"notify_url"`
}

// IsValidHdChannelPayType 校验宏达通道编号
func IsValidHdpayChannelPayType(payType string) bool {
	switch strings.TrimSpace(payType) {
	case HdpayChannelSmall, HdpayChannelMedium, HdpayChannelLarge:
		return true
	default:
		return false
	}
}
