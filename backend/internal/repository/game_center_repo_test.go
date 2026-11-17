package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildGamePointsWhere(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		where, args := buildGamePointsWhere("gl", "created_at", service.GamePointsLedgerFilter{})
		require.Empty(t, where)
		require.Empty(t, args)
	})

	t.Run("filter with user_id only", func(t *testing.T) {
		uid := int64(123)
		where, args := buildGamePointsWhere("gl", "created_at", service.GamePointsLedgerFilter{
			UserID: &uid,
		})
		require.Equal(t, " WHERE gl.user_id = $1", where)
		require.Equal(t, []any{int64(123)}, args)
	})

	t.Run("filter with start and end time", func(t *testing.T) {
		start := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
		where, args := buildGamePointsWhere("gl", "created_at", service.GamePointsLedgerFilter{
			StartTime: &start,
			EndTime:   &end,
		})
		require.Equal(t, " WHERE gl.created_at >= $1 AND gl.created_at < $2", where)
		require.Len(t, args, 2)
		require.Equal(t, start, args[0])
		require.Equal(t, end, args[1])
	})

	t.Run("filter with user_id and times", func(t *testing.T) {
		uid := int64(456)
		start := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
		where, args := buildGamePointsWhere("gpc", "claimed_at", service.GamePointsLedgerFilter{
			UserID:    &uid,
			StartTime: &start,
			EndTime:   &end,
		})
		require.Equal(t, " WHERE gpc.user_id = $1 AND gpc.claimed_at >= $2 AND gpc.claimed_at < $3", where)
		require.Len(t, args, 3)
		require.Equal(t, int64(456), args[0])
		require.Equal(t, start, args[1])
		require.Equal(t, end, args[2])
	})
}
