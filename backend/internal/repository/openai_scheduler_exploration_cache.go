package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAISchedulerExplorationWindow = time.Hour

var reserveOpenAISchedulerExplorationScript = redis.NewScript(`
local interval_ttl = tonumber(ARGV[1])
local max_samples = tonumber(ARGV[2])
local window_ttl = tonumber(ARGV[3])
if redis.call('EXISTS', KEYS[1]) == 1 then
	return -1
end
local current = tonumber(redis.call('GET', KEYS[2]) or '0')
if current >= max_samples then
	return -2
end
redis.call('SET', KEYS[1], '1', 'EX', interval_ttl)
local next = redis.call('INCR', KEYS[2])
if next == 1 or redis.call('TTL', KEYS[2]) == -1 then
  redis.call('EXPIRE', KEYS[2], window_ttl)
end
return 1
`)

type openAISchedulerExplorationCache struct {
	rdb *redis.Client
}

func NewOpenAISchedulerExplorationCache(rdb *redis.Client) service.OpenAISchedulerExplorationCache {
	return &openAISchedulerExplorationCache{rdb: rdb}
}

func (c *openAISchedulerExplorationCache) Reserve(
	ctx context.Context,
	key service.OpenAISchedulerHealthKey,
	minimumInterval time.Duration,
	maxSamplesPerHour int,
) (bool, error) {
	outcome, err := c.ReserveWithOutcome(ctx, key, minimumInterval, maxSamplesPerHour)
	return outcome == service.OpenAISchedulerExplorationReservationAllowed, err
}

func (c *openAISchedulerExplorationCache) ReserveWithOutcome(
	ctx context.Context,
	key service.OpenAISchedulerHealthKey,
	minimumInterval time.Duration,
	maxSamplesPerHour int,
) (service.OpenAISchedulerExplorationReservationOutcome, error) {
	if c == nil || c.rdb == nil {
		return "", fmt.Errorf("openai scheduler exploration redis is unavailable")
	}
	if key.AccountID <= 0 || strings.TrimSpace(key.ModelFamily) == "" || strings.TrimSpace(key.Endpoint) == "" || strings.TrimSpace(key.Transport) == "" {
		return "", fmt.Errorf("openai scheduler exploration health key is incomplete")
	}
	if minimumInterval < time.Second {
		minimumInterval = time.Second
	}
	if maxSamplesPerHour <= 0 {
		return service.OpenAISchedulerExplorationReservationDenied, nil
	}
	intervalKey, windowKey := openAISchedulerExplorationRedisKeys(key)
	reserved, err := reserveOpenAISchedulerExplorationScript.Run(
		ctx,
		c.rdb,
		[]string{intervalKey, windowKey},
		int64(minimumInterval/time.Second),
		maxSamplesPerHour,
		int64(openAISchedulerExplorationWindow/time.Second),
	).Int64()
	if err != nil {
		return "", fmt.Errorf("reserve openai scheduler exploration: %w", err)
	}
	if reserved == 1 {
		slog.Debug("openai_scheduler_exploration_reservation_allowed", "account_id", key.AccountID, "model_family", key.ModelFamily, "endpoint", key.Endpoint, "transport", key.Transport)
		return service.OpenAISchedulerExplorationReservationAllowed, nil
	}
	reason := "hourly_limit"
	if reserved == -1 {
		reason = "minimum_interval"
	}
	slog.Debug("openai_scheduler_exploration_reservation_denied", "account_id", key.AccountID, "model_family", key.ModelFamily, "endpoint", key.Endpoint, "transport", key.Transport, "reason", reason)
	if reserved == -1 {
		return service.OpenAISchedulerExplorationReservationMinimumInterval, nil
	}
	return service.OpenAISchedulerExplorationReservationHourlyLimit, nil
}

func openAISchedulerExplorationRedisKeys(key service.OpenAISchedulerHealthKey) (string, string) {
	raw := fmt.Sprintf("%d:%s:%s:%s",
		key.AccountID,
		strings.ToLower(strings.TrimSpace(key.ModelFamily)),
		strings.ToLower(strings.TrimSpace(key.Endpoint)),
		strings.ToLower(strings.TrimSpace(key.Transport)),
	)
	digest := sha256.Sum256([]byte(raw))
	tag := hex.EncodeToString(digest[:12])
	prefix := "openai:scheduler:exploration:{" + tag + "}:"
	return prefix + "interval", prefix + "window"
}

var _ service.OpenAISchedulerExplorationCache = (*openAISchedulerExplorationCache)(nil)
var _ service.OpenAISchedulerDetailedExplorationCache = (*openAISchedulerExplorationCache)(nil)
