package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIHealthRepoStub struct {
	monitors []*ChannelMonitor
	history  map[int64][]*ChannelMonitorHistoryEntry
	stats    map[int64]*ChannelMonitorWindowStats
}

func (r *openAIHealthRepoStub) Create(context.Context, *ChannelMonitor) error { return nil }
func (r *openAIHealthRepoStub) GetByID(context.Context, int64) (*ChannelMonitor, error) {
	return nil, ErrChannelMonitorNotFound
}
func (r *openAIHealthRepoStub) Update(context.Context, *ChannelMonitor) error { return nil }
func (r *openAIHealthRepoStub) Delete(context.Context, int64) error           { return nil }
func (r *openAIHealthRepoStub) List(ctx context.Context, params ChannelMonitorListParams) ([]*ChannelMonitor, int64, error) {
	items := make([]*ChannelMonitor, 0, len(r.monitors))
	for _, monitor := range r.monitors {
		if params.Provider != "" && monitor.Provider != params.Provider {
			continue
		}
		if params.Search != "" && !strings.Contains(strings.ToLower(monitor.Name), strings.ToLower(params.Search)) {
			continue
		}
		items = append(items, monitor)
	}
	return items, int64(len(items)), nil
}
func (r *openAIHealthRepoStub) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) MarkChecked(context.Context, int64, time.Time) error { return nil }
func (r *openAIHealthRepoStub) InsertHistoryBatch(context.Context, []*ChannelMonitorHistoryRow) error {
	return nil
}
func (r *openAIHealthRepoStub) DeleteHistoryBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *openAIHealthRepoStub) ListHistory(context.Context, int64, string, int) ([]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) ListLatestPerModel(context.Context, int64) ([]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) ComputeAvailability(context.Context, int64, int) ([]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) ListRecentHistoryForMonitors(_ context.Context, ids []int64, _ map[int64]string, _ int) (map[int64][]*ChannelMonitorHistoryEntry, error) {
	out := make(map[int64][]*ChannelMonitorHistoryEntry, len(ids))
	for _, id := range ids {
		out[id] = r.history[id]
	}
	return out, nil
}
func (r *openAIHealthRepoStub) ComputeWindowStatsForMonitors(_ context.Context, ids []int64, _ map[int64]string, _ time.Time) (map[int64]*ChannelMonitorWindowStats, error) {
	out := make(map[int64]*ChannelMonitorWindowStats, len(ids))
	for _, id := range ids {
		if stat := r.stats[id]; stat != nil {
			out[id] = stat
		}
	}
	return out, nil
}
func (r *openAIHealthRepoStub) UpsertDailyRollupsFor(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *openAIHealthRepoStub) DeleteRollupsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *openAIHealthRepoStub) LoadAggregationWatermark(context.Context) (*time.Time, error) {
	return nil, nil
}
func (r *openAIHealthRepoStub) UpdateAggregationWatermark(context.Context, time.Time) error {
	return nil
}

type openAIHealthPlainEncryptor struct{}

func (e openAIHealthPlainEncryptor) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (e openAIHealthPlainEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestChannelMonitorService_GetOpenAIHealthOverview_UsesWindowStats(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	latencyFast := 900
	latencySlow := 1800
	ping := 7
	repo := &openAIHealthRepoStub{
		monitors: []*ChannelMonitor{
			{
				ID:              7,
				Name:            "Kedaya",
				Provider:        MonitorProviderOpenAI,
				Endpoint:        "https://sub.kedaya.xyz",
				PrimaryModel:    "gpt-5.1",
				GroupName:       "GPT Plus",
				Enabled:         true,
				IntervalSeconds: 60,
				LastCheckedAt:   &now,
			},
		},
		stats: map[int64]*ChannelMonitorWindowStats{
			7: {
				TotalChecks:       3,
				OperationalChecks: 2,
				FailedChecks:      1,
				AvgLatencyMs:      &latencyFast,
				P95LatencyMs:      &latencySlow,
				AvgPingLatencyMs:  &ping,
			},
		},
		history: map[int64][]*ChannelMonitorHistoryEntry{
			7: {
				{Status: MonitorStatusOperational, LatencyMs: &latencyFast, PingLatencyMs: &ping, CheckedAt: now.Add(-1 * time.Minute)},
				{Status: MonitorStatusFailed, LatencyMs: nil, PingLatencyMs: &ping, CheckedAt: now.Add(-2 * time.Minute)},
			},
		},
	}
	svc := NewChannelMonitorService(repo, openAIHealthPlainEncryptor{})

	got, err := svc.GetOpenAIHealthOverview(context.Background(), OpenAIHealthQuery{
		Window: "6h",
		Now:    now,
	})

	require.NoError(t, err)
	require.Equal(t, "6h", got.TimeWindow)
	require.Equal(t, now.Add(-6*time.Hour), got.WindowStart)
	require.Len(t, got.Items, 1)
	item := got.Items[0]
	require.Equal(t, int64(7), item.ID)
	require.Equal(t, "Kedaya", item.Name)
	require.Equal(t, "GPT Plus", item.GroupName)
	require.Equal(t, 3, item.TotalChecks)
	require.Equal(t, 1, item.FailedChecks)
	require.InEpsilon(t, 66.666, item.AvailabilityPct, 0.001)
	require.Equal(t, &latencyFast, item.AvgFirstTokenMs)
	require.Equal(t, &latencySlow, item.P95FirstTokenMs)
	require.Equal(t, &ping, item.AvgPingLatencyMs)
	require.Len(t, item.Trend, 2)
}
