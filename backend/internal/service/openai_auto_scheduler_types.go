package service

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	OpenAIAutoSchedulerDefaultProbeModel = "gpt-5.4"
	OpenAIAutoSchedulerModeLegacy        = "legacy"
	OpenAIAutoSchedulerModeBalanced      = "balanced"
	OpenAIAutoSchedulerModePerformance   = "performance_first"
	OpenAIAutoSchedulerModeCost          = "cost_first"
	OpenAIAutoSchedulerModeEfficiency    = "efficiency"

	OpenAIAutoSchedulerStateRunning   = "running"
	OpenAIAutoSchedulerStateObserving = "observing"
	OpenAIAutoSchedulerStateOpen      = "open"
	OpenAIAutoSchedulerStateHalfOpen  = "half_open"

	OpenAIAutoSchedulerEventSuccess      = "success"
	OpenAIAutoSchedulerEventSlow         = "slow"
	OpenAIAutoSchedulerEventSevereSlow   = "severe_slow"
	OpenAIAutoSchedulerEventError        = "error"
	OpenAIAutoSchedulerEventRateLimited  = "rate_limited"
	OpenAIAutoSchedulerEventProbeSuccess = "probe_success"
	OpenAIAutoSchedulerEventProbeError   = "probe_error"
	OpenAIAutoSchedulerEventManualReset  = "manual_reset"
)

type OpenAISchedulerPolicyWeights struct {
	Latency     float64 `json:"latency"`
	Reliability float64 `json:"reliability"`
	Cost        float64 `json:"cost"`
	Capacity    float64 `json:"capacity"`
	Quota       float64 `json:"quota"`
	Priority    float64 `json:"priority"`
}

type OpenAIAutoSchedulerSettings struct {
	Enabled                          bool                         `json:"enabled"`
	Mode                             string                       `json:"mode"`
	ShadowMode                       bool                         `json:"shadow_mode"`
	TopK                             int                          `json:"top_k"`
	ExplorationRate                  float64                      `json:"exploration_rate"`
	SessionEscapeMinGapMS            int                          `json:"session_escape_min_gap_ms"`
	SessionEscapeRatio               float64                      `json:"session_escape_ratio"`
	HealthTTLSeconds                 int                          `json:"health_ttl_seconds"`
	RealSampleFreshSeconds           int                          `json:"real_sample_fresh_seconds"`
	ProbeJitterSeconds               int                          `json:"probe_jitter_seconds"`
	ProbeModel                       string                       `json:"probe_model"`
	ProbeIntervalSeconds             int                          `json:"probe_interval_seconds"`
	SlowThresholdMS                  int                          `json:"slow_threshold_ms"`
	SevereSlowThresholdMS            int                          `json:"severe_slow_threshold_ms"`
	ConsecutiveSlowBreakerThreshold  int                          `json:"consecutive_slow_breaker_threshold"`
	ConsecutiveErrorBreakerThreshold int                          `json:"consecutive_error_breaker_threshold"`
	CooldownSeconds                  int                          `json:"cooldown_seconds"`
	HalfOpenSuccessThreshold         int                          `json:"half_open_success_threshold"`
	CostWeight                       float64                      `json:"cost_weight"`
	RecoveryStep                     int                          `json:"recovery_step"`
	Temperature                      float64                      `json:"temperature"`
	MaxAccountShare                  float64                      `json:"max_account_share"`
	LowConfidenceMaxShare            float64                      `json:"low_confidence_max_share"`
	LatencyBudgetMS                  int                          `json:"latency_budget_ms"`
	Weights                          OpenAISchedulerPolicyWeights `json:"weights"`

	modeSet                   bool
	shadowModeSet             bool
	topKSet                   bool
	explorationRateSet        bool
	sessionEscapeMinGapMSSet  bool
	sessionEscapeRatioSet     bool
	healthTTLSecondsSet       bool
	realSampleFreshSecondsSet bool
	probeJitterSecondsSet     bool
	costWeightSet             bool
	temperatureSet            bool
	maxAccountShareSet        bool
	lowConfidenceMaxShareSet  bool
	latencyBudgetMSSet        bool
	weightsSet                bool
}

