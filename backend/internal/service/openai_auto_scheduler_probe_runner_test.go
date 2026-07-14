package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

type fakeOpenAIAutoSchedulerProbeHealthRepo struct {
	mu      sync.Mutex
	states  map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot
	getKeys [][]OpenAISchedulerHealthKey
	upserts []OpenAISchedulerHealthSnapshot
	err     error
}

func (r *fakeOpenAIAutoSchedulerProbeHealthRepo) GetBatch(_ context.Context, keys []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getKeys = append(r.getKeys, append([]OpenAISchedulerHealthKey(nil), keys...))
	if r.err != nil {
		return nil, r.err
	}
	states := make(map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, len(keys))
	for _, key := range keys {
		if state, ok := r.states[key]; ok {
			states[key] = state
		}
	}
	return states, nil
}

func (r *fakeOpenAIAutoSchedulerProbeHealthRepo) Upsert(_ context.Context, snapshot OpenAISchedulerHealthSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upserts = append(r.upserts, snapshot)
	if r.states == nil {
		r.states = map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}
	}
	r.states[snapshot.Key] = snapshot
	return r.err
}

type fakeOpenAIAutoSchedulerProbeLeaderLock struct {
	mu       sync.Mutex
	acquire  bool
	err      error
	keys     []string
	owners   []string
	ttls     []time.Duration
	releases []string
}

func (l *fakeOpenAIAutoSchedulerProbeLeaderLock) TryAcquireLeaderLock(_ context.Context, key, owner string, ttl time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.keys = append(l.keys, key)
	l.owners = append(l.owners, owner)
	l.ttls = append(l.ttls, ttl)
	return l.acquire, l.err
}

func (l *fakeOpenAIAutoSchedulerProbeLeaderLock) ReleaseLeaderLock(_ context.Context, key, owner string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.releases = append(l.releases, key+":"+owner)
	return nil
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
	mu         sync.Mutex
	results    map[string]OpenAIAutoSchedulerProbeResult
	calls      []string
	block      chan struct{}
	started    chan struct{}
	canceled   chan struct{}
	startOnce  sync.Once
	cancelOnce sync.Once
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
		c.startOnce.Do(func() {
			close(started)
		})
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			if canceled != nil {
				c.cancelOnce.Do(func() {
					close(canceled)
				})
			}
			return OpenAIAutoSchedulerProbeResult{Err: ctx.Err()}
		}
	}
	return result
}

type recordingOpenAIAutoSchedulerProbeUpstream struct {
	mu      sync.Mutex
	req     *http.Request
	body    string
	profile *tlsfingerprint.Profile
	resp    *http.Response
	err     error
}

func (u *recordingOpenAIAutoSchedulerProbeUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected Do call")
}

