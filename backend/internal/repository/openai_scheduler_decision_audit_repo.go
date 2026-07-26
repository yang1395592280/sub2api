package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAISchedulerDecisionAuditRepository struct {
	db *sql.DB
}

func NewOpenAISchedulerDecisionAuditRepository(db *sql.DB) service.OpenAISchedulerDecisionAuditRepository {
	return &openAISchedulerDecisionAuditRepository{db: db}
}

func (r *openAISchedulerDecisionAuditRepository) InsertOpenAISchedulerDecisionAudit(
	ctx context.Context,
	event service.OpenAISchedulerDecisionAuditEvent,
) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO openai_scheduler_decision_audits (
			event_type, group_id, effective_group_id, account_source_group_id, account_source_type,
			pool_group_id, pool_fallback_reason,
			account_id, legacy_account_id,
			model_family, endpoint, transport, reason, confidence, eligibility, traffic_class,
			predicted_ttft_difference_ms, target_share, candidate_count, top_k,
			scheduler_mode, shadow_mode, exploration_rate, exploration_budget,
			low_confidence_max_share, latency_weight, reliability_weight, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
		)
	`,
		strings.TrimSpace(event.EventType), event.GroupID, event.EffectiveGroupID,
		event.AccountSourceGroupID, strings.TrimSpace(event.AccountSourceType),
		event.PoolGroupID, strings.TrimSpace(event.PoolFallbackReason),
		event.AccountID, event.LegacyAccountID,
		strings.TrimSpace(event.ModelFamily), strings.TrimSpace(event.Endpoint), strings.TrimSpace(event.Transport),
		strings.TrimSpace(event.Reason), strings.TrimSpace(event.Confidence), strings.TrimSpace(event.Eligibility), strings.TrimSpace(event.TrafficClass),
		event.PredictedTTFTDifferenceMS, event.TargetShare, event.CandidateCount, event.TopK,
		strings.TrimSpace(event.SchedulerMode), event.ShadowMode, event.ExplorationRate, event.ExplorationBudget,
		event.LowConfidenceMaxShare, event.LatencyWeight, event.ReliabilityWeight, event.CreatedAt.UTC(),
	)
	return err
}

var _ service.OpenAISchedulerDecisionAuditRepository = (*openAISchedulerDecisionAuditRepository)(nil)
