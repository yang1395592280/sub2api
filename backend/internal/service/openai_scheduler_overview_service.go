package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	OpenAISchedulerDecisionCircuitRejected   = "circuit_rejected"
	OpenAISchedulerDecisionStale             = "stale"
	OpenAISchedulerDecisionHealthUnavailable = "health_unavailable"
	OpenAISchedulerDecisionHardFiltered      = "hard_filtered"
	OpenAISchedulerDecisionContextRequired   = "context_required"

	openAISchedulerOverviewDefaultWindow = 6 * time.Hour
	openAISchedulerOverviewMaxPageSize   = 200
)

// OpenAISchedulerOverviewParams bounds every control-console aggregate query.
type OpenAISchedulerOverviewParams struct {
	GroupID         int64
	Window          time.Duration
	Bucket          time.Duration
	StartTime       time.Time
	EndTime         time.Time
	SlowThresholdMS int
}

type OpenAISchedulerGroupSummary struct {
	ID           int64
	Name         string
	Enabled      bool
	AccountCount int64
	E2EP90MS     float64
	AlertLevel   string
}

type OpenAISchedulerTrendPoint struct {
	Bucket   time.Time
	E2EP50MS float64
	E2EP90MS float64
}

type OpenAISchedulerSlowCause struct {
	Reason string
	Count  int64
	Ratio  float64
}

type OpenAISchedulerOverviewMetrics struct {
	E2EP50MS float64
	E2EP90MS float64
	// SelectionP95MS uses usage_logs.routing_ms as the persisted selection-stage proxy.
	SelectionP95MS float64
	ProbeRatio     float64
	Groups         []OpenAISchedulerGroupSummary
	Trend          []OpenAISchedulerTrendPoint
	SlowCauses     []OpenAISchedulerSlowCause
	Runtime        OpenAISchedulerRuntimeMetrics
}

type OpenAISchedulerRuntimeMetrics struct {
	ExplorationAllowedTotal      int64
	ExplorationRejectedTotal     int64
	ExplorationIntervalTotal     int64
	ExplorationHourlyTotal       int64
	ExplorationErrorTotal        int64
	LowConfidenceFallbackTotal   int64
	UnifiedHealthReadsTotal      int64
	UnifiedHealthDimensionsTotal int64
	UnifiedHealthFallbacksTotal  int64
}

type OpenAISchedulerHealthParams struct {
	GroupID     int64
	State       string
	ModelFamily string
	Endpoint    string
	Transport   string
	Sort        string
	Order       string
	Page        int
	PageSize    int
}

// OpenAISchedulerHealthRecord is the bounded database view before live load is attached.
type OpenAISchedulerHealthRecord struct {
	AccountID                 int64
	AccountName               string
	GroupID                   int64
	GroupStatus               string
	GroupAutoSchedulerEnabled bool
	GroupPriority             int
	AccountStatus             string
	Schedulable               bool
	TempUnschedulableUntil    *time.Time
	TempUnschedulableReason   string
	AutoPauseOnExpired        bool
	AccountExpiresAt          *time.Time
	OverloadUntil             *time.Time
	RateLimitResetAt          *time.Time
	ModelFamily               string
	Endpoint                  string
	Transport                 string
	State                     string
	PredictedTTFTMS           float64
	RealSampleCount           int64
	ProbeSampleCount          int64
	ErrorRate                 float64
	RateLimitedRate           float64
	ServerErrorRate           float64
	CooldownUntil             *time.Time
	ExpiresAt                 time.Time
	LastRealAt                *time.Time
	UpdatedAt                 time.Time
	LoadCapacity              int
	ChannelPrice              *float64
}

type OpenAISchedulerHealthRow struct {
	AccountID        int64
	AccountName      string
	GroupID          int64
	ModelFamily      string
	Endpoint         string
	Transport        string
	State            string
	PredictedTTFTMS  float64
	RealSampleCount  int64
	ProbeSampleCount int64
	ErrorRate        float64
	RateLimitedRate  float64
	ServerErrorRate  float64
	LoadInflight     int
	LoadCapacity     int
	WaitingCount     int
	ChannelPrice     *float64
	Decision         string
	DecisionReason   string
	// SchedulerMode and ShadowMode describe runtime context; Decision remains a health classification.
	SchedulerMode      string
	ShadowMode         bool
	StickyEscapeReason *string
	SnapshotAgeMS      *int64
	CooldownUntil      *time.Time
}

