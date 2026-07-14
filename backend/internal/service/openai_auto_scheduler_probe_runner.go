package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/alitto/pond/v2"
	"github.com/google/uuid"
)

const (
	openAIAutoSchedulerProbeWorkerLimit  = 5
	openAIAutoSchedulerProbeTimeout      = 15 * time.Second
	openAIAutoSchedulerProbeMaxBodyBytes = 256 * 1024

	openAIAutoSchedulerProbeLeaderLockKey   = "openai-auto-scheduler-probe"
	openAIAutoSchedulerProbeMaxCycleRuntime = 45 * time.Second
	openAIAutoSchedulerProbeLeaderLockTTL   = 2 * time.Minute
)

type openAIAutoSchedulerProbeRunnerService interface {
	ListEnabledOpenAIGroups(ctx context.Context) ([]Group, error)
	Record(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error
}

type openAIAutoSchedulerProbeAccountRepository interface {
	ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error)
}

type OpenAIAutoSchedulerProbeChecker interface {
	Check(ctx context.Context, account *Account, model string, timeout time.Duration) OpenAIAutoSchedulerProbeResult
}

type OpenAIAutoSchedulerProbeRunner struct {
	svc                 openAIAutoSchedulerProbeRunnerService
	settingsProvider    OpenAIAutoSchedulerSettingsProvider
	accountRepo         openAIAutoSchedulerProbeAccountRepository
	checker             OpenAIAutoSchedulerProbeChecker
	healthSink          *OpenAISchedulerHealthEventSink
	lockCache           LeaderLockCache
	db                  *sql.DB
	owner               string
	tlsFPProfileService *TLSFingerprintProfileService
	now                 func() time.Time
	randInt64           func(int64) int64

	pool      pond.Pool
	parentCtx context.Context
	cancel    context.CancelFunc
	loopWG    sync.WaitGroup

	startOnce sync.Once
	stopOnce  sync.Once
	mu        sync.Mutex
	stopped   bool
	inFlight  map[string]struct{}
}

type openAIAutoSchedulerProbePlanItem struct {
	account   Account
	healthKey OpenAISchedulerHealthKey
	groupIDs  []int64
	model     string
}

func newOpenAIAutoSchedulerProbeRunner(
	svc openAIAutoSchedulerProbeRunnerService,
	settingsProvider OpenAIAutoSchedulerSettingsProvider,
	accountRepo openAIAutoSchedulerProbeAccountRepository,
	checker OpenAIAutoSchedulerProbeChecker,
	healthSink *OpenAISchedulerHealthEventSink,
	lockCache LeaderLockCache,
	db *sql.DB,
	tlsFPProfileService *TLSFingerprintProfileService,
) *OpenAIAutoSchedulerProbeRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpenAIAutoSchedulerProbeRunner{
		svc:                 svc,
		settingsProvider:    settingsProvider,
		accountRepo:         accountRepo,
		checker:             checker,
		healthSink:          healthSink,
		lockCache:           lockCache,
		db:                  db,
		owner:               uuid.NewString(),
		tlsFPProfileService: tlsFPProfileService,
		now:                 time.Now,
		randInt64:           rand.Int64N,
		pool:                pond.NewPool(openAIAutoSchedulerProbeWorkerLimit),
		parentCtx:           ctx,
		cancel:              cancel,
		inFlight:            map[string]struct{}{},
	}
}

func NewOpenAIAutoSchedulerProbeRunner(
	svc *OpenAIAutoSchedulerService,
	settingsProvider OpenAIAutoSchedulerSettingsProvider,
	accountRepo AccountRepository,
	checker OpenAIAutoSchedulerProbeChecker,
	healthSink *OpenAISchedulerHealthEventSink,
	lockCache LeaderLockCache,
	db *sql.DB,
	tlsFPProfileService *TLSFingerprintProfileService,
) *OpenAIAutoSchedulerProbeRunner {
	return newOpenAIAutoSchedulerProbeRunner(svc, settingsProvider, accountRepo, checker, healthSink, lockCache, db, tlsFPProfileService)
}

func (r *OpenAIAutoSchedulerProbeRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.startOnce.Do(func() {
		r.loopWG.Add(1)
		go func() {
			defer r.loopWG.Done()
			r.loop()
		}()
	})
}

func (r *OpenAIAutoSchedulerProbeRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		r.mu.Lock()
		r.stopped = true
		r.mu.Unlock()
		if r.cancel != nil {
			r.cancel()
		}
		r.loopWG.Wait()
		if r.pool != nil {
			r.pool.StopAndWait()
		}
	})
}

func (r *OpenAIAutoSchedulerProbeRunner) loop() {
	ctx := r.parentCtx
	timer := time.NewTimer(r.nextDelay(ctx))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.runOnce(ctx)
			if ctx.Err() != nil {
				return
			}
			timer.Reset(r.nextDelay(ctx))
		}
	}
}

func (r *OpenAIAutoSchedulerProbeRunner) nextDelay(ctx context.Context) time.Duration {
	interval, jitter := openAIAutoSchedulerProbeTiming(r.settingsProvider, ctx)
	return nextOpenAIProbeDelay(interval, jitter, r.randInt64)
}

func openAIAutoSchedulerProbeTiming(provider OpenAIAutoSchedulerSettingsProvider, ctx context.Context) (time.Duration, time.Duration) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	if provider != nil {
		settings = normalizeOpenAIAutoSchedulerSettings(provider.GetOpenAIAutoSchedulerSettings(ctx))
	}
	return time.Duration(settings.ProbeIntervalSeconds) * time.Second,
		time.Duration(settings.ProbeJitterSeconds) * time.Second
}

func nextOpenAIProbeDelay(interval, jitter time.Duration, randInt64 func(int64) int64) time.Duration {
	if interval <= 0 {
		return 0
	}
	if jitter <= 0 || randInt64 == nil {
		return interval
	}
	if jitter > interval {
		jitter = interval
	}
	span := int64(2*jitter) + 1
	draw := randInt64(span) % span
	if draw < 0 {
		draw += span
	}
	return interval + time.Duration(draw-int64(jitter))
}

func (r *OpenAIAutoSchedulerProbeRunner) runOnce(ctx context.Context) {
	if r == nil || r.svc == nil || r.accountRepo == nil || r.checker == nil {
		return
	}
	if r.settingsProvider == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	settings := normalizeOpenAIAutoSchedulerSettings(r.settingsProvider.GetOpenAIAutoSchedulerSettings(ctx))
	if !settings.Enabled {
		return
	}
	cycleCtx, cancel := context.WithTimeout(ctx, openAIAutoSchedulerProbeMaxCycleRuntime)
	defer cancel()
	releaseLeader, acquired := tryAcquireSingletonLeaderLock(
		cycleCtx,
		r.lockCache,
		r.db,
		openAIAutoSchedulerProbeLeaderLockKey,
		r.owner,
		openAIAutoSchedulerProbeLeaderLockTTL,
	)
	if !acquired {
		return
	}
	defer releaseLeader()

	groups, err := r.svc.ListEnabledOpenAIGroups(cycleCtx)
	if err != nil {
		slog.Warn("openai_auto_scheduler_probe: list groups failed", "error", err)
		return
	}

	model := selectOpenAIAutoSchedulerProbeModel(settings)
	plans := make(map[OpenAISchedulerHealthKey]*openAIAutoSchedulerProbePlanItem)
	for i := range groups {
		if cycleCtx.Err() != nil {
			return
		}
		group := groups[i]
		accounts, err := r.accountRepo.ListSchedulableByGroupIDAndPlatform(cycleCtx, group.ID, PlatformOpenAI)
		if err != nil {
			slog.Warn("openai_auto_scheduler_probe: list accounts failed", "group_id", group.ID, "error", err)
			continue
		}
		for i := range accounts {
			if cycleCtx.Err() != nil {
				return
			}
			account := accounts[i]
			healthKey := openAIAutoSchedulerProbeHealthKey(&account, model)
			if !isCompleteOpenAISchedulerHealthKey(healthKey) {
				continue
			}
			mergeOpenAIAutoSchedulerProbePlanItem(plans, openAIAutoSchedulerProbePlanItem{
				account: account, healthKey: healthKey, groupIDs: []int64{group.ID}, model: model,
			})
		}
	}
	if len(plans) == 0 {
		return
	}

	keys := make([]OpenAISchedulerHealthKey, 0, len(plans))
	for key := range plans {
		keys = append(keys, key)
	}
	healthSnapshots, err := r.loadProbeHealthSnapshots(cycleCtx, keys)
	if err != nil {
		slog.Warn("openai_auto_scheduler_probe: load health freshness failed", "error", err)
		healthSnapshots = map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}
	}

	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	var cycleWG sync.WaitGroup
	for key, plan := range plans {
		if cycleCtx.Err() != nil {
			break
		}
		if hasFreshOpenAIAutoSchedulerRealSample(healthSnapshots[key], now, settings.RealSampleFreshSeconds) {
			continue
		}
		inFlightKey := openAIAutoSchedulerPhysicalProbeKey(key)
		if !r.tryAcquireInFlight(inFlightKey) {
			continue
		}
		if r.pool == nil || r.pool.Stopped() {
			r.releaseInFlight(inFlightKey)
			continue
		}
		planCopy := *plan
		planCopy.groupIDs = append([]int64(nil), plan.groupIDs...)
		cycleWG.Add(1)
		if _, ok := r.pool.TrySubmit(func() {
			defer cycleWG.Done()
			defer r.releaseInFlight(inFlightKey)
			r.runProbe(cycleCtx, planCopy, openAIAutoSchedulerProbeTimeout, settings)
		}); !ok {
			cycleWG.Done()
			r.releaseInFlight(inFlightKey)
		}
	}
	cycleWG.Wait()
}

