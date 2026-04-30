# Lumen IM Backend

基于 Go 语言开发的即时通讯系统后端服务。

## ⭐ High Availability Support

Lumen IM 支持高可用部署，可以运行多个实例以实现负载均衡和容错。

**完整的高可用部署指南**: 📖 [HA_DEPLOYMENT.md](./HA_DEPLOYMENT.md)

关键特性：
- ✅ 分布式 ID 生成（Snowflake 算法，支持 1024 个实例）
- ✅ 基于 Redis 的会话管理和消息路由
- ✅ 支持水平扩展（HTTP API、WebSocket、队列处理）
- ✅ 自动故障转移

## 🔒 Security Features

Lumen IM 内置多层安全防护机制，保护系统免受攻击和滥用。

**完整的安全配置指南**: 📖 [SECURITY.md](./SECURITY.md)

安全特性：
- ✅ **QPS 限流**: 基于令牌桶算法的 IP 级和全局流量控制
- ✅ **暴力破解防护**: 登录失败次数限制和指数退避封禁
- ✅ **安全响应头**: 自动添加 XSS、CSRF、Clickjacking 防护头
- ✅ **请求大小限制**: 防止大负载 DoS 攻击
- ✅ **自动清理机制**: 内存高效的限流器自动清理

快速配置安全选项：

```yaml
security:
  rate_limit:
    enabled: true
    capacity: 100  # 突发请求容量
    rate: 50       # 每秒请求数 (QPS)
  
  brute_force:
    enabled: true
    max_attempts: 5     # 最大失败尝试次数
    block_duration: 15  # 封禁时长（分钟）
```

查看 [SECURITY.md](./SECURITY.md) 了解更多配置选项和最佳实践。

## 快速开始

### 前置要求

- Go >= 1.25.0
- MySQL >= 5.7
- Redis >= 6.0
- NSQ (消息队列)

### 安装

```bash
# 1. 安装依赖
go mod download
make install

# 2. 配置环境
make conf
vim config.yaml

# 3. 初始化数据库
mysql -u root -p -e "CREATE DATABASE go_chat CHARACTER SET utf8mb4"

# 4. 运行服务
make dev
```

### 开发命令

```bash
make install     # 安装开发工具
make conf        # 创建配置文件
make generate    # 生成代码
make dev         # 运行所有服务
make build       # 构建可执行文件
make lint        # 代码检查
make test        # 运行测试
```

### 服务端口

- HTTP API: `9501`
- WebSocket: `9502`
- TCP: `9505`

## 邮件服务配置

系统支持两种邮件发送方式：

### 方式一：本地 SMTP（推荐）

```yaml
email:
  use_local: true
  local_host: localhost
  local_port: 25
  username: noreply@yourdomain.com
  fromname: "Lumen IM"
```

需要在服务器上安装并配置 Postfix：

```bash
# Ubuntu/Debian
sudo apt-get install postfix
sudo systemctl start postfix
```

### 方式二：外部 SMTP

```yaml
email:
  use_local: false
  host: smtp.163.com
  port: 465
  username: your_email@163.com
  password: your_smtp_password
  fromname: "Lumen IM"
```

## 项目结构

```
backend/
├── api/              # API 定义（Proto）
├── cmd/              # 应用程序入口
│   └── lumenim/      # 主程序
├── config/           # 配置结构定义
├── internal/         # 内部代码
│   ├── apis/         # API 处理器
│   ├── logic/        # 业务逻辑
│   ├── service/      # 服务层
│   ├── repository/   # 数据访问层
│   └── pkg/          # 内部工具包
├── docs/             # Swagger 文档
├── bin/              # 编译输出
├── config.yaml       # 配置文件
└── Makefile          # 构建脚本
```

## 详细文档

完整的部署和配置指南请查看：

📖 **[部署文档 (DEPLOY.md)](./DEPLOY.md)**

包含：
- 详细的环境配置
- 本地 SMTP 服务器配置
- 生产环境部署指南
- Nginx 反向代理配置
- 性能优化建议
- 常见问题解决

## API 文档

启动服务后访问：

- Swagger UI: `http://localhost:9501/swagger/index.html`
- API JSON: `http://localhost:9501/swagger/doc.json`

## 技术栈

- **Web 框架**: Gin
- **ORM**: GORM
- **缓存**: Redis
- **消息队列**: NSQ
- **WebSocket**: Gorilla WebSocket
- **认证**: JWT
- **依赖注入**: Wire
- **配置**: Viper

## 开发

### 代码生成

```bash
# 生成 Wire 依赖注入代码
make generate

# 生成 API 文档
swag init -g cmd/lumenim/main.go -o docs
```

### 运行单个服务

```bash
make dev-http      # HTTP API 服务
make dev-comet     # WebSocket 服务
make dev-queue     # 队列处理服务
make dev-crontab   # 定时任务服务
```

## 生产环境

### 构建

```bash
make build
# 输出: bin/lumenim
```

### 部署

详见 [DEPLOY.md](./DEPLOY.md) 中的生产环境部署章节。

## 许可证

[查看 LICENSE 文件]

## 贡献

欢迎提交 Issue 和 Pull Request！

---

**注意**: 首次部署请务必查看 [DEPLOY.md](./DEPLOY.md) 了解详细配置说明。
