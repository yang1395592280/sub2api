package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeOpenAIAutoSchedulerSettingsProvider struct {
	settings OpenAIAutoSchedulerSettings
}

func (p fakeOpenAIAutoSchedulerSettingsProvider) GetOpenAIAutoSchedulerSettings(context.Context) OpenAIAutoSchedulerSettings {
	return p.settings
}

type fakeOpenAIAutoSchedulerRepo struct {
	mu            sync.Mutex
	groups        map[int64]Group
	states        map[string]OpenAIAutoSchedulerScoreState
	events        []OpenAIAutoSchedulerScoreEvent
	accounts      map[int64][]Account
	dailySamples  map[int64]OpenAIAutoSchedulerDailySample
	listStates    []OpenAIAutoSchedulerScoreState
	listEvents    []OpenAIAutoSchedulerScoreEvent
	listTotal     int64
	listParams    OpenAIAutoSchedulerListParams
	getStateCalls int
	summaryCalls  int
	err           error
}

func (r *fakeOpenAIAutoSchedulerRepo) GetGroup(ctx context.Context, groupID int64) (*Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	group, ok := r.groups[groupID]
	if !ok {
		return nil, nil
	}
	return &group, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) GetScoreState(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getStateCalls++
	if r.err != nil {
		return nil, r.err
	}
	state, ok := r.states[openAIAutoSchedulerStateKey(accountID, groupID, model)]
	if !ok {
		return nil, nil
	}
	return &state, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListScoreStatesForSummary(ctx context.Context, groupID int64, model string) ([]OpenAIAutoSchedulerScoreState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summaryCalls++
	if r.err != nil {
		return nil, r.err
	}
	out := make([]OpenAIAutoSchedulerScoreState, 0, len(r.states))
	for _, state := range r.states {
		if state.GroupID != groupID || strings.TrimSpace(state.Model) != strings.TrimSpace(model) {
			continue
		}
		out = append(out, state)
	}
	return out, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) HasOpenCircuitScoreState(ctx context.Context, accountID, groupID int64, model string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return false, r.err
	}
	model = strings.TrimSpace(model)
	for _, state := range r.states {
		if state.AccountID != accountID || state.GroupID != groupID || strings.TrimSpace(state.Model) != strings.TrimSpace(model) {
			continue
		}
		if state.State == OpenAIAutoSchedulerStateOpen {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) UpsertScoreState(ctx context.Context, state OpenAIAutoSchedulerScoreState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if r.states == nil {
		r.states = map[string]OpenAIAutoSchedulerScoreState{}
	}
	r.states[openAIAutoSchedulerStateKey(state.AccountID, state.GroupID, state.Model)] = state
	return nil
}

func (r *fakeOpenAIAutoSchedulerRepo) InsertScoreEvent(ctx context.Context, event OpenAIAutoSchedulerScoreEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListScoreStates(ctx context.Context, params OpenAIAutoSchedulerListParams) ([]OpenAIAutoSchedulerScoreState, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, 0, r.err
	}
	r.listParams = params
	return r.listStates, r.listTotal, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListScoreEvents(ctx context.Context, params OpenAIAutoSchedulerListParams) ([]OpenAIAutoSchedulerScoreEvent, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, 0, r.err
	}
	r.listParams = params
	return r.listEvents, r.listTotal, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListScoreDailySamples(ctx context.Context, params OpenAIAutoSchedulerListParams, since time.Time) (map[int64]OpenAIAutoSchedulerDailySample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[int64]OpenAIAutoSchedulerDailySample, len(r.dailySamples))
	for accountID, sample := range r.dailySamples {
		out[accountID] = sample
	}
	return out, nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListSchedulableOpenAIAccountsByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return r.accounts[groupID], nil
}

func (r *fakeOpenAIAutoSchedulerRepo) ListEnabledOpenAIGroups(ctx context.Context) ([]Group, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	var groups []Group
	for _, group := range r.groups {
		if group.Platform == PlatformOpenAI && group.Status == StatusActive && group.OpenAIAutoSchedulerEnabled {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func TestOpenAIAutoSchedulerService_IsEnabledForGroupRequiresGlobalAndGroup(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
			11: {ID: 11, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: false, Hydrated: true, Status: StatusActive},
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	require.True(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(10)))
	require.False(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(11)))
	require.False(t, svc.IsEnabledForGroup(context.Background(), nil))
}

func TestOpenAIAutoSchedulerService_IsEnabledForGroupDegradesToFalse(t *testing.T) {
	t.Run("global disabled", func(t *testing.T) {
		repo := &fakeOpenAIAutoSchedulerRepo{
			groups: map[int64]Group{
				10: {ID: 10, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
			},
		}
		svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: DefaultOpenAIAutoSchedulerSettings()})

		require.False(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(10)))
	})

	t.Run("repo error", func(t *testing.T) {
		svc := NewOpenAIAutoSchedulerService(
			&fakeOpenAIAutoSchedulerRepo{err: errors.New("database unavailable")},
			fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()},
		)

		require.False(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(10)))
	})

	t.Run("non-openai or inactive group", func(t *testing.T) {
		repo := &fakeOpenAIAutoSchedulerRepo{
			groups: map[int64]Group{
				10: {ID: 10, Platform: PlatformAnthropic, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
				11: {ID: 11, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusDisabled},
			},
		}
		svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

		require.False(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(10)))
		require.False(t, svc.IsEnabledForGroup(context.Background(), ptrOpenAIAutoSchedulerInt64(11)))
	})
}

