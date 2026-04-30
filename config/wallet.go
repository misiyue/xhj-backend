package config

// Wallet 钱包平台配置
type Wallet struct {
	BaseURL string `json:"base_url" yaml:"base_url"`
	Key     string `json:"key" yaml:"key"`
}
