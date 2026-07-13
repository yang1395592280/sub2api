package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	openAIAutoSchedulerOutcomeRecordTimeout = 2 * time.Second
	openAIAutoSchedulerOutcomeQueueSize     = 1024
	openAIAutoSchedulerOutcomeWorkerCount   = 2
	openAIAutoSchedulerOutcomeMetricsPeriod = 30 * time.Second
)

type openAIAutoSchedulerOutcomeSink interface {
	Record(context.Context, OpenAIAutoSchedulerRecordInput) error
}

type openAIAutoSchedulerStrictOutcomeSink interface {
	RecordOutcome(context.Context, OpenAIAutoSchedulerRecordInput) error
}

type OpenAIAutoSchedulerOutcomeRecorderMetrics struct {
	Accepted   uint64
	Failed     uint64
	Dropped    uint64
	QueueDepth int
}

// OpenAIAutoSchedulerOutcomeRecorder keeps scheduler feedback off the request
// path while bounding both queued work and background concurrency.
type OpenAIAutoSchedulerOutcomeRecorder struct {
	sink          openAIAutoSchedulerOutcomeSink
	queue         chan OpenAIAutoSchedulerRecordInput
	done          chan struct{}
	monitorStop   chan struct{}
	monitorDone   chan struct{}
	metricsPeriod time.Duration

	queueMu  sync.RWMutex
	stopOnce sync.Once
	stopped  atomic.Bool
	workers  atomic.Int32

	accepted        atomic.Uint64
	failed          atomic.Uint64
	dropped         atomic.Uint64
	reportedDropped atomic.Uint64
}

func NewOpenAIAutoSchedulerOutcomeRecorder(
	sink openAIAutoSchedulerOutcomeSink,
	queueSize int,
	workerCount int,
) *OpenAIAutoSchedulerOutcomeRecorder {
	return newOpenAIAutoSchedulerOutcomeRecorder(sink, queueSize, workerCount, openAIAutoSchedulerOutcomeMetricsPeriod)
}

func newOpenAIAutoSchedulerOutcomeRecorder(
	sink openAIAutoSchedulerOutcomeSink,
	queueSize int,
	workerCount int,
	metricsPeriod time.Duration,
) *OpenAIAutoSchedulerOutcomeRecorder {
	if queueSize < 1 {
		queueSize = 1
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if metricsPeriod <= 0 {
		metricsPeriod = openAIAutoSchedulerOutcomeMetricsPeriod
	}

	recorder := &OpenAIAutoSchedulerOutcomeRecorder{
		sink:          sink,
		queue:         make(chan OpenAIAutoSchedulerRecordInput, queueSize),
		done:          make(chan struct{}),
		monitorStop:   make(chan struct{}),
		monitorDone:   make(chan struct{}),
		metricsPeriod: metricsPeriod,
	}
	recorder.workers.Store(int32(workerCount))
	for range workerCount {
		go recorder.runWorker()
	}
	go recorder.runMetricsLogger()
	return recorder
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) TryRecord(input OpenAIAutoSchedulerRecordInput) bool {
	if r == nil {
		return false
	}
	if r.stopped.Load() {
		r.recordDropped()
		return false
	}

	if !r.queueMu.TryRLock() {
		r.recordDropped()
		return false
	}
	defer r.queueMu.RUnlock()
	if r.stopped.Load() {
		r.recordDropped()
		return false
	}

	select {
	case r.queue <- input:
		r.accepted.Add(1)
		return true
	default:
		r.recordDropped()
		return false
	}
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.stopOnce.Do(func() {
		r.queueMu.Lock()
		r.stopped.Store(true)
		close(r.queue)
		close(r.monitorStop)
		r.queueMu.Unlock()
	})

	select {
	case <-r.done:
		select {
		case <-r.monitorDone:
		case <-ctx.Done():
			return ctx.Err()
		}
		r.logDroppedFeedback()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) SnapshotMetrics() OpenAIAutoSchedulerOutcomeRecorderMetrics {
	if r == nil {
		return OpenAIAutoSchedulerOutcomeRecorderMetrics{}
	}
	return OpenAIAutoSchedulerOutcomeRecorderMetrics{
		Accepted:   r.accepted.Load(),
		Failed:     r.failed.Load(),
		Dropped:    r.dropped.Load(),
		QueueDepth: len(r.queue),
	}
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) runWorker() {
	defer func() {
		if r.workers.Add(-1) == 0 {
			close(r.done)
		}
	}()

	for input := range r.queue {
		recordCtx, cancel := context.WithTimeout(context.Background(), openAIAutoSchedulerOutcomeRecordTimeout)
		input = classifyOpenAIAutoSchedulerProductionOutcome(recordCtx, r.sink, input)
		err := r.persistOutcome(recordCtx, input)
		cancel()
		if err != nil {
			failed := r.failed.Add(1)
			if shouldLogOpenAIAutoSchedulerOutcomeRecorderCount(failed) {
				slog.Warn("OpenAI auto scheduler outcome recorder sink failed", "failed", failed, "error", err)
			}
		}
	}
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) persistOutcome(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error {
	if strict, ok := r.sink.(openAIAutoSchedulerStrictOutcomeSink); ok {
		return strict.RecordOutcome(ctx, input)
	}
	return r.sink.Record(ctx, input)
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) runMetricsLogger() {
	defer close(r.monitorDone)
	ticker := time.NewTicker(r.metricsPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			metrics := r.SnapshotMetrics()
			slog.Info("OpenAI auto scheduler outcome recorder metrics",
				"accepted", metrics.Accepted,
				"failed", metrics.Failed,
				"dropped", metrics.Dropped,
				"queue_depth", metrics.QueueDepth,
			)
		case <-r.monitorStop:
			return
		}
	}
}

func classifyOpenAIAutoSchedulerProductionOutcome(
	ctx context.Context,
	sink openAIAutoSchedulerOutcomeSink,
	input OpenAIAutoSchedulerRecordInput,
) OpenAIAutoSchedulerRecordInput {
	if input.EventType != OpenAIAutoSchedulerEventSuccess {
		return input
	}
	svc, ok := sink.(*OpenAIAutoSchedulerService)
	if !ok || svc == nil {
		return input
	}
	eventType := classifyOpenAIAutoSchedulerProbeEvent(OpenAIAutoSchedulerProbeResult{
		Success:   true,
		LatencyMS: input.LatencyMS,
		TtfbMS:    input.TtfbMS,
	}, svc.settings(ctx))
	if eventType == OpenAIAutoSchedulerEventProbeSuccess {
		eventType = OpenAIAutoSchedulerEventSuccess
	}
	input.EventType = eventType
	return input
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) recordDropped() {
	r.dropped.Add(1)
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) logDroppedFeedback() {
	dropped := r.dropped.Load()
	if dropped == 0 {
		return
	}
	for {
		reported := r.reportedDropped.Load()
		if dropped <= reported {
			return
		}
		if r.reportedDropped.CompareAndSwap(reported, dropped) {
			slog.Warn("OpenAI auto scheduler outcome recorder dropped feedback", "dropped", dropped)
			return
		}
	}
}

func shouldLogOpenAIAutoSchedulerOutcomeRecorderCount(count uint64) bool {
	return count == 1 || count&(count-1) == 0
}
