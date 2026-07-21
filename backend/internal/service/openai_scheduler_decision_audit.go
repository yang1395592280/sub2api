package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	OpenAISchedulerAuditShadowDecision      = "shadow_decision"
	OpenAISchedulerAuditExplorationAllowed  = "exploration_allowed"
	OpenAISchedulerAuditExplorationRejected = "exploration_rejected"
	OpenAISchedulerAuditExplorationError    = "exploration_error"
	OpenAISchedulerAuditLowConfidence       = "low_confidence_fallback"

	openAISchedulerDecisionAuditQueueSize = 4096
	openAISchedulerDecisionAuditTimeout   = 2 * time.Second
)

type OpenAISchedulerDecisionAuditEvent struct {
	EventType                 string
	GroupID                   int64
	AccountID                 int64
	LegacyAccountID           int64
	ModelFamily               string
	Endpoint                  string
	Transport                 string
	Reason                    string
	Confidence                string
	Eligibility               string
	TrafficClass              string
	PredictedTTFTDifferenceMS float64
	TargetShare               float64
	CandidateCount            int
	TopK                      int
	SchedulerMode             string
	ShadowMode                bool
	ExplorationRate           float64
	ExplorationBudget         float64
	LowConfidenceMaxShare     float64
	LatencyWeight             float64
	ReliabilityWeight         float64
	CreatedAt                 time.Time
}

type OpenAISchedulerDecisionAuditRepository interface {
	InsertOpenAISchedulerDecisionAudit(context.Context, OpenAISchedulerDecisionAuditEvent) error
}

type OpenAISchedulerDecisionAuditRecorderMetrics struct {
	Accepted   uint64
	Failed     uint64
	Dropped    uint64
	QueueDepth int
}

// OpenAISchedulerDecisionAuditRecorder keeps observability writes off the
// request path. Audit loss is preferable to delaying or failing a request.
type OpenAISchedulerDecisionAuditRecorder struct {
	repo     OpenAISchedulerDecisionAuditRepository
	queue    chan OpenAISchedulerDecisionAuditEvent
	done     chan struct{}
	queueMu  sync.RWMutex
	stopOnce sync.Once
	stopped  atomic.Bool
	accepted atomic.Uint64
	failed   atomic.Uint64
	dropped  atomic.Uint64
}

func NewOpenAISchedulerDecisionAuditRecorder(
	repo OpenAISchedulerDecisionAuditRepository,
	queueSize int,
) *OpenAISchedulerDecisionAuditRecorder {
	if queueSize < 1 {
		queueSize = 1
	}
	recorder := &OpenAISchedulerDecisionAuditRecorder{
		repo:  repo,
		queue: make(chan OpenAISchedulerDecisionAuditEvent, queueSize),
		done:  make(chan struct{}),
	}
	go recorder.run()
	return recorder
}

func (r *OpenAISchedulerDecisionAuditRecorder) TryRecord(event OpenAISchedulerDecisionAuditEvent) bool {
	if r == nil || r.repo == nil || r.stopped.Load() {
		return false
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
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
	case r.queue <- event:
		r.accepted.Add(1)
		return true
	default:
		r.recordDropped()
		return false
	}
}

func (r *OpenAISchedulerDecisionAuditRecorder) recordDropped() {
	dropped := r.dropped.Add(1)
	if shouldLogOpenAIAutoSchedulerOutcomeRecorderCount(dropped) {
		slog.Warn("OpenAI scheduler decision audit record dropped", "dropped", dropped, "queue_depth", len(r.queue))
	}
}

func (r *OpenAISchedulerDecisionAuditRecorder) Stop(ctx context.Context) error {
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
		r.queueMu.Unlock()
	})
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *OpenAISchedulerDecisionAuditRecorder) SnapshotMetrics() OpenAISchedulerDecisionAuditRecorderMetrics {
	if r == nil {
		return OpenAISchedulerDecisionAuditRecorderMetrics{}
	}
	return OpenAISchedulerDecisionAuditRecorderMetrics{
		Accepted: r.accepted.Load(), Failed: r.failed.Load(), Dropped: r.dropped.Load(), QueueDepth: len(r.queue),
	}
}

func (r *OpenAISchedulerDecisionAuditRecorder) run() {
	defer close(r.done)
	for event := range r.queue {
		ctx, cancel := context.WithTimeout(context.Background(), openAISchedulerDecisionAuditTimeout)
		err := r.repo.InsertOpenAISchedulerDecisionAudit(ctx, event)
		cancel()
		if err != nil {
			failed := r.failed.Add(1)
			if shouldLogOpenAIAutoSchedulerOutcomeRecorderCount(failed) {
				slog.Warn("OpenAI scheduler decision audit insert failed", "failed", failed, "error", err)
			}
		}
	}
}

func openAISchedulerDecisionAuditFromSettings(settings OpenAIBalancedSettings) OpenAISchedulerDecisionAuditEvent {
	return OpenAISchedulerDecisionAuditEvent{
		SchedulerMode: settings.Mode, ShadowMode: settings.ShadowMode,
		ExplorationRate: settings.ExplorationRate, ExplorationBudget: settings.ExplorationBudget,
		LowConfidenceMaxShare: settings.LowConfidenceMaxShare,
		LatencyWeight:         settings.Weights.Latency, ReliabilityWeight: settings.Weights.Reliability,
	}
}
