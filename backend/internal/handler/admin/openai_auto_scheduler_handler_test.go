package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeOpenAIAutoSchedulerSettingsService struct {
	settings service.OpenAIAutoSchedulerSettings
	updated  bool
}

func (s *fakeOpenAIAutoSchedulerSettingsService) GetOpenAIAutoSchedulerSettings(context.Context) service.OpenAIAutoSchedulerSettings {
	return s.settings
}

func (s *fakeOpenAIAutoSchedulerSettingsService) SetOpenAIAutoSchedulerSettings(_ context.Context, settings service.OpenAIAutoSchedulerSettings) error {
	s.updated = true
	s.settings = settings
	return nil
}

type fakeOpenAIAutoSchedulerAdminService struct {
	groups       []service.Group
	updatedID    int64
	updatedInput *service.UpdateGroupInput
}

func (s *fakeOpenAIAutoSchedulerAdminService) GetAllGroupsByPlatform(_ context.Context, platform string) ([]service.Group, error) {
	var groups []service.Group
	for _, group := range s.groups {
		if group.Platform == platform {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (s *fakeOpenAIAutoSchedulerAdminService) GetGroup(_ context.Context, id int64) (*service.Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			group := s.groups[i]
			return &group, nil
		}
	}
	return nil, errors.New("group not found")
}

func (s *fakeOpenAIAutoSchedulerAdminService) UpdateGroup(_ context.Context, id int64, input *service.UpdateGroupInput) (*service.Group, error) {
	s.updatedID = id
	s.updatedInput = input
	for i := range s.groups {
		if s.groups[i].ID == id {
			s.groups[i].OpenAIAutoSchedulerEnabled = *input.OpenAIAutoSchedulerEnabled
			group := s.groups[i]
			return &group, nil
		}
	}
	return nil, errors.New("group not found")
}

type fakeOpenAIAutoSchedulerService struct {
	scores       []service.OpenAIAutoSchedulerScoreState
	events       []service.OpenAIAutoSchedulerScoreEvent
	total        int64
	listParams   service.OpenAIAutoSchedulerListParams
	resetAccount int64
	resetGroup   int64
	resetModel   string
	recordInput  service.OpenAIAutoSchedulerRecordInput
	recordErr    error
}

func (s *fakeOpenAIAutoSchedulerService) ListScores(_ context.Context, params service.OpenAIAutoSchedulerListParams) (*service.OpenAIAutoSchedulerScoreListResult, error) {
	s.listParams = params
	return &service.OpenAIAutoSchedulerScoreListResult{Items: s.scores, Total: s.total}, nil
}

func (s *fakeOpenAIAutoSchedulerService) ListEvents(_ context.Context, params service.OpenAIAutoSchedulerListParams) (*service.OpenAIAutoSchedulerEventListResult, error) {
	s.listParams = params
	return &service.OpenAIAutoSchedulerEventListResult{Items: s.events, Total: s.total}, nil
}

func (s *fakeOpenAIAutoSchedulerService) ResetScore(_ context.Context, accountID, groupID int64, model string) error {
	s.resetAccount = accountID
	s.resetGroup = groupID
	s.resetModel = model
	return nil
}

func (s *fakeOpenAIAutoSchedulerService) Record(_ context.Context, input service.OpenAIAutoSchedulerRecordInput) error {
	return nil
}

func (s *fakeOpenAIAutoSchedulerService) RecordManualProbe(_ context.Context, input service.OpenAIAutoSchedulerRecordInput) error {
	s.recordInput = input
	return s.recordErr
}

type fakeOpenAIAutoSchedulerAccountRepo struct {
	account *service.Account
	err     error
}

func (r *fakeOpenAIAutoSchedulerAccountRepo) GetByID(context.Context, int64) (*service.Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.account, nil
}

type fakeOpenAIAutoSchedulerProbeChecker struct {
	result service.OpenAIAutoSchedulerProbeResult
	calls  *int
}

type fakeOpenAISchedulerOverviewService struct {
	overview       service.OpenAISchedulerOverviewMetrics
	health         *service.OpenAISchedulerHealthListResult
	rankings       *service.OpenAISchedulerRankingResult
	err            error
	overviewParams service.OpenAISchedulerOverviewParams
	healthParams   service.OpenAISchedulerHealthParams
	rankingParams  service.OpenAISchedulerRankingParams
	overviewCalls  int
	healthCalls    int
}

func (s *fakeOpenAISchedulerOverviewService) ListRankings(_ context.Context, params service.OpenAISchedulerRankingParams) (*service.OpenAISchedulerRankingResult, error) {
	s.rankingParams = params
	if s.rankings == nil {
		s.rankings = &service.OpenAISchedulerRankingResult{Items: []service.OpenAISchedulerRankingItem{}}
	}
	return s.rankings, s.err
}

func (s *fakeOpenAISchedulerOverviewService) GetOverview(_ context.Context, params service.OpenAISchedulerOverviewParams) (service.OpenAISchedulerOverviewMetrics, error) {
	s.overviewCalls++
	s.overviewParams = params
	return s.overview, s.err
}

func (s *fakeOpenAISchedulerOverviewService) ListHealth(_ context.Context, params service.OpenAISchedulerHealthParams) (*service.OpenAISchedulerHealthListResult, error) {
	s.healthCalls++
	s.healthParams = params
	if s.health == nil {
		s.health = &service.OpenAISchedulerHealthListResult{}
	}
	return s.health, s.err
}

func (c fakeOpenAIAutoSchedulerProbeChecker) Check(context.Context, *service.Account, string, time.Duration) service.OpenAIAutoSchedulerProbeResult {
	if c.calls != nil {
		*c.calls++
	}
	return c.result
}

func setupOpenAIAutoSchedulerHandlerRouter(
	settingsSvc *fakeOpenAIAutoSchedulerSettingsService,
	adminSvc *fakeOpenAIAutoSchedulerAdminService,
	schedulerSvc *fakeOpenAIAutoSchedulerService,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewOpenAIAutoSchedulerHandler(settingsSvc, adminSvc, schedulerSvc, nil, nil)
	group := router.Group("/api/v1/admin/openai-auto-scheduler")
	{
		group.GET("/settings", h.GetSettings)
		group.PUT("/settings", h.UpdateSettings)
		group.GET("/groups", h.ListGroups)
		group.PUT("/groups/:id", h.UpdateGroup)
		group.GET("/scores", h.ListScores)
		group.GET("/events", h.ListEvents)
		group.POST("/scores/accounts/:account_id/reset", h.ResetScore)
	}
	return router
}

func setupOpenAISchedulerControlRouter(svc *fakeOpenAISchedulerOverviewService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewOpenAIAutoSchedulerHandler(nil, nil, nil, nil, nil, svc)
	group := router.Group("/api/v1/admin/openai-auto-scheduler")
	group.GET("/overview", h.GetOverview)
	group.GET("/rankings", h.ListRankings)
	group.GET("/health", h.ListHealth)
	return router
}

func TestOpenAIAutoSchedulerHandler_GetOverviewValidatesWindowAndMapsResponse(t *testing.T) {
	t.Run("defaults to six hours", func(t *testing.T) {
		svc := &fakeOpenAISchedulerOverviewService{overview: service.OpenAISchedulerOverviewMetrics{
			E2EP50MS: 2970, E2EP90MS: 7210, SelectionP95MS: 18, ProbeRatio: 0.24,
			SlowCauses: []service.OpenAISchedulerSlowCause{{Reason: "queue", Count: 2, Ratio: 0.5}},
			Runtime:    service.OpenAISchedulerRuntimeMetrics{ExplorationAllowedTotal: 8, UnifiedHealthFallbacksTotal: 2},
		}}
		router := setupOpenAISchedulerControlRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/overview?group_id=33", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 6*time.Hour, svc.overviewParams.Window)
		require.Equal(t, int64(33), svc.overviewParams.GroupID)
		require.Contains(t, rec.Body.String(), `"e2e_ttft_p50_ms":2970`)
		require.Contains(t, rec.Body.String(), `"selection_p95_ms":18`)
		require.Contains(t, rec.Body.String(), `"probe_ratio":0.24`)
		require.Contains(t, rec.Body.String(), `"reason":"queue"`)
		require.Contains(t, rec.Body.String(), `"exploration_allowed_total":8`)
		require.Contains(t, rec.Body.String(), `"unified_health_fallbacks_total":2`)
	})

	t.Run("rejects unsupported window", func(t *testing.T) {
		svc := &fakeOpenAISchedulerOverviewService{}
		router := setupOpenAISchedulerControlRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/overview?window=2h", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "window must be one of 1h, 6h, 24h, 7d")
		require.Zero(t, svc.overviewCalls)
	})
}

func TestOpenAIAutoSchedulerHandler_ListHealthParsesBoundedFiltersAndPagination(t *testing.T) {
	age := int64(2500)
	svc := &fakeOpenAISchedulerOverviewService{health: &service.OpenAISchedulerHealthListResult{
		Total: 1,
		Items: []service.OpenAISchedulerHealthRow{{
			AccountID: 10, AccountName: "primary", GroupID: 33, ModelFamily: "gpt-5.4",
			Endpoint: "responses", Transport: "http_sse", State: "running", Decision: "context_required",
			DecisionReason: "request_context_required", SchedulerMode: "balanced", ShadowMode: true,
			PredictedTTFTMS: 1200, LoadInflight: 2, LoadCapacity: 4, WaitingCount: 1, SnapshotAgeMS: &age,
		}},
	}}
	router := setupOpenAISchedulerControlRouter(svc)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/health?group_id=33&state=running&model_family=gpt-5.4&endpoint=responses&transport=http_sse&sort=predicted_ttft_ms&order=asc&page=2&page_size=999", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(33), svc.healthParams.GroupID)
	require.Equal(t, "running", svc.healthParams.State)
	require.Equal(t, "gpt-5.4", svc.healthParams.ModelFamily)
	require.Equal(t, "responses", svc.healthParams.Endpoint)
	require.Equal(t, "http_sse", svc.healthParams.Transport)
	require.Equal(t, "predicted_ttft_ms", svc.healthParams.Sort)
	require.Equal(t, "asc", svc.healthParams.Order)
	require.Equal(t, 2, svc.healthParams.Page)
	require.Equal(t, 200, svc.healthParams.PageSize)
	require.Contains(t, rec.Body.String(), `"decision":"context_required"`)
	require.Contains(t, rec.Body.String(), `"sticky_escape_reason":null`)
	require.Contains(t, rec.Body.String(), `"page_size":200`)
}

func TestOpenAIAutoSchedulerHandler_ListRankingsRequiresGroupAndParsesFilters(t *testing.T) {
	svc := &fakeOpenAISchedulerOverviewService{rankings: &service.OpenAISchedulerRankingResult{
		PolicyContext: service.OpenAISchedulerPolicyContext{EffectiveMode: "balanced", PolicyVersion: "v2"},
		Items:         []service.OpenAISchedulerRankingItem{},
		Page:          2,
		PageSize:      200,
	}}
	router := setupOpenAISchedulerControlRouter(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/rankings?group_id=33&window=15m&model_family=GPT-5.4&endpoint=RESPONSES&transport=HTTP_SSE&eligibility=ELIGIBLE&page=2&page_size=999", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(33), svc.rankingParams.GroupID)
	require.Equal(t, 15*time.Minute, svc.rankingParams.Window)
	require.Equal(t, "gpt-5.4", svc.rankingParams.ModelFamily)
	require.Equal(t, "responses", svc.rankingParams.Endpoint)
	require.Equal(t, "http_sse", svc.rankingParams.Transport)
	require.Equal(t, "eligible", svc.rankingParams.Eligibility)
	require.Equal(t, 2, svc.rankingParams.Page)
	require.Equal(t, 200, svc.rankingParams.PageSize)
	require.Contains(t, rec.Body.String(), `"effective_mode":"balanced"`)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/rankings?group_id=33&window=2h", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/rankings?group_id=33&eligibility=unknown", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpenAIAutoSchedulerHandler_ListHealthRejectsInvalidFilterAndPropagatesError(t *testing.T) {
	t.Run("invalid sort", func(t *testing.T) {
		svc := &fakeOpenAISchedulerOverviewService{}
		router := setupOpenAISchedulerControlRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/health?sort=drop_table", nil)
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Zero(t, svc.healthCalls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &fakeOpenAISchedulerOverviewService{err: errors.New("query failed")}
		router := setupOpenAISchedulerControlRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/health", nil)
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func setupOpenAIAutoSchedulerProbeRouter(
	schedulerSvc *fakeOpenAIAutoSchedulerService,
	accountRepo *fakeOpenAIAutoSchedulerAccountRepo,
	checker service.OpenAIAutoSchedulerProbeChecker,
	settingsOverride ...service.OpenAIAutoSchedulerSettings,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	settings := service.DefaultOpenAIAutoSchedulerSettings()
	if len(settingsOverride) > 0 {
		settings = settingsOverride[0]
	}
	h := NewOpenAIAutoSchedulerHandler(&fakeOpenAIAutoSchedulerSettingsService{settings: settings}, nil, schedulerSvc, accountRepo, checker)
	group := router.Group("/api/v1/admin/openai-auto-scheduler")
	{
		group.POST("/scores/accounts/:account_id/probe", h.ProbeScore)
	}
	return router
}

func TestOpenAIAutoSchedulerHandler_GetSettingsReturnsCurrentSettings(t *testing.T) {
	settings := service.DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ProbeModel = "gpt-5.5"
	settings.ProbeIntervalSeconds = 90
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{settings: settings}, &fakeOpenAIAutoSchedulerAdminService{}, &fakeOpenAIAutoSchedulerService{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/settings", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"enabled":true`)
	require.Contains(t, rec.Body.String(), `"probe_model":"gpt-5.5"`)
	require.Contains(t, rec.Body.String(), `"probe_interval_seconds":90`)
}

func TestOpenAIAutoSchedulerHandler_UpdateSettingsRejectsInvalidThresholds(t *testing.T) {
	settingsSvc := &fakeOpenAIAutoSchedulerSettingsService{settings: service.DefaultOpenAIAutoSchedulerSettings()}
	router := setupOpenAIAutoSchedulerHandlerRouter(settingsSvc, &fakeOpenAIAutoSchedulerAdminService{}, &fakeOpenAIAutoSchedulerService{})

	body := `{
		"enabled": true,
		"probe_interval_seconds": 0,
		"slow_threshold_ms": 10000,
		"severe_slow_threshold_ms": 5000,
		"consecutive_slow_breaker_threshold": 3,
		"consecutive_error_breaker_threshold": 2,
		"cooldown_seconds": 120,
		"half_open_success_threshold": 3,
		"cost_weight": 0.2,
		"recovery_step": 800
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/openai-auto-scheduler/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, settingsSvc.updated)
	require.Contains(t, rec.Body.String(), "probe_interval_seconds")
}

func TestValidateOpenAIAutoSchedulerSettings_BalancedRanges(t *testing.T) {
	valid := service.DefaultOpenAIAutoSchedulerSettings()
	tests := []struct {
		name   string
		mutate func(*service.OpenAIAutoSchedulerSettings)
	}{
		{name: "mode", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.Mode = "random" }},
		{name: "top k below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.TopK = 0 }},
		{name: "top k above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.TopK = 11 }},
		{name: "exploration below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationRate = -0.01 }},
		{name: "exploration above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationRate = 0.11 }},
		{name: "exploration budget below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationBudget = -0.01 }},
		{name: "exploration budget above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationBudget = 0.11 }},
		{name: "exploration interval below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMinIntervalSeconds = 29 }},
		{name: "exploration interval above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMinIntervalSeconds = 3601 }},
		{name: "exploration max per hour below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMaxRealSamplesPerHour = 0 }},
		{name: "exploration max per hour above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMaxRealSamplesPerHour = 61 }},
		{name: "escape gap below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeMinGapMS = -1 }},
		{name: "escape gap above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeMinGapMS = 30001 }},
		{name: "escape ratio below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeRatio = -0.01 }},
		{name: "escape ratio above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeRatio = 2.01 }},
		{name: "health ttl below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.HealthTTLSeconds = 59 }},
		{name: "health ttl above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.HealthTTLSeconds = 86401 }},
		{name: "real freshness below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.RealSampleFreshSeconds = 29 }},
		{name: "real freshness above", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.RealSampleFreshSeconds = 3601 }},
		{name: "probe jitter below", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ProbeJitterSeconds = -1 }},
		{name: "probe jitter above half interval", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.ProbeJitterSeconds = 31 }},
		{name: "temperature", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.Temperature = 0 }},
		{name: "max account share", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.MaxAccountShare = 1.01 }},
		{name: "low confidence share", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.LowConfidenceMaxShare = 0 }},
		{name: "latency budget", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.LatencyBudgetMS = 30001 }},
		{name: "negative weight", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.Weights.Latency = -0.01 }},
		{name: "zero weights", mutate: func(s *service.OpenAIAutoSchedulerSettings) { s.Weights = service.OpenAISchedulerPolicyWeights{} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := valid
			tt.mutate(&settings)
			require.NotEmpty(t, validateOpenAIAutoSchedulerSettings(settings))
		})
	}

	for _, mutate := range []func(*service.OpenAIAutoSchedulerSettings){
		func(s *service.OpenAIAutoSchedulerSettings) { s.Mode = "legacy" },
		func(s *service.OpenAIAutoSchedulerSettings) { s.TopK = 1 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.TopK = 10 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationRate = 0 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationRate = 0.10 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationBudget = 0 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationBudget = 0.10 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMinIntervalSeconds = 30 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMinIntervalSeconds = 3600 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMaxRealSamplesPerHour = 1 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ExplorationMaxRealSamplesPerHour = 60 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeMinGapMS = 0 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeMinGapMS = 30000 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeRatio = 0 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.SessionEscapeRatio = 2 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.HealthTTLSeconds = 60 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.HealthTTLSeconds = 86400 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.RealSampleFreshSeconds = 30 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.RealSampleFreshSeconds = 3600 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.ProbeJitterSeconds = 30 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.Mode = service.OpenAIAutoSchedulerModePerformance },
		func(s *service.OpenAIAutoSchedulerSettings) { s.Mode = service.OpenAIAutoSchedulerModeCost },
		func(s *service.OpenAIAutoSchedulerSettings) { s.Mode = service.OpenAIAutoSchedulerModeEfficiency },
		func(s *service.OpenAIAutoSchedulerSettings) { s.Temperature = 1 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.MaxAccountShare = 1 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.LowConfidenceMaxShare = 1 },
		func(s *service.OpenAIAutoSchedulerSettings) { s.LatencyBudgetMS = 30000 },
	} {
		settings := valid
		mutate(&settings)
		require.Empty(t, validateOpenAIAutoSchedulerSettings(settings))
	}
}

func TestOpenAIAutoSchedulerHandler_UpdateSettingsOldPayloadKeepsShadowDefault(t *testing.T) {
	settingsSvc := &fakeOpenAIAutoSchedulerSettingsService{}
	router := setupOpenAIAutoSchedulerHandlerRouter(settingsSvc, &fakeOpenAIAutoSchedulerAdminService{}, &fakeOpenAIAutoSchedulerService{})
	body := `{
		"enabled": true,
		"probe_interval_seconds": 60,
		"slow_threshold_ms": 10000,
		"severe_slow_threshold_ms": 20000,
		"consecutive_slow_breaker_threshold": 3,
		"consecutive_error_breaker_threshold": 2,
		"cooldown_seconds": 120,
		"half_open_success_threshold": 3,
		"cost_weight": 0.2,
		"recovery_step": 800
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/openai-auto-scheduler/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, settingsSvc.updated)
	require.Equal(t, "balanced", settingsSvc.settings.Mode)
	require.True(t, settingsSvc.settings.ShadowMode)
	require.Equal(t, 3, settingsSvc.settings.TopK)
}

func TestOpenAIAutoSchedulerHandler_UpdateGroupRejectsNonOpenAIGroup(t *testing.T) {
	adminSvc := &fakeOpenAIAutoSchedulerAdminService{groups: []service.Group{
		{ID: 10, Name: "anthropic", Platform: service.PlatformAnthropic, Status: service.StatusActive},
	}}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, adminSvc, &fakeOpenAIAutoSchedulerService{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/openai-auto-scheduler/groups/10", bytes.NewBufferString(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, adminSvc.updatedInput)
	require.Contains(t, rec.Body.String(), "OpenAI")
}

func TestOpenAIAutoSchedulerHandler_ListGroupsReturnsOnlyOpenAIGroups(t *testing.T) {
	adminSvc := &fakeOpenAIAutoSchedulerAdminService{groups: []service.Group{
		{ID: 20, Name: "openai", Platform: service.PlatformOpenAI, Status: service.StatusActive, OpenAIAutoSchedulerEnabled: true},
		{ID: 30, Name: "anthropic", Platform: service.PlatformAnthropic, Status: service.StatusActive, OpenAIAutoSchedulerEnabled: true},
	}}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, adminSvc, &fakeOpenAIAutoSchedulerService{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/groups", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"openai"`)
	require.NotContains(t, rec.Body.String(), "anthropic")
}

func TestOpenAIAutoSchedulerHandler_UpdateGroupPersistsOpenAIGroupToggle(t *testing.T) {
	adminSvc := &fakeOpenAIAutoSchedulerAdminService{groups: []service.Group{
		{ID: 20, Name: "openai", Platform: service.PlatformOpenAI, Status: service.StatusActive, OpenAIAutoSchedulerEnabled: false},
	}}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, adminSvc, &fakeOpenAIAutoSchedulerService{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/openai-auto-scheduler/groups/20", bytes.NewBufferString(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(20), adminSvc.updatedID)
	require.NotNil(t, adminSvc.updatedInput)
	require.NotNil(t, adminSvc.updatedInput.OpenAIAutoSchedulerEnabled)
	require.True(t, *adminSvc.updatedInput.OpenAIAutoSchedulerEnabled)
}

func TestOpenAIAutoSchedulerHandler_ListScoresReturnsPaginatedRows(t *testing.T) {
	schedulerSvc := &fakeOpenAIAutoSchedulerService{
		total: 2,
		scores: []service.OpenAIAutoSchedulerScoreState{
			{AccountID: 101, AccountName: "plus特惠临时分组渠道", GroupID: 20, Model: "gpt-5.4", FinalScore: 8750, State: service.OpenAIAutoSchedulerStateRunning},
			{AccountID: 102, AccountName: "备用渠道", GroupID: 20, Model: "gpt-5.4", FinalScore: 6400, State: service.OpenAIAutoSchedulerStateOpen},
		},
	}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, &fakeOpenAIAutoSchedulerAdminService{}, schedulerSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/scores?page=2&page_size=1&group_id=20&model=gpt-5.4", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.OpenAIAutoSchedulerListParams{GroupID: 20, Model: "gpt-5.4", Page: 2, PageSize: 1}, schedulerSvc.listParams)

	var resp struct {
		Data struct {
			Items []struct {
				AccountID         int64   `json:"account_id"`
				AccountName       string  `json:"account_name"`
				GroupID           int64   `json:"group_id"`
				Model             string  `json:"model"`
				FinalScore        int     `json:"final_score"`
				FinalScorePercent float64 `json:"final_score_percent"`
				State             string  `json:"state"`
			} `json:"items"`
			Total int64 `json:"total"`
			Page  int   `json:"page"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(2), resp.Data.Total)
	require.Equal(t, 2, resp.Data.Page)
	require.Len(t, resp.Data.Items, 2)
	require.Equal(t, int64(101), resp.Data.Items[0].AccountID)
	require.Equal(t, "plus特惠临时分组渠道", resp.Data.Items[0].AccountName)
	require.Equal(t, 8750, resp.Data.Items[0].FinalScore)
	require.Equal(t, 87.5, resp.Data.Items[0].FinalScorePercent)
}

func TestOpenAIAutoSchedulerHandler_ListEventsReturnsPaginatedRows(t *testing.T) {
	latency := 123
	schedulerSvc := &fakeOpenAIAutoSchedulerService{
		total: 1,
		events: []service.OpenAIAutoSchedulerScoreEvent{
			{AccountID: 101, GroupID: 20, Model: "gpt-5", EventType: service.OpenAIAutoSchedulerEventProbeSuccess, ScoreBefore: 6000, ScoreAfter: 6800, LatencyMS: &latency, Message: "ok", CreatedAt: time.Date(2026, 6, 28, 1, 2, 3, 0, time.UTC)},
		},
	}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, &fakeOpenAIAutoSchedulerAdminService{}, schedulerSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/events?page=2&page_size=1&account_id=101&group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.OpenAIAutoSchedulerListParams{AccountID: 101, GroupID: 20, Model: "gpt-5", Page: 2, PageSize: 1}, schedulerSvc.listParams)
	require.Contains(t, rec.Body.String(), `"event_type":"probe_success"`)
	require.Contains(t, rec.Body.String(), `"created_at":"2026-06-28T01:02:03Z"`)
}

func TestOpenAIAutoSchedulerHandler_ResetInvokesService(t *testing.T) {
	schedulerSvc := &fakeOpenAIAutoSchedulerService{}
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, &fakeOpenAIAutoSchedulerAdminService{}, schedulerSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai-auto-scheduler/scores/accounts/101/reset?group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(101), schedulerSvc.resetAccount)
	require.Equal(t, int64(20), schedulerSvc.resetGroup)
	require.Equal(t, "gpt-5", schedulerSvc.resetModel)
}

func TestOpenAIAutoSchedulerHandler_ResetRejectsMissingMutationQueryParams(t *testing.T) {
	router := setupOpenAIAutoSchedulerHandlerRouter(&fakeOpenAIAutoSchedulerSettingsService{}, &fakeOpenAIAutoSchedulerAdminService{}, &fakeOpenAIAutoSchedulerService{})

	for _, path := range []string{
		"/api/v1/admin/openai-auto-scheduler/scores/accounts/101/reset?model=gpt-5",
		"/api/v1/admin/openai-auto-scheduler/scores/accounts/101/reset?group_id=20",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestOpenAIAutoSchedulerHandler_ProbeRecordsSuccess(t *testing.T) {
	latency := 88
	ttfb := 32
	schedulerSvc := &fakeOpenAIAutoSchedulerService{}
	router := setupOpenAIAutoSchedulerProbeRouter(
		schedulerSvc,
		&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformOpenAI}},
		fakeOpenAIAutoSchedulerProbeChecker{result: service.OpenAIAutoSchedulerProbeResult{Success: true, LatencyMS: &latency, TtfbMS: &ttfb, Message: "ok"}},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(101), schedulerSvc.recordInput.AccountID)
	require.Equal(t, int64(20), schedulerSvc.recordInput.GroupID)
	require.Equal(t, "gpt-5", schedulerSvc.recordInput.Model)
	require.Equal(t, service.OpenAIAutoSchedulerEventProbeSuccess, schedulerSvc.recordInput.EventType)
	require.Equal(t, &ttfb, schedulerSvc.recordInput.TtfbMS)
	require.Contains(t, rec.Body.String(), `"success":true`)
	require.Contains(t, rec.Body.String(), `"ttfb_ms":32`)
}

func TestOpenAIAutoSchedulerHandler_ProbeClassifiesSlowSuccessUsingEffectiveSettings(t *testing.T) {
	settings := service.DefaultOpenAIAutoSchedulerSettings()
	settings.SlowThresholdMS = 6000
	settings.SevereSlowThresholdMS = 15000

	tests := []struct {
		name string
		ttfb int
		want string
	}{
		{"slow", 7000, service.OpenAIAutoSchedulerEventSlow},
		{"severe slow", 16000, service.OpenAIAutoSchedulerEventSevereSlow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedulerSvc := &fakeOpenAIAutoSchedulerService{}
			router := setupOpenAIAutoSchedulerProbeRouter(
				schedulerSvc,
				&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformOpenAI}},
				fakeOpenAIAutoSchedulerProbeChecker{result: service.OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: &tt.ttfb}},
				settings,
			)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?group_id=20&model=gpt-5", nil)
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.want, schedulerSvc.recordInput.EventType)
			require.Contains(t, rec.Body.String(), `"success":true`)
		})
	}
}

func TestOpenAIAutoSchedulerHandler_ProbeSurfacesRecordError(t *testing.T) {
	schedulerSvc := &fakeOpenAIAutoSchedulerService{recordErr: errors.New("record skipped")}
	router := setupOpenAIAutoSchedulerProbeRouter(
		schedulerSvc,
		&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformOpenAI}},
		fakeOpenAIAutoSchedulerProbeChecker{result: service.OpenAIAutoSchedulerProbeResult{Success: true, Message: "ok"}},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "internal error")
}

func TestOpenAIAutoSchedulerHandler_ProbeChecksSettingsDependencyBeforeRemoteCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	calls := 0
	h := NewOpenAIAutoSchedulerHandler(
		nil,
		nil,
		&fakeOpenAIAutoSchedulerService{},
		&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformOpenAI}},
		fakeOpenAIAutoSchedulerProbeChecker{calls: &calls},
	)
	router := gin.New()
	router.POST("/scores/accounts/:account_id/probe", h.ProbeScore)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scores/accounts/101/probe?group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Zero(t, calls)
}

func TestOpenAIAutoSchedulerHandler_ProbeRejectsMissingMutationQueryParams(t *testing.T) {
	router := setupOpenAIAutoSchedulerProbeRouter(
		&fakeOpenAIAutoSchedulerService{},
		&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformOpenAI}},
		fakeOpenAIAutoSchedulerProbeChecker{},
	)

	for _, path := range []string{
		"/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?model=gpt-5",
		"/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?group_id=20",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestOpenAIAutoSchedulerHandler_ProbeRejectsNonOpenAIAccount(t *testing.T) {
	router := setupOpenAIAutoSchedulerProbeRouter(
		&fakeOpenAIAutoSchedulerService{},
		&fakeOpenAIAutoSchedulerAccountRepo{account: &service.Account{ID: 101, Platform: service.PlatformAnthropic}},
		fakeOpenAIAutoSchedulerProbeChecker{},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai-auto-scheduler/scores/accounts/101/probe?group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "OpenAI")
}