type OpenAISchedulerHealthListResult struct {
	Items []OpenAISchedulerHealthRow
	Total int64
}

type OpenAISchedulerOverviewRepository interface {
	GetOpenAISchedulerOverviewMetrics(context.Context, OpenAISchedulerOverviewParams) (OpenAISchedulerOverviewMetrics, error)
	ListOpenAISchedulerHealth(context.Context, OpenAISchedulerHealthParams) ([]OpenAISchedulerHealthRecord, int64, error)
}

type openAISchedulerOverviewLoadService interface {
	GetAccountsLoadBatch(context.Context, []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error)
}

type openAISchedulerEngineSettingsProvider interface {
	IsOpenAIAdvancedSchedulerEnabled(context.Context) bool
}

type openAISchedulerRuntimeMetricsProvider interface {
	SnapshotOpenAIAccountSchedulerMetrics() OpenAIAccountSchedulerMetricsSnapshot
}

type openAISchedulerSummaryMetricsProvider interface {
	SnapshotSummaryMetrics() OpenAIAutoSchedulerSummaryMetricsSnapshot
}

type OpenAISchedulerOverviewService struct {
	repo     OpenAISchedulerOverviewRepository
	loads    openAISchedulerOverviewLoadService
	settings OpenAIAutoSchedulerSettingsProvider
	runtime  openAISchedulerRuntimeMetricsProvider
	summary  openAISchedulerSummaryMetricsProvider
	now      func() time.Time
}

func NewOpenAISchedulerOverviewService(repo OpenAISchedulerOverviewRepository) *OpenAISchedulerOverviewService {
	return &OpenAISchedulerOverviewService{repo: repo, now: time.Now}
}

func ProvideOpenAISchedulerOverviewService(
	repo OpenAISchedulerOverviewRepository,
	loads *ConcurrencyService,
	settings *SettingService,
	runtime *OpenAIGatewayService,
	summary *OpenAIAutoSchedulerService,
) *OpenAISchedulerOverviewService {
	svc := NewOpenAISchedulerOverviewService(repo)
	svc.loads = loads
	svc.settings = settings
	svc.runtime = runtime
	svc.summary = summary
	return svc
}

