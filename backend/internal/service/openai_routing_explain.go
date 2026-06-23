package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

type OpenAIRoutingReasonCode string

const (
	OpenAIRoutingReasonStatusError           OpenAIRoutingReasonCode = "status_error"
	OpenAIRoutingReasonStatusInactive        OpenAIRoutingReasonCode = "status_inactive"
	OpenAIRoutingReasonManualUnschedulable   OpenAIRoutingReasonCode = "manual_unschedulable"
	OpenAIRoutingReasonRateLimited           OpenAIRoutingReasonCode = "rate_limited"
	OpenAIRoutingReasonOverloaded            OpenAIRoutingReasonCode = "overloaded"
	OpenAIRoutingReasonTempUnschedulable     OpenAIRoutingReasonCode = "temp_unschedulable"
	OpenAIRoutingReasonRuntimeBlocked        OpenAIRoutingReasonCode = "runtime_blocked"
	OpenAIRoutingReasonHealthDegraded        OpenAIRoutingReasonCode = "health_degraded"
	OpenAIRoutingReasonModelUnsupported      OpenAIRoutingReasonCode = "model_unsupported"
	OpenAIRoutingReasonCapabilityUnsupported OpenAIRoutingReasonCode = "capability_unsupported"
	OpenAIRoutingReasonTransportUnsupported  OpenAIRoutingReasonCode = "transport_unsupported"
	OpenAIRoutingReasonGroupMismatch         OpenAIRoutingReasonCode = "group_mismatch"
	OpenAIRoutingReasonPrivacyNotSet         OpenAIRoutingReasonCode = "privacy_not_set"
	OpenAIRoutingReasonQuotaAutoPaused       OpenAIRoutingReasonCode = "quota_auto_paused"
	OpenAIRoutingReasonConcurrencyFull       OpenAIRoutingReasonCode = "concurrency_full"
	OpenAIRoutingReasonChannelRestricted     OpenAIRoutingReasonCode = "channel_restricted"
	OpenAIRoutingReasonCompactUnsupported    OpenAIRoutingReasonCode = "compact_unsupported"
)

type OpenAIRoutingExplainParams struct {
	GroupID            *int64
	Model              string
	Platform           string
	RequiredCapability OpenAIEndpointCapability
	RequiredTransport  OpenAIUpstreamTransport
	RequireCompact     bool
}

type OpenAIRoutingScoreBreakdown struct {
	Total     float64 `json:"total"`
	Priority  float64 `json:"priority"`
	Load      float64 `json:"load"`
	Queue     float64 `json:"queue"`
	ErrorRate float64 `json:"error_rate"`
	TTFT      float64 `json:"ttft"`
	Price     float64 `json:"price"`
	Health    float64 `json:"health"`
}

type OpenAIRoutingQuotaDecision struct {
	Window      string    `json:"window,omitempty"`
	Threshold   float64   `json:"threshold,omitempty"`
	Utilization float64   `json:"utilization,omitempty"`
	SnapshotAt  time.Time `json:"snapshot_at"`
}

type OpenAIRoutingBlockDetail struct {
	Reason        OpenAIRoutingReasonCode     `json:"reason"`
	Source        string                      `json:"source"`
	Until         *time.Time                  `json:"until,omitempty"`
	QuotaDecision *OpenAIRoutingQuotaDecision `json:"quota_decision,omitempty"`
	SnapshotAt    time.Time                   `json:"snapshot_at"`
}

type OpenAIRoutingSummary struct {
	AccountID        int64                       `json:"account_id"`
	AccountName      string                      `json:"account_name"`
	Rank             int                         `json:"rank,omitempty"`
	Tier             string                      `json:"tier"`
	Score            OpenAIRoutingScoreBreakdown `json:"score"`
	StatusLabel      string                      `json:"status_label"`
	SummaryReason    string                      `json:"summary_reason"`
	SummaryReasons   []string                    `json:"summary_reasons"`
	IsSchedulableNow bool                        `json:"is_schedulable_now"`
	BlockReasons     []OpenAIRoutingReasonCode   `json:"block_reasons,omitempty"`
	BlockDetails     []OpenAIRoutingBlockDetail  `json:"block_details,omitempty"`
	SnapshotAt       time.Time                   `json:"snapshot_at"`
}

type OpenAIRoutingExplainResponse struct {
	Items      []OpenAIRoutingSummary `json:"items"`
	Source     string                 `json:"source"`
	SnapshotAt time.Time              `json:"snapshot_at"`
}

