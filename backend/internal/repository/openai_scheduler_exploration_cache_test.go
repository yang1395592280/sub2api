package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerExplorationCacheEnforcesIntervalAndWindowAtomically(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewOpenAISchedulerExplorationCache(client)
	key := service.OpenAISchedulerHealthKey{
		AccountID: 10, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "http_sse",
	}
	ctx := context.Background()

	allowed, err := cache.Reserve(ctx, key, 10*time.Second, 2)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = cache.Reserve(ctx, key, 10*time.Second, 2)
	require.NoError(t, err)
	require.False(t, allowed, "minimum interval must reject a duplicate reservation")

	server.FastForward(11 * time.Second)
	allowed, err = cache.Reserve(ctx, key, 10*time.Second, 2)
	require.NoError(t, err)
	require.True(t, allowed)
	server.FastForward(11 * time.Second)
	allowed, err = cache.Reserve(ctx, key, 10*time.Second, 2)
	require.NoError(t, err)
	require.False(t, allowed, "window quota must reject the third reservation")

	server.FastForward(time.Hour)
	allowed, err = cache.Reserve(ctx, key, 10*time.Second, 2)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestOpenAISchedulerExplorationRedisKeysUseOneClusterSlot(t *testing.T) {
	intervalKey, windowKey := openAISchedulerExplorationRedisKeys(service.OpenAISchedulerHealthKey{
		AccountID: 10, ModelFamily: " GPT-5 ", Endpoint: " Responses ", Transport: " HTTP_SSE ",
	})
	require.NotEqual(t, intervalKey, windowKey)
	intervalTagStart := len("openai:scheduler:exploration:")
	require.Equal(t, intervalKey[intervalTagStart:intervalTagStart+26], windowKey[intervalTagStart:intervalTagStart+26])
}

func TestOpenAISchedulerExplorationCacheReturnsStructuredDenialOutcome(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewOpenAISchedulerExplorationCache(client).(service.OpenAISchedulerDetailedExplorationCache)
	key := service.OpenAISchedulerHealthKey{AccountID: 11, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "http_sse"}

	outcome, err := cache.ReserveWithOutcome(context.Background(), key, 10*time.Second, 1)
	require.NoError(t, err)
	require.Equal(t, service.OpenAISchedulerExplorationReservationAllowed, outcome)
	outcome, err = cache.ReserveWithOutcome(context.Background(), key, 10*time.Second, 1)
	require.NoError(t, err)
	require.Equal(t, service.OpenAISchedulerExplorationReservationMinimumInterval, outcome)
	server.FastForward(11 * time.Second)
	outcome, err = cache.ReserveWithOutcome(context.Background(), key, 10*time.Second, 1)
	require.NoError(t, err)
	require.Equal(t, service.OpenAISchedulerExplorationReservationHourlyLimit, outcome)
}
