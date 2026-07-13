package service

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRequestTimingSnapshot(t *testing.T) {
	now := time.Unix(100, 0)
	timing := newOpenAIRequestTiming(func() time.Time { return now })
	timing.BeginRouting()
	now = now.Add(12 * time.Millisecond)
	timing.EndRouting()
	timing.AddQueue(40 * time.Millisecond)
	timing.AddRetry(25 * time.Millisecond)
	now = now.Add(900 * time.Millisecond)

	snapshot := timing.Snapshot()
	require.Equal(t, 12, snapshot.RoutingMS)
	require.Equal(t, 40, snapshot.QueueMS)
	require.Equal(t, 25, snapshot.RetryMS)
	require.Equal(t, 912, timing.E2EFirstTokenMS())
}

func TestOpenAIRequestTimingBeginIsIdempotent(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	first := BeginOpenAIRequestTiming(c)
	second := BeginOpenAIRequestTiming(c)

	require.Same(t, first, second)
	require.Same(t, first, OpenAIRequestTimingFromContext(c))
}

func TestOpenAIRequestTimingFromContextReturnsNilBeforeBegin(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.Nil(t, OpenAIRequestTimingFromContext(c))
}

func TestOpenAIRequestTimingConcurrentQueueAndRetry(t *testing.T) {
	timing := newOpenAIRequestTiming(time.Now)

	const additions = 100
	var wg sync.WaitGroup
	wg.Add(additions * 2)
	for range additions {
		go func() {
			defer wg.Done()
			timing.AddQueue(time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			timing.AddRetry(2 * time.Millisecond)
		}()
	}
	wg.Wait()

	snapshot := timing.Snapshot()
	require.Equal(t, additions, snapshot.QueueMS)
	require.Equal(t, additions*2, snapshot.RetryMS)
}

func TestApplyOpenAIWSTurnTimingPreservesFirstTokenAndUsesIndependentTurn(t *testing.T) {
	now := time.Unix(100, 0)
	timing := newOpenAIRequestTiming(func() time.Time { return now })
	timing.AddQueue(12 * time.Millisecond)
	now = now.Add(40 * time.Millisecond)
	firstTokenMS := 25
	result := &OpenAIForwardResult{FirstTokenMs: &firstTokenMS}

	applyOpenAIWSTurnTiming(timing, now, result)

	require.Equal(t, 25, *result.FirstTokenMs)
	require.Equal(t, 65, *result.E2EFirstTokenMs)
	require.Equal(t, 12, *result.QueueMs)
}
