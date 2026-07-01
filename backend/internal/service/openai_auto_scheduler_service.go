package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type OpenAIAutoSchedulerSettingsProvider interface {
	GetOpenAIAutoSchedulerSettings(ctx context.Context) OpenAIAutoSchedulerSettings
}

type OpenAIAutoSchedulerRepository interface {
	GetGroup(ctx context.Context, groupID int64) (*Group, error)
	GetScoreState(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error)
	HasActiveCooldownScoreState(ctx context.Context, accountID, groupID int64, now time.Time) (bool, error)
	UpsertScoreState(ctx context.Context, state OpenAIAutoSchedulerScoreState) error
	InsertScoreEvent(ctx context.Context, event OpenAIAutoSchedulerScoreEvent) error
	ListScoreStates(ctx context.Context, params OpenAIAutoSchedulerListParams) ([]OpenAIAutoSchedulerScoreState, int64, error)
	ListScoreEvents(ctx context.Context, params OpenAIAutoSchedulerListParams) ([]OpenAIAutoSchedulerScoreEvent, int64, error)
	ListScoreDailySamples(ctx context.Context, params OpenAIAutoSchedulerListParams, since time.Time) (map[int64]OpenAIAutoSchedulerDailySample, error)
	ListSchedulableOpenAIAccountsByGroup(ctx context.Context, groupID int64) ([]Account, error)
	ListEnabledOpenAIGroups(ctx context.Context) ([]Group, error)
}

type OpenAIAutoSchedulerRecordInput struct {
	AccountID  int64
	GroupID    int64
	Model      string
	EventType  string
	LatencyMS  *int
	TtfbMS     *int
	StatusCode *int
	Message    string
	CostScore  *int
}

type OpenAIAutoSchedulerScoreEvent struct {
	AccountID   int64
	GroupID     int64
	Model       string
	EventType   string
	ScoreBefore int
	ScoreAfter  int
	LatencyMS   *int
	TtfbMS      *int
	StatusCode  *int
	Message     string
	CreatedAt   time.Time
}

type OpenAIAutoSchedulerListParams struct {
	GroupID  int64
	Model    string
	Page     int
	PageSize int
}

type OpenAIAutoSchedulerScoreListResult struct {
	Items []OpenAIAutoSchedulerScoreState
	Total int64
}

type OpenAIAutoSchedulerEventListResult struct {
	Items []OpenAIAutoSchedulerScoreEvent
	Total int64
}

type OpenAIAutoSchedulerService struct {
	repo             OpenAIAutoSchedulerRepository
	settingsProvider OpenAIAutoSchedulerSettingsProvider
	mu               sync.Mutex
	keyLocks         map[string]*sync.Mutex
}

func NewOpenAIAutoSchedulerService(repo OpenAIAutoSchedulerRepository, settingsProvider OpenAIAutoSchedulerSettingsProvider) *OpenAIAutoSchedulerService {
	return &OpenAIAutoSchedulerService{
		repo:             repo,
		settingsProvider: settingsProvider,
		keyLocks:         map[string]*sync.Mutex{},
	}
}

func (s *OpenAIAutoSchedulerService) IsEnabledForGroup(ctx context.Context, groupID *int64) bool {
	if s == nil || s.repo == nil || groupID == nil || *groupID <= 0 {
		return false
	}
	if !s.settings(ctx).Enabled {
		return false
	}
	group, err := s.repo.GetGroup(ctx, *groupID)
	if err != nil || group == nil {
		return false
	}
	return group.Platform == PlatformOpenAI &&
		group.Status == StatusActive &&
		group.OpenAIAutoSchedulerEnabled
}