func (r *OpenAIAutoSchedulerProbeRunner) runProbe(ctx context.Context, plan openAIAutoSchedulerProbePlanItem, timeout time.Duration, settings OpenAIAutoSchedulerSettings) {
	attemptTime := time.Now()
	if r.now != nil {
		attemptTime = r.now()
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := r.checker.Check(probeCtx, &plan.account, plan.model, timeout)
	eventType := classifyOpenAIAutoSchedulerProbeEvent(result, settings)
	message := strings.TrimSpace(result.Message)
	if result.Err != nil && message == "" {
		message = result.Err.Error()
	}
	if err := r.recordProbeHealth(ctx, plan.healthKey, eventType, result, settings); err != nil {
		slog.Warn("openai_auto_scheduler_probe: record unified health failed", "account_id", plan.account.ID, "error", err)
	}
	for _, groupID := range plan.groupIDs {
		input := OpenAIAutoSchedulerRecordInput{
			AccountID:   plan.account.ID,
			GroupID:     groupID,
			Model:       plan.model,
			ModelFamily: plan.healthKey.ModelFamily,
			Endpoint:    plan.healthKey.Endpoint,
			Transport:   OpenAIUpstreamTransport(plan.healthKey.Transport),
			EventType:   eventType,
			OccurredAt:  attemptTime,
			LatencyMS:   result.LatencyMS,
			TtfbMS:      result.TtfbMS,
			Message:     message,
		}
		if err := r.svc.Record(ctx, input); err != nil {
			slog.Warn("openai_auto_scheduler_probe: record failed", "account_id", plan.account.ID, "group_id", groupID, "error", err)
		}
	}
}

func mergeOpenAIAutoSchedulerProbePlanItem(plans map[OpenAISchedulerHealthKey]*openAIAutoSchedulerProbePlanItem, item openAIAutoSchedulerProbePlanItem) {
	if plans == nil {
		return
	}
	item.healthKey = normalizeOpenAISchedulerHealthKey(item.healthKey)
	if !isCompleteOpenAISchedulerHealthKey(item.healthKey) {
		return
	}
	if existing := plans[item.healthKey]; existing != nil {
		for _, groupID := range item.groupIDs {
			if !containsOpenAIAutoSchedulerProbeGroup(existing.groupIDs, groupID) {
				existing.groupIDs = append(existing.groupIDs, groupID)
			}
		}
		return
	}
	item.groupIDs = append([]int64(nil), item.groupIDs...)
	plans[item.healthKey] = &item
}

func containsOpenAIAutoSchedulerProbeGroup(groupIDs []int64, groupID int64) bool {
	for _, candidate := range groupIDs {
		if candidate == groupID {
			return true
		}
	}
	return false
}

func openAIAutoSchedulerProbeHealthKey(account *Account, model string) OpenAISchedulerHealthKey {
	if account == nil {
		return OpenAISchedulerHealthKey{}
	}
	endpoint := openAISchedulerHealthEndpointResponses
	if account.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(account.Extra) {
		endpoint = openAISchedulerHealthEndpointChat
	}
	return normalizeOpenAISchedulerHealthKey(OpenAISchedulerHealthKey{
		AccountID:   account.ID,
		ModelFamily: normalizeOpenAIModelForUpstream(account, account.GetMappedModel(model)),
		Endpoint:    endpoint,
		Transport:   string(OpenAIUpstreamTransportHTTPSSE),
	})
}

func openAIAutoSchedulerPhysicalProbeKey(key OpenAISchedulerHealthKey) string {
	key = normalizeOpenAISchedulerHealthKey(key)
	return fmt.Sprintf("%d:%s:%s:%s", key.AccountID, key.ModelFamily, key.Endpoint, key.Transport)
}

func hasFreshOpenAIAutoSchedulerRealSample(snapshot OpenAISchedulerHealthSnapshot, now time.Time, freshSeconds int) bool {
	return snapshot.LastRealAt != nil && freshSeconds > 0 &&
		snapshot.LastRealAt.After(now.Add(-time.Duration(freshSeconds)*time.Second))
}

func (r *OpenAIAutoSchedulerProbeRunner) loadProbeHealthSnapshots(ctx context.Context, keys []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	if r == nil || r.healthSink == nil || r.healthSink.repo == nil {
		return map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}, nil
	}
	return r.healthSink.repo.GetBatch(ctx, keys)
}