type OpenAIRoutingAccountExplain struct {
	Account OpenAIRoutingSummary   `json:"account"`
	Top     []OpenAIRoutingSummary `json:"top"`
	Notes   []string               `json:"notes"`
}

func (s *OpenAIGatewayService) ExplainOpenAIRouting(ctx context.Context, params OpenAIRoutingExplainParams) (*OpenAIRoutingExplainResponse, error) {
	if strings.TrimSpace(params.Platform) == "" {
		params.Platform = PlatformOpenAI
	}
	if params.RequiredTransport == "" {
		params.RequiredTransport = OpenAIUpstreamTransportAny
	}
	now := time.Now()
	if s == nil || (s.accountRepo == nil && s.schedulerSnapshot == nil) {
		return &OpenAIRoutingExplainResponse{
			Items:      []OpenAIRoutingSummary{},
			Source:     "empty",
			SnapshotAt: now,
		}, nil
	}

	accounts, err := s.listSchedulableAccounts(ctx, params.GroupID)
	if err != nil {
		return nil, err
	}
	loadMap := s.openAIRoutingLoadMap(ctx, accounts)
	summaries := make([]OpenAIRoutingSummary, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if acc.Platform != params.Platform {
			continue
		}
		summaries = append(summaries, s.explainOpenAIRoutingAccount(ctx, acc, loadMap[acc.ID], params, now))
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		a, b := summaries[i], summaries[j]
		if a.IsSchedulableNow != b.IsSchedulableNow {
			return a.IsSchedulableNow
		}
		if a.Score.Total != b.Score.Total {
			return a.Score.Total > b.Score.Total
		}
		return a.AccountID < b.AccountID
	})

	rank := 1
	for i := range summaries {
		if summaries[i].IsSchedulableNow {
			summaries[i].Rank = rank
			rank++
		}
	}

	return &OpenAIRoutingExplainResponse{
		Items:      summaries,
		Source:     "scheduler_snapshot",
		SnapshotAt: now,
	}, nil
}

func (s *OpenAIGatewayService) ExplainOpenAIRoutingForAccount(ctx context.Context, accountID int64, params OpenAIRoutingExplainParams) (*OpenAIRoutingAccountExplain, error) {
	ranking, err := s.ExplainOpenAIRouting(ctx, params)
	if err != nil {
		return nil, err
	}
	var selected *OpenAIRoutingSummary
	for i := range ranking.Items {
		if ranking.Items[i].AccountID == accountID {
			selected = &ranking.Items[i]
			break
		}
	}
	if selected == nil {
		if s == nil || s.accountRepo == nil {
			return nil, ErrAccountNotFound
		}
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil || account == nil {
			return nil, ErrAccountNotFound
		}
		if strings.TrimSpace(params.Platform) == "" {
			params.Platform = PlatformOpenAI
		}
		if account.Platform != params.Platform {
			return nil, ErrAccountNotFound
		}
		loadMap := s.openAIRoutingLoadMap(ctx, []Account{*account})
		fallback := s.explainOpenAIRoutingAccount(ctx, account, loadMap[account.ID], params, ranking.SnapshotAt)
		selected = &fallback
	}
	top := ranking.Items
	if len(top) > 10 {
		top = top[:10]
	}
	return &OpenAIRoutingAccountExplain{
		Account: *selected,
		Top:     top,
		Notes: []string{
			"sticky_may_override_ranking",
			"weighted_top_k_not_strict_best",
		},
	}, nil
}

func (s *OpenAIGatewayService) explainOpenAIRoutingAccount(ctx context.Context, account *Account, loadInfo *AccountLoadInfo, params OpenAIRoutingExplainParams, now time.Time) OpenAIRoutingSummary {
	reasons, details := s.openAIRoutingBlockReasons(ctx, account, loadInfo, params, now)
	health, ok := s.SnapshotOpenAIAccountHealth(ctx, account.ID)
	if !ok {
		health = buildOpenAIAccountHealthSnapshot(account.ID, openAIAccountHealthRuntime{successEWMA: 1}, defaultOpenAISchedulerHealthSettings(), now)
	}
	score := s.openAIRoutingScore(account, loadInfo, health)
	statusLabel := "candidate"
	isSchedulableNow := len(reasons) == 0
	if len(reasons) > 0 {
		statusLabel = "skipped"
	}
	if health.Tier == OpenAISchedulerTierDegraded {
		isSchedulableNow = false
		statusLabel = "degraded"
		reasons = appendOpenAIRoutingReason(reasons, OpenAIRoutingReasonHealthDegraded)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonHealthDegraded,
			Source:     "advanced_scheduler_health",
			Until:      health.CooldownUntil,
			SnapshotAt: now,
		})
	}

	summaryReasons := openAIRoutingSummaryReasons(account, score, health, reasons)
	summaryReason := ""
	if len(summaryReasons) > 0 {
		summaryReason = summaryReasons[0]
	}

	return OpenAIRoutingSummary{
		AccountID:        account.ID,
		AccountName:      account.Name,
		Tier:             health.Tier,
		Score:            score,
		StatusLabel:      statusLabel,
		SummaryReason:    summaryReason,
		SummaryReasons:   summaryReasons,
		IsSchedulableNow: isSchedulableNow,
		BlockReasons:     reasons,
		BlockDetails:     details,
		SnapshotAt:       now,
	}
}