func (u *recordingOpenAIAutoSchedulerProbeUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	bodyBytes, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	u.mu.Lock()
	u.req = req
	u.body = string(bodyBytes)
	u.profile = profile
	resp := u.resp
	err := u.err
	u.mu.Unlock()
	if resp == nil {
		resp = &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"ok"}`)),
		}
	}
	return resp, err
}

func TestOpenAIAutoSchedulerProbeRunner_SkipsWhenDisabled(t *testing.T) {
	svc := &fakeOpenAIAutoSchedulerProbeService{settings: DefaultOpenAIAutoSchedulerSettings()}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, &fakeOpenAIAutoSchedulerProbeAccountRepo{}, &fakeOpenAIAutoSchedulerProbeChecker{}, nil, nil, nil, nil)

	runner.runOnce(context.Background())

	require.Zero(t, svc.listGroupsCalls)
}

func TestOpenAIAutoSchedulerProbeRunner_ProbesEnabledOpenAIGroups(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ProbeModel = "gpt-5.5"
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
			openAIAutoSchedulerProbeKey(1, 10, "gpt-5.5"): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)

	runner.runOnce(context.Background())

	require.Equal(t, 1, svc.listGroupsCalls)
	require.Equal(t, []int64{10}, repo.calls)
	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
	svc.mu.Lock()
	records := append([]OpenAIAutoSchedulerRecordInput(nil), svc.records...)
	svc.mu.Unlock()
	require.Len(t, records, 1)
	record := records[0]
	require.Equal(t, OpenAIAutoSchedulerEventProbeSuccess, record.EventType)
	require.Equal(t, int64(1), record.AccountID)
	require.Equal(t, int64(10), record.GroupID)
	require.Equal(t, "gpt-5.5", record.Model)
}

func TestOpenAIAutoSchedulerProbeRunner_DefaultProbeModel(t *testing.T) {
	require.Equal(t, "gpt-5.4", selectOpenAIAutoSchedulerProbeModel(OpenAIAutoSchedulerSettings{}))
}

func TestOpenAIAutoSchedulerProbeRunner_DeduplicateAcrossGroupsAndFansOutLegacyEvents(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
			{ID: 20, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: true}}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{accounts: map[int64][]Account{10: {account}, 20: {account}}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{}
	healthRepo := &fakeOpenAIAutoSchedulerProbeHealthRepo{}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, svc)
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: true}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, healthSink, lock, nil, nil)

	runner.runOnce(context.Background())

	checker.mu.Lock()
	require.Len(t, checker.calls, 1)
	checker.mu.Unlock()
	svc.mu.Lock()
	records := append([]OpenAIAutoSchedulerRecordInput(nil), svc.records...)
	svc.mu.Unlock()
	require.Len(t, records, 2)
	require.ElementsMatch(t, []int64{10, 20}, []int64{records[0].GroupID, records[1].GroupID})
	healthRepo.mu.Lock()
	require.Len(t, healthRepo.upserts, 1)
	require.Equal(t, OpenAISchedulerHealthKey{
		AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses,
		Transport: string(OpenAIUpstreamTransportHTTPSSE),
	}, healthRepo.upserts[0].Key)
	healthRepo.mu.Unlock()
}

func TestOpenAIAutoSchedulerProbeRunner_DeduplicateDoesNotDropKeysBeyondWorkerLimit(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups:   []Group{{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true}},
	}
	accounts := make([]Account, 0, openAIAutoSchedulerProbeWorkerLimit+1)
	for id := int64(1); id <= int64(openAIAutoSchedulerProbeWorkerLimit+1); id++ {
		accounts = append(accounts, Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true})
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{accounts: map[int64][]Account{10: accounts}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{block: make(chan struct{})}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)
	done := make(chan struct{})
	go func() {
		runner.runOnce(context.Background())
		close(done)
	}()
	require.Eventually(t, func() bool {
		checker.mu.Lock()
		defer checker.mu.Unlock()
		return len(checker.calls) == openAIAutoSchedulerProbeWorkerLimit
	}, time.Second, 10*time.Millisecond)
	close(checker.block)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe cycle did not finish")
	}

	checker.mu.Lock()
	require.Len(t, checker.calls, len(accounts))
	checker.mu.Unlock()
}

func TestOpenAIAutoSchedulerProbeRunner_DeduplicateUsesEveryPhysicalKeyDimension(t *testing.T) {
	plans := map[OpenAISchedulerHealthKey]*openAIAutoSchedulerProbePlanItem{}
	base := OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}
	keys := []OpenAISchedulerHealthKey{
		base,
		{AccountID: 2, ModelFamily: base.ModelFamily, Endpoint: base.Endpoint, Transport: base.Transport},
		{AccountID: base.AccountID, ModelFamily: "gpt-5.5", Endpoint: base.Endpoint, Transport: base.Transport},
		{AccountID: base.AccountID, ModelFamily: base.ModelFamily, Endpoint: openAISchedulerHealthEndpointChat, Transport: base.Transport},
		{AccountID: base.AccountID, ModelFamily: base.ModelFamily, Endpoint: base.Endpoint, Transport: string(OpenAIUpstreamTransportResponsesWebsocket)},
	}
	for i, key := range keys {
		mergeOpenAIAutoSchedulerProbePlanItem(plans, openAIAutoSchedulerProbePlanItem{healthKey: key, groupIDs: []int64{int64(i + 1)}})
	}
	mergeOpenAIAutoSchedulerProbePlanItem(plans, openAIAutoSchedulerProbePlanItem{healthKey: base, groupIDs: []int64{99}})

	require.Len(t, plans, len(keys))
	require.ElementsMatch(t, []int64{1, 99}, plans[base].groupIDs)
}

func TestOpenAIAutoSchedulerProbeRunner_DeduplicatePhysicalKeyUsesResolvedActualDimensions(t *testing.T) {
	account := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-*": " Actual-Upstream-Model ",
		}},
	}

	require.Equal(t, OpenAISchedulerHealthKey{
		AccountID: 1, ModelFamily: "actual-upstream-model", Endpoint: openAISchedulerHealthEndpointChat,
		Transport: string(OpenAIUpstreamTransportHTTPSSE),
	}, openAIAutoSchedulerProbeHealthKey(account, "gpt-5.4"))
}

func TestOpenAIAutoSchedulerProbeRunner_LeaderSkipsCycleAndReleasesOwnedLease(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{settings: settings, groups: []Group{{ID: 10}}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{}
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{acquire: false}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, &fakeOpenAIAutoSchedulerProbeAccountRepo{}, checker, nil, lock, nil, nil)

	runner.runOnce(context.Background())

	checker.mu.Lock()
	require.Empty(t, checker.calls)
	checker.mu.Unlock()
	require.Zero(t, svc.listGroupsCalls)
	require.Equal(t, []string{openAIAutoSchedulerProbeLeaderLockKey}, lock.keys)
	require.Empty(t, lock.releases)

	lock.acquire = true
	runner.runOnce(context.Background())
	require.Len(t, lock.owners, 2)
	require.NotEmpty(t, lock.owners[1])
	require.Equal(t, lock.owners[0], lock.owners[1], "owner must remain process-unique for the runner lifetime")
	require.Greater(t, lock.ttls[1], openAIAutoSchedulerProbeMaxCycleRuntime)
	require.Equal(t, []string{openAIAutoSchedulerProbeLeaderLockKey + ":" + lock.owners[1]}, lock.releases)
}

func TestOpenAIAutoSchedulerProbeRunner_LeaderCacheErrorFallsBackToDBAdvisoryLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	lockID := hashAdvisoryLockID(openAIAutoSchedulerProbeLeaderLockKey)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(lockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(lockID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{settings: settings}
	lock := &fakeOpenAIAutoSchedulerProbeLeaderLock{err: errors.New("redis unavailable")}
	runner := newOpenAIAutoSchedulerProbeRunner(
		svc, svc, &fakeOpenAIAutoSchedulerProbeAccountRepo{}, &fakeOpenAIAutoSchedulerProbeChecker{}, nil, lock, db, nil,
	)

	runner.runOnce(context.Background())

	require.Equal(t, 1, svc.listGroupsCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIAutoSchedulerProbeRunner_FreshRealSampleSkipsAndHealthErrorFallsBackToDeduplicatedProbe(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.RealSampleFreshSeconds = 300
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups: []Group{
			{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
			{ID: 20, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true},
		},
	}
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: true}}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{accounts: map[int64][]Account{10: {account}, 20: {account}}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{}
	now := time.Unix(10_000, 0)
	key := OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses, Transport: string(OpenAIUpstreamTransportHTTPSSE)}
	lastRealAt := now.Add(-time.Minute)
	healthRepo := &fakeOpenAIAutoSchedulerProbeHealthRepo{states: map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{
		key: {Key: key, LastRealAt: &lastRealAt},
	}}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, svc)
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, healthSink, nil, nil, nil)
	runner.now = func() time.Time { return now }
	healthSink.now = runner.now

	runner.runOnce(context.Background())

	checker.mu.Lock()
	require.Empty(t, checker.calls)
	checker.mu.Unlock()
	healthRepo.mu.Lock()
	require.Equal(t, [][]OpenAISchedulerHealthKey{{key}}, healthRepo.getKeys)
	healthRepo.err = errors.New("health repository unavailable")
	healthRepo.states = nil
	healthRepo.mu.Unlock()
	runner.runOnce(context.Background())
	checker.mu.Lock()
	require.Len(t, checker.calls, 1, "health errors retain the old probe behavior after physical-key deduplication")
	checker.mu.Unlock()
	svc.mu.Lock()
	require.Len(t, svc.records, 2, "the deduplicated result must still fan out to both legacy groups")
	svc.mu.Unlock()
}

func TestOpenAIAutoSchedulerProbeRunner_FreshRealSampleArrivingDuringProbePreventsProbeWrite(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups:   []Group{{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true}},
	}
	account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: true}}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{accounts: map[int64][]Account{10: {account}}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{block: make(chan struct{}), started: make(chan struct{})}
	healthRepo := &fakeOpenAIAutoSchedulerProbeHealthRepo{}
	healthSink := NewOpenAISchedulerHealthEventSink(healthRepo, svc)
	now := time.Unix(10_000, 0)
	healthSink.now = func() time.Time { return now }
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, healthSink, nil, nil, nil)
	runner.now = healthSink.now
	done := make(chan struct{})
	go func() {
		runner.runOnce(context.Background())
		close(done)
	}()
	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	ttfbMS := 400
	require.NoError(t, healthSink.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 1, ModelFamily: "gpt-5.4", Endpoint: openAISchedulerHealthEndpointResponses,
		Transport: OpenAIUpstreamTransportHTTPSSE, EventType: OpenAIAutoSchedulerEventSuccess, TtfbMS: &ttfbMS,
	}))
	close(checker.block)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe cycle did not finish")
	}

	healthRepo.mu.Lock()
	require.Len(t, healthRepo.upserts, 1, "the completed probe must recheck freshness under the shared key lock")
	require.Equal(t, int64(1), healthRepo.upserts[0].RealSampleCount)
	require.Zero(t, healthRepo.upserts[0].ProbeSampleCount)
	healthRepo.mu.Unlock()
}

func TestOpenAIAutoSchedulerProbeRunner_JitterDelayIsDeterministicAndBounded(t *testing.T) {
	interval := 60 * time.Second
	jitter := 10 * time.Second
	require.Equal(t, 50*time.Second, nextOpenAIProbeDelay(interval, jitter, func(int64) int64 { return 0 }))
	require.Equal(t, 60*time.Second, nextOpenAIProbeDelay(interval, jitter, func(int64) int64 { return int64(jitter) }))
	require.Equal(t, 70*time.Second, nextOpenAIProbeDelay(interval, jitter, func(n int64) int64 { return n - 1 }))
	require.Equal(t, interval, nextOpenAIProbeDelay(interval, 0, nil))
}

func TestOpenAIAutoSchedulerProbeRunner_JitterTimerWaitsBeforeFirstCycleAndStopCancelsIt(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ProbeIntervalSeconds = 60
	svc := &fakeOpenAIAutoSchedulerProbeService{
		settings: settings,
		groups:   []Group{{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, OpenAIAutoSchedulerEnabled: true, Hydrated: true}},
	}
	repo := &fakeOpenAIAutoSchedulerProbeAccountRepo{accounts: map[int64][]Account{
		10: {{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}},
	}}
	checker := &fakeOpenAIAutoSchedulerProbeChecker{started: make(chan struct{})}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)
	runner.Start()

	select {
	case <-checker.started:
		t.Fatal("probe ran immediately instead of waiting for the jittered first interval")
	case <-time.After(50 * time.Millisecond):
	}
	stopped := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the pending probe timer")
	}
}

func TestOpenAIAutoSchedulerProbeChecker_UsesChatCompletionsWhenAPIKeyResponsesUnsupported(t *testing.T) {
	upstream := &recordingOpenAIAutoSchedulerProbeUpstream{}
	checker := NewOpenAIAutoSchedulerProbeChecker(upstream, nil)
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: false,
		},
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example",
			"model_mapping": map[string]any{
				"gpt-5.4": "compat-model",
			},
		},
	}

	result := checker.Check(context.Background(), account, "gpt-5.4", time.Second)

	require.True(t, result.Success)
	require.NoError(t, result.Err)
	require.NotNil(t, upstream.req)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.req.URL.String())
	require.JSONEq(t, `{"model":"compat-model","messages":[{"role":"user","content":"probe"}],"stream":true}`, upstream.body)
}

func TestOpenAIAutoSchedulerProbeChecker_UsesConnectionTestResponsesPayload(t *testing.T) {
	upstream := &recordingOpenAIAutoSchedulerProbeUpstream{}
	checker := NewOpenAIAutoSchedulerProbeChecker(upstream, nil)
	account := &Account{
		ID:       2,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: true,
		},
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://responses-upstream.example",
		},
	}

	result := checker.Check(context.Background(), account, "gpt-5.4", time.Second)

	require.True(t, result.Success)
	require.NoError(t, result.Err)
	require.NotNil(t, upstream.req)
	require.Equal(t, "https://responses-upstream.example/v1/responses", upstream.req.URL.String())
	require.Equal(t, "gpt-5.4", gjson.Get(upstream.body, "model").String())
	require.True(t, gjson.Get(upstream.body, "stream").Bool())
	require.Equal(t, "hi", gjson.Get(upstream.body, "input.0.content.0.text").String())
	require.True(t, gjson.Get(upstream.body, "instructions").Exists())
	require.False(t, gjson.Get(upstream.body, "max_output_tokens").Exists())
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
		block:   make(chan struct{}),
		started: make(chan struct{}),
		results: map[string]OpenAIAutoSchedulerProbeResult{
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel(DefaultOpenAIAutoSchedulerSettings())): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)

	firstDone := make(chan struct{})
	go func() {
		runner.runOnce(context.Background())
		close(firstDone)
	}()
	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("first probe did not start")
	}
	runner.runOnce(context.Background())
	close(checker.block)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first probe cycle did not finish")
	}

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
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel(DefaultOpenAIAutoSchedulerSettings())): {
				Err:       errors.New("upstream refused probe"),
				LatencyMS: openAIAutoSchedulerProbeTestPtr(123),
			},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)

	runner.runOnce(context.Background())

	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
	svc.mu.Lock()
	records := append([]OpenAIAutoSchedulerRecordInput(nil), svc.records...)
	svc.mu.Unlock()
	record := probeRecordByModel(records, "gpt-5.4")
	require.NotNil(t, record)
	require.Equal(t, OpenAIAutoSchedulerEventProbeError, record.EventType)
	require.Equal(t, "upstream refused probe", record.Message)
	require.Equal(t, openAIAutoSchedulerProbeTestPtr(123), record.LatencyMS)
}

func TestOpenAIAutoSchedulerProbeRunner_RecordsProbeTTFB(t *testing.T) {
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
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel(DefaultOpenAIAutoSchedulerSettings())): {
				Success:   true,
				LatencyMS: openAIAutoSchedulerProbeTestPtr(300),
				TtfbMS:    openAIAutoSchedulerProbeTestPtr(120),
			},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)

	runner.runOnce(context.Background())

	require.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.records) == 1
	}, time.Second, 10*time.Millisecond)
	svc.mu.Lock()
	records := append([]OpenAIAutoSchedulerRecordInput(nil), svc.records...)
	svc.mu.Unlock()
	record := probeRecordByModel(records, "gpt-5.4")
	require.NotNil(t, record)
	require.Equal(t, OpenAIAutoSchedulerEventProbeSuccess, record.EventType)
	require.Equal(t, openAIAutoSchedulerProbeTestPtr(300), record.LatencyMS)
	require.Equal(t, openAIAutoSchedulerProbeTestPtr(120), record.TtfbMS)
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
			openAIAutoSchedulerProbeKey(1, 10, selectOpenAIAutoSchedulerProbeModel(DefaultOpenAIAutoSchedulerSettings())): {Success: true},
		},
	}
	runner := newOpenAIAutoSchedulerProbeRunner(svc, svc, repo, checker, nil, nil, nil, nil)
	go runner.runOnce(runner.parentCtx)
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

func probeRecordModels(records []OpenAIAutoSchedulerRecordInput) []string {
	models := make([]string, 0, len(records))
	for _, record := range records {
		models = append(models, record.Model)
	}
	return models
}

func probeRecordByModel(records []OpenAIAutoSchedulerRecordInput, model string) *OpenAIAutoSchedulerRecordInput {
	for i := range records {
		if records[i].Model == model {
			return &records[i]
		}
	}
	return nil
}

func openAIAutoSchedulerProbeTestPtr[T any](value T) *T {
	return &value
}
