package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	openAIHealthDefaultWindow = "6h"
	openAIHealthTrendLimit    = 60
)

// GetOpenAIHealthOverview 按窗口聚合 OpenAI 监控数据，供管理员健康看板展示。
func (s *ChannelMonitorService) GetOpenAIHealthOverview(ctx context.Context, query OpenAIHealthQuery) (*OpenAIHealthOverview, error) {
	if s == nil || s.repo == nil {
		return &OpenAIHealthOverview{TimeWindow: normalizeOpenAIHealthWindow(query.Window)}, nil
	}
	window, duration := parseOpenAIHealthWindow(query.Window)
	now := query.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	since := now.Add(-duration)

	monitors, _, err := s.List(ctx, ChannelMonitorListParams{
		Page:     1,
		PageSize: 200,
		Provider: MonitorProviderOpenAI,
		Search:   strings.TrimSpace(query.Search),
	})
	if err != nil {
		return nil, err
	}
	monitors = filterOpenAIHealthMonitorsByGroup(monitors, query.GroupName)

	ids, primaryModels := openAIHealthMonitorTargets(monitors)
	stats, err := s.repo.ComputeWindowStatsForMonitors(ctx, ids, primaryModels, since)
	if err != nil {
		return nil, fmt.Errorf("compute openai health stats: %w", err)
	}
	history, err := s.repo.ListRecentHistoryForMonitors(ctx, ids, primaryModels, openAIHealthTrendLimit)
	if err != nil {
		return nil, fmt.Errorf("load openai health trend: %w", err)
	}

	items := buildOpenAIHealthItems(monitors, stats, history)
	overview := summarizeOpenAIHealthItems(items)
	overview.TimeWindow = window
	overview.WindowStart = since
	overview.WindowEnd = now
	overview.Items = items
	return overview, nil
}

func filterOpenAIHealthMonitorsByGroup(monitors []*ChannelMonitor, groupName string) []*ChannelMonitor {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return monitors
	}
	out := make([]*ChannelMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor.GroupName == groupName {
			out = append(out, monitor)
		}
	}
	return out
}

func openAIHealthMonitorTargets(monitors []*ChannelMonitor) ([]int64, map[int64]string) {
	ids := make([]int64, 0, len(monitors))
	primaryModels := make(map[int64]string, len(monitors))
	for _, monitor := range monitors {
		ids = append(ids, monitor.ID)
		primaryModels[monitor.ID] = monitor.PrimaryModel
	}
	return ids, primaryModels
}

func buildOpenAIHealthItems(
	monitors []*ChannelMonitor,
	stats map[int64]*ChannelMonitorWindowStats,
	history map[int64][]*ChannelMonitorHistoryEntry,
) []OpenAIHealthItem {
	items := make([]OpenAIHealthItem, 0, len(monitors))
	for _, monitor := range monitors {
		stat := stats[monitor.ID]
		item := OpenAIHealthItem{
			ID:            monitor.ID,
			Name:          monitor.Name,
			Endpoint:      monitor.Endpoint,
			GroupName:     monitor.GroupName,
			PrimaryModel:  monitor.PrimaryModel,
			Enabled:       monitor.Enabled,
			LastCheckedAt: monitor.LastCheckedAt,
		}
		if stat != nil {
			item.TotalChecks = stat.TotalChecks
			item.OperationalChecks = stat.OperationalChecks
			item.FailedChecks = stat.FailedChecks
			item.ErrorChecks = stat.ErrorChecks
			item.AvgFirstTokenMs = stat.AvgLatencyMs
			item.P95FirstTokenMs = stat.P95LatencyMs
			item.AvgPingLatencyMs = stat.AvgPingLatencyMs
			if stat.TotalChecks > 0 {
				item.AvailabilityPct = float64(stat.OperationalChecks) * 100.0 / float64(stat.TotalChecks)
			}
		}
		item.Trend = openAIHealthTrendPoints(history[monitor.ID])
		if len(history[monitor.ID]) > 0 {
			latest := history[monitor.ID][0]
			item.LatestStatus = latest.Status
			item.LatestFirstTokenMs = latest.LatencyMs
			item.LatestPingLatencyMs = latest.PingLatencyMs
			if monitor.LastCheckedAt == nil {
				checkedAt := latest.CheckedAt
				item.LastCheckedAt = &checkedAt
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].AvailabilityPct != items[j].AvailabilityPct {
			return items[i].AvailabilityPct < items[j].AvailabilityPct
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func openAIHealthTrendPoints(entries []*ChannelMonitorHistoryEntry) []OpenAIHealthTrendPoint {
	out := make([]OpenAIHealthTrendPoint, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		out = append(out, OpenAIHealthTrendPoint{
			Status:        entry.Status,
			LatencyMs:     entry.LatencyMs,
			PingLatencyMs: entry.PingLatencyMs,
			CheckedAt:     entry.CheckedAt,
		})
	}
	return out
}

func summarizeOpenAIHealthItems(items []OpenAIHealthItem) *OpenAIHealthOverview {
	overview := &OpenAIHealthOverview{
		TotalMonitors: len(items),
	}
	var availabilitySum float64
	var latencySum int
	var latencyCount int
	for _, item := range items {
		availabilitySum += item.AvailabilityPct
		if item.LatestStatus == MonitorStatusOperational {
			overview.HealthyMonitors++
		} else if item.LatestStatus == MonitorStatusDegraded {
			overview.DegradedMonitors++
		} else if item.LatestStatus == MonitorStatusFailed || item.LatestStatus == MonitorStatusError {
			overview.FailedMonitors++
		}
		if item.AvgFirstTokenMs != nil {
			latencySum += *item.AvgFirstTokenMs
			latencyCount++
		}
	}
	if len(items) > 0 {
		overview.AverageAvailabilityPct = availabilitySum / float64(len(items))
	}
	if latencyCount > 0 {
		v := latencySum / latencyCount
		overview.AverageFirstTokenMs = &v
	}
	return overview
}

func parseOpenAIHealthWindow(raw string) (string, time.Duration) {
	switch normalizeOpenAIHealthWindow(raw) {
	case "24h":
		return "24h", 24 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	case "30d":
		return "30d", 30 * 24 * time.Hour
	default:
		return openAIHealthDefaultWindow, 6 * time.Hour
	}
}

func normalizeOpenAIHealthWindow(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "24h", "7d", "30d":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return openAIHealthDefaultWindow
	}
}