func (s *OpenAIGatewayService) openAIRoutingBlockReasons(ctx context.Context, account *Account, loadInfo *AccountLoadInfo, params OpenAIRoutingExplainParams, now time.Time) ([]OpenAIRoutingReasonCode, []OpenAIRoutingBlockDetail) {
	reasons := make([]OpenAIRoutingReasonCode, 0, 4)
	details := make([]OpenAIRoutingBlockDetail, 0, 4)
	if account == nil {
		return []OpenAIRoutingReasonCode{OpenAIRoutingReasonStatusInactive}, []OpenAIRoutingBlockDetail{{
			Reason:     OpenAIRoutingReasonStatusInactive,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		}}
	}
	if account.Status == StatusError {
		reasons = append(reasons, OpenAIRoutingReasonStatusError)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonStatusError,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if account.Status != StatusActive && account.Status != StatusError {
		reasons = append(reasons, OpenAIRoutingReasonStatusInactive)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonStatusInactive,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if !account.Schedulable {
		reasons = append(reasons, OpenAIRoutingReasonManualUnschedulable)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonManualUnschedulable,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if params.GroupID != nil && !openAIStickyAccountMatchesGroup(account, params.GroupID) {
		reasons = append(reasons, OpenAIRoutingReasonGroupMismatch)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonGroupMismatch,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonRateLimited)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonRateLimited,
			Source:     "ui_countdown_state",
			Until:      account.RateLimitResetAt,
			SnapshotAt: now,
		})
	}
	if account.OverloadUntil != nil && account.OverloadUntil.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonOverloaded)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonOverloaded,
			Source:     "ui_countdown_state",
			Until:      account.OverloadUntil,
			SnapshotAt: now,
		})
	}
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonTempUnschedulable)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonTempUnschedulable,
			Source:     "ui_countdown_state",
			Until:      account.TempUnschedulableUntil,
			SnapshotAt: now,
		})
	}
	if s != nil && s.isOpenAIAccountRuntimeBlocked(account) {
		reasons = append(reasons, OpenAIRoutingReasonRuntimeBlocked)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonRuntimeBlocked,
			Source:     "runtime_block",
			SnapshotAt: now,
		})
	}
	if params.Model != "" && !account.IsModelSupported(params.Model) {
		reasons = append(reasons, OpenAIRoutingReasonModelUnsupported)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonModelUnsupported,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if params.RequiredCapability != "" && !account.SupportsOpenAIEndpointCapability(params.RequiredCapability) {
		reasons = append(reasons, OpenAIRoutingReasonCapabilityUnsupported)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonCapabilityUnsupported,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if params.RequiredTransport != "" && params.RequiredTransport != OpenAIUpstreamTransportAny && !s.isOpenAIAccountTransportCompatible(account, params.RequiredTransport) {
		reasons = append(reasons, OpenAIRoutingReasonTransportUnsupported)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonTransportUnsupported,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if paused, decision := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
		reasons = append(reasons, OpenAIRoutingReasonQuotaAutoPaused)
		detail := OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonQuotaAutoPaused,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		}
		if decision.window != "" || decision.threshold > 0 || decision.utilization > 0 {
			detail.QuotaDecision = &OpenAIRoutingQuotaDecision{
				Window:      decision.window,
				Threshold:   roundOpenAIRoutingScore(decision.threshold),
				Utilization: roundOpenAIRoutingScore(decision.utilization),
				SnapshotAt:  now,
			}
		}
		details = appendOpenAIRoutingBlockDetail(details, detail)
	}
	if params.RequireCompact && openAICompactSupportTier(account) == 0 {
		reasons = append(reasons, OpenAIRoutingReasonCompactUnsupported)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonCompactUnsupported,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	if loadInfo != nil && account.Concurrency > 0 && loadInfo.CurrentConcurrency >= account.Concurrency {
		reasons = append(reasons, OpenAIRoutingReasonConcurrencyFull)
		details = appendOpenAIRoutingBlockDetail(details, OpenAIRoutingBlockDetail{
			Reason:     OpenAIRoutingReasonConcurrencyFull,
			Source:     "persistent_account_state",
			SnapshotAt: now,
		})
	}
	return reasons, details
}

func (s *OpenAIGatewayService) openAIRoutingScore(account *Account, loadInfo *AccountLoadInfo, health OpenAIAccountHealthSnapshot) OpenAIRoutingScoreBreakdown {
	priority := 1 / (1 + float64(maxInt(account.Priority, 0)))
	load := 1.0
	queue := 1.0
	if loadInfo != nil {
		load = 1 - clamp01(float64(loadInfo.LoadRate)/100.0)
		if loadInfo.WaitingCount > 0 {
			queue = 1 / (1 + float64(loadInfo.WaitingCount))
		}
	}
	errorRate := 1 - clamp01(health.ErrorRateEWMA)
	ttft := 0.5
	if health.TTFTEWMAMS > 0 {
		ttft = 1 / (1 + health.TTFTEWMAMS/1000)
	}
	price := 1 / (1 + account.EffectiveChannelPrice())
	healthScore := clamp01(health.HealthScore / 100)

	weights := GatewayOpenAIWSSchedulerScoreWeightsView{
		Priority:  1.0,
		Load:      1.0,
		Queue:     0.7,
		ErrorRate: 0.8,
		TTFT:      0.5,
		Price:     0.6,
	}
	if s != nil {
		weights = s.openAIWSSchedulerWeights()
	}

	total := weights.Priority*priority +
		weights.Load*load +
		weights.Queue*queue +
		weights.ErrorRate*errorRate +
		weights.TTFT*ttft +
		weights.Price*price
	if health.Tier != "" {
		total = total*0.65 + healthScore*0.35
	}

	return OpenAIRoutingScoreBreakdown{
		Total:     roundOpenAIRoutingScore(total),
		Priority:  roundOpenAIRoutingScore(priority),
		Load:      roundOpenAIRoutingScore(load),
		Queue:     roundOpenAIRoutingScore(queue),
		ErrorRate: roundOpenAIRoutingScore(errorRate),
		TTFT:      roundOpenAIRoutingScore(ttft),
		Price:     roundOpenAIRoutingScore(price),
		Health:    roundOpenAIRoutingScore(healthScore),
	}
}

func (s *OpenAIGatewayService) openAIRoutingLoadMap(ctx context.Context, accounts []Account) map[int64]*AccountLoadInfo {
	if s == nil || s.concurrencyService == nil || s.concurrencyService.cache == nil || len(accounts) == 0 {
		return map[int64]*AccountLoadInfo{}
	}
	items := make([]AccountWithConcurrency, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, AccountWithConcurrency{
			ID:             account.ID,
			MaxConcurrency: account.EffectiveLoadFactor(),
		})
	}
	loadMap, err := s.concurrencyService.cache.GetAccountsLoadBatch(ctx, items)
	if err != nil || loadMap == nil {
		return map[int64]*AccountLoadInfo{}
	}
	return loadMap
}