func (s *OpenAIAutoSchedulerSettings) UnmarshalJSON(data []byte) error {
	type settingsAlias OpenAIAutoSchedulerSettings
	defaults := DefaultOpenAIAutoSchedulerSettings()
	*s = defaults
	if err := json.Unmarshal(data, (*settingsAlias)(s)); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, s.modeSet = fields["mode"]
	_, s.shadowModeSet = fields["shadow_mode"]
	_, s.topKSet = fields["top_k"]
	_, s.explorationRateSet = fields["exploration_rate"]
	_, s.sessionEscapeMinGapMSSet = fields["session_escape_min_gap_ms"]
	_, s.sessionEscapeRatioSet = fields["session_escape_ratio"]
	_, s.healthTTLSecondsSet = fields["health_ttl_seconds"]
	_, s.realSampleFreshSecondsSet = fields["real_sample_fresh_seconds"]
	_, s.probeJitterSecondsSet = fields["probe_jitter_seconds"]
	_, s.costWeightSet = fields["cost_weight"]
	_, s.temperatureSet = fields["temperature"]
	_, s.maxAccountShareSet = fields["max_account_share"]
	_, s.lowConfidenceMaxShareSet = fields["low_confidence_max_share"]
	_, s.latencyBudgetMSSet = fields["latency_budget_ms"]
	_, s.weightsSet = fields["weights"]
	return nil
}

type OpenAIAutoSchedulerScoreState struct {
	AccountID               int64
	AccountName             string
	ChannelPrice            *float64
	GroupID                 int64
	Model                   string
	BaseScore               int
	FinalScore              int
	LatencyScore            int
	ErrorScore              int
	RecoveryScore           int
	CostScore               int
	State                   string
	ConsecutiveSlowCount    int
	ConsecutiveErrorCount   int
	ConsecutiveSuccessCount int
	RequestCount            int64
	TtfbSampleCount         int64
	SlowRate                float64
	ErrorRate               float64
	StuckRate               float64
	CooldownUntil           *time.Time
	LastLatencyMS           *int
	LastTtfbMS              *int
	LastStatusCode          *int
	LastError               *string
	Reason                  string
	LastCheckedAt           *time.Time
}

type OpenAIAutoSchedulerEventInput struct {
	EventType  string
	LatencyMS  *int
	TtfbMS     *int
	StatusCode *int
	Message    string
	CostScore  *int
}

type OpenAIAutoSchedulerProbeResult struct {
	Success   bool
	LatencyMS *int
	TtfbMS    *int
	Message   string
	Err       error
}

type OpenAIAutoSchedulerDailySample struct {
	AccountID       int64
	RequestCount    int64
	TtfbSampleCount int64
	LastTtfbMS      *int
}

type OpenAIAutoSchedulerAccountSummary struct {
	State         string
	SpeedPriority int
	SpeedMS       *int
	ProbeModel    string
	LastTtfbMS    *int
	LastLatencyMS *int
	LastError     *string
	Reason        string
	LastCheckedAt *time.Time
}

const (
	openAIAutoSchedulerListDefaultPageSize = 50
	openAIAutoSchedulerListMaxPageSize     = 200
)

