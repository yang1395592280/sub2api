package service

import "time"

const (
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

type OpenAIAutoSchedulerSettings struct {
	Enabled                          bool    `json:"enabled"`
	ProbeIntervalSeconds             int     `json:"probe_interval_seconds"`
	SlowThresholdMS                  int     `json:"slow_threshold_ms"`
	SevereSlowThresholdMS            int     `json:"severe_slow_threshold_ms"`
	ConsecutiveSlowBreakerThreshold  int     `json:"consecutive_slow_breaker_threshold"`
	ConsecutiveErrorBreakerThreshold int     `json:"consecutive_error_breaker_threshold"`
	CooldownSeconds                  int     `json:"cooldown_seconds"`
	HalfOpenSuccessThreshold         int     `json:"half_open_success_threshold"`
	CostWeight                       float64 `json:"cost_weight"`
	RecoveryStep                     int     `json:"recovery_step"`
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

const (
	openAIAutoSchedulerListDefaultPageSize = 50
	openAIAutoSchedulerListMaxPageSize     = 200
)

func DefaultOpenAIAutoSchedulerSettings() OpenAIAutoSchedulerSettings {
	return OpenAIAutoSchedulerSettings{
		Enabled:                          false,
		ProbeIntervalSeconds:             60,
		SlowThresholdMS:                  10000,
		SevereSlowThresholdMS:            20000,
		ConsecutiveSlowBreakerThreshold:  3,
		ConsecutiveErrorBreakerThreshold: 2,
		CooldownSeconds:                  120,
		HalfOpenSuccessThreshold:         3,
		CostWeight:                       0.2,
		RecoveryStep:                     800,
	}
}

func normalizeOpenAIAutoSchedulerSettings(settings OpenAIAutoSchedulerSettings) OpenAIAutoSchedulerSettings {
	defaults := DefaultOpenAIAutoSchedulerSettings()
	enabled := settings.Enabled
	if settings.ProbeIntervalSeconds <= 0 {
		settings.ProbeIntervalSeconds = defaults.ProbeIntervalSeconds
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
	settings.Enabled = enabled
	return settings
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
