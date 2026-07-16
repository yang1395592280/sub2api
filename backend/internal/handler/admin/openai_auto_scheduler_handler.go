package admin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const openAIAutoSchedulerMaxPageSize = 200

type openAIAutoSchedulerSettingsService interface {
	GetOpenAIAutoSchedulerSettings(ctx context.Context) service.OpenAIAutoSchedulerSettings
	SetOpenAIAutoSchedulerSettings(ctx context.Context, settings service.OpenAIAutoSchedulerSettings) error
}

type openAIAutoSchedulerAdminService interface {
	GetAllGroupsByPlatform(ctx context.Context, platform string) ([]service.Group, error)
	GetGroup(ctx context.Context, id int64) (*service.Group, error)
	UpdateGroup(ctx context.Context, id int64, input *service.UpdateGroupInput) (*service.Group, error)
}

type openAIAutoSchedulerService interface {
	ListScores(ctx context.Context, params service.OpenAIAutoSchedulerListParams) (*service.OpenAIAutoSchedulerScoreListResult, error)
	ListEvents(ctx context.Context, params service.OpenAIAutoSchedulerListParams) (*service.OpenAIAutoSchedulerEventListResult, error)
	ResetScore(ctx context.Context, accountID, groupID int64, model string) error
	Record(ctx context.Context, input service.OpenAIAutoSchedulerRecordInput) error
	RecordManualProbe(ctx context.Context, input service.OpenAIAutoSchedulerRecordInput) error
}

type openAIAutoSchedulerAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*service.Account, error)
}

type openAISchedulerOverviewService interface {
	GetOverview(ctx context.Context, params service.OpenAISchedulerOverviewParams) (service.OpenAISchedulerOverviewMetrics, error)
	ListHealth(ctx context.Context, params service.OpenAISchedulerHealthParams) (*service.OpenAISchedulerHealthListResult, error)
	ListRankings(ctx context.Context, params service.OpenAISchedulerRankingParams) (*service.OpenAISchedulerRankingResult, error)
}

// OpenAIAutoSchedulerHandler exposes admin APIs for OpenAI auto scheduler state.
type OpenAIAutoSchedulerHandler struct {
	settingsSvc openAIAutoSchedulerSettingsService
	adminSvc    openAIAutoSchedulerAdminService
	scheduler   openAIAutoSchedulerService
	accountRepo openAIAutoSchedulerAccountRepository
	checker     service.OpenAIAutoSchedulerProbeChecker
	overview    openAISchedulerOverviewService
}

func NewOpenAIAutoSchedulerHandler(
	settingsSvc openAIAutoSchedulerSettingsService,
	adminSvc openAIAutoSchedulerAdminService,
	scheduler openAIAutoSchedulerService,
	accountRepo openAIAutoSchedulerAccountRepository,
	checker service.OpenAIAutoSchedulerProbeChecker,
	overview ...openAISchedulerOverviewService,
) *OpenAIAutoSchedulerHandler {
	h := &OpenAIAutoSchedulerHandler{
		settingsSvc: settingsSvc,
		adminSvc:    adminSvc,
		scheduler:   scheduler,
		accountRepo: accountRepo,
		checker:     checker,
	}
	if len(overview) > 0 {
		h.overview = overview[0]
	}
	return h
}

func ProvideOpenAIAutoSchedulerHandler(
	settingService *service.SettingService,
	adminService service.AdminService,
	schedulerService *service.OpenAIAutoSchedulerService,
	accountRepo service.AccountRepository,
	checker service.OpenAIAutoSchedulerProbeChecker,
	overviewService *service.OpenAISchedulerOverviewService,
) *OpenAIAutoSchedulerHandler {
	return NewOpenAIAutoSchedulerHandler(settingService, adminService, schedulerService, accountRepo, checker, overviewService)
}

