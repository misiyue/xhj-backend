package config

// Email 邮件配置信息
type Email struct {
	Host      string `yaml:"host"`       // smtp.163.com
	Port      int    `yaml:"port"`       // 端口号
	UserName  string `yaml:"username"`   // 登录账号
	Password  string `yaml:"password"`   // 登录密码
	FromName  string `yaml:"fromname"`   // 发送人名称
	UseLocal  bool   `yaml:"use_local"`  // 是否使用本地SMTP服务器
	LocalHost string `yaml:"local_host"` // 本地SMTP服务器地址，默认localhost
	LocalPort int    `yaml:"local_port"` // 本地SMTP服务器端口，默认25
}
