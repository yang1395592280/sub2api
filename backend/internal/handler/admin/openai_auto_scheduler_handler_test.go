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
}

func (c fakeOpenAIAutoSchedulerProbeChecker) Check(context.Context, *service.Account, string, time.Duration) service.OpenAIAutoSchedulerProbeResult {
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai-auto-scheduler/events?page=2&page_size=1&group_id=20&model=gpt-5", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.OpenAIAutoSchedulerListParams{GroupID: 20, Model: "gpt-5", Page: 2, PageSize: 1}, schedulerSvc.listParams)
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
