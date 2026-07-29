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

func TestOpenAIAutoCheapestGroupCircuit_RequiresDistinctUsersBeforeOpening(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	circuit := NewOpenAIAutoCheapestGroupCircuit(rdb)
	ctx := context.Background()
	key := service.OpenAIAutoCheapestGroupHealthKey{GroupID: 42, UserID: 7, Model: "gpt-5.4"}

	allowed, err := circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, circuit.RecordFailure(ctx, key, "no_available_accounts"))
	require.NoError(t, circuit.RecordFailure(ctx, key, "no_available_accounts"))
	allowed, err = circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed, "repeated failures from one user must not open the global circuit")

	secondUserKey := key
	secondUserKey.UserID = 8
	require.NoError(t, circuit.RecordFailure(ctx, secondUserKey, "no_available_accounts"))
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

func TestOpenAIAutoCheapestGroupCircuit_CooldownIsTenSeconds(t *testing.T) {
	require.Equal(t, 10*time.Second, service.OpenAIAutoCheapestCooldown)
}

func TestOpenAIAutoCheapestGroupCircuit_IgnoresFailureWithoutUser(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	circuit := NewOpenAIAutoCheapestGroupCircuit(rdb)
	ctx := context.Background()
	key := service.OpenAIAutoCheapestGroupHealthKey{GroupID: 42, Model: "gpt-5.4"}

	require.NoError(t, circuit.RecordFailure(ctx, key, "no_available_accounts"))
	allowed, err := circuit.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)
	require.False(t, mr.Exists(openAIAutoCheapestGroupCircuitKey(key)))
}
