package service

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var intelligenceAnswerNumber = regexp.MustCompile(`^[0-9]{1,12}$`)

// Only the final answer is evidence. Numbers inside explanations cannot pass.
func parseIntelligenceAnswer(output string) string {
	text := strings.TrimSpace(output)
	if !intelligenceAnswerNumber.MatchString(text) {
		return ""
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

type generationTestUsage struct {
	AccountTestTokenUsage
	Estimate PelicanTest
}

// estimateTestCost uses the price file and upstream debit rate, independent of
// this installation's group selling price or recharge-adjusted cash cost.
func (s *PelicanTestService) estimateTestCost(account *Account, test PelicanTest, usage AccountTestTokenUsage, at time.Time) PelicanTest {
	out := PelicanTest{}
	if s.billing == nil || account == nil || usage.InputTokens == nil || usage.OutputTokens == nil {
		return out
	}
	rate, ok := generationTestUpstreamRate(account, at)
	if !ok {
		return out
	}
	model := account.GetMappedModel(test.ModelID)
	if usage.ModelID != "" {
		model = usage.ModelID
	}
	count := func(p *int64) int {
		if p == nil {
			return 0
		}
		return int(*p)
	}
	read, write := count(usage.CacheReadTokens), count(usage.CacheCreationTokens)
	input := count(usage.InputTokens) - read - write
	if input < 0 {
		return out
	}
	output := count(usage.OutputTokens) + count(usage.ThinkingTokens)
	if output < count(usage.OutputTokens) {
		return out
	}
	tokens := UsageTokens{InputTokens: input, OutputTokens: output, CacheReadTokens: read, CacheCreationTokens: write,
		CacheCreation5mTokens: count(usage.CacheCreation5mTokens), CacheCreation1hTokens: count(usage.CacheCreation1hTokens), CacheCreation12hTokens: count(usage.CacheCreation12hTokens)}
	cost, err := s.billing.calculateCostWithServiceTierPolicy(model, tokens, rate, usage.ServiceTier, (!account.IsOpenAI() || account.IsOpenAILongContextBillingEnabled()))
	if err != nil || cost == nil || math.IsNaN(cost.ActualCost) || math.IsInf(cost.ActualCost, 0) || cost.ActualCost < 0 {
		return out
	}
	out.EstimatedCostUSD, out.UpstreamRateMultiplier, out.CostModelID = &cost.ActualCost, &rate, model
	return out
}

func generationTestUpstreamRate(account *Account, at time.Time) (float64, bool) {
	if snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra); snapshot != nil && snapshot.Status == UpstreamBillingProbeStatusOK && snapshot.FreshUntil != nil && at.Before(*snapshot.FreshUntil) {
		if rate, ok := upstreamBillingRateAt(snapshot.Data, at); ok {
			return rate, true
		}
	}
	// Balance probes normalize these values by the recharge ratio for cash-cost
	// reporting. Restore the original rate to estimate upstream balance debits.
	if rate, ok := resolveAccountExtraNumber(account.Extra, "upstream_effective_rate_multiplier"); ok {
		rate *= account.EffectiveUpstreamRechargeRatio()
		if rate >= 0 && !math.IsNaN(rate) && !math.IsInf(rate, 0) {
			return rate, true
		}
	}
	return 0, false
}