func (h *OpenAIAutoSchedulerHandler) GetOverview(c *gin.Context) {
	if h == nil || h.overview == nil {
		response.InternalError(c, "openai scheduler overview service is not configured")
		return
	}
	groupID, ok := parseOptionalPositiveInt64Query(c, "group_id")
	if !ok {
		return
	}
	window, ok := parseOpenAISchedulerOverviewWindow(c)
	if !ok {
		return
	}
	metrics, err := h.overview.GetOverview(c.Request.Context(), service.OpenAISchedulerOverviewParams{
		GroupID: groupID,
		Window:  window,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, openAISchedulerOverviewToResponse(metrics))
}

func (h *OpenAIAutoSchedulerHandler) ListHealth(c *gin.Context) {
	if h == nil || h.overview == nil {
		response.InternalError(c, "openai scheduler overview service is not configured")
		return
	}
	page, pageSize := parseOpenAIAutoSchedulerPagination(c)
	params, ok := parseOpenAISchedulerHealthParams(c, page, pageSize)
	if !ok {
		return
	}
	result, err := h.overview.ListHealth(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]openAISchedulerHealthResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, openAISchedulerHealthToResponse(item))
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *OpenAIAutoSchedulerHandler) ListRankings(c *gin.Context) {
	if h == nil || h.overview == nil {
		response.InternalError(c, "openai scheduler overview service is not configured")
		return
	}
	groupID, ok := parseRequiredPositiveInt64Query(c, "group_id")
	if !ok {
		return
	}
	window, ok := parseOpenAISchedulerRankingWindow(c)
	if !ok {
		return
	}
	eligibility := strings.ToLower(strings.TrimSpace(c.Query("eligibility")))
	if eligibility != "" && eligibility != service.OpenAISchedulerEligibilityEligible &&
		eligibility != service.OpenAISchedulerEligibilityLowConfidence &&
		eligibility != service.OpenAISchedulerEligibilityLatencyTail &&
		eligibility != service.OpenAISchedulerEligibilityRejected {
		response.BadRequest(c, "eligibility must be eligible, low_confidence, latency_tail, or hard_rejected")
		return
	}
	page, pageSize := parseOpenAIAutoSchedulerPagination(c)
	result, err := h.overview.ListRankings(c.Request.Context(), service.OpenAISchedulerRankingParams{
		GroupID: groupID, Window: window,
		ModelFamily: strings.ToLower(strings.TrimSpace(c.Query("model_family"))),
		Endpoint:    strings.ToLower(strings.TrimSpace(c.Query("endpoint"))),
		Transport:   strings.ToLower(strings.TrimSpace(c.Query("transport"))),
		Eligibility: eligibility,
		Page:        page, PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *OpenAIAutoSchedulerHandler) GetSettings(c *gin.Context) {
	if h == nil || h.settingsSvc == nil {
		response.InternalError(c, "openai auto scheduler settings service is not configured")
		return
	}
	response.Success(c, h.settingsSvc.GetOpenAIAutoSchedulerSettings(c.Request.Context()))
}

func (h *OpenAIAutoSchedulerHandler) UpdateSettings(c *gin.Context) {
	if h == nil || h.settingsSvc == nil {
		response.InternalError(c, "openai auto scheduler settings service is not configured")
		return
	}
	req := service.DefaultOpenAIAutoSchedulerSettings()
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if msg := validateOpenAIAutoSchedulerSettings(req); msg != "" {
		response.BadRequest(c, msg)
		return
	}
	if err := h.settingsSvc.SetOpenAIAutoSchedulerSettings(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.settingsSvc.GetOpenAIAutoSchedulerSettings(c.Request.Context()))
}

func (h *OpenAIAutoSchedulerHandler) ListGroups(c *gin.Context) {
	if h == nil || h.adminSvc == nil {
		response.InternalError(c, "admin service is not configured")
		return
	}
	groups, err := h.adminSvc.GetAllGroupsByPlatform(c.Request.Context(), service.PlatformOpenAI)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]openAIAutoSchedulerGroupResponse, 0, len(groups))
	for _, group := range groups {
		out = append(out, openAIAutoSchedulerGroupToResponse(group))
	}
	response.Success(c, out)
}