func TestOpenAIAutoSchedulerService_RecordAppliesEventAndStoresAudit(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			200: {ID: 200, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
		},
		states: map[string]OpenAIAutoSchedulerScoreState{},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	err := svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID:  100,
		GroupID:    200,
		Model:      "  gpt-5  ",
		EventType:  OpenAIAutoSchedulerEventSuccess,
		LatencyMS:  ptrOpenAIAutoSchedulerInt(900),
		StatusCode: ptrOpenAIAutoSchedulerInt(200),
		Message:    "ok",
		CostScore:  ptrOpenAIAutoSchedulerInt(500),
	})

	require.NoError(t, err)
	state := repo.states[openAIAutoSchedulerStateKey(100, 200, "gpt-5")]
	require.Equal(t, int64(100), state.AccountID)
	require.Equal(t, int64(200), state.GroupID)
	require.Equal(t, "gpt-5", state.Model)
	require.Equal(t, OpenAIAutoSchedulerStateRunning, state.State)
	require.Greater(t, state.FinalScore, state.BaseScore)
	require.Len(t, repo.events, 1)
	require.Equal(t, OpenAIAutoSchedulerEventSuccess, repo.events[0].EventType)
	require.Equal(t, 6000, repo.events[0].ScoreBefore)
	require.Equal(t, state.FinalScore, repo.events[0].ScoreAfter)
}

func TestOpenAIAutoSchedulerService_AuditCreatedAtDoesNotMoveStateClockBackward(t *testing.T) {
	auditCreatedAt := time.Date(2026, 7, 14, 12, 0, 0, 123456789, time.UTC)
	realUpdatedAt := time.Now().Add(-time.Second)
	state10 := NewOpenAIAutoSchedulerScoreState(100, 200, "gpt-5")
	state10.LastCheckedAt = &realUpdatedAt
	state20 := NewOpenAIAutoSchedulerScoreState(100, 201, "gpt-5")
	state20.LastCheckedAt = &realUpdatedAt
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			200: {ID: 200, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
			201: {ID: 201, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
		},
		states: map[string]OpenAIAutoSchedulerScoreState{
			openAIAutoSchedulerStateKey(100, 200, "gpt-5"): state10,
			openAIAutoSchedulerStateKey(100, 201, "gpt-5"): state20,
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	processedAfter := time.Now()
	for _, groupID := range []int64{200, 201} {
		require.NoError(t, svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
			AccountID: 100, GroupID: groupID, Model: "gpt-5", EventType: OpenAIAutoSchedulerEventProbeSuccess,
			AuditCreatedAt: auditCreatedAt,
		}))
	}

	require.Len(t, repo.events, 2)
	require.Equal(t, auditCreatedAt, repo.events[0].CreatedAt)
	require.Equal(t, auditCreatedAt, repo.events[1].CreatedAt)
	for _, groupID := range []int64{200, 201} {
		lastCheckedAt := repo.states[openAIAutoSchedulerStateKey(100, groupID, "gpt-5")].LastCheckedAt
		require.NotNil(t, lastCheckedAt)
		require.False(t, lastCheckedAt.Before(processedAfter))
		require.True(t, lastCheckedAt.After(realUpdatedAt))
	}
}