func DefaultOpenAIAutoSchedulerSettings() OpenAIAutoSchedulerSettings {
	return OpenAIAutoSchedulerSettings{
		Enabled:                          false,
		Mode:                             OpenAIAutoSchedulerModeBalanced,
		ShadowMode:                       true,
		TopK:                             3,
		ExplorationRate:                  0.03,
		SessionEscapeMinGapMS:            1000,
		SessionEscapeRatio:               0.25,
		HealthTTLSeconds:                 int(openAISchedulerHealthStateTTL / time.Second),
		RealSampleFreshSeconds:           openAISchedulerHealthRealFreshSeconds,
		ProbeJitterSeconds:               0,
		ProbeModel:                       OpenAIAutoSchedulerDefaultProbeModel,
		ProbeIntervalSeconds:             60,
		SlowThresholdMS:                  10000,
		SevereSlowThresholdMS:            20000,
		ConsecutiveSlowBreakerThreshold:  3,
		ConsecutiveErrorBreakerThreshold: 2,
		CooldownSeconds:                  120,
		HalfOpenSuccessThreshold:         3,
		CostWeight:                       0.2,
		RecoveryStep:                     800,
		Temperature:                      0.18,
		MaxAccountShare:                  0.70,
		LowConfidenceMaxShare:            0.10,
		LatencyBudgetMS:                  1000,
		Weights:                          defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModeBalanced),
		modeSet:                          true,
		shadowModeSet:                    true,
		topKSet:                          true,
		explorationRateSet:               true,
		sessionEscapeMinGapMSSet:         true,
		sessionEscapeRatioSet:            true,
		healthTTLSecondsSet:              true,
		realSampleFreshSecondsSet:        true,
		probeJitterSecondsSet:            true,
		costWeightSet:                    true,
		temperatureSet:                   true,
		maxAccountShareSet:               true,
		lowConfidenceMaxShareSet:         true,
		latencyBudgetMSSet:               true,
		weightsSet:                       true,
	}
}

