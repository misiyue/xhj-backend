package config

import "strings"

// Hmpay 汇美支付配置（config.yaml hmpay 节点）
type Hmpay struct {
	OrderURL    string `yaml:"order_url"`
	Parter      string `yaml:"parter"`
	ReqType     string `yaml:"req_type"`
	Key         string `yaml:"key"`
	CallbackURL string `yaml:"callback_url"`
}

func (h *Hmpay) Valid() bool {
	if h == nil {
		return false
	}
	return strings.TrimSpace(h.OrderURL) != "" &&
		strings.TrimSpace(h.Parter) != "" &&
		strings.TrimSpace(h.Key) != "" &&
		strings.TrimSpace(h.CallbackURL) != ""
}
