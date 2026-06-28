//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeOpenAIAutoSchedulerProbeSettingsProvider struct {
	settings OpenAIAutoSchedulerSettings
}

func (p fakeOpenAIAutoSchedulerProbeSettingsProvider) GetOpenAIAutoSchedulerSettings(context.Context) OpenAIAutoSchedulerSettings {
	return p.settings
}

type fakeOpenAIAutoSchedulerProbeService struct {
	mu              sync.Mutex
	settings        OpenAIAutoSchedulerSettings
	groups          []Group
	listGroupsCalls int
	records         []OpenAIAutoSchedulerRecordInput
}

func (s *fakeOpenAIAutoSchedulerProbeService) GetOpenAIAutoSchedulerSettings(context.Context) OpenAIAutoSchedulerSettings {
	return s.settings
}

func (s *fakeOpenAIAutoSchedulerProbeService) ListEnabledOpenAIGroups(_ context.Context) ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listGroupsCalls++
	return append([]Group(nil), s.groups...), nil
}

func (s *fakeOpenAIAutoSchedulerProbeService) Record(_ context.Context, input OpenAIAutoSchedulerRecordInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, input)
	return nil
}

type fakeOpenAIAutoSchedulerProbeAccountRepo struct {
	mu       sync.Mutex
	accounts map[int64][]Account
	calls    []int64
	err      error
}

func (r *fakeOpenAIAutoSchedulerProbeAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, _ string) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, groupID)
	if r.err != nil {
		return nil, r.err
	}
	return append([]Account(nil), r.accounts[groupID]...), nil
}

type fakeOpenAIAutoSchedulerProbeChecker struct {
	mu       sync.Mutex
	results  map[string]OpenAIAutoSchedulerProbeResult
	calls    []string
	block    chan struct{}
	started  chan struct{}
	canceled chan struct{}
}

func (c *fakeOpenAIAutoSchedulerProbeChecker) Check(ctx context.Context, account *Account, model string, timeout time.Duration) OpenAIAutoSchedulerProbeResult {
	key := openAIAutoSchedulerProbeKey(account.ID, 10, model)
	c.mu.Lock()
	c.calls = append(c.calls, key)
	block := c.block
	started := c.started
	canceled := c.canceled
	result := c.results[key]
	c.mu.Unlock()
	if started != nil {
		close(started)
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			if canceled != nil {
				close(canceled)
			}
			return OpenAIAutoSchedulerProbeResult{Err: ctx.Err()}
		}
	}
	return result
}

func TestOpenAIAutoSchedulerProbeRunner_SkipsWhenDisabled(t *testing.T) {
	svc := &fakeOpenAIAutoSchedulerProbeService{settings: DefaultOpenAIAutoSchedulerSettings()}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, &fakeOpenAIAutoSchedulerProbeAccountRepo{}, &fakeOpenAIAutoSchedulerProbeChecker{}, nil)

	runner.runOnce(context.Background())

	require.Zero(t, svc.listGroupsCalls)
}

func TestOpenAIAutoSchedulerProbeRunner_ProbesEnabledOpenAIGroups(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{
		accounts: map[int64][]Account{
			10: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
		},
	}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{
		results: map[string]OpenAIAutoSchedulerProbeResult{
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel()): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil)

	runner.runOnce(context.Background())

	require.Equal(t, 1, svc.listGroupsCalls)
	require.Equal(t, []int64{10}, repo.calls)
	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
	svc.mu.Lock()
	record := svc.records[0]
	svc.mu.Unlock()
	require.Equal(t, OpenAIAutoSchedulerEventProbeSuccess, record.EventType)
	require.Equal(t, int64(1), record.AccountID)
	require.Equal(t, int64(10), record.GroupID)
}

func TestOpenAIAutoSchedulerProbeRunner_DedupesInFlightChecks(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{
		accounts: map[int64][]Account{
			10: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
		},
	}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{
		block: make(chan struct{}),
		results: map[string]OpenAIAutoSchedulerProbeResult{
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel()): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil)

	runner.runOnce(context.Background())
	runner.runOnce(context.Background())
	close(checker.block)

	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestOpenAIAutoSchedulerProbeRunner_RecordsProbeError(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{
		accounts: map[int64][]Account{
			10: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
		},
	}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{
		results: map[string]OpenAIAutoSchedulerProbeResult{
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel()): {
				Err:       errors.New("upstream refused probe"),
				LatencyMS: ptr(123),
			},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil)

	runner.runOnce(context.Background())

	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
	svc.mu.Lock()
	record := svc.records[0]
	svc.mu.Unlock()
	require.Equal(t, OpenAIAutoSchedulerEventProbeError, record.EventType)
	require.Equal(t, "upstream refused probe", record.Message)
	require.Equal(t, ptr(123), record.LatencyMS)
}

func TestOpenAIAutoSchedulerProbeRunner_StopCancelsInFlightProbe(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ProbeIntervalSeconds = 60
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{
		accounts: map[int64][]Account{
			10: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
		},
	}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{
		block:    make(chan struct{}),
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
		results: map[string]OpenAIAutoSchedulerProbeResult{
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel()): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil)
	runner.Start()
	defer close(checker.block)

	require.Eventually(t, func() bool {
		select {
		case <-checker.started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	stopped := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopped)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-checker.canceled:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-stopped:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}
