package service

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	channelPriceRefreshJobName        = "channel:price_refresh"
	defaultChannelPriceRefreshSeconds = 600
	defaultChannelPriceRefreshTimeout = 30
	defaultChannelPriceRefreshWorkers = 3
	minChannelPriceRefreshWorkers     = 1
	maxChannelPriceRefreshWorkers     = 5
)

type channelPriceRefreshAccountRepository interface {
	ListActive(ctx context.Context) ([]Account, error)
}

type upstreamBalanceRefreshCandidateRepository interface {
	ListUpstreamBalanceRefreshCandidates(ctx context.Context, limit int) ([]Account, error)
}

type channelPriceRefresher interface {
	Refresh(ctx context.Context, accountID int64) (*Account, error)
}

type ChannelPriceRefreshResult struct {
	Attempted int `json:"attempted"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
}

type ChannelPriceRefreshSettings struct {
	Enabled         bool                       `json:"enabled"`
	IntervalSeconds int                        `json:"interval_seconds"`
	Concurrency     int                        `json:"concurrency"`
	TimeoutSeconds  int                        `json:"timeout_seconds"`
	LastRunAt       *time.Time                 `json:"last_run_at,omitempty"`
	LastResult      *ChannelPriceRefreshResult `json:"last_result,omitempty"`
}

type ChannelPriceRefreshJob struct {
	repo        channelPriceRefreshAccountRepository
	refresher   channelPriceRefresher
	timingWheel *TimingWheelService
	settingRepo SettingRepository
	mu          sync.RWMutex
	cfg         config.ChannelPriceRefreshConfig
	lastRunAt   *time.Time
	lastResult  *ChannelPriceRefreshResult
	running     int32
}

func NewChannelPriceRefreshJob(repo channelPriceRefreshAccountRepository, refresher channelPriceRefresher, timingWheel *TimingWheelService, cfg *config.Config) *ChannelPriceRefreshJob {
	jobCfg := config.ChannelPriceRefreshConfig{
		IntervalSeconds: defaultChannelPriceRefreshSeconds,
		Concurrency:     defaultChannelPriceRefreshWorkers,
		TimeoutSeconds:  defaultChannelPriceRefreshTimeout,
	}
	if cfg != nil {
		jobCfg = cfg.ChannelPriceRefresh
	}
	jobCfg.IntervalSeconds = normalizeChannelPriceRefreshInterval(jobCfg.IntervalSeconds)
	jobCfg.Concurrency = normalizeChannelPriceRefreshConcurrency(jobCfg.Concurrency)
	jobCfg.TimeoutSeconds = normalizeChannelPriceRefreshTimeout(jobCfg.TimeoutSeconds)
	return &ChannelPriceRefreshJob{
		repo:        repo,
		refresher:   refresher,
		timingWheel: timingWheel,
		cfg:         jobCfg,
	}
}

func ProvideChannelPriceRefreshJob(repo AccountRepository, refresher *OpenAIUpstreamBalanceService, timingWheel *TimingWheelService, settingRepo SettingRepository, cfg *config.Config) *ChannelPriceRefreshJob {
	job := NewChannelPriceRefreshJob(repo, refresher, timingWheel, cfg)
	job.SetSettingRepository(settingRepo)
	job.loadPersistedSettings(context.Background())
	job.Start()
	return job
}

func (j *ChannelPriceRefreshJob) Start() {
	if j == nil || j.repo == nil || j.refresher == nil || j.timingWheel == nil {
		return
	}
	cfg := j.currentConfig()
	if !cfg.Enabled {
		logger.LegacyPrintf("service.channel_price_refresh", "%s", "[ChannelPriceRefresh] job disabled")
		return
	}
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	j.timingWheel.ScheduleRecurring(channelPriceRefreshJobName, interval, func() {
		j.runScheduled()
	})
	logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] job started (interval=%v concurrency=%d timeout=%ds)", interval, cfg.Concurrency, cfg.TimeoutSeconds)
}

func (j *ChannelPriceRefreshJob) Stop() {
	if j == nil || j.timingWheel == nil {
		return
	}
	j.timingWheel.Cancel(channelPriceRefreshJobName)
}

func (j *ChannelPriceRefreshJob) RunOnce(ctx context.Context) ChannelPriceRefreshResult {
	var result ChannelPriceRefreshResult
	if j == nil || j.repo == nil || j.refresher == nil {
		return result
	}
	cfg := j.currentConfig()
	if !cfg.Enabled {
		return result
	}
	defer func() {
		j.recordLastResult(result)
	}()

	accounts, err := j.listCandidates(ctx)
	if err != nil {
		logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] list candidates failed: %v", err)
		return result
	}

	candidates := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if accountSupportsUpstreamBalance(&account) {
			candidates = append(candidates, account)
		}
	}
	result.Attempted = len(candidates)
	if len(candidates) == 0 {
		return result
	}

	workerCount := cfg.Concurrency
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}

	jobs := make(chan Account)
	var wg sync.WaitGroup
	var success int64
	var failed int64
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for account := range jobs {
				accountCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
				_, err := j.refresher.Refresh(accountCtx, account.ID)
				cancel()
				if err != nil {
					atomic.AddInt64(&failed, 1)
					logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] refresh failed account_id=%d: %v", account.ID, err)
					continue
				}
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	for _, account := range candidates {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			result.Success = int(atomic.LoadInt64(&success))
			result.Failed = int(atomic.LoadInt64(&failed))
			return result
		case jobs <- account:
		}
	}
	close(jobs)
	wg.Wait()

	result.Success = int(atomic.LoadInt64(&success))
	result.Failed = int(atomic.LoadInt64(&failed))
	return result
}

func (j *ChannelPriceRefreshJob) SetSettingRepository(repo SettingRepository) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.settingRepo = repo
}

func (j *ChannelPriceRefreshJob) GetSettings(ctx context.Context) ChannelPriceRefreshSettings {
	if j == nil {
		return normalizeChannelPriceRefreshSettings(ChannelPriceRefreshSettings{})
	}
	if settings, ok := j.readPersistedSettings(ctx); ok {
		j.mu.Lock()
		j.cfg = settings.toConfig()
		j.mu.Unlock()
		return j.attachLastRun(settings)
	}
	j.mu.RLock()
	settings := settingsFromChannelPriceRefreshConfig(j.cfg)
	j.mu.RUnlock()
	return j.attachLastRun(settings)
}

func (j *ChannelPriceRefreshJob) UpdateSettings(ctx context.Context, settings ChannelPriceRefreshSettings) (ChannelPriceRefreshSettings, error) {
	if j == nil {
		return normalizeChannelPriceRefreshSettings(settings), nil
	}
	normalized := normalizeChannelPriceRefreshSettings(settings)
	if err := j.persistSettings(ctx, normalized); err != nil {
		return ChannelPriceRefreshSettings{}, err
	}
	j.mu.Lock()
	j.cfg = normalized.toConfig()
	j.mu.Unlock()
	j.reschedule()
	return j.attachLastRun(normalized), nil
}

func (j *ChannelPriceRefreshJob) loadPersistedSettings(ctx context.Context) {
	if j == nil {
		return
	}
	settings, ok := j.readPersistedSettings(ctx)
	if !ok {
		return
	}
	j.mu.Lock()
	j.cfg = settings.toConfig()
	j.mu.Unlock()
}

func (j *ChannelPriceRefreshJob) readPersistedSettings(ctx context.Context) (ChannelPriceRefreshSettings, bool) {
	j.mu.RLock()
	repo := j.settingRepo
	j.mu.RUnlock()
	if repo == nil {
		return ChannelPriceRefreshSettings{}, false
	}
	value, err := repo.GetValue(ctx, SettingKeyChannelPriceRefreshSettings)
	if err != nil || value == "" {
		return ChannelPriceRefreshSettings{}, false
	}
	var settings ChannelPriceRefreshSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] invalid persisted settings: %v", err)
		return ChannelPriceRefreshSettings{}, false
	}
	return normalizeChannelPriceRefreshSettings(settings), true
}

func (j *ChannelPriceRefreshJob) persistSettings(ctx context.Context, settings ChannelPriceRefreshSettings) error {
	j.mu.RLock()
	repo := j.settingRepo
	j.mu.RUnlock()
	if repo == nil {
		return nil
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return repo.Set(ctx, SettingKeyChannelPriceRefreshSettings, string(payload))
}

func (j *ChannelPriceRefreshJob) reschedule() {
	if j == nil || j.timingWheel == nil {
		return
	}
	j.Stop()
	j.Start()
}

func (j *ChannelPriceRefreshJob) currentConfig() config.ChannelPriceRefreshConfig {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.cfg
}

func (j *ChannelPriceRefreshJob) recordLastResult(result ChannelPriceRefreshResult) {
	now := time.Now()
	resultCopy := result
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lastRunAt = &now
	j.lastResult = &resultCopy
}

func (j *ChannelPriceRefreshJob) attachLastRun(settings ChannelPriceRefreshSettings) ChannelPriceRefreshSettings {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.lastRunAt != nil {
		lastRunAt := *j.lastRunAt
		settings.LastRunAt = &lastRunAt
	}
	if j.lastResult != nil {
		lastResult := *j.lastResult
		settings.LastResult = &lastResult
	}
	return settings
}

func settingsFromChannelPriceRefreshConfig(cfg config.ChannelPriceRefreshConfig) ChannelPriceRefreshSettings {
	return normalizeChannelPriceRefreshSettings(ChannelPriceRefreshSettings{
		Enabled:         cfg.Enabled,
		IntervalSeconds: cfg.IntervalSeconds,
		Concurrency:     cfg.Concurrency,
		TimeoutSeconds:  cfg.TimeoutSeconds,
	})
}

func normalizeChannelPriceRefreshSettings(settings ChannelPriceRefreshSettings) ChannelPriceRefreshSettings {
	settings.IntervalSeconds = normalizeChannelPriceRefreshInterval(settings.IntervalSeconds)
	settings.Concurrency = normalizeChannelPriceRefreshConcurrency(settings.Concurrency)
	settings.TimeoutSeconds = normalizeChannelPriceRefreshTimeout(settings.TimeoutSeconds)
	settings.LastRunAt = nil
	settings.LastResult = nil
	return settings
}

func (s ChannelPriceRefreshSettings) toConfig() config.ChannelPriceRefreshConfig {
	return config.ChannelPriceRefreshConfig{
		Enabled:         s.Enabled,
		IntervalSeconds: s.IntervalSeconds,
		Concurrency:     s.Concurrency,
		TimeoutSeconds:  s.TimeoutSeconds,
	}
}

func (j *ChannelPriceRefreshJob) listCandidates(ctx context.Context) ([]Account, error) {
	if repo, ok := j.repo.(upstreamBalanceRefreshCandidateRepository); ok {
		return repo.ListUpstreamBalanceRefreshCandidates(ctx, 0)
	}
	return j.repo.ListActive(ctx)
}

func (j *ChannelPriceRefreshJob) runScheduled() {
	if !atomic.CompareAndSwapInt32(&j.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&j.running, 0)
	result := j.RunOnce(context.Background())
	if result.Attempted > 0 || result.Failed > 0 {
		logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] run completed attempted=%d success=%d failed=%d", result.Attempted, result.Success, result.Failed)
	}
}

func normalizeChannelPriceRefreshInterval(seconds int) int {
	if seconds <= 0 {
		return defaultChannelPriceRefreshSeconds
	}
	return seconds
}

func normalizeChannelPriceRefreshConcurrency(concurrency int) int {
	if concurrency < minChannelPriceRefreshWorkers {
		return minChannelPriceRefreshWorkers
	}
	if concurrency > maxChannelPriceRefreshWorkers {
		return maxChannelPriceRefreshWorkers
	}
	return concurrency
}

func normalizeChannelPriceRefreshTimeout(seconds int) int {
	if seconds <= 0 {
		return defaultChannelPriceRefreshTimeout
	}
	return seconds
}
