# Lumen IM systemd 模板

本目录提供可直接部署到服务器的 systemd 模板文件。

## 文件说明

- `lumenim@.service`: `systemd` 实例化服务模板，支持 `http/comet/queue/crontab`。
- `lumenim.env.example`: 环境变量模板，用于统一配置 `config.yaml` 路径和额外参数。

## 快速部署

```bash
# 1) 创建运行用户（如已存在可跳过）
sudo useradd --system --no-create-home --shell /sbin/nologin lumenim

# 2) 准备目录
sudo mkdir -p /opt/lumenim /etc/lumenim /var/log/lumenim /var/data/lumenim
sudo chown -R lumenim:lumenim /opt/lumenim /etc/lumenim /var/log/lumenim /var/data/lumenim

# 3) 放置二进制和配置
sudo cp bin/lumenim /opt/lumenim/
sudo cp config.yaml /opt/lumenim/

# 4) 安装 systemd 模板
sudo cp scripts/systemd/lumenim@.service /etc/systemd/system/
sudo cp scripts/systemd/lumenim.env.example /etc/lumenim/lumenim.env

# 5) 启动并设置开机自启
sudo systemctl daemon-reload
sudo systemctl enable --now lumenim@http lumenim@comet lumenim@queue lumenim@crontab

# 6) 查看状态
systemctl status 'lumenim@*'
```

## 常用命令

```bash
# 重启单个实例
sudo systemctl restart lumenim@http

# 查看单个实例日志
sudo journalctl -u lumenim@http -f

# 关闭全部实例
sudo systemctl stop lumenim@http lumenim@comet lumenim@queue lumenim@crontab
```
