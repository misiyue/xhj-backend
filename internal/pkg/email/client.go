package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	"gopkg.in/gomail.v2"
)

type Client struct {
	config    *Config
	useLocal  bool   // 是否使用本地SMTP服务器
	localHost string // 本地SMTP服务器地址
	localPort int    // 本地SMTP服务器端口
}

type Config struct {
	Host     string // 例如 smtp.163.com
	Port     int    // 端口号
	UserName string // 登录账号
	Password string // 登录密码
	FromName string // 发送人名称
}

func NewEmail(config *Config) *Client {
	return &Client{
		config: config,
	}
}

// NewLocalEmail 创建使用本地SMTP服务器的邮件客户端
func NewLocalEmail(fromEmail, fromName string) *Client {
	return &Client{
		config: &Config{
			UserName: fromEmail,
			FromName: fromName,
		},
		useLocal:  true,
		localHost: "localhost",
		localPort: 25,
	}
}

// SetLocalSMTP 设置本地SMTP服务器地址和端口
func (c *Client) SetLocalSMTP(host string, port int) {
	c.useLocal = true
	c.localHost = host
	c.localPort = port
}

type Option struct {
	To      []string // 收件人
	Subject string   // 邮件主题
	Body    string   // 邮件正文
}

type OptionFunc func(msg *gomail.Message)

func (c *Client) do(msg *gomail.Message) error {
	// 如果使用本地SMTP服务器
	if c.useLocal {
		return c.sendViaLocalSMTP(msg)
	}

	// 使用外部SMTP服务器
	dialer := gomail.NewDialer(c.config.Host, c.config.Port, c.config.UserName, c.config.Password)

	// 自动开启SSL
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	return dialer.DialAndSend(msg)
}

// sendViaLocalSMTP 通过本地SMTP服务器发送邮件
func (c *Client) sendViaLocalSMTP(msg *gomail.Message) error {
	// 获取邮件内容
	var recipients []string
	to := msg.GetHeader("To")
	recipients = append(recipients, to...)

	// 构建邮件内容
	var emailContent []byte
	msg.WriteTo(&emailWriter{content: &emailContent})

	// 连接到本地SMTP服务器
	addr := fmt.Sprintf("%s:%d", c.localHost, c.localPort)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("无法连接到本地SMTP服务器: %w", err)
	}
	defer client.Close()

	// 设置发件人
	if err := client.Mail(c.config.UserName); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("准备发送邮件数据失败: %w", err)
	}

	_, err = w.Write(emailContent)
	if err != nil {
		w.Close()
		return fmt.Errorf("写入邮件数据失败: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭邮件数据流失败: %w", err)
	}

	return client.Quit()
}

// emailWriter 用于捕获gomail生成的邮件内容
type emailWriter struct {
	content *[]byte
}

func (w *emailWriter) Write(p []byte) (n int, err error) {
	*w.content = append(*w.content, p...)
	return len(p), nil
}

func (c *Client) SendMail(email *Option, opt ...OptionFunc) error {
	m := gomail.NewMessage()

	// 这种方式可以添加别名，即“XX官方”
	m.SetHeader("From", m.FormatAddress(c.config.UserName, c.config.FromName))

	if len(email.To) > 0 {
		m.SetHeader("To", email.To...)
	}

	if len(email.Subject) > 0 {
		m.SetHeader("Subject", email.Subject)
	}

	if len(email.Body) > 0 {
		m.SetBody("text/html", email.Body)
	}

	// m.SetHeader("Cc", m.FormatAddress("xxxx@foxmail.com", "收件人")) //抄送
	// m.SetHeader("Bcc", m.FormatAddress("xxxx@gmail.com", "收件人"))  //暗送

	for _, o := range opt {
		o(m)
	}

	return c.do(m)
}
