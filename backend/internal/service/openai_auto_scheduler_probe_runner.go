package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/alitto/pond/v2"
)

const (
	openAIAutoSchedulerProbeWorkerLimit  = 5
	openAIAutoSchedulerProbeTimeout      = 15 * time.Second
	openAIAutoSchedulerProbeMaxBodyBytes = 256 * 1024
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
	tlsFPProfileService *TLSFingerprintProfileService

	pool      pond.Pool
	parentCtx context.Context
	cancel    context.CancelFunc

	startOnce sync.Once
	stopOnce  sync.Once
	mu        sync.Mutex
	stopped   bool
	inFlight  map[string]struct{}
}

func newOpenAIAutoSchedulerProbeRunner(svc openAIAutoSchedulerProbeRunnerService, settingsProvider OpenAIAutoSchedulerSettingsProvider, accountRepo openAIAutoSchedulerProbeAccountRepository, checker OpenAIAutoSchedulerProbeChecker, tlsFPProfileService *TLSFingerprintProfileService) *OpenAIAutoSchedulerProbeRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpenAIAutoSchedulerProbeRunner{
		svc:                 svc,
		settingsProvider:    settingsProvider,
		accountRepo:         accountRepo,
		checker:             checker,
		tlsFPProfileService: tlsFPProfileService,
		pool:                pond.NewPool(openAIAutoSchedulerProbeWorkerLimit),
		parentCtx:           ctx,
		cancel:              cancel,
		inFlight:            map[string]struct{}{},
	}
}

func NewOpenAIAutoSchedulerProbeRunner(svc *OpenAIAutoSchedulerService, settingsProvider OpenAIAutoSchedulerSettingsProvider, accountRepo AccountRepository, checker OpenAIAutoSchedulerProbeChecker, tlsFPProfileService *TLSFingerprintProfileService) *OpenAIAutoSchedulerProbeRunner {
	return newOpenAIAutoSchedulerProbeRunner(svc, settingsProvider, accountRepo, checker, tlsFPProfileService)
}

func (r *OpenAIAutoSchedulerProbeRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.startOnce.Do(func() {
		go r.loop()
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
		if r.pool != nil {
			r.pool.StopAndWait()
		}
	})
}

func (r *OpenAIAutoSchedulerProbeRunner) loop() {
	ctx := r.parentCtx
	r.runOnce(ctx)
	timer := time.NewTimer(openAIAutoSchedulerProbeInterval(r.settingsProvider, ctx))
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
			timer.Reset(openAIAutoSchedulerProbeInterval(r.settingsProvider, ctx))
		}
	}
}

func openAIAutoSchedulerProbeInterval(provider OpenAIAutoSchedulerSettingsProvider, ctx context.Context) time.Duration {
	interval := 60
	if provider != nil {
		settings := normalizeOpenAIAutoSchedulerSettings(provider.GetOpenAIAutoSchedulerSettings(ctx))
		if settings.ProbeIntervalSeconds > 0 {
			interval = settings.ProbeIntervalSeconds
		}
	}
	return time.Duration(interval) * time.Second
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

	groups, err := r.svc.ListEnabledOpenAIGroups(ctx)
	if err != nil {
		slog.Warn("openai_auto_scheduler_probe: list groups failed", "error", err)
		return
	}

	model := selectOpenAIAutoSchedulerProbeModel()
	timeout := openAIAutoSchedulerProbeTimeout
	for i := range groups {
		if ctx.Err() != nil {
			return
		}
		group := groups[i]
		accounts, err := r.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, PlatformOpenAI)
		if err != nil {
			slog.Warn("openai_auto_scheduler_probe: list accounts failed", "group_id", group.ID, "error", err)
			continue
		}
		for i := range accounts {
			if ctx.Err() != nil {
				return
			}
			account := accounts[i]
			if account.ID <= 0 {
				continue
			}
			key := openAIAutoSchedulerProbeKey(account.ID, group.ID, model)
			if !r.tryAcquireInFlight(key) {
				continue
			}
			accountCopy := account
			groupID := group.ID
			if r.pool == nil || r.pool.Stopped() {
				r.releaseInFlight(key)
				continue
			}
			if _, ok := r.pool.TrySubmit(func() {
				defer r.releaseInFlight(key)
				r.runProbe(ctx, &accountCopy, groupID, model, timeout)
			}); !ok {
				r.releaseInFlight(key)
			}
		}
	}
}

func (r *OpenAIAutoSchedulerProbeRunner) runProbe(ctx context.Context, account *Account, groupID int64, model string, timeout time.Duration) {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := r.checker.Check(probeCtx, account, model, timeout)
	eventType := OpenAIAutoSchedulerEventProbeError
	if result.Success && result.Err == nil {
		eventType = OpenAIAutoSchedulerEventProbeSuccess
	}
	input := OpenAIAutoSchedulerRecordInput{
		AccountID: account.ID,
		GroupID:   groupID,
		Model:     model,
		EventType: eventType,
		LatencyMS: result.LatencyMS,
		Message:   strings.TrimSpace(result.Message),
	}
	if result.Err != nil && input.Message == "" {
		input.Message = result.Err.Error()
	}
	if err := r.svc.Record(ctx, input); err != nil {
		slog.Warn("openai_auto_scheduler_probe: record failed", "account_id", account.ID, "group_id", groupID, "error", err)
	}
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

func selectOpenAIAutoSchedulerProbeModel() string {
	return "gpt-5.4"
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
	targetURL := buildOpenAIResponsesURL(baseURL)
	upstreamModel := normalizeOpenAIModelForUpstream(account, account.GetMappedModel(model))
	payload, _ := json.Marshal(map[string]any{
		"model":             upstreamModel,
		"input":             "probe",
		"max_output_tokens": 1,
		"stream":            false,
	})

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return OpenAIAutoSchedulerProbeResult{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
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
	resp, err := c.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, c.resolveTLSProfile(account))
	if err != nil {
		return OpenAIAutoSchedulerProbeResult{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, openAIAutoSchedulerProbeMaxBodyBytes))
	if readErr != nil {
		return OpenAIAutoSchedulerProbeResult{Err: readErr}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OpenAIAutoSchedulerProbeResult{Message: fmt.Sprintf("upstream HTTP %d", resp.StatusCode)}
	}
	if strings.TrimSpace(string(body)) == "" {
		return OpenAIAutoSchedulerProbeResult{Err: errors.New("empty probe response")}
	}
	return OpenAIAutoSchedulerProbeResult{Success: true}
}

func (c *openAIAutoSchedulerProbeHTTPChecker) resolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if c == nil || c.tlsFPProfileService == nil {
		return nil
	}
	return c.tlsFPProfileService.ResolveTLSProfile(account)
}