func (h *OpenAIAutoSchedulerHandler) UpdateGroup(c *gin.Context) {
	if h == nil || h.adminSvc == nil {
		response.InternalError(c, "admin service is not configured")
		return
	}
	id, ok := parsePositiveInt64Param(c, "id", "invalid group id")
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	group, err := h.adminSvc.GetGroup(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if group == nil {
		response.NotFound(c, "group not found")
		return
	}
	if group.Platform != service.PlatformOpenAI {
		response.BadRequest(c, "OpenAI auto scheduler can only be toggled for OpenAI groups")
		return
	}
	updated, err := h.adminSvc.UpdateGroup(c.Request.Context(), id, &service.UpdateGroupInput{
		OpenAIAutoSchedulerEnabled: req.Enabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if updated == nil {
		response.NotFound(c, "group not found")
		return
	}
	response.Success(c, openAIAutoSchedulerGroupToResponse(*updated))
}

func (h *OpenAIAutoSchedulerHandler) ListScores(c *gin.Context) {
	if h == nil || h.scheduler == nil {
		response.InternalError(c, "openai auto scheduler service is not configured")
		return
	}
	page, pageSize := parseOpenAIAutoSchedulerPagination(c)
	params, ok := parseOpenAIAutoSchedulerListParams(c, page, pageSize)
	if !ok {
		return
	}
	result, err := h.scheduler.ListScores(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]openAIAutoSchedulerScoreResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, openAIAutoSchedulerScoreToResponse(item))
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *OpenAIAutoSchedulerHandler) ListEvents(c *gin.Context) {
	if h == nil || h.scheduler == nil {
		response.InternalError(c, "openai auto scheduler service is not configured")
		return
	}
	page, pageSize := parseOpenAIAutoSchedulerPagination(c)
	params, ok := parseOpenAIAutoSchedulerListParams(c, page, pageSize)
	if !ok {
		return
	}
	result, err := h.scheduler.ListEvents(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]openAIAutoSchedulerEventResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, openAIAutoSchedulerEventToResponse(item))
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *OpenAIAutoSchedulerHandler) ResetScore(c *gin.Context) {
	accountID, ok := parsePositiveInt64Param(c, "account_id", "invalid account id")
	if !ok {
		return
	}
	groupID, model, ok := parseScoreMutationQuery(c)
	if !ok {
		return
	}
	if h == nil || h.scheduler == nil {
		response.InternalError(c, "openai auto scheduler service is not configured")
		return
	}
	if err := h.scheduler.ResetScore(c.Request.Context(), accountID, groupID, model); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "score reset"})
}

func (h *OpenAIAutoSchedulerHandler) ProbeScore(c *gin.Context) {
	accountID, ok := parsePositiveInt64Param(c, "account_id", "invalid account id")
	if !ok {
		return
	}
	groupID, model, ok := parseScoreMutationQuery(c)
	if !ok {
		return
	}
	if h == nil || h.scheduler == nil || h.accountRepo == nil || h.checker == nil || h.settingsSvc == nil {
		response.InternalError(c, "openai auto scheduler probe dependencies are not configured")
		return
	}
	account, err := h.accountRepo.GetByID(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account == nil {
		response.NotFound(c, "account not found")
		return
	}
	if !account.IsOpenAI() {
		response.BadRequest(c, "manual probe only supports OpenAI accounts")
		return
	}
	result := h.checker.Check(c.Request.Context(), account, model, 15*time.Second)
	settings := h.settingsSvc.GetOpenAIAutoSchedulerSettings(c.Request.Context())
	eventType := service.ClassifyOpenAIAutoSchedulerProbeEvent(result, settings)
	message := strings.TrimSpace(result.Message)
	if result.Err != nil && message == "" {
		message = result.Err.Error()
	}
	if err := h.scheduler.RecordManualProbe(c.Request.Context(), service.OpenAIAutoSchedulerRecordInput{
		AccountID: accountID,
		GroupID:   groupID,
		Model:     model,
		EventType: eventType,
		LatencyMS: result.LatencyMS,
		TtfbMS:    result.TtfbMS,
		Message:   message,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"event_type": eventType,
		"success":    result.Success && result.Err == nil,
		"message":    message,
		"latency_ms": result.LatencyMS,
		"ttfb_ms":    result.TtfbMS,
	})
}

type openAIAutoSchedulerGroupResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Enabled bool   `json:"enabled"`
}

