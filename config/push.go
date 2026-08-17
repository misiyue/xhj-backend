package config

import "strings"

// Push OneSignal 推送配置（config.yaml push 节点）
type Push struct {
	AppID string `yaml:"app_id"`
	Key   string `yaml:"key"`
	URL   string `yaml:"url"`
}

func (p *Push) Valid() bool {
	if p == nil {
		return false
	}
	return strings.TrimSpace(p.AppID) != "" &&
		strings.TrimSpace(p.Key) != "" &&
		strings.TrimSpace(p.URL) != ""
}