func TestOpenAIAutoSchedulerService_RecordWithoutAuditCreatedAtKeepsWallClockSemantics(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{200: {ID: 200, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive}},
		states: map[string]OpenAIAutoSchedulerScoreState{},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})
	before := time.Now()

	err := svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 100, GroupID: 200, Model: "gpt-5", EventType: OpenAIAutoSchedulerEventSuccess,
	})

	after := time.Now()
	require.NoError(t, err)
	require.Len(t, repo.events, 1)
	require.False(t, repo.events[0].CreatedAt.Before(before))
	require.False(t, repo.events[0].CreatedAt.After(after))
}

func TestOpenAIAutoSchedulerService_RecordSkipsWhenSettingsDisabled(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: DefaultOpenAIAutoSchedulerSettings()})

	err := svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: OpenAIAutoSchedulerEventError,
	})

	require.NoError(t, err)
	require.Empty(t, repo.states)
	require.Empty(t, repo.events)
}

func TestOpenAIAutoSchedulerService_RecordSkipsWhenGroupDisabled(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			2: {ID: 2, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: false, Hydrated: true, Status: StatusActive},
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	err := svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 1,
		GroupID:   2,
		Model:     "gpt-5",
		EventType: OpenAIAutoSchedulerEventError,
	})

	require.NoError(t, err)
	require.Empty(t, repo.states)
	require.Empty(t, repo.events)
}

func TestOpenAIAutoSchedulerService_RecordManualProbeSurfacesSkippedAndRepositoryErrors(t *testing.T) {
	t.Run("settings disabled", func(t *testing.T) {
		repo := &fakeOpenAIAutoSchedulerRepo{}
		svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: DefaultOpenAIAutoSchedulerSettings()})

		err := svc.RecordManualProbe(context.Background(), OpenAIAutoSchedulerRecordInput{
			AccountID: 1,
			GroupID:   2,
			Model:     "gpt-5",
			EventType: OpenAIAutoSchedulerEventProbeSuccess,
		})

		require.Error(t, err)
		require.Empty(t, repo.states)
		require.Empty(t, repo.events)
	})

	t.Run("group disabled", func(t *testing.T) {
		repo := &fakeOpenAIAutoSchedulerRepo{
			groups: map[int64]Group{
				2: {ID: 2, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: false, Hydrated: true, Status: StatusActive},
			},
		}
		svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

		err := svc.RecordManualProbe(context.Background(), OpenAIAutoSchedulerRecordInput{
			AccountID: 1,
			GroupID:   2,
			Model:     "gpt-5",
			EventType: OpenAIAutoSchedulerEventProbeSuccess,
		})

		require.Error(t, err)
		require.Empty(t, repo.states)
		require.Empty(t, repo.events)
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeOpenAIAutoSchedulerRepo{
			groups: map[int64]Group{
				2: {ID: 2, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
			},
			err: errors.New("database unavailable"),
		}
		svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

		err := svc.RecordManualProbe(context.Background(), OpenAIAutoSchedulerRecordInput{
			AccountID: 1,
			GroupID:   2,
			Model:     "gpt-5",
			EventType: OpenAIAutoSchedulerEventProbeSuccess,
		})

		require.Error(t, err)
	})
}

func TestOpenAIAutoSchedulerService_RecordManualProbeStoresAuditOnSuccess(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			2: {ID: 2, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
		},
		states: map[string]OpenAIAutoSchedulerScoreState{},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	err := svc.RecordManualProbe(context.Background(), OpenAIAutoSchedulerRecordInput{
		AccountID: 1,
		GroupID:   2,
		Model:     " gpt-5 ",
		EventType: OpenAIAutoSchedulerEventProbeSuccess,
		Message:   "ok",
	})

	require.NoError(t, err)
	require.Contains(t, repo.states, openAIAutoSchedulerStateKey(1, 2, "gpt-5"))
	require.Len(t, repo.events, 1)
	require.Equal(t, OpenAIAutoSchedulerEventProbeSuccess, repo.events[0].EventType)
}

