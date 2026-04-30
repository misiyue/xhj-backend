package provider

import (
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/email"
)

func NewEmailClient(conf *config.Config) *email.Client {
	// 如果配置为使用本地SMTP服务器
	if conf.Email.UseLocal {
		client := email.NewLocalEmail(conf.Email.UserName, conf.Email.FromName)

		// 如果指定了自定义的本地SMTP地址和端口
		if conf.Email.LocalHost != "" && conf.Email.LocalPort > 0 {
			client.SetLocalSMTP(conf.Email.LocalHost, conf.Email.LocalPort)
		} else if conf.Email.LocalHost != "" {
			client.SetLocalSMTP(conf.Email.LocalHost, 25)
		} else if conf.Email.LocalPort > 0 {
			client.SetLocalSMTP("localhost", conf.Email.LocalPort)
		}

		return client
	}

	// 使用外部SMTP服务器
	return email.NewEmail(&email.Config{
		Host:     conf.Email.Host,
		Port:     conf.Email.Port,
		UserName: conf.Email.UserName,
		Password: conf.Email.Password,
		FromName: conf.Email.FromName,
	})
}
