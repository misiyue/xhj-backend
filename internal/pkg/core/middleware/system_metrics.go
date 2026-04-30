package middleware

import (
	"context"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	// CPU使用率
	systemCPUUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_cpu_usage_percent",
			Help: "Current CPU usage in percent",
		},
	)

	// 内存使用量
	systemMemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	// 内存使用率
	systemMemoryUsagePercent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_memory_usage_percent",
			Help: "Current memory usage in percent",
		},
	)

	// Goroutine数量
	systemGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_goroutines_count",
			Help: "Current number of goroutines",
		},
	)

	// Go堆内存分配
	systemGoHeapAlloc = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_go_heap_alloc_bytes",
			Help: "Bytes of allocated heap objects",
		},
	)

	// Go堆内存使用
	systemGoHeapInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_go_heap_inuse_bytes",
			Help: "Bytes in in-use spans",
		},
	)

	// GC次数
	systemGoGCCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "system_go_gc_count_total",
			Help: "Total number of GC runs",
		},
	)
)

// collectionInterval is the interval for system metrics collection.
// Exported for testing purposes.
var collectionInterval = 5 * time.Second

// StartSystemMetricsCollector 启动系统指标收集器
// ctx: 用于优雅关闭收集器的上下文
func StartSystemMetricsCollector(ctx context.Context) {
	go func() {
		var lastGCCount uint32

		ticker := time.NewTicker(collectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectSystemMetrics(&lastGCCount)
			}
		}
	}()
}

func collectSystemMetrics(lastGCCount *uint32) {
	// 收集CPU使用率
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		systemCPUUsage.Set(cpuPercent[0])
	}

	// 收集内存使用情况
	if vmStat, err := mem.VirtualMemory(); err == nil {
		systemMemoryUsage.Set(float64(vmStat.Used))
		systemMemoryUsagePercent.Set(vmStat.UsedPercent)
	}

	// 收集Go运行时指标
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	systemGoroutines.Set(float64(runtime.NumGoroutine()))
	systemGoHeapAlloc.Set(float64(memStats.HeapAlloc))
	systemGoHeapInUse.Set(float64(memStats.HeapInuse))

	// GC计数（累计）
	if memStats.NumGC > *lastGCCount {
		systemGoGCCount.Add(float64(memStats.NumGC - *lastGCCount))
		*lastGCCount = memStats.NumGC
	}
}
