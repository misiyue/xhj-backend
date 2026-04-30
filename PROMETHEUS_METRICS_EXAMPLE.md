# Sample Prometheus Metrics Output

This is an example of the metrics exposed at the `/metrics` endpoint after running the application and making a few HTTP requests.

## HTTP Request Metrics

```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",path="/",status="200"} 15
http_requests_total{method="GET",path="/api/v1/user/setting",status="200"} 8
http_requests_total{method="POST",path="/api/v1/auth/login",status="200"} 3
http_requests_total{method="GET",path="/health/check",status="200"} 120
http_requests_total{method="GET",path="/api/v1/talk/session/list",status="200"} 5

# HELP http_request_duration_seconds HTTP request latencies in seconds
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.005"} 15
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.01"} 15
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.025"} 15
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.05"} 15
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.1"} 15
http_request_duration_seconds_bucket{method="GET",path="/",status="200",le="0.25"} 15
http_request_duration_seconds_sum{method="GET",path="/",status="200"} 0.023456789
http_request_duration_seconds_count{method="GET",path="/",status="200"} 15

# HELP http_requests_in_flight Number of HTTP requests currently being processed
# TYPE http_requests_in_flight gauge
http_requests_in_flight 2

# HELP http_request_size_bytes HTTP request size in bytes
# TYPE http_request_size_bytes histogram
http_request_size_bytes_bucket{method="POST",path="/api/v1/auth/login",le="100"} 0
http_request_size_bytes_bucket{method="POST",path="/api/v1/auth/login",le="1000"} 3
http_request_size_bytes_sum{method="POST",path="/api/v1/auth/login"} 256
http_request_size_bytes_count{method="POST",path="/api/v1/auth/login"} 3

# HELP http_response_size_bytes HTTP response size in bytes
# TYPE http_response_size_bytes histogram
http_response_size_bytes_bucket{method="GET",path="/",le="100"} 15
http_response_size_bytes_bucket{method="GET",path="/",le="1000"} 15
http_response_size_bytes_sum{method="GET",path="/"} 630
http_response_size_bytes_count{method="GET",path="/"} 15
```

## System Metrics

```
# HELP system_cpu_usage_percent Current CPU usage in percent
# TYPE system_cpu_usage_percent gauge
system_cpu_usage_percent 23.456789

# HELP system_memory_usage_bytes Current memory usage in bytes
# TYPE system_memory_usage_bytes gauge
system_memory_usage_bytes 1.23456789e+09

# HELP system_memory_usage_percent Current memory usage in percent
# TYPE system_memory_usage_percent gauge
system_memory_usage_percent 45.67

# HELP system_goroutines_count Current number of goroutines
# TYPE system_goroutines_count gauge
system_goroutines_count 42

# HELP system_go_heap_alloc_bytes Bytes of allocated heap objects
# TYPE system_go_heap_alloc_bytes gauge
system_go_heap_alloc_bytes 5.6789012e+07

# HELP system_go_heap_inuse_bytes Bytes in in-use spans
# TYPE system_go_heap_inuse_bytes gauge
system_go_heap_inuse_bytes 6.7890123e+07

# HELP system_go_gc_count_total Total number of GC runs
# TYPE system_go_gc_count_total counter
system_go_gc_count_total 15
```

## Using These Metrics

### Prometheus Configuration

Add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'go-chat'
    static_configs:
      - targets: ['localhost:9501']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Example PromQL Queries

**Request rate (requests per second):**
```promql
rate(http_requests_total[5m])
```

**95th percentile latency:**
```promql
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

**Error rate (4xx and 5xx responses):**
```promql
sum(rate(http_requests_total{status=~"[45].*"}[5m])) / sum(rate(http_requests_total[5m]))
```

**Memory usage trend:**
```promql
system_memory_usage_percent
```

**Goroutine growth:**
```promql
delta(system_goroutines_count[1h])
```