func (s *OpenAISchedulerOverviewService) GetOverview(ctx context.Context, params OpenAISchedulerOverviewParams) (OpenAISchedulerOverviewMetrics, error) {
	if s == nil || s.repo == nil {
		return OpenAISchedulerOverviewMetrics{}, nil
	}
	window, bucket, err := normalizeOpenAISchedulerOverviewWindow(params.Window)
	if err != nil {
		return OpenAISchedulerOverviewMetrics{}, err
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	settings := s.schedulerSettings(ctx)
	params.Window = window
	params.Bucket = bucket
	params.EndTime = now
	params.StartTime = now.Add(-window)
	params.SlowThresholdMS = settings.SlowThresholdMS
	metrics, err := s.repo.GetOpenAISchedulerOverviewMetrics(ctx, params)
	if err != nil {
		return OpenAISchedulerOverviewMetrics{}, err
	}
	if metrics.Groups == nil {
		metrics.Groups = []OpenAISchedulerGroupSummary{}
	}
	for i := range metrics.Groups {
		metrics.Groups[i].AlertLevel = openAISchedulerGroupAlertLevel(metrics.Groups[i], settings)
	}
	if metrics.Trend == nil {
		metrics.Trend = []OpenAISchedulerTrendPoint{}
	}
	if metrics.SlowCauses == nil {
		metrics.SlowCauses = []OpenAISchedulerSlowCause{}
	}
	if s.runtime != nil {
		snapshot := s.runtime.SnapshotOpenAIAccountSchedulerMetrics()
		metrics.Runtime.ExplorationAllowedTotal = snapshot.ExplorationAllowedTotal
		metrics.Runtime.ExplorationRejectedTotal = snapshot.ExplorationRejectedTotal
		metrics.Runtime.ExplorationIntervalTotal = snapshot.ExplorationIntervalTotal
		metrics.Runtime.ExplorationHourlyTotal = snapshot.ExplorationHourlyTotal
		metrics.Runtime.ExplorationErrorTotal = snapshot.ExplorationErrorTotal
		metrics.Runtime.LowConfidenceFallbackTotal = snapshot.LowConfidenceFallbackTotal
	}
	if s.summary != nil {
		snapshot := s.summary.SnapshotSummaryMetrics()
		metrics.Runtime.UnifiedHealthReadsTotal = snapshot.UnifiedReadsTotal
		metrics.Runtime.UnifiedHealthDimensionsTotal = snapshot.UnifiedDimensionsTotal
		metrics.Runtime.UnifiedHealthFallbacksTotal = snapshot.LegacyFallbacksTotal
	}
	return metrics, nil
}

func (s *OpenAISchedulerOverviewService) ListHealth(ctx context.Context, params OpenAISchedulerHealthParams) (*OpenAISchedulerHealthListResult, error) {
	if s == nil || s.repo == nil {
		return &OpenAISchedulerHealthListResult{Items: []OpenAISchedulerHealthRow{}}, nil
	}
	params = normalizeOpenAISchedulerHealthParams(params)
	records, total, err := s.repo.ListOpenAISchedulerHealth(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return &OpenAISchedulerHealthListResult{Items: []OpenAISchedulerHealthRow{}, Total: total}, nil
	}

	loadRequest := uniqueOpenAISchedulerHealthLoadRequest(records)
	loadMap := map[int64]*AccountLoadInfo{}
	if s.loads != nil && len(loadRequest) > 0 {
		loadMap, err = s.loads.GetAccountsLoadBatch(ctx, loadRequest)
		if err != nil {
			return nil, err
		}
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	settings := s.schedulerSettings(ctx)
	items := make([]OpenAISchedulerHealthRow, 0, len(records))
	for _, record := range records {
		items = append(items, openAISchedulerHealthRecordToRow(record, loadMap[record.AccountID], settings, now))
	}
	return &OpenAISchedulerHealthListResult{Items: items, Total: total}, nil
}

func normalizeOpenAISchedulerOverviewWindow(window time.Duration) (time.Duration, time.Duration, error) {
	if window == 0 {
		window = openAISchedulerOverviewDefaultWindow
	}
	switch window {
	case time.Hour, 6 * time.Hour, 24 * time.Hour:
		return window, time.Hour, nil
	case 7 * 24 * time.Hour:
		return window, 6 * time.Hour, nil
	default:
		return 0, 0, fmt.Errorf("window must be one of 1h, 6h, 24h, 7d")
	}
}

func normalizeOpenAISchedulerHealthParams(params OpenAISchedulerHealthParams) OpenAISchedulerHealthParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > openAISchedulerOverviewMaxPageSize {
		params.PageSize = openAISchedulerOverviewMaxPageSize
	}
	params.State = strings.ToLower(strings.TrimSpace(params.State))
	params.ModelFamily = strings.ToLower(strings.TrimSpace(params.ModelFamily))
	params.Endpoint = strings.ToLower(strings.TrimSpace(params.Endpoint))
	params.Transport = strings.ToLower(strings.TrimSpace(params.Transport))
	params.Sort = strings.ToLower(strings.TrimSpace(params.Sort))
	if params.Sort == "" {
		params.Sort = "predicted_ttft_ms"
	}
	params.Order = strings.ToLower(strings.TrimSpace(params.Order))
	if params.Order != "asc" {
		params.Order = "desc"
	}
	return params
}

func uniqueOpenAISchedulerHealthLoadRequest(records []OpenAISchedulerHealthRecord) []AccountWithConcurrency {
	request := make([]AccountWithConcurrency, 0, len(records))
	seen := make(map[int64]struct{}, len(records))
	for _, record := range records {
		if record.AccountID <= 0 {
			continue
		}
		if _, ok := seen[record.AccountID]; ok {
			continue
		}
		seen[record.AccountID] = struct{}{}
		request = append(request, AccountWithConcurrency{ID: record.AccountID, MaxConcurrency: record.LoadCapacity})
	}
	return request
}

func openAISchedulerHealthRecordToRow(record OpenAISchedulerHealthRecord, load *AccountLoadInfo, settings OpenAIAutoSchedulerSettings, now time.Time) OpenAISchedulerHealthRow {
	decision, reason := classifyOpenAISchedulerHealthRecord(record, now)
	row := OpenAISchedulerHealthRow{
		AccountID: record.AccountID, AccountName: record.AccountName, GroupID: record.GroupID,
		ModelFamily: record.ModelFamily, Endpoint: record.Endpoint, Transport: record.Transport,
		State: record.State, PredictedTTFTMS: record.PredictedTTFTMS,
		RealSampleCount: record.RealSampleCount, ProbeSampleCount: record.ProbeSampleCount,
		ErrorRate: record.ErrorRate, RateLimitedRate: record.RateLimitedRate, ServerErrorRate: record.ServerErrorRate,
		LoadCapacity: record.LoadCapacity, ChannelPrice: record.ChannelPrice,
		Decision: decision, DecisionReason: reason, SchedulerMode: settings.Mode, ShadowMode: settings.ShadowMode,
		StickyEscapeReason: nil, CooldownUntil: record.CooldownUntil,
	}
	if load != nil {
		row.LoadInflight = load.CurrentConcurrency
		row.WaitingCount = load.WaitingCount
	}
	if !record.UpdatedAt.IsZero() {
		age := now.Sub(record.UpdatedAt).Milliseconds()
		if age < 0 {
			age = 0
		}
		row.SnapshotAgeMS = &age
	}
	return row
}

func classifyOpenAISchedulerHealthRecord(record OpenAISchedulerHealthRecord, now time.Time) (string, string) {
	if record.ModelFamily == "" || record.Endpoint == "" || record.Transport == "" || record.State == "" {
		return OpenAISchedulerDecisionHealthUnavailable, "snapshot_missing"
	}
	if record.ExpiresAt.IsZero() || !record.ExpiresAt.After(now) {
		return OpenAISchedulerDecisionStale, "snapshot_expired"
	}
	state := normalizeOpenAIAutoSchedulerState(record.State)
	if state == OpenAIAutoSchedulerStateOpen || state == OpenAIAutoSchedulerStateHalfOpen {
		return OpenAISchedulerDecisionCircuitRejected, state
	}
	if record.GroupStatus != StatusActive {
		return OpenAISchedulerDecisionHardFiltered, "group_inactive"
	}
	if !record.GroupAutoSchedulerEnabled {
		return OpenAISchedulerDecisionHardFiltered, "group_scheduler_disabled"
	}
	if record.AccountStatus != StatusActive {
		return OpenAISchedulerDecisionHardFiltered, "account_inactive"
	}
	if !record.Schedulable {
		return OpenAISchedulerDecisionHardFiltered, "account_unschedulable"
	}
	if record.TempUnschedulableUntil != nil && now.Before(*record.TempUnschedulableUntil) {
		reason := strings.TrimSpace(record.TempUnschedulableReason)
		if reason == "" {
			return OpenAISchedulerDecisionHardFiltered, "temporarily_blocked"
		}
		return OpenAISchedulerDecisionHardFiltered, "temporarily_blocked: " + reason
	}
	if record.AutoPauseOnExpired && record.AccountExpiresAt != nil && !now.Before(*record.AccountExpiresAt) {
		return OpenAISchedulerDecisionHardFiltered, "account_expired"
	}
	if record.OverloadUntil != nil && now.Before(*record.OverloadUntil) {
		return OpenAISchedulerDecisionHardFiltered, "account_overloaded"
	}
	if record.RateLimitResetAt != nil && now.Before(*record.RateLimitResetAt) {
		return OpenAISchedulerDecisionHardFiltered, "account_rate_limited"
	}
	return OpenAISchedulerDecisionContextRequired, "request_context_required"
}

func (s *OpenAISchedulerOverviewService) schedulerSettings(ctx context.Context) OpenAIAutoSchedulerSettings {
	if s != nil && s.settings != nil {
		return s.settings.GetOpenAIAutoSchedulerSettings(ctx)
	}
	return DefaultOpenAIAutoSchedulerSettings()
}

func openAISchedulerGroupAlertLevel(group OpenAISchedulerGroupSummary, settings OpenAIAutoSchedulerSettings) string {
	if !group.Enabled {
		return "disabled"
	}
	if group.E2EP90MS >= float64(settings.SevereSlowThresholdMS) {
		return "critical"
	}
	if group.E2EP90MS >= float64(settings.SlowThresholdMS) {
		return "warning"
	}
	return "ok"
}