type openAIAutoSchedulerScoreResponse struct {
	AccountID               int64    `json:"account_id"`
	AccountName             string   `json:"account_name"`
	ChannelPrice            *float64 `json:"channel_price"`
	GroupID                 int64    `json:"group_id"`
	Model                   string   `json:"model"`
	BaseScore               int      `json:"base_score"`
	BaseScorePercent        float64  `json:"base_score_percent"`
	FinalScore              int      `json:"final_score"`
	FinalScorePercent       float64  `json:"final_score_percent"`
	LatencyScore            int      `json:"latency_score"`
	LatencyScorePercent     float64  `json:"latency_score_percent"`
	ErrorScore              int      `json:"error_score"`
	ErrorScorePercent       float64  `json:"error_score_percent"`
	RecoveryScore           int      `json:"recovery_score"`
	RecoveryScorePercent    float64  `json:"recovery_score_percent"`
	CostScore               int      `json:"cost_score"`
	CostScorePercent        float64  `json:"cost_score_percent"`
	State                   string   `json:"state"`
	ConsecutiveSlowCount    int      `json:"consecutive_slow_count"`
	ConsecutiveErrorCount   int      `json:"consecutive_error_count"`
	ConsecutiveSuccessCount int      `json:"consecutive_success_count"`
	RequestCount            int64    `json:"request_count"`
	TtfbSampleCount         int64    `json:"ttfb_sample_count"`
	SlowRate                float64  `json:"slow_rate"`
	ErrorRate               float64  `json:"error_rate"`
	StuckRate               float64  `json:"stuck_rate"`
	CooldownUntil           *string  `json:"cooldown_until"`
	LastLatencyMS           *int     `json:"last_latency_ms"`
	LastTtfbMS              *int     `json:"last_ttfb_ms"`
	LastStatusCode          *int     `json:"last_status_code"`
	LastError               *string  `json:"last_error"`
	Reason                  string   `json:"reason"`
	LastCheckedAt           *string  `json:"last_checked_at"`
}

type openAIAutoSchedulerEventResponse struct {
	AccountID          int64   `json:"account_id"`
	GroupID            int64   `json:"group_id"`
	Model              string  `json:"model"`
	EventType          string  `json:"event_type"`
	ScoreBefore        int     `json:"score_before"`
	ScoreBeforePercent float64 `json:"score_before_percent"`
	ScoreAfter         int     `json:"score_after"`
	ScoreAfterPercent  float64 `json:"score_after_percent"`
	LatencyMS          *int    `json:"latency_ms"`
	TtfbMS             *int    `json:"ttfb_ms"`
	StatusCode         *int    `json:"status_code"`
	Message            string  `json:"message"`
	CreatedAt          string  `json:"created_at"`
}

type openAISchedulerOverviewResponse struct {
	E2ETTFTP50MS *float64 `json:"e2e_ttft_p50_ms"`
	E2ETTFTP90MS *float64 `json:"e2e_ttft_p90_ms"`
	// SelectionP95MS is the routing_ms proxy because no narrower persisted selection timer exists.
	SelectionP95MS *float64                           `json:"selection_p95_ms"`
	ProbeRatio     float64                            `json:"probe_ratio"`
	Groups         []openAISchedulerGroupResponse     `json:"groups"`
	Trend          []openAISchedulerTrendResponse     `json:"trend"`
	SlowCauses     []openAISchedulerSlowCauseResponse `json:"slow_causes"`
}

type openAISchedulerGroupResponse struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Enabled      bool     `json:"enabled"`
	AccountCount int64    `json:"account_count"`
	E2ETTFTP90MS *float64 `json:"e2e_ttft_p90_ms"`
	AlertLevel   string   `json:"alert_level"`
}

type openAISchedulerTrendResponse struct {
	Bucket       string   `json:"bucket"`
	E2ETTFTP50MS *float64 `json:"e2e_ttft_p50_ms"`
	E2ETTFTP90MS *float64 `json:"e2e_ttft_p90_ms"`
}