func (r *OpenAIAutoSchedulerProbeRunner) recordProbeHealth(
	ctx context.Context,
	key OpenAISchedulerHealthKey,
	eventType string,
	result OpenAIAutoSchedulerProbeResult,
	settings OpenAIAutoSchedulerSettings,
) error {
	if r == nil || r.healthSink == nil || r.healthSink.repo == nil {
		return nil
	}
	key = normalizeOpenAISchedulerHealthKey(key)
	if !isCompleteOpenAISchedulerHealthKey(key) {
		return nil
	}
	release := r.healthSink.lockKey(key)
	defer release()
	states, err := r.healthSink.repo.GetBatch(ctx, []OpenAISchedulerHealthKey{key})
	if err != nil {
		return err
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	current := states[key]
	if hasFreshOpenAIAutoSchedulerRealSample(current, now, settings.RealSampleFreshSeconds) {
		return nil
	}
	current.Key = key
	ttftMS := 0.0
	if result.LatencyMS != nil && *result.LatencyMS > 0 {
		ttftMS = float64(*result.LatencyMS)
	}
	if result.TtfbMS != nil && *result.TtfbMS > 0 {
		ttftMS = float64(*result.TtfbMS)
	}
	next := ApplyOpenAISchedulerHealthEvent(now, current, OpenAISchedulerHealthEvent{
		Source:     HealthSourceProbe,
		EventType:  eventType,
		TTFTMS:     ttftMS,
		OccurredAt: now,
	}, openAISchedulerHealthSettingsFromAutoScheduler(settings))
	return r.healthSink.repo.Upsert(ctx, next)
}

func classifyOpenAIAutoSchedulerProbeEvent(result OpenAIAutoSchedulerProbeResult, settings OpenAIAutoSchedulerSettings) string {
	if !result.Success || result.Err != nil {
		return OpenAIAutoSchedulerEventProbeError
	}
	normalized := normalizeOpenAIAutoSchedulerSettings(settings)
	observedMS := 0
	if result.LatencyMS != nil && *result.LatencyMS > 0 {
		observedMS = *result.LatencyMS
	}
	if result.TtfbMS != nil && *result.TtfbMS > 0 {
		observedMS = *result.TtfbMS
	}
	switch {
	case observedMS >= normalized.SevereSlowThresholdMS:
		return OpenAIAutoSchedulerEventSevereSlow
	case observedMS >= normalized.SlowThresholdMS:
		return OpenAIAutoSchedulerEventSlow
	default:
		return OpenAIAutoSchedulerEventProbeSuccess
	}
}

// ClassifyOpenAIAutoSchedulerProbeEvent exposes the shared probe classifier to admin handlers.
func ClassifyOpenAIAutoSchedulerProbeEvent(result OpenAIAutoSchedulerProbeResult, settings OpenAIAutoSchedulerSettings) string {
	return classifyOpenAIAutoSchedulerProbeEvent(result, settings)
}

func (r *OpenAIAutoSchedulerProbeRunner) tryAcquireInFlight(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return false
	}
	if _, ok := r.inFlight[key]; ok {
		return false
	}
	r.inFlight[key] = struct{}{}
	return true
}

