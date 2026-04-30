# Lumen IM 后端部署文档

## 目录

- [项目简介](#项目简介)
- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [邮件服务配置](#邮件服务配置)
- [数据库初始化](#数据库初始化)
- [本地开发](#本地开发)
- [生产环境部署](#生产环境部署)
- [常见问题](#常见问题)

---

## 项目简介

Lumen IM 是一个基于 Go 语言开发的即时通讯系统后端服务，采用微服务架构，支持 HTTP API、WebSocket 实时通信、消息队列等功能。

### 主要服务模块

- **HTTP Server**: 提供 RESTful API 接口
- **Comet Server**: WebSocket 长连接服务
- **Queue Worker**: 消息队列处理服务
- **Crontab**: 定时任务服务

---

## 环境要求

### 系统要求

- **操作系统**: Linux / macOS / Windows
- **Go 版本**: >= 1.25.0
- **数据库**: MySQL >= 5.7
- **缓存**: Redis >= 6.0
- **消息队列**: NSQ

### 可选组件

- **SMTP 服务器**: 用于发送邮件验证码（可选择本地或外部 SMTP）
- **对象存储**: 支持本地存储、MinIO、阿里云 OSS、腾讯云 COS 等

---

## 快速开始

### 1. 克隆项目

```bash
cd /path/to/your/workspace
git clone <repository-url>
cd backend
```

### 2. 安装依赖

```bash
# 安装 Go 依赖
go mod download

# 安装开发工具
make install
```

### 3. 配置文件

```bash
# 复制配置文件模板
make conf

# 或手动复制
cp config.example.yaml config.yaml

# 编辑配置文件
vim config.yaml
```

### 4. 数据库初始化

```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE go_chat CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"

# 导入数据库结构（假设有 SQL 文件）
mysql -u root -p go_chat < database/schema.sql
```

### 5. 运行服务

```bash
# 开发模式（一次性启动所有服务）
make dev

# 或分别启动各个服务
make dev-http      # HTTP API 服务
make dev-comet     # WebSocket 服务
make dev-queue     # 队列服务
make dev-crontab   # 定时任务服务
```

---

## 配置说明

### 核心配置项

编辑 `config.yaml` 文件，配置以下关键项：

#### 应用配置

```yaml
app:
  env: prod                          # 环境：dev/test/prod
  debug: false                       # 是否开启调试模式
  admin_email:                       # 管理员邮箱
    - admin@yourdomain.com
  allow_phone_registration: false    # 是否允许手机号注册
  require_invite_code: false         # 是否需要邀请码注册
```

#### 服务端口配置

```yaml
server:
  http_addr: ":9501"        # HTTP API 端口
  websocket_addr: ":9502"   # WebSocket 端口
  tcp_addr: ":9505"         # TCP 端口
```

#### 数据库配置

```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  username: root
  password: your_password
  database: go_chat
  charset: utf8mb4
  collation: utf8mb4_general_ci
```

#### Redis 配置

```yaml
redis:
  host: 127.0.0.1:6379
  auth: your_redis_password
  database: 0
```

#### JWT 配置

```yaml
jwt:
  secret: your_jwt_secret_key_here    # 建议使用强随机字符串
  expires_time: 3600                   # token 过期时间（秒）
  buffer_time: 3600                    # 缓冲时间（秒）
```

#### 日志配置

```yaml
log:
  path: "/var/log/lumenim"   # 日志文件路径（请使用绝对路径）
```

#### 文件存储配置

```yaml
filesystem:
  default: local
  local:
    root: "/var/data/lumenim/"   # 文件保存根目录（绝对路径）
```

---

## 邮件服务配置

本系统支持两种邮件发送方式：

### 方式一：使用本地 SMTP 服务器（推荐）

使用服务器自身的 SMTP 服务发送邮件，无需外部 SMTP 账号密码。

#### 配置示例

```yaml
email:
  use_local: true                      # 启用本地 SMTP
  local_host: localhost                # 本地 SMTP 地址
  local_port: 25                       # 本地 SMTP 端口
  username: noreply@yourdomain.com     # 发件人邮箱地址
  fromname: "Lumen IM 在线聊天"        # 发件人显示名称
```

#### 安装和配置 Postfix（Linux）

**Ubuntu/Debian:**

```bash
# 安装 Postfix
sudo apt-get update
sudo apt-get install postfix

# 配置类型选择 "Internet Site"
# 配置域名输入您的域名（如 yourdomain.com）

# 启动服务
sudo systemctl start postfix
sudo systemctl enable postfix
```

**CentOS/RHEL:**

```bash
# 安装 Postfix
sudo yum install postfix

# 启动服务
sudo systemctl start postfix
sudo systemctl enable postfix
```

**配置 Postfix:**

编辑 `/etc/postfix/main.cf`:

```bash
myhostname = yourdomain.com
mydomain = yourdomain.com
myorigin = $mydomain
inet_interfaces = localhost         # 仅本地访问
inet_protocols = ipv4
mydestination = $myhostname, localhost.$mydomain, localhost
relayhost =                         # 留空表示直接发送
```

重启服务：

```bash
sudo systemctl restart postfix
```

#### 安装和配置 Postfix（macOS）

```bash
# 使用 Homebrew 安装
brew install postfix

# 启动服务
sudo postfix start

# 配置文件位置：/usr/local/etc/postfix/main.cf
```

#### 测试本地 SMTP

```bash
# 发送测试邮件
echo "Test email body" | mail -s "Test Subject" test@example.com

# 查看邮件队列
mailq

# 查看日志
tail -f /var/log/mail.log
```

### 方式二：使用外部 SMTP 服务器

使用第三方邮件服务提供商（如网易、QQ、Gmail 等）。

#### 配置示例

```yaml
email:
  use_local: false                 # 禁用本地 SMTP
  host: smtp.163.com               # SMTP 服务器地址
  port: 465                        # SMTP 端口（465/587）
  username: your_email@163.com     # SMTP 账号
  password: your_smtp_password     # SMTP 密码或授权码
  fromname: "Lumen IM 在线聊天"    # 发件人显示名称
```

#### 常用 SMTP 服务器配置

**网易邮箱 (163.com):**
- SMTP: `smtp.163.com`
- 端口: `465` (SSL) / `25`
- 需要授权码（非登录密码）

**QQ 邮箱:**
- SMTP: `smtp.qq.com`
- 端口: `465` (SSL) / `587` (TLS)
- 需要授权码

**Gmail:**
- SMTP: `smtp.gmail.com`
- 端口: `465` (SSL) / `587` (TLS)
- 需要应用专用密码

---

## 数据库初始化

### 创建数据库

```bash
mysql -u root -p
```

```sql
CREATE DATABASE go_chat CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

### 运行数据迁移

系统使用 GORM AutoMigrate 自动创建和更新数据库表结构：

```bash
# 运行数据库迁移（使用 GORM AutoMigrate）
go run ./cmd/lumenim migrate
```

此命令会：
- 自动创建所有必需的数据库表
- 根据 GORM 模型定义更新表结构
- 确保数据库架构与代码保持一致

**注意**: 旧的 SQL 文件 (`internal/mission/resource/lumenim.sql`) 已弃用，仅保留作为参考。系统现在完全使用 GORM 模型进行数据库初始化。

---

## 本地开发

### 代码生成

```bash
# 生成 Wire 依赖注入代码
make generate

# 或手动执行
cd cmd/lumenim && wire
```

### 运行开发服务器

```bash
# 方式一：同时运行所有服务（推荐）
make dev

# 方式二：分别运行各服务
make dev-http      # 运行 HTTP API 服务 (端口 9501)
make dev-comet     # 运行 WebSocket 服务 (端口 9502)
make dev-queue     # 运行队列处理服务
make dev-crontab   # 运行定时任务服务
```

### 代码检查

```bash
# 运行代码检查
make lint

# 运行测试
make test
```

### API 文档生成

```bash
# 生成 Swagger 文档
swag init -g cmd/lumenim/main.go -o docs

# 访问 API 文档
# http://localhost:9501/swagger/index.html
```

---

## 生产环境部署

### 1. 构建可执行文件

```bash
# 构建二进制文件
make build

# 或手动构建
go build -o bin/lumenim ./cmd/lumenim

# 生成的可执行文件位于 bin/lumenim
```

### 2. 准备部署目录

```bash
# 创建部署目录
sudo mkdir -p /opt/lumenim
sudo mkdir -p /var/log/lumenim
sudo mkdir -p /var/data/lumenim

# 复制文件
sudo cp bin/lumenim /opt/lumenim/
sudo cp config.yaml /opt/lumenim/
```

### 3. 配置系统服务（Systemd）

推荐使用仓库内置模板：

- `scripts/systemd/lumenim@.service`
- `scripts/systemd/lumenim.env.example`

安装步骤：

```bash
# 创建运行用户（如已存在可跳过）
sudo useradd --system --no-create-home --shell /sbin/nologin lumenim

# 准备 systemd 相关目录
sudo mkdir -p /etc/lumenim

# 安装模板与环境变量文件
sudo cp scripts/systemd/lumenim@.service /etc/systemd/system/
sudo cp scripts/systemd/lumenim.env.example /etc/lumenim/lumenim.env
```

### 4. 启动服务

```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启动所有服务
sudo systemctl start lumenim@http
sudo systemctl start lumenim@comet
sudo systemctl start lumenim@queue
sudo systemctl start lumenim@crontab

# 设置开机自启
sudo systemctl enable lumenim@http
sudo systemctl enable lumenim@comet
sudo systemctl enable lumenim@queue
sudo systemctl enable lumenim@crontab

# 查看服务状态
sudo systemctl status lumenim@http
sudo systemctl status lumenim@comet
sudo systemctl status lumenim@queue
sudo systemctl status lumenim@crontab
```

### 5. 日志管理

```bash
# 查看实时日志
sudo journalctl -u lumenim@http -f
sudo journalctl -u lumenim@comet -f

# 查看历史日志
sudo journalctl -u lumenim@http --since "1 hour ago"
```

---

## Nginx 反向代理配置

### HTTP API 配置

```nginx
server {
    listen 80;
    server_name api.yourdomain.com;
    
    location / {
        proxy_pass http://127.0.0.1:9501;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### WebSocket 配置

```nginx
server {
    listen 80;
    server_name ws.yourdomain.com;
    
    location / {
        proxy_pass http://127.0.0.1:9502;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_connect_timeout 60s;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### HTTPS 配置（使用 Let's Encrypt）

```bash
# 安装 Certbot
sudo apt-get install certbot python3-certbot-nginx

# 获取证书
sudo certbot --nginx -d api.yourdomain.com -d ws.yourdomain.com

# 自动续期
sudo certbot renew --dry-run
```

---

## 性能优化建议

### 1. Go 应用优化

```yaml
# config.yaml
app:
  max_cpu_cores: 4         # 限制 CPU 核心数
  max_connections: 10000   # 最大连接数
```

### 2. MySQL 优化

```sql
-- 优化连接池
SET GLOBAL max_connections = 1000;
SET GLOBAL wait_timeout = 300;
SET GLOBAL interactive_timeout = 300;

-- 启用慢查询日志
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 2;
```

### 3. Redis 优化

```bash
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
timeout 300
tcp-keepalive 60
```

### 4. 系统参数优化

```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 1024 65535

# 应用设置
sudo sysctl -p
```

---

## 监控和维护

### 健康检查

```bash
# HTTP 服务健康检查
curl http://localhost:9501/health

# 查看服务状态
systemctl status 'lumenim@*'
```

### 日志轮转

创建 `/etc/logrotate.d/lumenim`:

```
/var/log/lumenim/*.log {
    daily
    rotate 30
    missingok
    notifempty
    compress
    delaycompress
    sharedscripts
    postrotate
      systemctl try-restart lumenim@http lumenim@comet lumenim@queue lumenim@crontab > /dev/null 2>&1 || true
    endscript
}
```

### 备份策略

```bash
#!/bin/bash
# backup.sh

# 备份数据库
mysqldump -u root -p go_chat > /backup/go_chat_$(date +%Y%m%d).sql

# 备份配置文件
cp /opt/lumenim/config.yaml /backup/config_$(date +%Y%m%d).yaml

# 备份上传文件
tar -czf /backup/uploads_$(date +%Y%m%d).tar.gz /var/data/lumenim/

# 删除 30 天前的备份
find /backup -type f -mtime +30 -delete
```

定时备份（crontab）:

```bash
# 每天凌晨 2 点备份
0 2 * * * /path/to/backup.sh
```

---

## 常见问题

### 1. 邮件发送失败

**问题**: 使用本地 SMTP 发送邮件失败

**解决方案**:
```bash
# 检查 Postfix 是否运行
sudo systemctl status postfix

# 查看邮件日志
tail -f /var/log/mail.log

# 测试 SMTP 连接
telnet localhost 25

# 检查防火墙（如果需要对外发送）
sudo ufw allow 25/tcp
```

### 2. 连接数据库失败

**问题**: `Error 1045: Access denied`

**解决方案**:
```sql
-- 检查用户权限
SHOW GRANTS FOR 'root'@'localhost';

-- 重新授权
GRANT ALL PRIVILEGES ON go_chat.* TO 'root'@'localhost';
FLUSH PRIVILEGES;
```

### 3. Redis 连接失败

**问题**: `NOAUTH Authentication required`

**解决方案**:
```bash
# 检查 Redis 配置
redis-cli
> CONFIG GET requirepass

# 修改 config.yaml 中的 redis.auth
```

### 4. 端口被占用

**问题**: `bind: address already in use`

**解决方案**:
```bash
# 查找占用端口的进程
sudo lsof -i :9501
sudo netstat -tuln | grep 9501

# 结束进程
sudo kill -9 <PID>
```

### 5. 磁盘空间不足

**问题**: 日志文件占用大量空间

**解决方案**:
```bash
# 清理旧日志
find /var/log/lumenim -name "*.log" -mtime +7 -delete

# 压缩历史日志
gzip /var/log/lumenim/*.log

# 配置日志轮转（见上文）
```

### 6. WebSocket 连接断开

**问题**: WebSocket 连接频繁断开

**解决方案**:
```nginx
# Nginx 配置增加超时时间
proxy_read_timeout 3600s;
proxy_send_timeout 3600s;

# 启用心跳检测
proxy_set_header Connection "upgrade";
```

---

## 技术支持

- **问题反馈**: 提交 Issue 到项目仓库
- **文档**: 查看项目 Wiki
- **社区**: 加入开发者社区

---

## 版本历史

- **v1.0.0** (2026-02-09): 初始版本，支持本地 SMTP 邮件发送
- 更多版本信息请查看 CHANGELOG.md

---

## 许可证

本项目采用 [LICENSE] 许可证，详情请参阅 LICENSE 文件。
