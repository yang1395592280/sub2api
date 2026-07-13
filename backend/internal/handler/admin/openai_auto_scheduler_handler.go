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

// OpenAIAutoSchedulerHandler exposes admin APIs for OpenAI auto scheduler state.
type OpenAIAutoSchedulerHandler struct {
	settingsSvc openAIAutoSchedulerSettingsService
	adminSvc    openAIAutoSchedulerAdminService
	scheduler   openAIAutoSchedulerService
	accountRepo openAIAutoSchedulerAccountRepository
	checker     service.OpenAIAutoSchedulerProbeChecker
}

func NewOpenAIAutoSchedulerHandler(
	settingsSvc openAIAutoSchedulerSettingsService,
	adminSvc openAIAutoSchedulerAdminService,
	scheduler openAIAutoSchedulerService,
	accountRepo openAIAutoSchedulerAccountRepository,
	checker service.OpenAIAutoSchedulerProbeChecker,
) *OpenAIAutoSchedulerHandler {
	return &OpenAIAutoSchedulerHandler{
		settingsSvc: settingsSvc,
		adminSvc:    adminSvc,
		scheduler:   scheduler,
		accountRepo: accountRepo,
		checker:     checker,
	}
}

func ProvideOpenAIAutoSchedulerHandler(
	settingService *service.SettingService,
	adminService service.AdminService,
	schedulerService *service.OpenAIAutoSchedulerService,
	accountRepo service.AccountRepository,
	checker service.OpenAIAutoSchedulerProbeChecker,
) *OpenAIAutoSchedulerHandler {
	return NewOpenAIAutoSchedulerHandler(settingService, adminService, schedulerService, accountRepo, checker)
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
	var req service.OpenAIAutoSchedulerSettings
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
	if h == nil || h.scheduler == nil || h.accountRepo == nil || h.checker == nil {
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
	if h.settingsSvc == nil {
		response.InternalError(c, "openai auto scheduler settings service is not configured")
		return
	}
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

func validateOpenAIAutoSchedulerSettings(settings service.OpenAIAutoSchedulerSettings) string {
	switch {
	case settings.ProbeIntervalSeconds <= 0:
		return "probe_interval_seconds must be > 0"
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
	default:
		return ""
	}
}

func parseOpenAIAutoSchedulerPagination(c *gin.Context) (int, int) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > openAIAutoSchedulerMaxPageSize {
		pageSize = openAIAutoSchedulerMaxPageSize
	}
	return page, pageSize
}

func parseOpenAIAutoSchedulerListParams(c *gin.Context, page, pageSize int) (service.OpenAIAutoSchedulerListParams, bool) {
	groupID, ok := parseOptionalPositiveInt64Query(c, "group_id")
	if !ok {
		return service.OpenAIAutoSchedulerListParams{}, false
	}
	return service.OpenAIAutoSchedulerListParams{
		GroupID:  groupID,
		Model:    strings.TrimSpace(c.Query("model")),
		Page:     page,
		PageSize: pageSize,
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
