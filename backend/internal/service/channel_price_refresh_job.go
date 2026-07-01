package service

import (
	"context"
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
	Attempted int
	Success   int
	Failed    int
}

type ChannelPriceRefreshJob struct {
	repo        channelPriceRefreshAccountRepository
	refresher   channelPriceRefresher
	timingWheel *TimingWheelService
	cfg         config.ChannelPriceRefreshConfig
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

func ProvideChannelPriceRefreshJob(repo AccountRepository, refresher *OpenAIUpstreamBalanceService, timingWheel *TimingWheelService, cfg *config.Config) *ChannelPriceRefreshJob {
	job := NewChannelPriceRefreshJob(repo, refresher, timingWheel, cfg)
	job.Start()
	return job
}

func (j *ChannelPriceRefreshJob) Start() {
	if j == nil || j.repo == nil || j.refresher == nil || j.timingWheel == nil {
		return
	}
	if !j.cfg.Enabled {
		logger.LegacyPrintf("service.channel_price_refresh", "%s", "[ChannelPriceRefresh] job disabled")
		return
	}
	interval := time.Duration(j.cfg.IntervalSeconds) * time.Second
	j.timingWheel.ScheduleRecurring(channelPriceRefreshJobName, interval, func() {
		j.runScheduled()
	})
	logger.LegacyPrintf("service.channel_price_refresh", "[ChannelPriceRefresh] job started (interval=%v concurrency=%d timeout=%ds)", interval, j.cfg.Concurrency, j.cfg.TimeoutSeconds)
}

func (j *ChannelPriceRefreshJob) Stop() {
	if j == nil || j.timingWheel == nil {
		return
	}
	j.timingWheel.Cancel(channelPriceRefreshJobName)
}

func (j *ChannelPriceRefreshJob) RunOnce(ctx context.Context) ChannelPriceRefreshResult {
	var result ChannelPriceRefreshResult
	if j == nil || j.repo == nil || j.refresher == nil || !j.cfg.Enabled {
		return result
	}

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

	workerCount := j.cfg.Concurrency
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
				accountCtx, cancel := context.WithTimeout(ctx, time.Duration(j.cfg.TimeoutSeconds)*time.Second)
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
