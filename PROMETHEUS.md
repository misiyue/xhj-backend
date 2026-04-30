# Prometheus 监控集成

本项目已集成 Prometheus 监控支持，主要关注系统负载和响应时间等指标。

## 功能特性

### HTTP 请求指标

- **http_requests_total**: HTTP 请求总数
  - 标签: `method`, `path`, `status`
  - 类型: Counter

- **http_request_duration_seconds**: HTTP 请求持续时间（秒）
  - 标签: `method`, `path`, `status`
  - 类型: Histogram

- **http_requests_in_flight**: 当前正在处理的请求数
  - 类型: Gauge

- **http_request_size_bytes**: HTTP 请求大小（字节）
  - 标签: `method`, `path`
  - 类型: Histogram

- **http_response_size_bytes**: HTTP 响应大小（字节）
  - 标签: `method`, `path`
  - 类型: Histogram

### 系统指标

- **system_cpu_usage_percent**: CPU 使用率（百分比）
  - 类型: Gauge

- **system_memory_usage_bytes**: 内存使用量（字节）
  - 类型: Gauge

- **system_memory_usage_percent**: 内存使用率（百分比）
  - 类型: Gauge

- **system_goroutines_count**: Goroutine 数量
  - 类型: Gauge

- **system_go_heap_alloc_bytes**: Go 堆内存分配（字节）
  - 类型: Gauge

- **system_go_heap_inuse_bytes**: Go 堆内存使用（字节）
  - 类型: Gauge

- **system_go_gc_count_total**: GC 运行次数
  - 类型: Counter

## 使用方法

### 1. 访问监控指标

启动 HTTP 服务后，可以通过以下地址访问 Prometheus 指标：

```
http://localhost:9501/metrics
```

### 2. Prometheus 配置

在 Prometheus 配置文件中添加以下内容：

```yaml
scrape_configs:
  - job_name: 'go-chat'
    static_configs:
      - targets: ['localhost:9501']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### 3. 监控面板示例 (Grafana)

可以使用以下 PromQL 查询来创建监控面板：

#### HTTP 请求速率
```promql
rate(http_requests_total[5m])
```

#### HTTP 请求延迟 (P99)
```promql
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

#### CPU 使用率
```promql
system_cpu_usage_percent
```

#### 内存使用率
```promql
system_memory_usage_percent
```

#### Goroutine 数量
```promql
system_goroutines_count
```

## 实现细节

### 中间件集成

Prometheus 监控通过 Gin 中间件实现，在 `internal/apis/router/route.go` 中注册：

```go
router.Use(middleware.PrometheusMiddleware())
```

### 系统指标收集

系统指标通过后台 goroutine 每 5 秒收集一次，在服务器启动时自动启动。收集器支持优雅关闭：

```go
// 创建一个可取消的上下文
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// 启动系统指标收集器
middleware.StartSystemMetricsCollector(ctx)

// 当需要停止收集器时，取消上下文
cancel()
```

## 性能影响

- HTTP 指标收集对性能影响极小（< 1%）
- 系统指标每 5 秒收集一次，对性能影响可忽略不计
- /metrics 端点自身不会被计入指标统计

## 测试

运行以下命令测试 Prometheus 集成：

```bash
go test -v ./internal/pkg/core/middleware -run TestPrometheusMiddleware
```