type openAISchedulerSlowCauseResponse struct {
	Reason string  `json:"reason"`
	Count  int64   `json:"count"`
	Ratio  float64 `json:"ratio"`
}

type openAISchedulerHealthResponse struct {
	AccountID          int64    `json:"account_id"`
	AccountName        string   `json:"account_name"`
	GroupID            int64    `json:"group_id"`
	ModelFamily        string   `json:"model_family"`
	Endpoint           string   `json:"endpoint"`
	Transport          string   `json:"transport"`
	State              string   `json:"state"`
	PredictedTTFTMS    *float64 `json:"predicted_ttft_ms"`
	RealSampleCount    int64    `json:"real_sample_count"`
	ProbeSampleCount   int64    `json:"probe_sample_count"`
	ErrorRate          float64  `json:"error_rate"`
	RateLimitedRate    float64  `json:"rate_limited_rate"`
	ServerErrorRate    float64  `json:"server_error_rate"`
	LoadInflight       int      `json:"load_inflight"`
	LoadCapacity       int      `json:"load_capacity"`
	WaitingCount       int      `json:"waiting_count"`
	ChannelPrice       *float64 `json:"channel_price"`
	Decision           string   `json:"decision"`
	DecisionReason     string   `json:"decision_reason"`
	SchedulerMode      string   `json:"scheduler_mode"`
	ShadowMode         bool     `json:"shadow_mode"`
	StickyEscapeReason *string  `json:"sticky_escape_reason"`
	SnapshotAgeMS      *int64   `json:"snapshot_age_ms"`
	CooldownUntil      *string  `json:"cooldown_until"`
}

func validateOpenAIAutoSchedulerSettings(settings service.OpenAIAutoSchedulerSettings) string {
	switch {
	case !service.IsSupportedOpenAISchedulerMode(settings.Mode):
		return "mode must be legacy, balanced, performance_first, cost_first, or efficiency"
	case settings.TopK < 1 || settings.TopK > 10:
		return "top_k must be between 1 and 10"
	case settings.ExplorationRate < 0 || settings.ExplorationRate > 0.10:
		return "exploration_rate must be between 0 and 0.10"
	case settings.SessionEscapeMinGapMS < 0 || settings.SessionEscapeMinGapMS > 30000:
		return "session_escape_min_gap_ms must be between 0 and 30000"
	case settings.SessionEscapeRatio < 0 || settings.SessionEscapeRatio > 2:
		return "session_escape_ratio must be between 0 and 2"
	case settings.HealthTTLSeconds < 60 || settings.HealthTTLSeconds > 86400:
		return "health_ttl_seconds must be between 60 and 86400"
	case settings.RealSampleFreshSeconds < 30 || settings.RealSampleFreshSeconds > 3600:
		return "real_sample_fresh_seconds must be between 30 and 3600"
	case settings.ProbeIntervalSeconds <= 0:
		return "probe_interval_seconds must be > 0"
	case settings.ProbeJitterSeconds < 0 || settings.ProbeJitterSeconds > settings.ProbeIntervalSeconds/2:
		return "probe_jitter_seconds must be between 0 and half probe_interval_seconds"
	case settings.SlowThresholdMS <= 0:
		return "slow_threshold_ms must be > 0"
	case settings.SevereSlowThresholdMS < settings.SlowThresholdMS:
		return "severe_slow_threshold_ms must be >= slow_threshold_ms"
	case settings.ConsecutiveSlowBreakerThreshold <= 0:
		return "consecutive_slow_breaker_threshold must be > 0"
	case settings.ConsecutiveErrorBreakerThreshold <= 0:
		return "consecutive_error_breaker_threshold must be > 0"
	case settings.CooldownSeconds <= 0:
		return "cooldown_seconds must be > 0"
	case settings.HalfOpenSuccessThreshold <= 0:
		return "half_open_success_threshold must be > 0"
	case settings.CostWeight < 0 || settings.CostWeight > 1:
		return "cost_weight must be between 0 and 1"
	case settings.RecoveryStep <= 0:
		return "recovery_step must be > 0"
	case settings.Temperature <= 0 || settings.Temperature > 1:
		return "temperature must be between 0 (exclusive) and 1"
	case settings.MaxAccountShare <= 0 || settings.MaxAccountShare > 1:
		return "max_account_share must be between 0 (exclusive) and 1"
	case settings.LowConfidenceMaxShare <= 0 || settings.LowConfidenceMaxShare > 1:
		return "low_confidence_max_share must be between 0 (exclusive) and 1"
	case settings.LatencyBudgetMS <= 0 || settings.LatencyBudgetMS > 30000:
		return "latency_budget_ms must be between 1 and 30000"
	case settings.Weights.Latency < 0 || settings.Weights.Latency > 1 ||
		settings.Weights.Reliability < 0 || settings.Weights.Reliability > 1 ||
		settings.Weights.Cost < 0 || settings.Weights.Cost > 1 ||
		settings.Weights.Capacity < 0 || settings.Weights.Capacity > 1 ||
		settings.Weights.Quota < 0 || settings.Weights.Quota > 1 ||
		settings.Weights.Priority < 0 || settings.Weights.Priority > 1:
		return "each policy weight must be between 0 and 1"
	case settings.Weights.Latency+settings.Weights.Reliability+settings.Weights.Cost+
		settings.Weights.Capacity+settings.Weights.Quota+settings.Weights.Priority <= 0:
		return "policy weights must have a positive total"
	default:
		return ""
	}
}

