//go:build linux
// +build linux

package sysinfo

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemInfo 系统信息
type SystemInfo struct {
	CPUCores      int   // CPU 核心数
	MemoryTotal   int64 // 总内存（字节）
	MemoryFree    int64 // 可用内存（字节）
	MaxOpenFiles  int64 // 最大文件描述符数（硬限制）
	CurrentRLimit int64 // 当前文件描述符限制（软限制）
}

// GetSystemInfo 获取系统信息（Linux版）
func GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// 获取 CPU 核心数
	info.CPUCores = runtime.NumCPU()

	// 获取内存信息
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("获取内存信息失败: %w", err)
	}
	info.MemoryTotal = int64(vmStat.Total)
	info.MemoryFree = int64(vmStat.Available)

	// 获取文件描述符限制（Linux专属）
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return nil, fmt.Errorf("获取文件描述符限制失败: %w", err)
	}
	info.CurrentRLimit = int64(rLimit.Cur)
	info.MaxOpenFiles = int64(rLimit.Max)

	return info, nil
}

// TrySetMaxOpenFiles 尝试设置最大文件描述符数（Linux版）
func TrySetMaxOpenFiles(target int64) (int64, error) {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return 0, fmt.Errorf("获取文件描述符限制失败: %w", err)
	}

	// 如果目标值超过硬限制，使用硬限制
	if target > int64(rLimit.Max) {
		target = int64(rLimit.Max)
	}

	// 如果当前软限制已经大于等于目标值，不需要设置
	if int64(rLimit.Cur) >= target {
		return int64(rLimit.Cur), nil
	}

	// 尝试设置新的限制
	rLimit.Cur = uint64(target)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return int64(rLimit.Cur), fmt.Errorf("设置文件描述符限制失败: %w", err)
	}

	return target, nil
}

// CalculateMaxConnections 根据系统资源计算最大连接数（通用逻辑，Linux/Windows都可用）
func CalculateMaxConnections(reservePercent int) (int, error) {
	if reservePercent < 0 || reservePercent > 100 {
		reservePercent = 35 // 默认预留35%
	}

	info, err := GetSystemInfo()
	if err != nil {
		return 0, err
	}

	// 基于不同因素计算的最大连接数
	var limits []int

	// 1. 基于文件描述符限制
	reservedFDs := int64(500) // 预留500个给系统和其他服务
	fdBasedLimit := int(info.CurrentRLimit - reservedFDs)
	if fdBasedLimit > 0 {
		fdBasedLimit = fdBasedLimit * (100 - reservePercent) / 100
		limits = append(limits, fdBasedLimit)
	}

	// 2. 基于内存限制
	memoryPerConn := int64(64 * 1024) // 每个连接平均占用64KB内存
	reservedMemory := info.MemoryTotal * int64(reservePercent) / 100
	availableMemory := info.MemoryTotal - reservedMemory
	if availableMemory > 0 {
		memBasedLimit := int(availableMemory / memoryPerConn)
		limits = append(limits, memBasedLimit)
	}

	// 3. 基于 CPU 核心数的经验值
	cpuBasedLimit := info.CPUCores * 3000
	limits = append(limits, cpuBasedLimit)

	// 取最小值作为最终限制
	maxConns := limits[0]
	for _, limit := range limits[1:] {
		if limit < maxConns {
			maxConns = limit
		}
	}

	// 设置合理的下限和上限
	if maxConns < 100 {
		maxConns = 100 // 最小100个连接
	}
	if maxConns > 100000 {
		maxConns = 100000 // 最大10万个连接
	}

	return maxConns, nil
}

// AutoConfigureMaxConnections 自动配置系统并计算最大连接数（Linux版）
func AutoConfigureMaxConnections(reservePercent int) (int, error) {
	info, err := GetSystemInfo()
	if err != nil {
		return 0, err
	}

	// 尝试将文件描述符限制设置为硬限制
	actualLimit, err := TrySetMaxOpenFiles(info.MaxOpenFiles)
	if err != nil {
		logger.Errorf("警告: %v，将使用当前限制 %d", err, info.CurrentRLimit)
	} else if actualLimit > info.CurrentRLimit {
		logger.Infof("成功将文件描述符限制从 %d 提升到 %d", info.CurrentRLimit, actualLimit)
	}

	// 重新获取系统信息
	info, err = GetSystemInfo()
	if err != nil {
		return 0, err
	}

	// 计算最大连接数
	maxConns, err := CalculateMaxConnections(reservePercent)
	if err != nil {
		return 0, err
	}

	// 打印系统信息和计算结果
	logger.Infof("========== 系统资源信息 ==========")
	logger.Infof("CPU 核心数: %d", info.CPUCores)
	logger.Infof("总内存: %.2f GB", float64(info.MemoryTotal)/1024/1024/1024)
	logger.Infof("可用内存: %.2f GB", float64(info.MemoryFree)/1024/1024/1024)
	logger.Infof("文件描述符限制（当前/最大）: %d / %d", info.CurrentRLimit, info.MaxOpenFiles)
	logger.Infof("预留资源百分比: %d%%", reservePercent)
	logger.Infof("计算得出的最大 WebSocket 连接数: %d", maxConns)
	logger.Infof("==================================")

	return maxConns, nil
}

// GetCPUUsage 获取CPU使用率（通用逻辑）
func GetCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) > 0 {
		return percentages[0], nil
	}
	return 0, nil
}
