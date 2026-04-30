//go:build windows
// +build windows

package sysinfo

import (
	"fmt"
	"runtime"

	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemInfo 系统信息（Windows版，去掉文件描述符相关字段的Linux依赖）
type SystemInfo struct {
	CPUCores      int   // CPU 核心数
	MemoryTotal   int64 // 总内存（字节）
	MemoryFree    int64 // 可用内存（字节）
	MaxOpenFiles  int64 // Windows无此概念，返回默认值
	CurrentRLimit int64 // Windows无此概念，返回默认值
}

// GetSystemInfo 获取系统信息（Windows版）
func GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// 获取 CPU 核心数（Windows兼容）
	info.CPUCores = runtime.NumCPU()

	// 获取内存信息（gopsutil已兼容Windows）
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("获取内存信息失败: %w", err)
	}
	info.MemoryTotal = int64(vmStat.Total)
	info.MemoryFree = int64(vmStat.Available)

	// Windows无文件描述符限制（RLIMIT_NOFILE），返回默认值
	info.CurrentRLimit = 10240 // Windows默认最大打开文件数（经验值）
	info.MaxOpenFiles = 10240

	return info, nil
}

// TrySetMaxOpenFiles Windows无文件描述符限制设置，直接返回默认值
func TrySetMaxOpenFiles(target int64) (int64, error) {
	defaultLimit := int64(10240)
	logger.Warnf("Windows系统不支持设置文件描述符限制，使用默认值: %d", defaultLimit)
	return defaultLimit, nil
}

// CalculateMaxConnections 根据系统资源计算最大连接数（复用通用逻辑）
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

	// 1. 基于文件描述符限制（Windows返回默认值）
	reservedFDs := int64(500)
	fdBasedLimit := int(info.CurrentRLimit - reservedFDs)
	if fdBasedLimit > 0 {
		fdBasedLimit = fdBasedLimit * (100 - reservePercent) / 100
		limits = append(limits, fdBasedLimit)
	}

	// 2. 基于内存限制（Windows兼容）
	memoryPerConn := int64(64 * 1024)
	reservedMemory := info.MemoryTotal * int64(reservePercent) / 100
	availableMemory := info.MemoryTotal - reservedMemory
	if availableMemory > 0 {
		memBasedLimit := int(availableMemory / memoryPerConn)
		limits = append(limits, memBasedLimit)
	}

	// 3. 基于 CPU 核心数的经验值（Windows兼容）
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
		maxConns = 100
	}
	if maxConns > 100000 {
		maxConns = 100000
	}

	return maxConns, nil
}

// AutoConfigureMaxConnections 自动配置系统并计算最大连接数（Windows版）
func AutoConfigureMaxConnections(reservePercent int) (int, error) {
	info, err := GetSystemInfo()
	if err != nil {
		return 0, err
	}

	// Windows无需设置文件描述符限制，直接返回默认值
	defaultLimit := int64(10240)
	logger.Infof("Windows系统使用默认文件描述符限制: %d", defaultLimit)

	// 计算最大连接数
	maxConns, err := CalculateMaxConnections(reservePercent)
	if err != nil {
		return 0, err
	}

	// 打印系统信息和计算结果（适配Windows）
	logger.Infof("========== 系统资源信息 ==========")
	logger.Infof("CPU 核心数: %d", info.CPUCores)
	logger.Infof("总内存: %.2f GB", float64(info.MemoryTotal)/1024/1024/1024)
	logger.Infof("可用内存: %.2f GB", float64(info.MemoryFree)/1024/1024/1024)
	logger.Infof("文件描述符限制（Windows默认）: %d", info.CurrentRLimit)
	logger.Infof("预留资源百分比: %d%%", reservePercent)
	logger.Infof("计算得出的最大 WebSocket 连接数: %d", maxConns)
	logger.Infof("==================================")

	return maxConns, nil
}

// GetCPUUsage 获取CPU使用率（Windows兼容）
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
