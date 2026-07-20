package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type OpenAISchedulerRankingParams struct {
	GroupID     int64
	Window      time.Duration
	ModelFamily string
	Endpoint    string
	Transport   string
	Eligibility string
	Page        int
	PageSize    int
}

type OpenAISchedulerRankingPartition struct {
	GroupID     int64  `json:"group_id"`
	ModelFamily string `json:"model_family"`
	Endpoint    string `json:"endpoint"`
	Transport   string `json:"transport"`
}

type OpenAISchedulerPolicyContext struct {
	EngineEnabled  bool      `json:"engine_enabled"`
	GlobalEnabled  bool      `json:"global_enabled"`
	GroupEnabled   bool      `json:"group_enabled"`
	ConfiguredMode string    `json:"configured_mode"`
	EffectiveMode  string    `json:"effective_mode"`
	ShadowMode     bool      `json:"shadow_mode"`
	FallbackReason string    `json:"fallback_reason,omitempty"`
	PolicyVersion  string    `json:"policy_version"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type OpenAISchedulerRankingActualKey struct {
	AccountID   int64
	ModelFamily string
	Endpoint    string
	Transport   string
}

type OpenAISchedulerRankingActual struct {
	Key           OpenAISchedulerRankingActualKey
	RequestCount  int64
	TTFTP50MS     float64
	TTFTP90MS     float64
	EstimatedCost float64
}

type OpenAISchedulerRankingActualParams struct {
	GroupID     int64
	StartTime   time.Time
	EndTime     time.Time
	ModelFamily string
	Endpoint    string
	Transport   string
}

type OpenAISchedulerRankingActualRepository interface {
	ListOpenAISchedulerRankingActual(context.Context, OpenAISchedulerRankingActualParams) ([]OpenAISchedulerRankingActual, error)
}

type OpenAISchedulerRankingItem struct {
	Partition         OpenAISchedulerRankingPartition `json:"partition"`
	PartitionCount    int                             `json:"partition_count"`
	Rank              int                             `json:"rank"`
	AccountID         int64                           `json:"account_id"`
	AccountName       string                          `json:"account_name"`
	Eligibility       string                          `json:"eligibility"`
	EligibilityReason string                          `json:"eligibility_reason"`
	TrafficClass      string                          `json:"traffic_class"`
	UtilityScore      float64                         `json:"utility_score"`
	TargetShare       float64                         `json:"target_share"`
	ActualShare       float64                         `json:"actual_share"`
	SelectedRequests  int64                           `json:"selected_requests"`
	PredictedTTFTMS   float64                         `json:"predicted_ttft_ms"`
	TTFTP50MS         float64                         `json:"ttft_p50_ms"`
	TTFTP90MS         float64                         `json:"ttft_p90_ms"`
	ErrorRate         float64                         `json:"error_rate"`
	RateLimitedRate   float64                         `json:"rate_limited_rate"`
	ServerErrorRate   float64                         `json:"server_error_rate"`
	LoadInflight      int                             `json:"load_inflight"`
	LoadCapacity      int                             `json:"load_capacity"`
	WaitingCount      int                             `json:"waiting_count"`
	ChannelPrice      *float64                        `json:"channel_price"`
	EstimatedCost     float64                         `json:"estimated_cost"`
	Confidence        string                          `json:"confidence"`
	RealSampleCount   int64                           `json:"real_sample_count"`
	ProbeSampleCount  int64                           `json:"probe_sample_count"`
	SnapshotAgeMS     int64                           `json:"snapshot_age_ms"`
	LatencyScore      float64                         `json:"latency_score"`
	ReliabilityScore  float64                         `json:"reliability_score"`
	CostScore         float64                         `json:"cost_score"`
	CapacityScore     float64                         `json:"capacity_score"`
	QuotaScore        float64                         `json:"quota_score"`
	PriorityScore     float64                         `json:"priority_score"`
	DeviationReasons  []string                        `json:"deviation_reasons"`
	DecisionSummary   string                          `json:"decision_summary"`
}

type OpenAISchedulerRankingSummary struct {
	CandidateCount     int   `json:"candidate_count"`
	EligibleCount      int   `json:"eligible_count"`
	RejectedCount      int   `json:"rejected_count"`
	LowConfidenceCount int   `json:"low_confidence_count"`
	RequestCount       int64 `json:"request_count"`
}

type OpenAISchedulerRankingResult struct {
	PolicyContext OpenAISchedulerPolicyContext  `json:"policy_context"`
	Summary       OpenAISchedulerRankingSummary `json:"summary"`
	Items         []OpenAISchedulerRankingItem  `json:"items"`
	Total         int64                         `json:"total"`
	Page          int                           `json:"page"`
	PageSize      int                           `json:"page_size"`
}

func (s *OpenAISchedulerOverviewService) ListRankings(ctx context.Context, params OpenAISchedulerRankingParams) (*OpenAISchedulerRankingResult, error) {
	if s == nil || s.repo == nil {
		return &OpenAISchedulerRankingResult{Items: []OpenAISchedulerRankingItem{}}, nil
	}
	if params.GroupID <= 0 {
		return nil, fmt.Errorf("group_id is required")
	}
	params = normalizeOpenAISchedulerRankingParams(params)
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	records, err := s.listOpenAISchedulerRankingHealth(ctx, params)
	if err != nil {
		return nil, err
	}
	loadMap := map[int64]*AccountLoadInfo{}
	if s.loads != nil {
		loadMap, err = s.loads.GetAccountsLoadBatch(ctx, uniqueOpenAISchedulerHealthLoadRequest(records))
		if err != nil {
			return nil, err
		}
	}
	actualByKey := map[OpenAISchedulerRankingActualKey]OpenAISchedulerRankingActual{}
	if actualRepo, ok := s.repo.(OpenAISchedulerRankingActualRepository); ok {
		actualRows, actualErr := actualRepo.ListOpenAISchedulerRankingActual(ctx, OpenAISchedulerRankingActualParams{
			GroupID: params.GroupID, StartTime: now.Add(-params.Window), EndTime: now,
			ModelFamily: params.ModelFamily, Endpoint: params.Endpoint, Transport: params.Transport,
		})
		if actualErr != nil {
			return nil, actualErr
		}
		for _, actual := range actualRows {
			actualByKey[actual.Key] = actual
		}
	}

	settings := normalizeOpenAIAutoSchedulerSettings(s.schedulerSettings(ctx))
	engineEnabled := false
	if provider, ok := s.settings.(openAISchedulerEngineSettingsProvider); ok {
		engineEnabled = provider.IsOpenAIAdvancedSchedulerEnabled(ctx)
	}
	partitions := make(map[OpenAISchedulerRankingPartition][]OpenAISchedulerHealthRecord)
	groupEnabled := false
	for _, record := range records {
		partition := OpenAISchedulerRankingPartition{
			GroupID: record.GroupID, ModelFamily: record.ModelFamily, Endpoint: record.Endpoint, Transport: record.Transport,
		}
		partitions[partition] = append(partitions[partition], record)
		groupEnabled = groupEnabled || record.GroupAutoSchedulerEnabled
	}
	partitionKeys := make([]OpenAISchedulerRankingPartition, 0, len(partitions))
	for partition := range partitions {
		partitionKeys = append(partitionKeys, partition)
	}
	sort.Slice(partitionKeys, func(i, j int) bool {
		left, right := partitionKeys[i], partitionKeys[j]
		if left.ModelFamily != right.ModelFamily {
			return left.ModelFamily < right.ModelFamily
		}
		if left.Endpoint != right.Endpoint {
			return left.Endpoint < right.Endpoint
		}
		return left.Transport < right.Transport
	})
	partitionRankings := make(map[OpenAISchedulerRankingPartition][]OpenAISchedulerRankingItem, len(partitions))
	for _, partition := range partitionKeys {
		partitionRecords := partitions[partition]
		partitionItems := buildOpenAISchedulerPartitionRanking(partition, partitionRecords, loadMap, actualByKey, settings, now)
		partitionRankings[partition] = partitionItems
	}
	allItems := aggregateOpenAISchedulerAccountRankings(params, partitionKeys, partitionRankings)
	summary := summarizeOpenAISchedulerAccountRankings(allItems)
	items := make([]OpenAISchedulerRankingItem, 0, len(allItems))
	for _, item := range allItems {
		if params.Eligibility == "" || item.Eligibility == params.Eligibility {
			items = append(items, item)
		}
	}
	total := int64(len(items))
	start := (params.Page - 1) * params.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + params.PageSize
	if end > len(items) {
		end = len(items)
	}
	effectiveMode := OpenAIAutoSchedulerModeLegacy
	shadow := false
	fallbackReason := ""
	if engineEnabled && settings.Enabled && groupEnabled {
		if settings.ShadowMode {
			shadow = true
		} else {
			effectiveMode = settings.Mode
		}
	} else {
		switch {
		case !engineEnabled:
			fallbackReason = "engine_disabled"
		case !settings.Enabled:
			fallbackReason = "global_disabled"
		case !groupEnabled:
			fallbackReason = "group_disabled"
		}
	}
	for i := range items {
		if shadow {
			items[i].DeviationReasons = appendUniqueOpenAISchedulerRankingReason(items[i].DeviationReasons, "shadow_mode")
		}
		if fallbackReason != "" {
			items[i].DeviationReasons = appendUniqueOpenAISchedulerRankingReason(items[i].DeviationReasons, "legacy_fallback")
		}
	}
	return &OpenAISchedulerRankingResult{
		PolicyContext: OpenAISchedulerPolicyContext{
			EngineEnabled: engineEnabled, GlobalEnabled: settings.Enabled, GroupEnabled: groupEnabled,
			ConfiguredMode: settings.Mode, EffectiveMode: effectiveMode, ShadowMode: shadow,
			FallbackReason: fallbackReason, PolicyVersion: "v2", CalculatedAt: now,
		},
		Summary: summary, Items: items[start:end], Total: total, Page: params.Page, PageSize: params.PageSize,
	}, nil
}

func (s *OpenAISchedulerOverviewService) listOpenAISchedulerRankingHealth(
	ctx context.Context,
	params OpenAISchedulerRankingParams,
) ([]OpenAISchedulerHealthRecord, error) {
	records := make([]OpenAISchedulerHealthRecord, 0)
	for page := 1; ; page++ {
		batch, total, err := s.repo.ListOpenAISchedulerHealth(ctx, OpenAISchedulerHealthParams{
			GroupID: params.GroupID, ModelFamily: params.ModelFamily, Endpoint: params.Endpoint,
			Transport: params.Transport, Sort: "account_id", Order: "asc", Page: page, PageSize: openAISchedulerOverviewMaxPageSize,
		})
		if err != nil {
			return nil, err
		}
		records = append(records, batch...)
		if len(batch) == 0 || total <= int64(len(records)) || len(batch) < openAISchedulerOverviewMaxPageSize {
			return records, nil
		}
	}
}

func appendUniqueOpenAISchedulerRankingReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func normalizeOpenAISchedulerRankingParams(params OpenAISchedulerRankingParams) OpenAISchedulerRankingParams {
	if params.Window <= 0 {
		params.Window = time.Hour
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > openAISchedulerOverviewMaxPageSize {
		params.PageSize = openAISchedulerOverviewMaxPageSize
	}
	params.ModelFamily = strings.ToLower(strings.TrimSpace(params.ModelFamily))
	params.Endpoint = strings.ToLower(strings.TrimSpace(params.Endpoint))
	params.Transport = strings.ToLower(strings.TrimSpace(params.Transport))
	params.Eligibility = strings.ToLower(strings.TrimSpace(params.Eligibility))
	return params
}

func buildOpenAISchedulerPartitionRanking(
	partition OpenAISchedulerRankingPartition,
	records []OpenAISchedulerHealthRecord,
	loadMap map[int64]*AccountLoadInfo,
	actualByKey map[OpenAISchedulerRankingActualKey]OpenAISchedulerRankingActual,
	settings OpenAIAutoSchedulerSettings,
	now time.Time,
) []OpenAISchedulerRankingItem {
	records = uniqueOpenAISchedulerPartitionRecords(records)
	candidates := make([]OpenAIBalancedCandidate, 0, len(records))
	recordByID := make(map[int64]OpenAISchedulerHealthRecord, len(records))
	confidenceByID := make(map[int64]string, len(records))
	for index, record := range records {
		load := loadMap[record.AccountID]
		if load == nil {
			load = &AccountLoadInfo{AccountID: record.AccountID}
		}
		decision, reason := classifyOpenAISchedulerHealthRecord(record, now)
		confidence := openAISchedulerRankingConfidence(record, settings, now)
		candidate := OpenAIBalancedCandidate{
			AccountID: record.AccountID, PredictedTTFTMS: record.PredictedTTFTMS, State: record.State,
			ErrorRate: record.ErrorRate, RateLimitedRate: record.RateLimitedRate, ServerErrorRate: record.ServerErrorRate,
			WaitingCount: load.WaitingCount, LoadRate: load.LoadRate, GroupPriority: record.GroupPriority,
			QuotaHeadroom: 0.5, LegacyOrderPosition: index, HealthConfidence: confidence,
		}
		if record.ChannelPrice != nil {
			candidate.Price = *record.ChannelPrice
		}
		if decision == OpenAISchedulerDecisionHardFiltered || decision == OpenAISchedulerDecisionCircuitRejected {
			candidate.HardRejectedReason = reason
		}
		candidates = append(candidates, candidate)
		recordByID[record.AccountID] = record
		confidenceByID[record.AccountID] = confidence
	}
	evaluation := EvaluateOpenAISchedulerPolicy(candidates, settings, 1)
	totalRequests := int64(0)
	for _, record := range records {
		key := OpenAISchedulerRankingActualKey{AccountID: record.AccountID, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport}
		totalRequests += actualByKey[key].RequestCount
	}
	items := make([]OpenAISchedulerRankingItem, 0, len(evaluation.Scores))
	for _, score := range evaluation.Scores {
		record := recordByID[score.AccountID]
		load := loadMap[score.AccountID]
		actualKey := OpenAISchedulerRankingActualKey{AccountID: score.AccountID, ModelFamily: partition.ModelFamily, Endpoint: partition.Endpoint, Transport: partition.Transport}
		actual := actualByKey[actualKey]
		actualShare := 0.0
		if totalRequests > 0 {
			actualShare = float64(actual.RequestCount) / float64(totalRequests)
		}
		item := OpenAISchedulerRankingItem{
			Partition: partition, PartitionCount: 1, Rank: score.Rank, AccountID: score.AccountID, AccountName: record.AccountName,
			Eligibility: score.Eligibility, EligibilityReason: score.EligibilityReason, TrafficClass: score.TrafficClass,
			UtilityScore: score.Utility * 100, TargetShare: score.TargetShare, ActualShare: actualShare,
			SelectedRequests: actual.RequestCount, PredictedTTFTMS: record.PredictedTTFTMS,
			TTFTP50MS: actual.TTFTP50MS, TTFTP90MS: actual.TTFTP90MS,
			ErrorRate: record.ErrorRate, RateLimitedRate: record.RateLimitedRate, ServerErrorRate: record.ServerErrorRate,
			LoadCapacity: record.LoadCapacity, ChannelPrice: record.ChannelPrice, EstimatedCost: actual.EstimatedCost,
			Confidence: confidenceByID[score.AccountID], RealSampleCount: record.RealSampleCount, ProbeSampleCount: record.ProbeSampleCount,
			LatencyScore: score.LatencyScore, ReliabilityScore: score.ReliabilityScore, CostScore: score.CostScore,
			CapacityScore: score.CapacityScore, QuotaScore: score.QuotaScore, PriorityScore: score.PriorityScore,
			DeviationReasons: openAISchedulerRankingDeviationReasons(score, actual),
			DecisionSummary:  openAISchedulerRankingDecisionSummary(score),
		}
		if load != nil {
			item.LoadInflight = load.CurrentConcurrency
			item.WaitingCount = load.WaitingCount
		}
		if !record.UpdatedAt.IsZero() {
			item.SnapshotAgeMS = now.Sub(record.UpdatedAt).Milliseconds()
			if item.SnapshotAgeMS < 0 {
				item.SnapshotAgeMS = 0
			}
		}
		items = append(items, item)
	}
	return items
}

func uniqueOpenAISchedulerPartitionRecords(records []OpenAISchedulerHealthRecord) []OpenAISchedulerHealthRecord {
	byAccountID := make(map[int64]OpenAISchedulerHealthRecord, len(records))
	for _, record := range records {
		current, exists := byAccountID[record.AccountID]
		if !exists || record.UpdatedAt.After(current.UpdatedAt) {
			byAccountID[record.AccountID] = record
		}
	}
	result := make([]OpenAISchedulerHealthRecord, 0, len(byAccountID))
	for _, record := range byAccountID {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AccountID < result[j].AccountID })
	return result
}

type openAISchedulerAccountRankingAggregate struct {
	item                  OpenAISchedulerRankingItem
	metricWeight          float64
	fallbackMetricCount   float64
	fallbackPredictedTTFT float64
	fallbackErrorRate     float64
	fallbackRateLimited   float64
	fallbackServerError   float64
	fallbackTTFTP50       float64
	fallbackTTFTP90       float64
	ttftRequestWeight     float64
	hasEligible           bool
	hasLowConfidence      bool
	hasLatencyTail        bool
	reason                string
}

func aggregateOpenAISchedulerAccountRankings(
	params OpenAISchedulerRankingParams,
	partitionKeys []OpenAISchedulerRankingPartition,
	partitionRankings map[OpenAISchedulerRankingPartition][]OpenAISchedulerRankingItem,
) []OpenAISchedulerRankingItem {
	partitionRequests := make(map[OpenAISchedulerRankingPartition]int64, len(partitionKeys))
	var totalRequests int64
	for _, partition := range partitionKeys {
		for _, item := range partitionRankings[partition] {
			partitionRequests[partition] += item.SelectedRequests
		}
		totalRequests += partitionRequests[partition]
	}

	aggregates := make(map[int64]*openAISchedulerAccountRankingAggregate)
	for _, partition := range partitionKeys {
		partitionWeight := 0.0
		if totalRequests > 0 {
			partitionWeight = float64(partitionRequests[partition]) / float64(totalRequests)
		} else if len(partitionKeys) > 0 {
			partitionWeight = 1 / float64(len(partitionKeys))
		}
		for _, source := range partitionRankings[partition] {
			aggregate := aggregates[source.AccountID]
			if aggregate == nil {
				aggregate = &openAISchedulerAccountRankingAggregate{item: OpenAISchedulerRankingItem{
					Partition: OpenAISchedulerRankingPartition{
						GroupID: params.GroupID, ModelFamily: params.ModelFamily,
						Endpoint: params.Endpoint, Transport: params.Transport,
					},
					AccountID: source.AccountID, AccountName: source.AccountName,
					Confidence: "high", DeviationReasons: []string{},
				}}
				aggregates[source.AccountID] = aggregate
			}
			aggregate.item.PartitionCount++
			if aggregate.item.PartitionCount == 1 {
				aggregate.item.Partition = source.Partition
				aggregate.item.TrafficClass = source.TrafficClass
			} else {
				aggregate.item.Partition = OpenAISchedulerRankingPartition{
					GroupID: params.GroupID, ModelFamily: params.ModelFamily,
					Endpoint: params.Endpoint, Transport: params.Transport,
				}
				if aggregate.item.TrafficClass != source.TrafficClass {
					aggregate.item.TrafficClass = "mixed"
				}
			}

			aggregate.item.TargetShare += partitionWeight * source.TargetShare
			aggregate.item.UtilityScore += partitionWeight * source.UtilityScore
			aggregate.item.LatencyScore += partitionWeight * source.LatencyScore
			aggregate.item.ReliabilityScore += partitionWeight * source.ReliabilityScore
			aggregate.item.CostScore += partitionWeight * source.CostScore
			aggregate.item.CapacityScore += partitionWeight * source.CapacityScore
			aggregate.item.QuotaScore += partitionWeight * source.QuotaScore
			aggregate.item.PriorityScore += partitionWeight * source.PriorityScore

			aggregate.item.SelectedRequests += source.SelectedRequests
			aggregate.item.EstimatedCost += source.EstimatedCost
			aggregate.item.RealSampleCount += source.RealSampleCount
			aggregate.item.ProbeSampleCount += source.ProbeSampleCount
			if source.SnapshotAgeMS > aggregate.item.SnapshotAgeMS {
				aggregate.item.SnapshotAgeMS = source.SnapshotAgeMS
			}
			if source.LoadInflight > aggregate.item.LoadInflight {
				aggregate.item.LoadInflight = source.LoadInflight
			}
			if source.LoadCapacity > aggregate.item.LoadCapacity {
				aggregate.item.LoadCapacity = source.LoadCapacity
			}
			if source.WaitingCount > aggregate.item.WaitingCount {
				aggregate.item.WaitingCount = source.WaitingCount
			}
			if aggregate.item.ChannelPrice == nil && source.ChannelPrice != nil {
				price := *source.ChannelPrice
				aggregate.item.ChannelPrice = &price
			}
			aggregate.item.Confidence = lowerOpenAISchedulerRankingConfidence(aggregate.item.Confidence, source.Confidence)
			for _, reason := range source.DeviationReasons {
				aggregate.item.DeviationReasons = appendUniqueOpenAISchedulerRankingReason(aggregate.item.DeviationReasons, reason)
			}

			aggregate.metricWeight += partitionWeight
			aggregate.item.PredictedTTFTMS += partitionWeight * source.PredictedTTFTMS
			aggregate.item.ErrorRate += partitionWeight * source.ErrorRate
			aggregate.item.RateLimitedRate += partitionWeight * source.RateLimitedRate
			aggregate.item.ServerErrorRate += partitionWeight * source.ServerErrorRate
			aggregate.fallbackMetricCount++
			aggregate.fallbackPredictedTTFT += source.PredictedTTFTMS
			aggregate.fallbackErrorRate += source.ErrorRate
			aggregate.fallbackRateLimited += source.RateLimitedRate
			aggregate.fallbackServerError += source.ServerErrorRate
			aggregate.fallbackTTFTP50 += source.TTFTP50MS
			aggregate.fallbackTTFTP90 += source.TTFTP90MS
			if source.SelectedRequests > 0 {
				requestWeight := float64(source.SelectedRequests)
				aggregate.item.TTFTP50MS += requestWeight * source.TTFTP50MS
				aggregate.item.TTFTP90MS += requestWeight * source.TTFTP90MS
				aggregate.ttftRequestWeight += requestWeight
			}

			switch source.Eligibility {
			case OpenAISchedulerEligibilityEligible:
				aggregate.hasEligible = true
			case OpenAISchedulerEligibilityLowConfidence:
				aggregate.hasLowConfidence = true
			case OpenAISchedulerEligibilityLatencyTail:
				aggregate.hasLatencyTail = true
			}
			if aggregate.reason == "" && source.EligibilityReason != "" {
				aggregate.reason = source.EligibilityReason
			}
		}
	}

	items := make([]OpenAISchedulerRankingItem, 0, len(aggregates))
	for _, aggregate := range aggregates {
		item := aggregate.item
		if totalRequests > 0 {
			item.ActualShare = float64(item.SelectedRequests) / float64(totalRequests)
		}
		if aggregate.metricWeight > 0 {
			item.PredictedTTFTMS /= aggregate.metricWeight
			item.ErrorRate /= aggregate.metricWeight
			item.RateLimitedRate /= aggregate.metricWeight
			item.ServerErrorRate /= aggregate.metricWeight
		} else if aggregate.fallbackMetricCount > 0 {
			item.PredictedTTFTMS = aggregate.fallbackPredictedTTFT / aggregate.fallbackMetricCount
			item.ErrorRate = aggregate.fallbackErrorRate / aggregate.fallbackMetricCount
			item.RateLimitedRate = aggregate.fallbackRateLimited / aggregate.fallbackMetricCount
			item.ServerErrorRate = aggregate.fallbackServerError / aggregate.fallbackMetricCount
		}
		if aggregate.ttftRequestWeight > 0 {
			item.TTFTP50MS /= aggregate.ttftRequestWeight
			item.TTFTP90MS /= aggregate.ttftRequestWeight
		} else if aggregate.fallbackMetricCount > 0 {
			item.TTFTP50MS = aggregate.fallbackTTFTP50 / aggregate.fallbackMetricCount
			item.TTFTP90MS = aggregate.fallbackTTFTP90 / aggregate.fallbackMetricCount
		}
		switch {
		case aggregate.hasEligible:
			item.Eligibility = OpenAISchedulerEligibilityEligible
		case aggregate.hasLowConfidence:
			item.Eligibility = OpenAISchedulerEligibilityLowConfidence
			item.EligibilityReason = aggregate.reason
		case aggregate.hasLatencyTail:
			item.Eligibility = OpenAISchedulerEligibilityLatencyTail
			item.EligibilityReason = aggregate.reason
		default:
			item.Eligibility = OpenAISchedulerEligibilityRejected
			item.EligibilityReason = aggregate.reason
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		leftRanked := items[i].Eligibility == OpenAISchedulerEligibilityEligible || items[i].Eligibility == OpenAISchedulerEligibilityLowConfidence
		rightRanked := items[j].Eligibility == OpenAISchedulerEligibilityEligible || items[j].Eligibility == OpenAISchedulerEligibilityLowConfidence
		if leftRanked != rightRanked {
			return leftRanked
		}
		if items[i].UtilityScore != items[j].UtilityScore {
			return items[i].UtilityScore > items[j].UtilityScore
		}
		if items[i].TargetShare != items[j].TargetShare {
			return items[i].TargetShare > items[j].TargetShare
		}
		return items[i].AccountID < items[j].AccountID
	})
	rank := 0
	for index := range items {
		if items[index].Eligibility == OpenAISchedulerEligibilityEligible || items[index].Eligibility == OpenAISchedulerEligibilityLowConfidence {
			rank++
			items[index].Rank = rank
		}
		items[index].DecisionSummary = openAISchedulerRankingDecisionSummary(OpenAISchedulerPolicyCandidateScore{
			Rank: items[index].Rank, EligibilityReason: items[index].EligibilityReason, TargetShare: items[index].TargetShare,
		})
	}
	return items
}

func summarizeOpenAISchedulerAccountRankings(items []OpenAISchedulerRankingItem) OpenAISchedulerRankingSummary {
	summary := OpenAISchedulerRankingSummary{CandidateCount: len(items)}
	for _, item := range items {
		summary.RequestCount += item.SelectedRequests
		switch item.Eligibility {
		case OpenAISchedulerEligibilityEligible:
			summary.EligibleCount++
		case OpenAISchedulerEligibilityLowConfidence:
			summary.LowConfidenceCount++
		default:
			summary.RejectedCount++
		}
	}
	return summary
}

func lowerOpenAISchedulerRankingConfidence(left, right string) string {
	level := map[string]int{"low": 1, "medium": 2, "high": 3}
	if level[right] < level[left] {
		return right
	}
	return left
}

func openAISchedulerRankingDeviationReasons(score OpenAISchedulerPolicyCandidateScore, actual OpenAISchedulerRankingActual) []string {
	reasons := make([]string, 0, 2)
	if score.Eligibility == OpenAISchedulerEligibilityLowConfidence {
		reasons = append(reasons, "health_low_confidence")
	}
	if actual.RequestCount < 10 {
		reasons = append(reasons, "insufficient_window_samples")
	}
	return reasons
}

func openAISchedulerRankingConfidence(record OpenAISchedulerHealthRecord, settings OpenAIAutoSchedulerSettings, now time.Time) string {
	if record.ExpiresAt.IsZero() || !now.Before(record.ExpiresAt) || record.ModelFamily == "" || record.Endpoint == "" || record.Transport == "" {
		return "low"
	}
	if record.RealSampleCount > 0 && !record.UpdatedAt.IsZero() && now.Sub(record.UpdatedAt) <= time.Duration(settings.RealSampleFreshSeconds)*time.Second {
		return "high"
	}
	if record.RealSampleCount+record.ProbeSampleCount > 0 {
		return "medium"
	}
	return "low"
}

func openAISchedulerRankingDecisionSummary(score OpenAISchedulerPolicyCandidateScore) string {
	if score.EligibilityReason != "" {
		return score.EligibilityReason
	}
	if score.Rank == 1 {
		return "highest_utility"
	}
	if score.TargetShare > 0 {
		return "weighted_allocation"
	}
	return "fallback_only"
}
