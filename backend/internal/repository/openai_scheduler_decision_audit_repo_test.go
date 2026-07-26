package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerDecisionAuditRepositoryInsertsExplicitColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewOpenAISchedulerDecisionAuditRepository(db)
	createdAt := time.Date(2026, 7, 21, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	event := service.OpenAISchedulerDecisionAuditEvent{
		EventType: service.OpenAISchedulerAuditShadowDecision, GroupID: 82, AccountID: 12818, LegacyAccountID: 11892,
		EffectiveGroupID: 82, AccountSourceGroupID: 99, AccountSourceType: service.GroupRoleSelfHostedPool,
		PoolGroupID: 99, PoolFallbackReason: "",
		ModelFamily: "gpt-5.5", Endpoint: "responses", Transport: "http_sse", Reason: "balanced_order_changed",
		Confidence: service.OpenAISchedulerHealthConfidenceLow, Eligibility: service.OpenAISchedulerEligibilityLowConfidence,
		TrafficClass: service.OpenAISchedulerTrafficExploration, PredictedTTFTDifferenceMS: -4200, TargetShare: 0.05,
		CandidateCount: 10, TopK: 4, SchedulerMode: service.OpenAIAutoSchedulerModeBalanced, ShadowMode: true,
		ExplorationRate: 0.05, ExplorationBudget: 0.05, LowConfidenceMaxShare: 0.11,
		LatencyWeight: 0.28, ReliabilityWeight: 0.30, CreatedAt: createdAt,
	}

	mock.ExpectExec(`INSERT INTO openai_scheduler_decision_audits`).
		WithArgs(
			event.EventType, event.GroupID, event.EffectiveGroupID, event.AccountSourceGroupID, event.AccountSourceType,
			event.PoolGroupID, event.PoolFallbackReason,
			event.AccountID, event.LegacyAccountID,
			event.ModelFamily, event.Endpoint, event.Transport, event.Reason, event.Confidence, event.Eligibility, event.TrafficClass,
			event.PredictedTTFTDifferenceMS, event.TargetShare, event.CandidateCount, event.TopK,
			event.SchedulerMode, event.ShadowMode, event.ExplorationRate, event.ExplorationBudget,
			event.LowConfidenceMaxShare, event.LatencyWeight, event.ReliabilityWeight, createdAt.UTC(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.InsertOpenAISchedulerDecisionAudit(context.Background(), event))
	require.NoError(t, mock.ExpectationsWereMet())
}