func TestOpenAIAutoSchedulerService_ListScoresDelegatesToRepositoryWithoutScopedModel(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		listStates: []OpenAIAutoSchedulerScoreState{
			{AccountID: 1, GroupID: 2, Model: "gpt-5", FinalScore: 7000},
		},
		listTotal: 1,
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	result, err := svc.ListScores(context.Background(), OpenAIAutoSchedulerListParams{
		GroupID:  2,
		Model:    " ",
		Page:     2,
		PageSize: 50,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(2), repo.listParams.GroupID)
	require.Equal(t, "", repo.listParams.Model)
	require.Equal(t, 2, repo.listParams.Page)
	require.Equal(t, 50, repo.listParams.PageSize)
}

func TestOpenAIAutoSchedulerService_ListScoresIncludesDefaultStatesForUnscoredGroupAccounts(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		accounts: map[int64][]Account{
			2: {
				{ID: 1, Name: "已打分渠道", Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
				{ID: 3, Name: "新渠道", Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
			},
		},
		listStates: []OpenAIAutoSchedulerScoreState{
			{AccountID: 1, AccountName: "旧名称", GroupID: 2, Model: "gpt-5", FinalScore: 7200, State: OpenAIAutoSchedulerStateObserving},
		},
		listTotal: 1,
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	result, err := svc.ListScores(context.Background(), OpenAIAutoSchedulerListParams{
		GroupID:  2,
		Model:    "gpt-5",
		Page:     1,
		PageSize: 50,
	})

	require.NoError(t, err)
	require.Equal(t, int64(2), result.Total)
	require.Len(t, result.Items, 2)
	require.Equal(t, int64(1), result.Items[0].AccountID)
	require.Equal(t, "已打分渠道", result.Items[0].AccountName)
	require.Equal(t, 7200, result.Items[0].FinalScore)
	require.Equal(t, int64(3), result.Items[1].AccountID)
	require.Equal(t, "新渠道", result.Items[1].AccountName)
	require.Equal(t, "gpt-5", result.Items[1].Model)
	require.Equal(t, OpenAIAutoSchedulerStateRunning, result.Items[1].State)
	require.Equal(t, 6000, result.Items[1].FinalScore)
}

func TestOpenAIAutoSchedulerService_ListScoresUsesDailySamplesAndChannelPriceForDisplay(t *testing.T) {
	channelPrice := 0.15
	lastTtfb := 456
	repo := &fakeOpenAIAutoSchedulerRepo{
		accounts: map[int64][]Account{
			2: {
				{ID: 1, Name: "带价格渠道", Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, ChannelPrice: &channelPrice},
			},
		},
		dailySamples: map[int64]OpenAIAutoSchedulerDailySample{
			1: {AccountID: 1, RequestCount: 2, TtfbSampleCount: 1, LastTtfbMS: &lastTtfb},
		},
		listStates: []OpenAIAutoSchedulerScoreState{
			{
				AccountID:       1,
				AccountName:     "旧名称",
				GroupID:         2,
				Model:           "gpt-5",
				FinalScore:      7200,
				State:           OpenAIAutoSchedulerStateObserving,
				RequestCount:    99,
				TtfbSampleCount: 40,
			},
		},
		listTotal: 1,
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	result, err := svc.ListScores(context.Background(), OpenAIAutoSchedulerListParams{
		GroupID:  2,
		Model:    "gpt-5",
		Page:     1,
		PageSize: 50,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "带价格渠道", result.Items[0].AccountName)
	require.Equal(t, &channelPrice, result.Items[0].ChannelPrice)
	require.Equal(t, int64(2), result.Items[0].RequestCount)
	require.Equal(t, int64(1), result.Items[0].TtfbSampleCount)
	require.Equal(t, &lastTtfb, result.Items[0].LastTtfbMS)
}

func TestOpenAIAutoSchedulerService_ListScoresHidesStatesForAccountsNoLongerInGroup(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		accounts: map[int64][]Account{
			2: {
				{ID: 1, Name: "仍在分组", Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
			},
		},
		listStates: []OpenAIAutoSchedulerScoreState{
			{AccountID: 1, AccountName: "旧名称", GroupID: 2, Model: "gpt-5", FinalScore: 7200, State: OpenAIAutoSchedulerStateObserving},
			{AccountID: 9, AccountName: "已移出分组", GroupID: 2, Model: "gpt-5", FinalScore: 9900, State: OpenAIAutoSchedulerStateRunning},
		},
		listTotal: 2,
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	result, err := svc.ListScores(context.Background(), OpenAIAutoSchedulerListParams{
		GroupID:  2,
		Model:    "gpt-5",
		Page:     1,
		PageSize: 50,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(1), result.Items[0].AccountID)
	require.Equal(t, "仍在分组", result.Items[0].AccountName)
}

func TestOpenAIAutoSchedulerService_ListEventsDelegatesToRepository(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		listEvents: []OpenAIAutoSchedulerScoreEvent{
			{AccountID: 1, GroupID: 2, Model: "gpt-5", EventType: OpenAIAutoSchedulerEventSlow},
		},
		listTotal: 1,
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	result, err := svc.ListEvents(context.Background(), OpenAIAutoSchedulerListParams{
		GroupID:  2,
		Model:    " gpt-5 ",
		Page:     2,
		PageSize: 50,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	require.Equal(t, OpenAIAutoSchedulerEventSlow, result.Items[0].EventType)
	require.Equal(t, int64(2), repo.listParams.GroupID)
	require.Equal(t, "gpt-5", repo.listParams.Model)
	require.Equal(t, 2, repo.listParams.Page)
	require.Equal(t, 50, repo.listParams.PageSize)
}

func TestOpenAIAutoSchedulerService_ResetScoreRequiresExistingState(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	err := svc.ResetScore(context.Background(), 1, 2, "gpt-5")

	require.Error(t, err)
	require.Empty(t, repo.states)
	require.Empty(t, repo.events)
}

func TestOpenAIAutoSchedulerService_ResetScoreStoresAuditForExistingState(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{
			openAIAutoSchedulerStateKey(1, 2, "gpt-5"): {
				AccountID:  1,
				GroupID:    2,
				Model:      "gpt-5",
				BaseScore:  6000,
				FinalScore: 3000,
				State:      OpenAIAutoSchedulerStateOpen,
			},
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()})

	err := svc.ResetScore(context.Background(), 1, 2, " gpt-5 ")

	require.NoError(t, err)
	state := repo.states[openAIAutoSchedulerStateKey(1, 2, "gpt-5")]
	require.Equal(t, 6000, state.FinalScore)
	require.Len(t, repo.events, 1)
	require.Equal(t, OpenAIAutoSchedulerEventManualReset, repo.events[0].EventType)
	require.Equal(t, 3000, repo.events[0].ScoreBefore)
	require.Equal(t, 6000, repo.events[0].ScoreAfter)
}

func TestOpenAIAutoSchedulerService_ResetScoreWorksWhenGlobalSchedulerDisabled(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{
			openAIAutoSchedulerStateKey(1, 2, "gpt-5"): {
				AccountID:     1,
				GroupID:       2,
				Model:         "gpt-5",
				BaseScore:     6000,
				FinalScore:    500,
				LatencyScore:  -3500,
				ErrorScore:    -6000,
				State:         OpenAIAutoSchedulerStateOpen,
				CooldownUntil: ptrOpenAIAutoSchedulerTime(time.Now().Add(time.Minute)),
			},
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: DefaultOpenAIAutoSchedulerSettings()})

	err := svc.ResetScore(context.Background(), 1, 2, "gpt-5")

	require.NoError(t, err)
	state := repo.states[openAIAutoSchedulerStateKey(1, 2, "gpt-5")]
	require.Equal(t, OpenAIAutoSchedulerStateRunning, state.State)
	require.Equal(t, 6000, state.FinalScore)
	require.Equal(t, 0, state.LatencyScore)
	require.Equal(t, 0, state.ErrorScore)
	require.Nil(t, state.CooldownUntil)
	require.Len(t, repo.events, 1)
}

func TestOpenAIAutoSchedulerService_RecordConcurrentErrorsTripsBreaker(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			2: {ID: 2, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
		},
		states: map[string]OpenAIAutoSchedulerScoreState{},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ConsecutiveErrorBreakerThreshold = 2
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, svc.Record(context.Background(), OpenAIAutoSchedulerRecordInput{
				AccountID: 1,
				GroupID:   2,
				Model:     "gpt-5",
				EventType: OpenAIAutoSchedulerEventError,
			}))
		}()
	}
	wg.Wait()

	state := repo.states[openAIAutoSchedulerStateKey(1, 2, "gpt-5")]
	require.Equal(t, 2, state.ConsecutiveErrorCount)
	require.Equal(t, OpenAIAutoSchedulerStateOpen, state.State)
	require.Len(t, repo.events, 2)
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesRanksBySpeed(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{},
		accounts: map[int64][]Account{
			10: {{ID: 1}, {ID: 2}},
		},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	fast := 200
	slow := 900
	repo.states[openAIAutoSchedulerStateKey(1, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &slow}
	repo.states[openAIAutoSchedulerStateKey(2, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 2, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &fast}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1, 2})

	require.NoError(t, err)
	require.Equal(t, 2, summaries[1].SpeedPriority)
	require.Equal(t, 1, summaries[2].SpeedPriority)
	require.Equal(t, "gpt-5.5", summaries[1].ProbeModel)
	require.Equal(t, &slow, summaries[1].SpeedMS)
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesRanksRunningAccountsOnly(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{},
		accounts: map[int64][]Account{
			10: {{ID: 1}, {ID: 2}},
		},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	openSpeed := 100
	runningSpeed := 300
	repo.states[openAIAutoSchedulerStateKey(1, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateOpen, FinalScore: 1000, LastTtfbMS: &openSpeed}
	repo.states[openAIAutoSchedulerStateKey(2, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 2, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &runningSpeed}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1, 2})

	require.NoError(t, err)
	require.Zero(t, summaries[1].SpeedPriority)
	require.Equal(t, 1, summaries[2].SpeedPriority)
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesRanksWithinWholeGroup(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{},
		accounts: map[int64][]Account{
			10: {{ID: 1}, {ID: 2}},
		},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	fast := 200
	second := 400
	otherModel := 100
	repo.states[openAIAutoSchedulerStateKey(1, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &fast}
	repo.states[openAIAutoSchedulerStateKey(2, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 2, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &second}
	repo.states[openAIAutoSchedulerStateKey(3, 10, "gpt-5.4")] = OpenAIAutoSchedulerScoreState{AccountID: 3, GroupID: 10, Model: "gpt-5.4", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &otherModel}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{2})

	require.NoError(t, err)
	require.Equal(t, 1, repo.summaryCalls)
	require.Zero(t, repo.getStateCalls)
	require.Len(t, summaries, 1)
	require.Equal(t, 2, summaries[2].SpeedPriority)
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesIgnoresUnschedulableStateRanks(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{},
		accounts: map[int64][]Account{
			10: {{ID: 2}},
		},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	unschedulableFast := 100
	schedulableSlow := 400
	repo.states[openAIAutoSchedulerStateKey(1, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &unschedulableFast}
	repo.states[openAIAutoSchedulerStateKey(2, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 2, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &schedulableSlow}
	svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1, 2})

	require.NoError(t, err)
	require.NotContains(t, summaries, int64(1))
	require.Equal(t, 1, summaries[2].SpeedPriority)
}

type openAISchedulerHealthSummaryRepoStub struct {
	snapshots []OpenAISchedulerHealthSnapshot
	err       error
}

func (r *openAISchedulerHealthSummaryRepoStub) GetBatch(context.Context, []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	return map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot{}, nil
}

func (r *openAISchedulerHealthSummaryRepoStub) Upsert(context.Context, OpenAISchedulerHealthSnapshot) error {
	return nil
}

func (r *openAISchedulerHealthSummaryRepoStub) ListByAccountIDs(context.Context, []int64) ([]OpenAISchedulerHealthSnapshot, error) {
	return r.snapshots, r.err
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesUsesUnifiedAvailability(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{
			openAIAutoSchedulerStateKey(1, 10, "gpt-5.5"): {AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateOpen},
		},
		accounts: map[int64][]Account{10: {{ID: 1}, {ID: 2}}},
	}
	now := time.Now()
	healthRepo := &openAISchedulerHealthSummaryRepoStub{snapshots: []OpenAISchedulerHealthSnapshot{
		{Key: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "http_sse"}, State: OpenAIAutoSchedulerStateRunning, PredictedTTFTMS: 450, ExpiresAt: now.Add(time.Hour)},
		{Key: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5", Endpoint: "chat_completions", Transport: "http_sse"}, State: OpenAIAutoSchedulerStateOpen, ExpiresAt: now.Add(time.Hour)},
		{Key: OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "http_sse"}, State: OpenAIAutoSchedulerStateOpen, ExpiresAt: now.Add(time.Hour)},
	}}
	settingsProvider := fakeOpenAIAutoSchedulerSettingsProvider{settings: enabledOpenAIAutoSchedulerSettings()}
	svc := NewOpenAIAutoSchedulerService(repo, settingsProvider)
	svc.healthSink = NewOpenAISchedulerHealthEventSink(healthRepo, settingsProvider)

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1, 2})

	require.NoError(t, err)
	require.Equal(t, OpenAIAutoSchedulerStateObserving, summaries[1].State)
	require.Equal(t, OpenAIAutoSchedulerStatusSourceUnified, summaries[1].StatusSource)
	require.Equal(t, 2, summaries[1].HealthDimensions)
	require.Equal(t, 1, summaries[1].AvailableDimensions)
	require.Equal(t, ptrOpenAIAutoSchedulerInt(450), summaries[1].SpeedMS)
	require.Equal(t, OpenAIAutoSchedulerStateOpen, summaries[2].State)
	require.Equal(t, "all_dimensions_circuit_open", summaries[2].Reason)
	require.Zero(t, repo.summaryCalls, "unified health must replace the legacy score read")
	require.Equal(t, OpenAIAutoSchedulerSummaryMetricsSnapshot{UnifiedReadsTotal: 1, UnifiedDimensionsTotal: 3}, svc.SnapshotSummaryMetrics())
}

func TestOpenAIAutoSchedulerService_ListAccountSummariesFallsBackWhenUnifiedReadFails(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		states: map[string]OpenAIAutoSchedulerScoreState{
			openAIAutoSchedulerStateKey(1, 10, "gpt-5.5"): {AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning},
		},
		accounts: map[int64][]Account{10: {{ID: 1}}},
	}
	settings := enabledOpenAIAutoSchedulerSettings()
	settings.ProbeModel = "gpt-5.5"
	healthRepo := &openAISchedulerHealthSummaryRepoStub{err: errors.New("health unavailable")}
	settingsProvider := fakeOpenAIAutoSchedulerSettingsProvider{settings: settings}
	svc := NewOpenAIAutoSchedulerService(repo, settingsProvider)
	svc.healthSink = NewOpenAISchedulerHealthEventSink(healthRepo, settingsProvider)

	summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1})

	require.NoError(t, err)
	require.Equal(t, OpenAIAutoSchedulerStatusSourceLegacy, summaries[1].StatusSource)
	require.Equal(t, 1, repo.summaryCalls)
	require.Equal(t, OpenAIAutoSchedulerSummaryMetricsSnapshot{UnifiedReadsTotal: 1, LegacyFallbacksTotal: 1}, svc.SnapshotSummaryMetrics())
}

func enabledOpenAIAutoSchedulerSettings() OpenAIAutoSchedulerSettings {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	return settings
}

func ptrOpenAIAutoSchedulerInt64(v int64) *int64 {
	return &v
}

func openAIAutoSchedulerStateKey(accountID, groupID int64, model string) string {
	return fmt.Sprintf("%d|%d|%s", accountID, groupID, strings.TrimSpace(model))
}