func parseOpenAISchedulerRankingWindow(c *gin.Context) (time.Duration, bool) {
	raw := strings.ToLower(strings.TrimSpace(c.DefaultQuery("window", "1h")))
	windows := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"1h":  time.Hour,
		"6h":  6 * time.Hour,
		"24h": 24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
	}
	window, ok := windows[raw]
	if !ok {
		response.BadRequest(c, "window must be one of 15m, 1h, 6h, 24h, 7d")
		return 0, false
	}
	return window, true
}

func parseOpenAIAutoSchedulerPagination(c *gin.Context) (int, int) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > openAIAutoSchedulerMaxPageSize {
		pageSize = openAIAutoSchedulerMaxPageSize
	}
	return page, pageSize
}

func parseOpenAIAutoSchedulerListParams(c *gin.Context, page, pageSize int) (service.OpenAIAutoSchedulerListParams, bool) {
	accountID, ok := parseOptionalPositiveInt64Query(c, "account_id")
	if !ok {
		return service.OpenAIAutoSchedulerListParams{}, false
	}
	groupID, ok := parseOptionalPositiveInt64Query(c, "group_id")
	if !ok {
		return service.OpenAIAutoSchedulerListParams{}, false
	}
	return service.OpenAIAutoSchedulerListParams{
		AccountID: accountID,
		GroupID:   groupID,
		Model:     strings.TrimSpace(c.Query("model")),
		Page:      page,
		PageSize:  pageSize,
	}, true
}

func parseOpenAISchedulerOverviewWindow(c *gin.Context) (time.Duration, bool) {
	switch strings.ToLower(strings.TrimSpace(c.Query("window"))) {
	case "", "6h":
		return 6 * time.Hour, true
	case "1h":
		return time.Hour, true
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	default:
		response.BadRequest(c, "window must be one of 1h, 6h, 24h, 7d")
		return 0, false
	}
}

func parseOpenAISchedulerHealthParams(c *gin.Context, page, pageSize int) (service.OpenAISchedulerHealthParams, bool) {
	groupID, ok := parseOptionalPositiveInt64Query(c, "group_id")
	if !ok {
		return service.OpenAISchedulerHealthParams{}, false
	}
	sortField := strings.ToLower(strings.TrimSpace(c.Query("sort")))
	if sortField == "" {
		sortField = "predicted_ttft_ms"
	}
	validSort := map[string]struct{}{
		"account_id": {}, "predicted_ttft_ms": {}, "error_rate": {}, "real_sample_count": {},
		"probe_sample_count": {}, "snapshot_age_ms": {}, "channel_price": {},
	}
	if _, valid := validSort[sortField]; !valid {
		response.BadRequest(c, "invalid health sort field")
		return service.OpenAISchedulerHealthParams{}, false
	}
	order := strings.ToLower(strings.TrimSpace(c.Query("order")))
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		response.BadRequest(c, "order must be asc or desc")
		return service.OpenAISchedulerHealthParams{}, false
	}
	return service.OpenAISchedulerHealthParams{
		GroupID: groupID, State: strings.ToLower(strings.TrimSpace(c.Query("state"))),
		ModelFamily: strings.ToLower(strings.TrimSpace(c.Query("model_family"))),
		Endpoint:    strings.ToLower(strings.TrimSpace(c.Query("endpoint"))),
		Transport:   strings.ToLower(strings.TrimSpace(c.Query("transport"))),
		Sort:        sortField, Order: order, Page: page, PageSize: pageSize,
	}, true
}