func (r *OpenAIAutoSchedulerProbeRunner) releaseInFlight(key string) {
	r.mu.Lock()
	delete(r.inFlight, key)
	r.mu.Unlock()
}

func openAIAutoSchedulerProbeKey(accountID, groupID int64, model string) string {
	return fmt.Sprintf("%d:%d:%s", accountID, groupID, strings.TrimSpace(model))
}

func selectOpenAIAutoSchedulerProbeModel(settings OpenAIAutoSchedulerSettings) string {
	return normalizeOpenAIAutoSchedulerSettings(settings).ProbeModel
}

type openAIAutoSchedulerProbeHTTPChecker struct {
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
}

func NewOpenAIAutoSchedulerProbeChecker(httpUpstream HTTPUpstream, tlsFPProfileService *TLSFingerprintProfileService) OpenAIAutoSchedulerProbeChecker {
	return &openAIAutoSchedulerProbeHTTPChecker{
		httpUpstream:        httpUpstream,
		tlsFPProfileService: tlsFPProfileService,
	}
}

func (c *openAIAutoSchedulerProbeHTTPChecker) Check(ctx context.Context, account *Account, model string, timeout time.Duration) OpenAIAutoSchedulerProbeResult {
	if c == nil || c.httpUpstream == nil || account == nil {
		return OpenAIAutoSchedulerProbeResult{Err: errors.New("probe checker not configured")}
	}
	if !account.IsOpenAI() {
		return OpenAIAutoSchedulerProbeResult{Err: errors.New("account is not openai")}
	}

	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	upstreamModel := normalizeOpenAIModelForUpstream(account, account.GetMappedModel(model))
	targetURL, payload, accept := openAIAutoSchedulerProbeRequest(baseURL, upstreamModel, account)

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return OpenAIAutoSchedulerProbeResult{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	token := account.GetOpenAIAccessToken()
	if token == "" {
		token = account.GetOpenAIApiKey()
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	startedAt := time.Now()
	resp, err := c.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, c.resolveTLSProfile(account))
	if err != nil {
		return OpenAIAutoSchedulerProbeResult{Err: err}
	}
	ttfbMS := openAIAutoSchedulerDurationMS(time.Since(startedAt))
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, openAIAutoSchedulerProbeMaxBodyBytes))
	latencyMS := openAIAutoSchedulerDurationMS(time.Since(startedAt))
	if readErr != nil {
		return OpenAIAutoSchedulerProbeResult{LatencyMS: &latencyMS, TtfbMS: &ttfbMS, Err: readErr}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OpenAIAutoSchedulerProbeResult{LatencyMS: &latencyMS, TtfbMS: &ttfbMS, Message: fmt.Sprintf("upstream HTTP %d", resp.StatusCode)}
	}
	if strings.TrimSpace(string(body)) == "" {
		return OpenAIAutoSchedulerProbeResult{LatencyMS: &latencyMS, TtfbMS: &ttfbMS, Err: errors.New("empty probe response")}
	}
	return OpenAIAutoSchedulerProbeResult{Success: true, LatencyMS: &latencyMS, TtfbMS: &ttfbMS}
}

func openAIAutoSchedulerProbeRequest(baseURL string, upstreamModel string, account *Account) (string, []byte, string) {
	if account != nil && account.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(account.Extra) {
		payload, _ := json.Marshal(map[string]any{
			"model": upstreamModel,
			"messages": []map[string]any{
				{
					"role":    "user",
					"content": "probe",
				},
			},
			"stream": true,
		})
		return buildOpenAIChatCompletionsURL(baseURL), payload, "text/event-stream"
	}
	isOAuth := account != nil && account.IsOAuth()
	payload, _ := json.Marshal(createOpenAITestPayload(upstreamModel, isOAuth))
	return buildOpenAIResponsesURL(baseURL), payload, "text/event-stream"
}

func openAIAutoSchedulerDurationMS(duration time.Duration) int {
	ms := int(duration / time.Millisecond)
	if ms <= 0 {
		return 1
	}
	return ms
}

func (c *openAIAutoSchedulerProbeHTTPChecker) resolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if c == nil || c.tlsFPProfileService == nil {
		return nil
	}
	return c.tlsFPProfileService.ResolveTLSProfile(account)
}
