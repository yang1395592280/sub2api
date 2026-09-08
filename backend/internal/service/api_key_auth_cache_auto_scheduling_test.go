//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotRoundTrip_PreservesAutomaticSchedulingAndPriceGuard(t *testing.T) {
	groupID := int64(41)
	apiKey := &APIKey{
		ID:      7,
		UserID:  9,
		GroupID: &groupID,
		Key:     "sk-auto-scheduling-roundtrip",
		Status:  StatusActive,
		User:    &User{ID: 9, Status: StatusActive},
		Group: &Group{
			ID:                                    groupID,
			Name:                                  "openai-auto",
			Platform:                              PlatformOpenAI,
			Status:                                StatusActive,
			OpenAIAutoSchedulerEnabled:            true,
			AllowAutoCheapestScheduling:           true,
			UpstreamBalanceRefreshEnabled:         true,
			UpstreamBalanceRefreshIntervalSeconds: 777,
			UpstreamPriceMaxMultiplier:            1.25,
			UpstreamPriceGroupingEnabled:          true,
			UpstreamPriceGroupingMin:              0.1,
			UpstreamPriceGroupingMax:              0.3,
		},
	}

	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	require.NotNil(t, snapshot)
	require.NotNil(t, snapshot.Group)

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	var restored APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &restored))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &restored)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized)
	require.NotNil(t, materialized.Group)
	require.True(t, materialized.Group.OpenAIAutoSchedulerEnabled)
	require.True(t, materialized.Group.AllowAutoCheapestScheduling)
	require.True(t, materialized.Group.UpstreamBalanceRefreshEnabled)
	require.Equal(t, 777, materialized.Group.UpstreamBalanceRefreshIntervalSeconds)
	require.InDelta(t, 1.25, materialized.Group.UpstreamPriceMaxMultiplier, 1e-12)
	require.True(t, materialized.Group.UpstreamPriceGroupingEnabled)
	require.InDelta(t, 0.1, materialized.Group.UpstreamPriceGroupingMin, 1e-12)
	require.InDelta(t, 0.3, materialized.Group.UpstreamPriceGroupingMax, 1e-12)
}
