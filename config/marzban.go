package config

// Marzban 定义 Marzban 面板的连接信息和新用户默认套餐。
type Marzban struct {
	BaseURL          string              `json:"base_url" yaml:"base_url"`
	AdminUsername    string              `json:"admin_username" yaml:"admin_username"`
	AdminPassword    string              `json:"admin_password" yaml:"admin_password"`
	UserPrefix       string              `json:"user_prefix" yaml:"user_prefix"`
	DefaultProtocols []string            `json:"default_protocols" yaml:"default_protocols"`
	DefaultInbounds  map[string][]string `json:"default_inbounds" yaml:"default_inbounds"`
}
