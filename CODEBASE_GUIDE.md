# Lumen IM 后端代码熟悉指南

本文档帮助你在较短时间内掌握本项目的代码结构和核心流程，适合新人或需要快速上手的开发者。

---

## 一、先读这些（约 15 分钟）

| 文档 | 作用 |
|------|------|
| [README.md](./README.md) | 技术栈、目录结构、启动方式 |
| [DEPLOY.md](./DEPLOY.md) 前几章 | 配置项、端口、服务组成 |
| 本文件（CODEBASE_GUIDE.md） | 学习路径与关键入口 |

---

## 二、整体架构（一句话）

- **HTTP API**（Gin）处理登录、用户、会话、消息发送等 REST 请求。
- **WebSocket/TCP**（longnet）维护在线连接，接收客户端上行消息并转发给目标用户。
- **消息落库与推送**：HTTP 发消息 → service 写 DB + 通过 Redis/NSQ 通知 comet → comet 通过 longnet 推给在线端。

---

## 三、入口与进程模型（必看）

**主入口**：`cmd/lumenim/main.go`

- 使用 `urfave/cli` 注册多个**子命令**，每个子命令对应一个进程类型：
  - `http`：HTTP API 服务（端口 9501）
  - `comet`：WebSocket + TCP 长连接服务（如 9502、9505）
  - `queue`：队列消费（如 NSQ）
  - `crontab`：定时任务
  - `migrate`：数据库迁移/初始化

**建议**：先看 `main.go` 里各 `NewXxxCommand()`，再看 `make dev` 如何同时拉起 http/comet/queue 等，对「单机多进程」有清晰印象。

---

## 四、按「请求路径」顺藤摸瓜（推荐路线）

### 1. 一次 HTTP 请求怎么走

1. **路由**：`internal/apis/router/route.go` 创建 Gin 引擎并挂中间件；`api.go` 里 `RegisterWebRoute` 注册所有 Web 路由（认证、Swagger、各业务 Handler）。
2. **Handler**：每个接口对应到 `internal/apis/handler/web/v1/` 下的某个 Handler 方法（如 `Auth`、`User`、`Talk`、`Message`、`STS` 等）。
3. **业务逻辑**：Handler 通常调 `internal/service/` 下的服务（如 `auth.go`、`user.go`、`message/`）。
4. **数据层**：service 再调 `internal/repository/repo/` 与 `internal/repository/cache/`，写 MySQL 或读/写 Redis。

**实操**：选一个你关心的接口（例如「发送单聊消息」），在 `api.go` 里搜路径（如 `/api/v1/message/send`），找到对应 Handler 方法，再顺着调用栈看到 service → repository。

### 2. WebSocket 长连接与消息推送

1. **建立连接**：`internal/pkg/longnet/` 提供 WebSocket/TCP 服务器；连接建立后由 `handler` 的 `OnOpen` 处理（绑定用户、加入会话管理）。
2. **收消息**：客户端通过 WebSocket 发上来的包由 `OnMessage` 处理；内部会解析协议并调用业务（如发私聊/群聊）。
3. **推送给目标用户**：业务层（如 `internal/service/message/`）在落库后，通过 Redis/NSQ 等通知 comet；comet 根据用户 ID 找到对应连接，经 longnet 写回客户端。

**建议**：先看 `internal/pkg/longnet/interface.go`（接口定义），再看 `handler.go`、`session_manager.go`，最后在业务里搜「发往 comet」或「推送」的调用。

### 3. 配置与依赖注入

- **配置**：`config.yaml` + `config/` 下结构体；应用启动时加载。
- **依赖注入**：`cmd/lumenim/wire.go` + `wire_gen.go`，把 config、repository、service、handler 串起来；新增接口时往往要在这里补一层。

---

## 五、目录与阅读顺序建议

| 顺序 | 目录/文件 | 建议关注点 |
|------|-----------|------------|
| 1 | `cmd/lumenim/main.go` | 子命令、进程类型、启动入口 |
| 2 | `internal/apis/router/route.go`、`api.go` | 中间件、路由注册、Handler 与路径对应关系 |
| 3 | `internal/apis/handler/web/v1/` | 按业务挑几个看：auth、user、message、sts、upload |
| 4 | `internal/service/` | 业务主逻辑；先看 `auth.go`、`message/service.go`、`talk*.go` |
| 5 | `internal/repository/repo/`、`cache/` | 表访问、缓存 key 设计 |
| 6 | `internal/pkg/longnet/` | 长连接抽象、会话管理、消息上行/下发 |
| 7 | `internal/logic/` | 若存在，多为可复用的业务规则 |
| 8 | `api/proto/`、`api/pb/` | 部分接口的请求/响应定义（Proto/生成代码） |

---

## 六、如何「跟踪一个功能」（以发送私聊为例）

1. 在 `internal/apis/router/api.go` 搜 `message/send`，找到 Handler（如 `handler.V1.Message.Send`）。
2. 打开对应 Handler 文件，看 `Send` 如何解析请求、调 service。
3. 在 `internal/service/message/` 里找到发私聊的入口（如 `CreateMessage` 或专门私聊方法），看落库与「通知 comet」的调用。
4. 在 comet 侧搜「消费队列」或「从 Redis 读待推送」，看如何根据 user_id 找到连接并写入 longnet。

按「路由 → Handler → Service → Repository / 推送」走一遍，就能串起一条完整链路。

---

## 七、常用命令与调试技巧

- **本地跑全栈**：`make dev`（会起 http、comet、queue 等）。
- **只跑 HTTP**：`make dev-http`（便于用 Postman/curl 调试接口）。
- **API 文档**：服务起来后访问 `/swagger/index.html`（如 `http://localhost:9501/swagger/index.html`）。
- **压测**：`k6 run k6.js` 做 WebSocket 压测；`cmd/stress-test/` 下可能有更多脚本。
- **配置**：复制并修改 `config.yaml`，重点看 `app`、`mysql`、`redis`、`jwt`、`filesystem`、`nsq` 等与当前环境一致。

---

## 八、小结

- **先建立「进程 + 路由」地图**：main → 子命令 → 路由注册 → Handler 列表。
- **再按「一条请求、一条推送」走通**：选一个接口 + 一个推送场景，从 HTTP 或 WebSocket 入口跟到 DB/缓存/推送。
- **最后按需深入**：repository 模型、缓存 key、longnet 协议、队列 topic 等。

遇到具体文件或函数时，用 IDE 的「查找引用」和「跳转定义」沿调用链来回看，会进步更快。
