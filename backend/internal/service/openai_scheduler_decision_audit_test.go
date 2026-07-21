package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAISchedulerDecisionAuditRepoStub struct {
	mu     sync.Mutex
	events []OpenAISchedulerDecisionAuditEvent
	err    error
}

func (r *openAISchedulerDecisionAuditRepoStub) InsertOpenAISchedulerDecisionAudit(_ context.Context, event OpenAISchedulerDecisionAuditEvent) error {
	if r.err != nil {
		return r.err
	}
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	return nil
}

func (r *openAISchedulerDecisionAuditRepoStub) snapshot() []OpenAISchedulerDecisionAuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]OpenAISchedulerDecisionAuditEvent(nil), r.events...)
}

func TestOpenAISchedulerDecisionAuditRecorderPersistsShadowDecisionOffRequestPath(t *testing.T) {
	repo := &openAISchedulerDecisionAuditRepoStub{}
	recorder := NewOpenAISchedulerDecisionAuditRecorder(repo, 8)
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	scheduler := NewOpenAIBalancedScheduler(nil, recorder)

	result, err := scheduler.Order(context.Background(), OpenAIBalancedSelectionInput{
		GroupID: 82,
		Now:     now,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, HealthKey: OpenAISchedulerHealthKey{AccountID: 1, ModelFamily: "gpt-5.5", Endpoint: "responses", Transport: "http_sse"}, PredictedTTFTMS: 2000, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, HealthKey: OpenAISchedulerHealthKey{AccountID: 2, ModelFamily: "gpt-5.5", Endpoint: "responses", Transport: "http_sse"}, PredictedTTFTMS: 500, State: OpenAIAutoSchedulerStateRunning},
		},
		LegacyOrderedAccountIDs: []int64{1, 2},
		Settings: OpenAIBalancedSettings{
			Mode: OpenAIAutoSchedulerModeBalanced, ShadowMode: true, TopK: 2,
			Weights: defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModeBalanced),
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.ShadowAccountID)
	require.NoError(t, recorder.Stop(context.Background()))

	events := repo.snapshot()
	require.Len(t, events, 1)
	require.Equal(t, OpenAISchedulerAuditShadowDecision, events[0].EventType)
	require.Equal(t, int64(82), events[0].GroupID)
	require.Equal(t, int64(1), events[0].LegacyAccountID)
	require.Equal(t, int64(2), events[0].AccountID)
	require.Equal(t, "gpt-5.5", events[0].ModelFamily)
	require.Equal(t, "balanced_order_changed", events[0].Reason)
	require.Equal(t, now, events[0].CreatedAt)
	require.Equal(t, uint64(1), recorder.SnapshotMetrics().Accepted)
}

func TestDefaultOpenAIAccountSchedulerPersistsExplorationReservationOutcome(t *testing.T) {
	repo := &openAISchedulerDecisionAuditRepoStub{}
	recorder := NewOpenAISchedulerDecisionAuditRecorder(repo, 8)
	cache := &openAISchedulerExplorationCacheStub{outcome: OpenAISchedulerExplorationReservationMinimumInterval}
	balanced := NewOpenAIBalancedScheduler(nil, recorder)
	gateway := &OpenAIGatewayService{openaiExplorationCache: cache}
	gateway.SetOpenAIBalancedScheduler(balanced)
	scheduler := &defaultOpenAIAccountScheduler{service: gateway}
	groupID := int64(82)
	candidate := openAIAccountCandidateScore{
		account: &Account{ID: 12818}, schedulerTrafficClass: OpenAISchedulerTrafficExploration,
		schedulerEligibility: OpenAISchedulerEligibilityLowConfidence,
		schedulerHealthKey:   OpenAISchedulerHealthKey{AccountID: 12818, ModelFamily: "gpt-5.5", Endpoint: "responses", Transport: "http_sse"},
		schedulerTargetShare: 0.025,
	}
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, balancedPolicySettings: DefaultOpenAIBalancedSettings()}

	require.False(t, scheduler.reserveOpenAISchedulerExploration(context.Background(), req, candidate))
	require.NoError(t, recorder.Stop(context.Background()))

	events := repo.snapshot()
	require.Len(t, events, 1)
	require.Equal(t, OpenAISchedulerAuditExplorationRejected, events[0].EventType)
	require.Equal(t, "minimum_interval", events[0].Reason)
	require.Equal(t, int64(12818), events[0].AccountID)
	require.Equal(t, int64(82), events[0].GroupID)
	require.InDelta(t, 0.025, events[0].TargetShare, 0.000001)
}