func parseScoreMutationQuery(c *gin.Context) (int64, string, bool) {
	groupID, ok := parseRequiredPositiveInt64Query(c, "group_id")
	if !ok {
		return 0, "", false
	}
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		response.BadRequest(c, "model is required")
		return 0, "", false
	}
	return groupID, model, true
}

func parsePositiveInt64Param(c *gin.Context, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, message)
		return 0, false
	}
	return id, true
}

func parseRequiredPositiveInt64Query(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Query(name)), 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, name+" must be > 0")
		return 0, false
	}
	return value, true
}

func parseOptionalPositiveInt64Query(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, name+" must be > 0")
		return 0, false
	}
	return value, true
}

func openAIAutoSchedulerGroupToResponse(group service.Group) openAIAutoSchedulerGroupResponse {
	return openAIAutoSchedulerGroupResponse{
		ID:      group.ID,
		Name:    group.Name,
		Status:  group.Status,
		Enabled: group.OpenAIAutoSchedulerEnabled,
	}
}

func openAIAutoSchedulerScoreToResponse(state service.OpenAIAutoSchedulerScoreState) openAIAutoSchedulerScoreResponse {
	return openAIAutoSchedulerScoreResponse{
		AccountID:               state.AccountID,
		AccountName:             state.AccountName,
		ChannelPrice:            state.ChannelPrice,
		GroupID:                 state.GroupID,
		Model:                   state.Model,
		BaseScore:               state.BaseScore,
		BaseScorePercent:        openAIAutoSchedulerScorePercent(state.BaseScore),
		FinalScore:              state.FinalScore,
		FinalScorePercent:       openAIAutoSchedulerScorePercent(state.FinalScore),
		LatencyScore:            state.LatencyScore,
		LatencyScorePercent:     openAIAutoSchedulerScorePercent(state.LatencyScore),
		ErrorScore:              state.ErrorScore,
		ErrorScorePercent:       openAIAutoSchedulerScorePercent(state.ErrorScore),
		RecoveryScore:           state.RecoveryScore,
		RecoveryScorePercent:    openAIAutoSchedulerScorePercent(state.RecoveryScore),
		CostScore:               state.CostScore,
		CostScorePercent:        openAIAutoSchedulerScorePercent(state.CostScore),
		State:                   state.State,
		ConsecutiveSlowCount:    state.ConsecutiveSlowCount,
		ConsecutiveErrorCount:   state.ConsecutiveErrorCount,
		ConsecutiveSuccessCount: state.ConsecutiveSuccessCount,
		RequestCount:            state.RequestCount,
		TtfbSampleCount:         state.TtfbSampleCount,
		SlowRate:                state.SlowRate,
		ErrorRate:               state.ErrorRate,
		StuckRate:               state.StuckRate,
		CooldownUntil:           openAIAutoSchedulerTimePtr(state.CooldownUntil),
		LastLatencyMS:           state.LastLatencyMS,
		LastTtfbMS:              state.LastTtfbMS,
		LastStatusCode:          state.LastStatusCode,
		LastError:               state.LastError,
		Reason:                  state.Reason,
		LastCheckedAt:           openAIAutoSchedulerTimePtr(state.LastCheckedAt),
	}
}

