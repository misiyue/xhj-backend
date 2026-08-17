package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config 配置信息
type Config struct {
	App        *App        `json:"app" yaml:"app"`
	Redis      *Redis      `json:"redis" yaml:"redis"`
	MySQL      *MySQL      `json:"mysql" yaml:"mysql"`
	Jwt        *Jwt        `json:"jwt" yaml:"jwt"`
	Cors       *Cors       `json:"cors" yaml:"cors"`
	Log        *Log        `json:"log" yaml:"log"`
	Filesystem *Filesystem `json:"filesystem" yaml:"filesystem"`
	Email      *Email      `json:"email" yaml:"email"`
	Server     *Server     `json:"server" yaml:"server"`
	Nsq        *Nsq        `json:"nsq" yaml:"nsq"`
	OAuth      *OAuth      `json:"oauth" yaml:"oauth"`
	Yunxin     *Yunxin     `json:"yunxin" yaml:"yunxin"`
	Wallet     *Wallet     `json:"wallet" yaml:"wallet"`
	Marzban    *Marzban    `json:"marzban" yaml:"marzban"`
	Hdpay      *Hdpay      `json:"hdpay" yaml:"hdpay"`
	Hmpay      *Hmpay      `json:"hmpay" yaml:"hmpay"`
	Push       *Push       `json:"push" yaml:"push"`
	Security   *Security   `json:"security" yaml:"security"`
}

type Server struct {
	HttpAddr       string `json:"http_addr" yaml:"http_addr"`
	WebsocketAddr  string `json:"websocket_addr" yaml:"websocket_addr"`
	TcpAddr        string `json:"tcp_addr" yaml:"tcp_addr"`
	MaxOpenConns   int    `json:"max_open_conns" yaml:"max_open_conns"`   // WebSocket 最大连接数，0或不配置则自动检测
	ReservePercent int    `json:"reserve_percent" yaml:"reserve_percent"` // 为其他服务预留的资源百分比（0-100），默认35
}

type Yunxin struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	AppKey    string `json:"app_key" yaml:"app_key"`
	AppSecret string `json:"app_secret" yaml:"app_secret"`
	// ServerAPIBaseURL is primarily useful for private deployments and tests.
	// Leave empty to use the public Yunxin NIM server API.
	ServerAPIBaseURL string `json:"server_api_base_url" yaml:"server_api_base_url"`
}

func New(filename string) *Config {
	content, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("读取配置文件失败: %v", err))
	}

	var conf Config
	if err := yaml.Unmarshal(content, &conf); err != nil {
		panic(fmt.Sprintf("解析 config.yaml 读取错误: %v", err))
	}

	// 如果没有配置安全选项，使用默认配置
	if conf.Security == nil {
		conf.Security = DefaultSecurity()
	}

	return &conf
}

// Debug 调试模式
func (c *Config) Debug() bool {
	return c.App != nil && c.App.Debug
}