func normalizeOpenAIAutoSchedulerSettings(settings OpenAIAutoSchedulerSettings) OpenAIAutoSchedulerSettings {
	defaults := DefaultOpenAIAutoSchedulerSettings()
	enabled := settings.Enabled
	settings.Mode = strings.ToLower(strings.TrimSpace(settings.Mode))
	if !settings.modeSet || !isSupportedOpenAISchedulerMode(settings.Mode) {
		settings.Mode = defaults.Mode
	}
	if !settings.shadowModeSet {
		settings.ShadowMode = defaults.ShadowMode
	}
	if !settings.topKSet || settings.TopK <= 0 {
		settings.TopK = defaults.TopK
	} else if settings.TopK > 10 {
		settings.TopK = 10
	}
	if !settings.explorationRateSet {
		settings.ExplorationRate = defaults.ExplorationRate
	} else if settings.ExplorationRate < 0 {
		settings.ExplorationRate = 0
	} else if settings.ExplorationRate > 0.10 {
		settings.ExplorationRate = 0.10
	}
	if !settings.sessionEscapeMinGapMSSet {
		settings.SessionEscapeMinGapMS = defaults.SessionEscapeMinGapMS
	} else if settings.SessionEscapeMinGapMS < 0 {
		settings.SessionEscapeMinGapMS = 0
	} else if settings.SessionEscapeMinGapMS > 30000 {
		settings.SessionEscapeMinGapMS = 30000
	}
	if !settings.sessionEscapeRatioSet {
		settings.SessionEscapeRatio = defaults.SessionEscapeRatio
	} else if settings.SessionEscapeRatio < 0 {
		settings.SessionEscapeRatio = 0
	} else if settings.SessionEscapeRatio > 2 {
		settings.SessionEscapeRatio = 2
	}
	if !settings.healthTTLSecondsSet || settings.HealthTTLSeconds <= 0 {
		settings.HealthTTLSeconds = defaults.HealthTTLSeconds
	} else if settings.HealthTTLSeconds < 60 {
		settings.HealthTTLSeconds = 60
	} else if settings.HealthTTLSeconds > 86400 {
		settings.HealthTTLSeconds = 86400
	}
	if !settings.realSampleFreshSecondsSet || settings.RealSampleFreshSeconds <= 0 {
		settings.RealSampleFreshSeconds = defaults.RealSampleFreshSeconds
	} else if settings.RealSampleFreshSeconds < 30 {
		settings.RealSampleFreshSeconds = 30
	} else if settings.RealSampleFreshSeconds > 3600 {
		settings.RealSampleFreshSeconds = 3600
	}
	settings.ProbeModel = strings.TrimSpace(settings.ProbeModel)
	if settings.ProbeModel == "" {
		settings.ProbeModel = defaults.ProbeModel
	}
	if settings.ProbeIntervalSeconds <= 0 {
		settings.ProbeIntervalSeconds = defaults.ProbeIntervalSeconds
	}
	if !settings.probeJitterSecondsSet || settings.ProbeJitterSeconds < 0 {
		settings.ProbeJitterSeconds = defaults.ProbeJitterSeconds
	}
	if maxJitter := settings.ProbeIntervalSeconds / 2; settings.ProbeJitterSeconds > maxJitter {
		settings.ProbeJitterSeconds = maxJitter
	}
	if settings.SlowThresholdMS <= 0 {
		settings.SlowThresholdMS = defaults.SlowThresholdMS
	}
	if settings.SevereSlowThresholdMS < settings.SlowThresholdMS {
		if settings.SevereSlowThresholdMS <= 0 {
			settings.SevereSlowThresholdMS = defaults.SevereSlowThresholdMS
		}
		if settings.SevereSlowThresholdMS < settings.SlowThresholdMS {
			settings.SevereSlowThresholdMS = settings.SlowThresholdMS
		}
	}
	if settings.ConsecutiveSlowBreakerThreshold <= 0 {
		settings.ConsecutiveSlowBreakerThreshold = defaults.ConsecutiveSlowBreakerThreshold
	}
	if settings.ConsecutiveErrorBreakerThreshold <= 0 {
		settings.ConsecutiveErrorBreakerThreshold = defaults.ConsecutiveErrorBreakerThreshold
	}
	if settings.CooldownSeconds <= 0 {
		settings.CooldownSeconds = defaults.CooldownSeconds
	}
	if settings.HalfOpenSuccessThreshold <= 0 {
		settings.HalfOpenSuccessThreshold = defaults.HalfOpenSuccessThreshold
	}
	if settings.CostWeight < 0 {
		settings.CostWeight = 0
	}
	if settings.CostWeight > 1 {
		settings.CostWeight = 1
	}
	if settings.RecoveryStep <= 0 {
		settings.RecoveryStep = defaults.RecoveryStep
	}
	if !settings.temperatureSet || settings.Temperature <= 0 {
		settings.Temperature = defaultOpenAISchedulerPolicyTemperature(settings.Mode)
	} else if settings.Temperature > 1 {
		settings.Temperature = 1
	}
	if !settings.maxAccountShareSet || settings.MaxAccountShare <= 0 {
		settings.MaxAccountShare = defaultOpenAISchedulerMaxAccountShare(settings.Mode)
	} else if settings.MaxAccountShare > 1 {
		settings.MaxAccountShare = 1
	}
	if !settings.lowConfidenceMaxShareSet || settings.LowConfidenceMaxShare <= 0 {
		settings.LowConfidenceMaxShare = defaults.LowConfidenceMaxShare
	} else if settings.LowConfidenceMaxShare > 1 {
		settings.LowConfidenceMaxShare = 1
	}
	if !settings.latencyBudgetMSSet || settings.LatencyBudgetMS <= 0 {
		settings.LatencyBudgetMS = defaults.LatencyBudgetMS
	} else if settings.LatencyBudgetMS > 30000 {
		settings.LatencyBudgetMS = 30000
	}
	if !settings.weightsSet || openAISchedulerPolicyWeightSum(settings.Weights) <= 0 {
		settings.Weights = defaultOpenAISchedulerPolicyWeights(settings.Mode)
		if settings.costWeightSet {
			settings.Weights.Cost = settings.CostWeight
			settings.Weights = normalizeOpenAISchedulerPolicyWeights(settings.Weights)
		}
	} else {
		settings.Weights = normalizeOpenAISchedulerPolicyWeights(settings.Weights)
	}
	settings.Enabled = enabled
	return settings
}