func openAIAutoSchedulerEventToResponse(event service.OpenAIAutoSchedulerScoreEvent) openAIAutoSchedulerEventResponse {
	return openAIAutoSchedulerEventResponse{
		AccountID:          event.AccountID,
		GroupID:            event.GroupID,
		Model:              event.Model,
		EventType:          event.EventType,
		ScoreBefore:        event.ScoreBefore,
		ScoreBeforePercent: openAIAutoSchedulerScorePercent(event.ScoreBefore),
		ScoreAfter:         event.ScoreAfter,
		ScoreAfterPercent:  openAIAutoSchedulerScorePercent(event.ScoreAfter),
		LatencyMS:          event.LatencyMS,
		TtfbMS:             event.TtfbMS,
		StatusCode:         event.StatusCode,
		Message:            event.Message,
		CreatedAt:          event.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func openAISchedulerOverviewToResponse(metrics service.OpenAISchedulerOverviewMetrics) openAISchedulerOverviewResponse {
	groups := make([]openAISchedulerGroupResponse, 0, len(metrics.Groups))
	for _, group := range metrics.Groups {
		groups = append(groups, openAISchedulerGroupResponse{
			ID: group.ID, Name: group.Name, Enabled: group.Enabled, AccountCount: group.AccountCount,
			E2ETTFTP90MS: positiveOpenAISchedulerFloatPtr(group.E2EP90MS), AlertLevel: group.AlertLevel,
		})
	}
	trend := make([]openAISchedulerTrendResponse, 0, len(metrics.Trend))
	for _, point := range metrics.Trend {
		trend = append(trend, openAISchedulerTrendResponse{
			Bucket: point.Bucket.UTC().Format(time.RFC3339), E2ETTFTP50MS: positiveOpenAISchedulerFloatPtr(point.E2EP50MS),
			E2ETTFTP90MS: positiveOpenAISchedulerFloatPtr(point.E2EP90MS),
		})
	}
	slowCauses := make([]openAISchedulerSlowCauseResponse, 0, len(metrics.SlowCauses))
	for _, cause := range metrics.SlowCauses {
		slowCauses = append(slowCauses, openAISchedulerSlowCauseResponse{Reason: cause.Reason, Count: cause.Count, Ratio: cause.Ratio})
	}
	return openAISchedulerOverviewResponse{
		E2ETTFTP50MS: positiveOpenAISchedulerFloatPtr(metrics.E2EP50MS), E2ETTFTP90MS: positiveOpenAISchedulerFloatPtr(metrics.E2EP90MS),
		SelectionP95MS: positiveOpenAISchedulerFloatPtr(metrics.SelectionP95MS), ProbeRatio: metrics.ProbeRatio,
		Groups: groups, Trend: trend, SlowCauses: slowCauses,
	}
}

func openAISchedulerHealthToResponse(row service.OpenAISchedulerHealthRow) openAISchedulerHealthResponse {
	return openAISchedulerHealthResponse{
		AccountID: row.AccountID, AccountName: row.AccountName, GroupID: row.GroupID,
		ModelFamily: row.ModelFamily, Endpoint: row.Endpoint, Transport: row.Transport, State: row.State,
		PredictedTTFTMS: positiveOpenAISchedulerFloatPtr(row.PredictedTTFTMS), RealSampleCount: row.RealSampleCount,
		ProbeSampleCount: row.ProbeSampleCount, ErrorRate: row.ErrorRate, RateLimitedRate: row.RateLimitedRate,
		ServerErrorRate: row.ServerErrorRate, LoadInflight: row.LoadInflight, LoadCapacity: row.LoadCapacity,
		WaitingCount: row.WaitingCount, ChannelPrice: row.ChannelPrice, Decision: row.Decision,
		DecisionReason: row.DecisionReason, SchedulerMode: row.SchedulerMode, ShadowMode: row.ShadowMode,
		StickyEscapeReason: row.StickyEscapeReason, SnapshotAgeMS: row.SnapshotAgeMS,
		CooldownUntil: openAIAutoSchedulerTimePtr(row.CooldownUntil),
	}
}

func positiveOpenAISchedulerFloatPtr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func openAIAutoSchedulerScorePercent(score int) float64 {
	return float64(score) / 100
}

func openAIAutoSchedulerTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}