func openAIRoutingSummaryReasons(account *Account, score OpenAIRoutingScoreBreakdown, health OpenAIAccountHealthSnapshot, reasons []OpenAIRoutingReasonCode) []string {
	if len(reasons) > 0 {
		out := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			out = append(out, string(reason))
		}
		return out
	}
	out := make([]string, 0, 4)
	if score.Price >= 0.9 {
		out = append(out, "cost_advantage")
	}
	if score.Load >= 0.9 && score.Queue >= 0.9 {
		out = append(out, "low_load")
	}
	if health.TTFTEWMAMS > 0 && health.TTFTEWMAMS <= 1000 {
		out = append(out, "low_latency")
	}
	if account != nil && account.Priority <= 5 {
		out = append(out, "high_priority")
	}
	if len(out) == 0 {
		out = append(out, "schedulable")
	}
	return out
}

func appendOpenAIRoutingReason(reasons []OpenAIRoutingReasonCode, reason OpenAIRoutingReasonCode) []OpenAIRoutingReasonCode {
	for _, item := range reasons {
		if item == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func appendOpenAIRoutingBlockDetail(details []OpenAIRoutingBlockDetail, detail OpenAIRoutingBlockDetail) []OpenAIRoutingBlockDetail {
	for _, item := range details {
		if item.Reason == detail.Reason {
			return details
		}
	}
	return append(details, detail)
}

func roundOpenAIRoutingScore(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
