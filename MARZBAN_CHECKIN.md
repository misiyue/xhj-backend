# Marzban 每6小时签到

`config.example.yaml` 是模板。把其中的 `marzban` 配置填入实际运行的 `config.yaml`（或通过 `--config` 指定的配置文件）。签到已有账号需要 `base_url`、`admin_username`、`admin_password`，以及与现有 Marzban 用户名一致的 `user_prefix`。`default_protocols`、`default_proxies`、`default_inbounds` 只在本系统创建新的 Marzban 用户时使用。

服务连接 MySQL 时会自动创建 `marzban_checkin` 和 `marzban_checkin_state` 表，也可以运行 `go run ./cmd/lumenim migrate` 创建。升级时会按已有签到记录的最后成功时间计算剩余冷却时间。

- `GET /api/v1/marzban/check-in?id=123`：查看系统用户 `123` 是否仍在签到冷却期。`checked_in=true` 表示距离上次成功签到不足6小时，`false` 表示可以签到；`reward_hours` 为6，`next_reset_at` 为上次成功签到后6小时的北京时间（RFC3339），从未签到时为空字符串。
- `POST /api/v1/marzban/check-in`：无需登录，请求体为 `{"id":123}`。成功后返回更新后的 Marzban 用户信息，其中 `expire` 为 Unix 秒，`expire_at` 为 RFC3339 时间。

两个接口都不校验 JWT。用户 ID 由客户端传入，从上次成功签到开始满6小时可再次签到（恰好6小时也允许）。例如10:00签到成功后，16:00可再次签到；23:59签到成功后，次日05:59可再次签到。冷却期内重复签到返回 HTTP 429，提示“距离上次签到未满6小时，请稍后再试”。Marzban 用户不存在返回 HTTP 404。

每次签到把 Marzban 到期时间增加6小时。已过期时从签到时刻起算；无限期账号无需签到。签到状态、冷却时间和每日最新签到记录在同一数据库事务中更新，同一用户的并发请求由数据库串行处理。Marzban 更新失败时本地签到记录和冷却时间会回滚，可再次尝试。
