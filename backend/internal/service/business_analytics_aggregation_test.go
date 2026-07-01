package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type businessAnalyticsAggregationRepoSpy struct {
	mu          sync.Mutex
	dailyCalls  []businessAnalyticsDailyCall
	weeklyCalls []time.Time

	blockDaily chan struct{}
	started    chan struct{}
	maxRunning int32
	running    int32
}

type businessAnalyticsDailyCall struct {
	start time.Time
	end   time.Time
}

func (r *businessAnalyticsAggregationRepoSpy) RecomputeDaily(ctx context.Context, startDate, endDate time.Time) error {
	current := atomic.AddInt32(&r.running, 1)
	defer atomic.AddInt32(&r.running, -1)
	for {
		max := atomic.LoadInt32(&r.maxRunning)
		if current <= max || atomic.CompareAndSwapInt32(&r.maxRunning, max, current) {
			break
		}
	}
	if r.started != nil {
		select {
		case r.started <- struct{}{}:
		default:
		}
	}
	if r.blockDaily != nil {
		select {
		case <-r.blockDaily:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.dailyCalls = append(r.dailyCalls, businessAnalyticsDailyCall{start: startDate, end: endDate})
	return nil
}

func (r *businessAnalyticsAggregationRepoSpy) RecomputeWeekly(ctx context.Context, weekStart time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.weeklyCalls = append(r.weeklyCalls, weekStart)
	return nil
}

func (r *businessAnalyticsAggregationRepoSpy) snapshot() ([]businessAnalyticsDailyCall, []time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	daily := append([]businessAnalyticsDailyCall(nil), r.dailyCalls...)
	weekly := append([]time.Time(nil), r.weeklyCalls...)
	return daily, weekly
}

type businessAnalyticsSchedulerSpy struct {
	mu        sync.Mutex
	name      string
	interval  time.Duration
	fn        func()
	cancelled []string
}

func (s *businessAnalyticsSchedulerSpy) ScheduleRecurring(name string, interval time.Duration, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
	s.interval = interval
	s.fn = fn
}

func (s *businessAnalyticsSchedulerSpy) Cancel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelled = append(s.cancelled, name)
}

func (s *businessAnalyticsSchedulerSpy) trigger() {
	s.mu.Lock()
	fn := s.fn
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func TestBusinessAnalyticsAggregationService_DisabledDoesNotScheduleOrExecute(t *testing.T) {
	repo := &businessAnalyticsAggregationRepoSpy{}
	scheduler := &businessAnalyticsSchedulerSpy{}
	svc := NewBusinessAnalyticsAggregationService(repo, nil, &config.Config{
		BusinessAnalytics: config.BusinessAnalyticsConfig{
			Enabled:                    false,
			AggregationIntervalSeconds: 1,
			LookbackSeconds:            7200,
			BackfillEnabled:            true,
			BackfillMaxDays:            90,
		},
	})
	svc.timingWheel = scheduler

	svc.Start()
	svc.runScheduledAggregation()

	if scheduler.name != "" {
		t.Fatalf("disabled service scheduled job %q", scheduler.name)
	}
	daily, weekly := repo.snapshot()
	if len(daily) != 0 || len(weekly) != 0 {
		t.Fatalf("disabled service executed repo calls: daily=%d weekly=%d", len(daily), len(weekly))
	}
}

func TestBusinessAnalyticsAggregationService_TriggerRecomputeRangeRejectsInvalidRanges(t *testing.T) {
	repo := &businessAnalyticsAggregationRepoSpy{}
	svc := NewBusinessAnalyticsAggregationService(repo, nil, &config.Config{
		BusinessAnalytics: config.BusinessAnalyticsConfig{
			Enabled:         true,
			BackfillEnabled: true,
			BackfillMaxDays: 90,
		},
	})

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := svc.TriggerRecomputeRange(now, now); err == nil {
		t.Fatal("expected equal start/end to be rejected")
	}
	if err := svc.TriggerRecomputeRange(now, now.Add(-time.Second)); err == nil {
		t.Fatal("expected end before start to be rejected")
	}
	if err := svc.TriggerRecomputeRange(now.AddDate(0, 0, -91), now); err == nil {
		t.Fatal("expected range beyond backfill_max_days to be rejected")
	}

	daily, weekly := repo.snapshot()
	if len(daily) != 0 || len(weekly) != 0 {
		t.Fatalf("invalid ranges should not call repo: daily=%d weekly=%d", len(daily), len(weekly))
	}
}

func TestBusinessAnalyticsAggregationService_ScheduledAggregationRecomputesRecentDailyAndCurrentWeek(t *testing.T) {
	repo := &businessAnalyticsAggregationRepoSpy{}
	scheduler := &businessAnalyticsSchedulerSpy{}
	svc := NewBusinessAnalyticsAggregationService(repo, nil, &config.Config{
		BusinessAnalytics: config.BusinessAnalyticsConfig{
			Enabled:                    true,
			AggregationIntervalSeconds: 300,
			LookbackSeconds:            7200,
			BackfillEnabled:            true,
			BackfillMaxDays:            90,
		},
	})
	svc.timingWheel = scheduler

	svc.Start()
	if scheduler.name != businessAnalyticsAggregationJobName {
		t.Fatalf("scheduled job name = %q, want %q", scheduler.name, businessAnalyticsAggregationJobName)
	}
	if scheduler.interval != 5*time.Minute {
		t.Fatalf("scheduled interval = %v, want 5m", scheduler.interval)
	}

	before := time.Now().UTC()
	scheduler.trigger()
	after := time.Now().UTC()

	daily, weekly := repo.snapshot()
	if len(daily) != 1 {
		t.Fatalf("daily calls = %d, want 1", len(daily))
	}
	if len(weekly) != 1 {
		t.Fatalf("weekly calls = %d, want 1", len(weekly))
	}
	if daily[0].start.Before(before.Add(-2*time.Hour-2*time.Second)) || daily[0].start.After(after.Add(-2*time.Hour+2*time.Second)) {
		t.Fatalf("daily start = %s, want about now-2h", daily[0].start)
	}
	if daily[0].end.Before(before.Add(-2*time.Second)) || daily[0].end.After(after.Add(2*time.Second)) {
		t.Fatalf("daily end = %s, want about now", daily[0].end)
	}
	wantWeekStart := currentWeekStartUTC(after)
	if !weekly[0].Equal(wantWeekStart) {
		t.Fatalf("weekly start = %s, want %s", weekly[0], wantWeekStart)
	}
}

func TestBusinessAnalyticsAggregationService_ScheduledRunsDoNotOverlap(t *testing.T) {
	repo := &businessAnalyticsAggregationRepoSpy{
		blockDaily: make(chan struct{}),
		started:    make(chan struct{}, 1),
	}
	svc := NewBusinessAnalyticsAggregationService(repo, nil, &config.Config{
		BusinessAnalytics: config.BusinessAnalyticsConfig{
			Enabled:                    true,
			AggregationIntervalSeconds: 300,
			LookbackSeconds:            7200,
		},
	})

	go svc.runScheduledAggregation()
	select {
	case <-repo.started:
	case <-time.After(time.Second):
		t.Fatal("first scheduled run did not start")
	}

	svc.runScheduledAggregation()
	close(repo.blockDaily)

	deadline := time.After(time.Second)
	for {
		daily, _ := repo.snapshot()
		if len(daily) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("daily calls = %d, want 1", len(daily))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if atomic.LoadInt32(&repo.maxRunning) != 1 {
		t.Fatalf("repo calls overlapped, maxRunning=%d", atomic.LoadInt32(&repo.maxRunning))
	}
}