func isSupportedOpenAISchedulerMode(mode string) bool {
	switch mode {
	case OpenAIAutoSchedulerModeLegacy,
		OpenAIAutoSchedulerModeBalanced,
		OpenAIAutoSchedulerModePerformance,
		OpenAIAutoSchedulerModeCost,
		OpenAIAutoSchedulerModeEfficiency:
		return true
	default:
		return false
	}
}

func IsSupportedOpenAISchedulerMode(mode string) bool {
	return isSupportedOpenAISchedulerMode(strings.ToLower(strings.TrimSpace(mode)))
}

func defaultOpenAISchedulerPolicyWeights(mode string) OpenAISchedulerPolicyWeights {
	switch mode {
	case OpenAIAutoSchedulerModePerformance:
		return OpenAISchedulerPolicyWeights{Latency: 0.55, Reliability: 0.25, Cost: 0.05, Capacity: 0.10, Quota: 0.03, Priority: 0.02}
	case OpenAIAutoSchedulerModeCost:
		return OpenAISchedulerPolicyWeights{Latency: 0.20, Reliability: 0.25, Cost: 0.40, Capacity: 0.08, Quota: 0.04, Priority: 0.03}
	case OpenAIAutoSchedulerModeEfficiency:
		return OpenAISchedulerPolicyWeights{Latency: 0.35, Reliability: 0.25, Cost: 0.25, Capacity: 0.08, Quota: 0.04, Priority: 0.03}
	default:
		return OpenAISchedulerPolicyWeights{Latency: 0.35, Reliability: 0.25, Cost: 0.15, Capacity: 0.15, Quota: 0.05, Priority: 0.05}
	}
}

func defaultOpenAISchedulerPolicyTemperature(mode string) float64 {
	switch mode {
	case OpenAIAutoSchedulerModePerformance:
		return 0.10
	case OpenAIAutoSchedulerModeCost:
		return 0.16
	case OpenAIAutoSchedulerModeEfficiency:
		return 0.14
	default:
		return 0.18
	}
}

func defaultOpenAISchedulerMaxAccountShare(mode string) float64 {
	switch mode {
	case OpenAIAutoSchedulerModePerformance:
		return 0.85
	case OpenAIAutoSchedulerModeCost:
		return 0.75
	case OpenAIAutoSchedulerModeEfficiency:
		return 0.80
	default:
		return 0.70
	}
}

func openAISchedulerPolicyWeightSum(weights OpenAISchedulerPolicyWeights) float64 {
	return weights.Latency + weights.Reliability + weights.Cost + weights.Capacity + weights.Quota + weights.Priority
}

func normalizeOpenAISchedulerPolicyWeights(weights OpenAISchedulerPolicyWeights) OpenAISchedulerPolicyWeights {
	weights.Latency = clamp01(weights.Latency)
	weights.Reliability = clamp01(weights.Reliability)
	weights.Cost = clamp01(weights.Cost)
	weights.Capacity = clamp01(weights.Capacity)
	weights.Quota = clamp01(weights.Quota)
	weights.Priority = clamp01(weights.Priority)
	sum := openAISchedulerPolicyWeightSum(weights)
	if sum <= 0 {
		return defaultOpenAISchedulerPolicyWeights(OpenAIAutoSchedulerModeBalanced)
	}
	weights.Latency /= sum
	weights.Reliability /= sum
	weights.Cost /= sum
	weights.Capacity /= sum
	weights.Quota /= sum
	weights.Priority /= sum
	return weights
}

func normalizeOpenAIAutoSchedulerListPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = openAIAutoSchedulerListDefaultPageSize
	}
	if pageSize > openAIAutoSchedulerListMaxPageSize {
		pageSize = openAIAutoSchedulerListMaxPageSize
	}
	return page, pageSize
}

func openAIAutoSchedulerSpeedMS(state OpenAIAutoSchedulerScoreState) (int, bool) {
	if state.LastTtfbMS != nil && *state.LastTtfbMS > 0 {
		return *state.LastTtfbMS, true
	}
	if state.LastLatencyMS != nil && *state.LastLatencyMS > 0 {
		return *state.LastLatencyMS, true
	}
	return 0, false
}
