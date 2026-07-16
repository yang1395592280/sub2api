package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoCheapestGroupCircuit_OpensAfterSecondFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	circuit := NewOpenAIAutoCheapestGroupCircuit(rdb)
	ctx := context.Background()
	key := service.OpenAIAutoCheapestGroupHealthKey{GroupID: 42, Model: "gpt-5.4"}

	allowed, err := circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, circuit.RecordFailure(ctx, key, "upstream_503"))
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, circuit.RecordFailure(ctx, key, "upstream_503"))
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.False(t, allowed)

	mr.FastForward(service.OpenAIAutoCheapestCooldown + time.Second)
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, circuit.RecordSuccess(ctx, key))
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, circuit.RecordSuccess(ctx, key))
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
}
