package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAIAutoCheapestGroupCircuitPrefix = "openai:auto-group:circuit:"

type openAIAutoCheapestGroupCircuit struct{ rdb *redis.Client }

func NewOpenAIAutoCheapestGroupCircuit(rdb *redis.Client) service.OpenAIAutoCheapestGroupCircuit {
	return &openAIAutoCheapestGroupCircuit{rdb: rdb}
}

func openAIAutoCheapestGroupCircuitKey(key service.OpenAIAutoCheapestGroupHealthKey) string {
	return fmt.Sprintf("%s%d:%s:%s:%s", openAIAutoCheapestGroupCircuitPrefix, key.GroupID,
		service.NormalizeOpenAIAutoCheapestHealthModel(key.Model),
		service.NormalizeOpenAIAutoCheapestHealthPart(key.Endpoint),
		service.NormalizeOpenAIAutoCheapestHealthPart(key.Transport))
}

// Allow atomically skips open circuits and grants a single half-open probe.
var allowOpenAIAutoCheapestProbeScript = redis.NewScript(`
local state = redis.call('HGET', KEYS[1], 'state')
if not state then return 1 end
if state == 'open' then
  local cooldownUntil = tonumber(redis.call('HGET', KEYS[1], 'cooldown_until') or '0')
  local now = tonumber(ARGV[1])
  if now < cooldownUntil then return 0 end
  redis.call('HSET', KEYS[1], 'state', 'half_open', 'probe', '1', 'successes', '0')
  return 1
end
if state == 'half_open' then
  if redis.call('HGET', KEYS[1], 'probe') == '1' then return 0 end
  redis.call('HSET', KEYS[1], 'probe', '1')
  return 1
end
return 1
`)

func (c *openAIAutoCheapestGroupCircuit) Allow(ctx context.Context, key service.OpenAIAutoCheapestGroupHealthKey) (bool, error) {
	if c == nil || c.rdb == nil || !key.Valid() { return true, nil }
	result, err := allowOpenAIAutoCheapestProbeScript.Run(ctx, c.rdb, []string{openAIAutoCheapestGroupCircuitKey(key)}, time.Now().Unix()).Int()
	return result == 1, err
}

var recordOpenAIAutoCheapestFailureScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local cooldown = tonumber(ARGV[4])
local state = redis.call('HGET', key, 'state')
if state == 'half_open' then
  redis.call('HSET', key, 'state', 'open', 'opened_at', now, 'cooldown_until', now + cooldown, 'probe', '0', 'successes', '0')
  redis.call('EXPIRE', key, cooldown)
  return 0
end
local raw = redis.call('HMGET', key, 'window_start', 'failures')
local start = tonumber(raw[1]) or 0
local failures = tonumber(raw[2]) or 0
if start == 0 or now - start >= window then start = now; failures = 0 end
failures = failures + 1
if failures >= limit then
  redis.call('HSET', key, 'state', 'open', 'opened_at', now, 'cooldown_until', now + cooldown, 'window_start', start, 'failures', failures, 'probe', '0')
  redis.call('EXPIRE', key, cooldown)
else
  redis.call('HSET', key, 'state', 'closed', 'window_start', start, 'failures', failures)
  redis.call('EXPIRE', key, window)
end
return failures
`)

func (c *openAIAutoCheapestGroupCircuit) RecordFailure(ctx context.Context, key service.OpenAIAutoCheapestGroupHealthKey, reason string) error {
	if c == nil || c.rdb == nil || !key.Valid() { return nil }
	_, err := recordOpenAIAutoCheapestFailureScript.Run(ctx, c.rdb, []string{openAIAutoCheapestGroupCircuitKey(key)}, time.Now().Unix(), int64(service.OpenAIAutoCheapestFailureWindow/time.Second), service.OpenAIAutoCheapestFailureLimit, int64(service.OpenAIAutoCheapestCooldown/time.Second)).Result()
	return err
}

func (c *openAIAutoCheapestGroupCircuit) RecordSuccess(ctx context.Context, key service.OpenAIAutoCheapestGroupHealthKey) error {
	if c == nil || c.rdb == nil || !key.Valid() { return nil }
	redisKey := openAIAutoCheapestGroupCircuitKey(key)
	state, err := c.rdb.HGet(ctx, redisKey, "state").Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "half_open" {
		return nil
	}
	result, err := c.rdb.HIncrBy(ctx, redisKey, "successes", 1).Result()
	if err != nil {
		return err
	}
	if result >= 2 {
		return c.rdb.Del(ctx, redisKey).Err()
	}
	pipe := c.rdb.TxPipeline()
	pipe.HSet(ctx, redisKey, "state", "half_open", "probe", "0")
	pipe.Expire(ctx, redisKey, service.OpenAIAutoCheapestCooldown)
	_, err = pipe.Exec(ctx)
	return err
}

var _ service.OpenAIAutoCheapestGroupCircuit = (*openAIAutoCheapestGroupCircuit)(nil)