func (s *OpenAIAutoSchedulerService) Record(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error {
	return s.record(ctx, input, true)
}

func (s *OpenAIAutoSchedulerService) RecordManualProbe(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error {
	return s.record(ctx, input, false)
}

func (s *OpenAIAutoSchedulerService) record(ctx context.Context, input OpenAIAutoSchedulerRecordInput, bestEffort bool) error {
	if s == nil || s.repo == nil {
		if bestEffort {
			return nil
		}
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_NOT_CONFIGURED", "openai auto scheduler service is not configured")
	}
	if input.AccountID <= 0 || input.GroupID <= 0 {
		if bestEffort {
			return nil
		}
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_INVALID_IDENTITY", "account_id and group_id are required")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		if bestEffort {
			return nil
		}
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_MODEL_REQUIRED", "model is required")
	}

	settings := s.settings(ctx)
	if !settings.Enabled {
		if bestEffort {
			return nil
		}
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_DISABLED", "openai auto scheduler is disabled")
	}
	group, err := s.repo.GetGroup(ctx, input.GroupID)
	if err != nil {
		if bestEffort {
			return nil
		}
		return err
	}
	if group == nil {
		if bestEffort {
			return nil
		}
		return infraerrors.NotFound("OPENAI_AUTO_SCHEDULER_GROUP_NOT_FOUND", "group not found")
	}
	if group.Platform != PlatformOpenAI || group.Status != StatusActive || !group.OpenAIAutoSchedulerEnabled {
		if bestEffort {
			return nil
		}
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_GROUP_DISABLED", "openai auto scheduler is not enabled for this group")
	}

	now := time.Now()
	unlock := s.lockScoreState(input.AccountID, input.GroupID, model)
	defer unlock()
	state, err := s.repo.GetScoreState(ctx, input.AccountID, input.GroupID, model)
	if err != nil {
		if bestEffort {
			return nil
		}
		return err
	}
	if state == nil {
		newState := NewOpenAIAutoSchedulerScoreState(input.AccountID, input.GroupID, model)
		state = &newState
	}
	before := state.FinalScore
	next := ApplyOpenAIAutoSchedulerEvent(now, *state, OpenAIAutoSchedulerEventInput{
		EventType:  input.EventType,
		LatencyMS:  input.LatencyMS,
		TtfbMS:     input.TtfbMS,
		StatusCode: input.StatusCode,
		Message:    input.Message,
		CostScore:  input.CostScore,
	}, settings)
	next.AccountID = input.AccountID
	next.GroupID = input.GroupID
	next.Model = model
	if err := s.repo.UpsertScoreState(ctx, next); err != nil {
		if bestEffort {
			return nil
		}
		return err
	}
	if err := s.repo.InsertScoreEvent(ctx, OpenAIAutoSchedulerScoreEvent{
		AccountID:   input.AccountID,
		GroupID:     input.GroupID,
		Model:       model,
		EventType:   input.EventType,
		ScoreBefore: before,
		ScoreAfter:  next.FinalScore,
		LatencyMS:   input.LatencyMS,
		TtfbMS:      input.TtfbMS,
		StatusCode:  input.StatusCode,
		Message:     strings.TrimSpace(input.Message),
		CreatedAt:   now,
	}); err != nil {
		if bestEffort {
			return nil
		}
		return err
	}
	return nil
}

func (s *OpenAIAutoSchedulerService) ListEnabledOpenAIGroups(ctx context.Context) ([]Group, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListEnabledOpenAIGroups(ctx)
}

func (s *OpenAIAutoSchedulerService) ListScores(ctx context.Context, params OpenAIAutoSchedulerListParams) (*OpenAIAutoSchedulerScoreListResult, error) {
	if s == nil || s.repo == nil {
		return &OpenAIAutoSchedulerScoreListResult{}, nil
	}
	params.Model = strings.TrimSpace(params.Model)
	if params.GroupID > 0 && params.Model != "" {
		return s.listScoresWithGroupAccounts(ctx, params)
	}
	items, total, err := s.repo.ListScoreStates(ctx, params)
	if err != nil {
		return nil, err
	}
	return &OpenAIAutoSchedulerScoreListResult{Items: items, Total: total}, nil
}

func (s *OpenAIAutoSchedulerService) listScoresWithGroupAccounts(ctx context.Context, params OpenAIAutoSchedulerListParams) (*OpenAIAutoSchedulerScoreListResult, error) {
	page, pageSize := normalizeOpenAIAutoSchedulerListPage(params.Page, params.PageSize)
	scoreParams := params
	scoreParams.Page = 1
	scoreParams.PageSize = openAIAutoSchedulerListMaxPageSize
	items, _, err := s.repo.ListScoreStates(ctx, scoreParams)
	if err != nil {
		return nil, err
	}
	accounts, err := s.repo.ListSchedulableOpenAIAccountsByGroup(ctx, params.GroupID)
	if err != nil {
		return nil, err
	}

	currentAccounts := make(map[int64]Account, len(accounts))
	for _, account := range accounts {
		currentAccounts[account.ID] = account
	}

	byAccountID := make(map[int64]OpenAIAutoSchedulerScoreState, len(accounts))
	dailySamples, err := s.repo.ListScoreDailySamples(ctx, params, openAIAutoSchedulerTodayStart(time.Now()))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		account, ok := currentAccounts[item.AccountID]
		if !ok {
			continue
		}
		item.AccountName = account.Name
		item.ChannelPrice = copyOpenAIAutoSchedulerFloatPtr(account.ChannelPrice)
		item = applyOpenAIAutoSchedulerDailySample(item, dailySamples[item.AccountID])
		byAccountID[item.AccountID] = item
	}
	for _, account := range accounts {
		if item, ok := byAccountID[account.ID]; ok {
			item.AccountName = account.Name
			item.ChannelPrice = copyOpenAIAutoSchedulerFloatPtr(account.ChannelPrice)
			item = applyOpenAIAutoSchedulerDailySample(item, dailySamples[item.AccountID])
			byAccountID[account.ID] = item
			continue
		}
		state := NewOpenAIAutoSchedulerScoreState(account.ID, params.GroupID, params.Model)
		state.AccountName = account.Name
		state.ChannelPrice = copyOpenAIAutoSchedulerFloatPtr(account.ChannelPrice)
		state = applyOpenAIAutoSchedulerDailySample(state, dailySamples[state.AccountID])
		byAccountID[account.ID] = state
	}

	merged := make([]OpenAIAutoSchedulerScoreState, 0, len(byAccountID))
	for _, item := range byAccountID {
		merged = append(merged, item)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].FinalScore != merged[j].FinalScore {
			return merged[i].FinalScore > merged[j].FinalScore
		}
		return merged[i].AccountID < merged[j].AccountID
	})

	total := len(merged)
	start := (page - 1) * pageSize
	if start >= total {
		return &OpenAIAutoSchedulerScoreListResult{Items: []OpenAIAutoSchedulerScoreState{}, Total: int64(total)}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &OpenAIAutoSchedulerScoreListResult{Items: merged[start:end], Total: int64(total)}, nil
}

