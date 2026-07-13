package service

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const openAIRequestTimingContextKey = "sub2api.openai_request_timing"

// OpenAIRequestTimingSnapshot is an immutable view of completed scheduling phases.
type OpenAIRequestTimingSnapshot struct {
	RoutingMS int
	QueueMS   int
	RetryMS   int
}

// OpenAIRequestTiming tracks request-scoped OpenAI scheduling durations.
type OpenAIRequestTiming struct {
	mu             sync.Mutex
	now            func() time.Time
	startedAt      time.Time
	routingStarted time.Time
	routing        time.Duration
	queue          time.Duration
	retry          time.Duration
}

func newOpenAIRequestTiming(now func() time.Time) *OpenAIRequestTiming {
	return &OpenAIRequestTiming{
		now:       now,
		startedAt: now(),
	}
}

// NewOpenAIRequestTiming starts timing for an independent WebSocket turn.
func NewOpenAIRequestTiming() *OpenAIRequestTiming {
	return newOpenAIRequestTiming(time.Now)
}

// BeginOpenAIRequestTiming creates request timing once and reuses it for retries.
func BeginOpenAIRequestTiming(c *gin.Context) *OpenAIRequestTiming {
	if timing := OpenAIRequestTimingFromContext(c); timing != nil {
		return timing
	}

	timing := newOpenAIRequestTiming(time.Now)
	c.Set(openAIRequestTimingContextKey, timing)
	return timing
}

// OpenAIRequestTimingFromContext returns the timing associated with c, if any.
func OpenAIRequestTimingFromContext(c *gin.Context) *OpenAIRequestTiming {
	value, ok := c.Get(openAIRequestTimingContextKey)
	if !ok {
		return nil
	}

	timing, _ := value.(*OpenAIRequestTiming)
	return timing
}

// BeginRouting starts a routing interval unless one is already active.
func (t *OpenAIRequestTiming) BeginRouting() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.routingStarted.IsZero() {
		t.routingStarted = t.now()
	}
}

// EndRouting completes the active routing interval.
func (t *OpenAIRequestTiming) EndRouting() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.routingStarted.IsZero() {
		return
	}

	t.routing += t.now().Sub(t.routingStarted)
	t.routingStarted = time.Time{}
}

// AddQueue adds time spent waiting for scheduling capacity.
func (t *OpenAIRequestTiming) AddQueue(duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queue += duration
}

// AddRetry adds time spent waiting between attempts.
func (t *OpenAIRequestTiming) AddRetry(duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.retry += duration
}

// E2EFirstTokenMS returns elapsed time from the first request start.
func (t *OpenAIRequestTiming) E2EFirstTokenMS() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return int(t.now().Sub(t.startedAt).Milliseconds())
}

// Snapshot returns completed scheduling durations in milliseconds.
func (t *OpenAIRequestTiming) Snapshot() OpenAIRequestTimingSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return OpenAIRequestTimingSnapshot{
		RoutingMS: int(t.routing.Milliseconds()),
		QueueMS:   int(t.queue.Milliseconds()),
		RetryMS:   int(t.retry.Milliseconds()),
	}
}

func applyOpenAIWSTurnTiming(timing *OpenAIRequestTiming, forwardStartedAt time.Time, result *OpenAIForwardResult) {
	if timing == nil || result == nil {
		return
	}
	snapshot := timing.Snapshot()
	result.RoutingMs = openAIOptionalTimingInt(snapshot.RoutingMS)
	result.QueueMs = openAIOptionalTimingInt(snapshot.QueueMS)
	result.RetryMs = openAIOptionalTimingInt(snapshot.RetryMS)
	if result.FirstTokenMs == nil {
		return
	}
	timing.mu.Lock()
	preForwardMS := int(forwardStartedAt.Sub(timing.startedAt).Milliseconds())
	timing.mu.Unlock()
	if preForwardMS < 0 {
		preForwardMS = 0
	}
	e2e := preForwardMS + *result.FirstTokenMs
	result.E2EFirstTokenMs = &e2e
}

func openAIOptionalTimingInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
