package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type liyuBalanceCacheRepository struct {
	service.ZenxiangLiyuRepository
	service.UserRepository
	balance     float64
	delta       float64
	replay      bool
	skipped     bool
	err         error
	afterCommit func()
}

func (r *liyuBalanceCacheRepository) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 42, Balance: r.balance}, nil
}

func (r *liyuBalanceCacheRepository) commit() error {
	if r.err != nil {
		return r.err
	}
	if !r.replay && !r.skipped {
		r.balance += r.delta
	}
	if r.afterCommit != nil {
		r.afterCommit()
	}
	return nil
}

func (r *liyuBalanceCacheRepository) Play(_ context.Context, cmd service.ZenxiangLiyuPlayCommand) (*service.ZenxiangLiyuPlayResult, error) {
	if err := r.commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuPlayResult{Applied: !r.replay, RequestID: cmd.RequestID, RewardAmount: r.delta, BalanceAfterReward: r.balance}, nil
}

func (r *liyuBalanceCacheRepository) PlayLuckyCoin(_ context.Context, cmd service.ZenxiangLiyuLuckyCoinCommand) (*service.ZenxiangLiyuLuckyCoinResult, error) {
	if err := r.commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuLuckyCoinResult{RecordID: cmd.RecordID, AdjustmentAmount: r.delta, BalanceAfter: r.balance}, nil
}

func (r *liyuBalanceCacheRepository) PlayGuessSize(_ context.Context, cmd service.ZenxiangLiyuGuessSizeCommand) (*service.ZenxiangLiyuGuessSizeResult, error) {
	if err := r.commit(); err != nil {
		return nil, err
	}
	return &service.ZenxiangLiyuGuessSizeResult{RecordID: cmd.RecordID, Choice: cmd.Choice, AdjustmentAmount: r.delta, BalanceAfter: r.balance, Skipped: r.skipped}, nil
}

type liyuBalanceAuthInvalidator struct {
	service.APIKeyAuthCacheInvalidator
	users       []int64
	contextErr  error
	hasDeadline bool
}

func (a *liyuBalanceAuthInvalidator) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	a.users = append(a.users, userID)
	a.contextErr = ctx.Err()
	_, a.hasDeadline = ctx.Deadline()
}

func TestZenxiangLiyuBalanceCacheAfterHTTPCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name              string
		operation         string
		delta             float64
		replay            bool
		skipped           bool
		storageFailure    bool
		cancelAfterCommit bool
		cacheFailure      bool
	}{
		{name: "reward restores concurrent request admission", operation: "play", delta: 3.5},
		{name: "paid play reduces balance", operation: "play", delta: -1},
		{name: "replayed reward heals stale cache without crediting again", operation: "play", delta: 3.5, replay: true},
		{name: "lucky coin win", operation: "lucky-coin", delta: 1},
		{name: "lucky coin loss", operation: "lucky-coin", delta: -1},
		{name: "guess size win", operation: "guess-size", delta: 1},
		{name: "guess size loss", operation: "guess-size", delta: -1},
		{name: "guess size skipped", operation: "guess-size", skipped: true},
		{name: "play transaction fails", operation: "play", storageFailure: true},
		{name: "lucky coin transaction fails", operation: "lucky-coin", storageFailure: true},
		{name: "guess size transaction fails", operation: "guess-size", storageFailure: true},
		{name: "request canceled after commit", operation: "play", delta: 3.5, cancelAfterCommit: true},
		{name: "cache failure does not undo committed reward", operation: "play", delta: 3.5, cacheFailure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			redisServer := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr(), MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
			t.Cleanup(func() { _ = rdb.Close() })
			cache := repository.NewBillingCache(rdb)
			const initial = 0.644217
			require.NoError(t, cache.SetUserBalance(ctx, 42, initial))
			repo := &liyuBalanceCacheRepository{balance: initial, delta: tt.delta, replay: tt.replay, skipped: tt.skipped}
			if tt.replay {
				repo.balance += tt.delta
			}
			if tt.storageFailure {
				repo.err = errors.New("transaction failed")
			}
			if tt.cancelAfterCommit {
				repo.afterCommit = cancel
			}
			if tt.cacheFailure {
				repo.afterCommit = redisServer.Close
			}
			cfg := &config.Config{}
			cfg.Billing.InflightReservation.Enabled = true
			billing := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			auth := &liyuBalanceAuthInvalidator{}
			svc := service.ProvideZenxiangLiyuService(repo, auth, billing)
			router := authenticatedZenxiangLiyuTestRouter(NewZenxiangLiyuHandler(svc))

			inflight := cache.(service.InflightBalanceReservationCache)
			allowed, _, err := inflight.ReserveInflightBalance(ctx, 42, "existing-request", 0.537553, initial, time.Minute)
			require.NoError(t, err)
			require.True(t, allowed)
			_, err = billing.ReserveInflight(ctx, &service.User{ID: 42}, nil, nil, 0.144576)
			require.ErrorIs(t, err, service.ErrInsufficientBalance)

			path, body := "/zenxiang-liyu/play", `{"request_id":"reward-1"}`
			if tt.operation != "play" {
				path = "/zenxiang-liyu/records/1/" + tt.operation
				body = `{"choice":"big"}`
				if tt.skipped {
					body = `{"choice":"skip"}`
				}
			}
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if tt.storageFailure {
				require.Equal(t, http.StatusInternalServerError, response.Code)
			} else {
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			}
			if tt.storageFailure || tt.skipped {
				require.Empty(t, auth.users)
				balance, err := cache.GetUserBalance(context.Background(), 42)
				require.NoError(t, err)
				require.Equal(t, initial, balance)
				require.Equal(t, initial, repo.balance)
				return
			}
			require.Equal(t, []int64{42}, auth.users)
			require.NoError(t, auth.contextErr)
			require.True(t, auth.hasDeadline)
			require.InDelta(t, initial+tt.delta, repo.balance, 1e-9)
			if tt.cacheFailure {
				return
			}
			require.False(t, redisServer.Exists("billing:balance:{42}"), "reward must invalidate the cached balance before returning")
			require.True(t, redisServer.Exists("billing:inflight:{42}"), "existing requests retain their reservations")
			balance, err := billing.GetUserBalance(context.Background(), 42)
			require.NoError(t, err)
			require.InDelta(t, repo.balance, balance, 1e-9)
			reservation, err := billing.ReserveInflight(context.Background(), &service.User{ID: 42}, nil, nil, 0.144576)
			if tt.delta < 0 {
				require.ErrorIs(t, err, service.ErrInsufficientBalance)
			} else {
				require.NoError(t, err)
				require.NotNil(t, reservation)
				reservation.HandlerDone()
			}
		})
	}
}