func (s *OpenAIAutoSchedulerService) ListEvents(ctx context.Context, params OpenAIAutoSchedulerListParams) (*OpenAIAutoSchedulerEventListResult, error) {
	if s == nil || s.repo == nil {
		return &OpenAIAutoSchedulerEventListResult{}, nil
	}
	params.Model = strings.TrimSpace(params.Model)
	items, total, err := s.repo.ListScoreEvents(ctx, params)
	if err != nil {
		return nil, err
	}
	return &OpenAIAutoSchedulerEventListResult{Items: items, Total: total}, nil
}

func (s *OpenAIAutoSchedulerService) ResetScore(ctx context.Context, accountID, groupID int64, model string) error {
	if s == nil || s.repo == nil || accountID <= 0 || groupID <= 0 {
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_INVALID_IDENTITY", "account_id and group_id are required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return infraerrors.BadRequest("OPENAI_AUTO_SCHEDULER_MODEL_REQUIRED", "model is required")
	}
	now := time.Now()
	unlock := s.lockScoreState(accountID, groupID, model)
	defer unlock()
	state, err := s.repo.GetScoreState(ctx, accountID, groupID, model)
	if err != nil {
		return err
	}
	if state == nil {
		return infraerrors.NotFound("OPENAI_AUTO_SCHEDULER_SCORE_NOT_FOUND", "score state not found")
	}
	before := state.FinalScore
	next := ApplyOpenAIAutoSchedulerEvent(now, *state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventManualReset,
		Message:   "manual reset",
	}, s.settings(ctx))
	next.AccountID = accountID
	next.GroupID = groupID
	next.Model = model
	if err := s.repo.UpsertScoreState(ctx, next); err != nil {
		return err
	}
	return s.repo.InsertScoreEvent(ctx, OpenAIAutoSchedulerScoreEvent{
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		EventType:   OpenAIAutoSchedulerEventManualReset,
		ScoreBefore: before,
		ScoreAfter:  next.FinalScore,
		Message:     "manual reset",
		CreatedAt:   now,
	})
}

func (s *OpenAIAutoSchedulerService) settings(ctx context.Context) OpenAIAutoSchedulerSettings {
	if s == nil || s.settingsProvider == nil {
		return DefaultOpenAIAutoSchedulerSettings()
	}
	return normalizeOpenAIAutoSchedulerSettings(s.settingsProvider.GetOpenAIAutoSchedulerSettings(ctx))
}

func openAIAutoSchedulerTodayStart(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	local := now.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func applyOpenAIAutoSchedulerDailySample(state OpenAIAutoSchedulerScoreState, sample OpenAIAutoSchedulerDailySample) OpenAIAutoSchedulerScoreState {
	if sample.AccountID <= 0 {
		state.RequestCount = 0
		state.TtfbSampleCount = 0
		state.LastTtfbMS = nil
		return state
	}
	state.RequestCount = sample.RequestCount
	state.TtfbSampleCount = sample.TtfbSampleCount
	state.LastTtfbMS = copyOpenAIAutoSchedulerIntPtr(sample.LastTtfbMS)
	return state
}

func copyOpenAIAutoSchedulerFloatPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func (s *OpenAIAutoSchedulerService) lockScoreState(accountID, groupID int64, model string) func() {
	if s == nil {
		return func() {}
	}
	key := fmt.Sprintf("%d|%d|%s", accountID, groupID, strings.TrimSpace(model))
	s.mu.Lock()
	if s.keyLocks == nil {
		s.keyLocks = map[string]*sync.Mutex{}
	}
	lock := s.keyLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.keyLocks[key] = lock
	}
	s.mu.Unlock()

	lock.Lock()
	return lock.Unlock
}
