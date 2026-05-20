package op

import (
	"context"
	"sync/atomic"
	"time"
)

var activeRequests atomic.Int64

// ActiveRequestsAdd 增加活跃请求计数
func ActiveRequestsAdd(delta int64) {
	activeRequests.Add(delta)
}

// ActiveRequestsCount 返回当前活跃请求数
func ActiveRequestsCount() int64 {
	return activeRequests.Load()
}

// RealtimeStats 实时监控统计数据
type RealtimeStats struct {
	ActiveRequests int64   `json:"active_requests"`
	QPS            float64 `json:"qps"`
	AvgResponseMs  float64 `json:"avg_response_ms"`
	ErrorRate      float64 `json:"error_rate"`
	ChannelCount   int     `json:"channel_count"`
}

// GetRealtimeStats 获取实时监控统计数据
func GetRealtimeStats() RealtimeStats {
	stats := RealtimeStats{
		ActiveRequests: ActiveRequestsCount(),
	}

	// 从当前小时统计计算 QPS、平均响应时间、错误率
	hourly := StatsHourlyGet()
	now := time.Now()
	elapsedSeconds := float64(now.Minute()*60 + now.Second())
	if elapsedSeconds < 1 {
		elapsedSeconds = 1
	}

	var totalRequests int64
	var totalWaitTime int64
	var totalFailed int64

	for _, h := range hourly {
		totalRequests += h.RequestSuccess + h.RequestFailed
		totalWaitTime += h.WaitTime
		totalFailed += h.RequestFailed
	}

	if totalRequests > 0 {
		stats.QPS = float64(totalRequests) / elapsedSeconds
		stats.AvgResponseMs = float64(totalWaitTime) / float64(totalRequests)
		stats.ErrorRate = float64(totalFailed) / float64(totalRequests) * 100
	}

	// 统计已启用的渠道数量
	channels, err := ChannelList(context.Background())
	if err == nil {
		for _, ch := range channels {
			if ch.Enabled {
				stats.ChannelCount++
			}
		}
	}

	return stats
}
