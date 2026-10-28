// Package accountquality owns account-bound probes and their independent routing evidence.
package accountquality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const RoutingMode = "openai_auto_cheapest"
const SettingsKey = "openai_account_quality"
const MaxResponseBytes = 64 << 10

type Error struct {
	Status               int
	Code, Message, Field string
}

func (e *Error) Error() string { return e.Message }
func Fail(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}
func Invalid(field, message string) error {
	return &Error{Status: 400, Code: "QUALITY_INVALID_CONFIG", Message: message, Field: field}
}

type Settings struct {
	Revision                    int64  `json:"revision"`
	ControlEpoch                int64  `json:"control_epoch"`
	AutomationEnabled           bool   `json:"automation_enabled"`
	ControlEnabled              bool   `json:"control_enabled"`
	GlobalConcurrency           int    `json:"global_concurrency"`
	QueueLimit                  int    `json:"queue_limit"`
	QueueTimeoutSeconds         int    `json:"queue_timeout_seconds"`
	GlobalDailyCallLimit        int    `json:"global_daily_call_limit"`
	DefaultPolicyDailyCallLimit int    `json:"default_policy_daily_call_limit"`
	RawRetentionDays            int    `json:"raw_retention_days"`
	MetadataRetentionDays       int    `json:"metadata_retention_days"`
	StoreFailurePolicy          string `json:"store_failure_policy"`
}

func DefaultSettings() Settings {
	return Settings{Revision: 1, GlobalConcurrency: 3, QueueLimit: 300, QueueTimeoutSeconds: 300, GlobalDailyCallLimit: 1000, DefaultPolicyDailyCallLimit: 100, RawRetentionDays: 7, MetadataRetentionDays: 30, StoreFailurePolicy: "fail_closed"}
}
func (s Settings) Validate() error {
	if s.GlobalConcurrency < 1 || s.GlobalConcurrency > 10 {
		return Invalid("global_concurrency", "并发必须为1–10")
	}
	if s.QueueLimit < 100 || s.QueueLimit > 1000 {
		return Invalid("queue_limit", "队列必须为100–1000")
	}
	if s.QueueTimeoutSeconds < 60 || s.QueueTimeoutSeconds > 1800 {
		return Invalid("queue_timeout_seconds", "排队超时必须为60–1800秒")
	}
	if s.GlobalDailyCallLimit < 1 || s.GlobalDailyCallLimit > 100000 {
		return Invalid("global_daily_call_limit", "全局每日上限必须为1–100000")
	}
	if s.DefaultPolicyDailyCallLimit < 1 || s.DefaultPolicyDailyCallLimit > 1000 {
		return Invalid("default_policy_daily_call_limit", "策略默认日上限必须为1–1000")
	}
	if s.RawRetentionDays < 1 || s.RawRetentionDays > 30 || s.MetadataRetentionDays < 7 || s.MetadataRetentionDays > 180 || s.RawRetentionDays > s.MetadataRetentionDays {
		return Invalid("raw_retention_days", "正文保留1–30天，元数据7–180天，正文不得超过元数据保留期")
	}
	if s.StoreFailurePolicy != "fail_closed" {
		return Invalid("store_failure_policy", "当前只支持fail_closed")
	}
	return nil
}

type Condition struct {
	ModelID          string `json:"model_id"`
	Protocol         string `json:"protocol"`
	Transport        string `json:"transport"`
	ReasoningEffort  string `json:"reasoning_effort"`
	ServiceTier      string `json:"service_tier"`
	BenchmarkID      string `json:"benchmark_id"`
	BenchmarkVersion string `json:"benchmark_version"`
	ScorerVersion    string `json:"scorer_version"`
}

func (c Condition) Validate() error {
	if strings.TrimSpace(c.ModelID) == "" || len(c.ModelID) > 255 {
		return Invalid("model_id", "请选择有效文本模型")
	}
	if c.Protocol != "responses" && c.Protocol != "chat_completions" {
		return Invalid("protocol", "不支持的协议")
	}
	if c.Transport != "http" && !(c.Transport == "websocket_v2" && c.Protocol == "responses") {
		return Invalid("transport", "不支持的传输组合")
	}
	if c.BenchmarkID != "candy" || c.BenchmarkVersion != "1" || c.ScorerVersion != "strict-json-v1" {
		return Invalid("benchmark_id", "不支持的题目或评分版本")
	}
	if c.ReasoningEffort == "" || c.ServiceTier == "" {
		return Invalid("condition", "effort与档位缺省值必须使用upstream_default")
	}
	return nil
}

type Rules struct {
	AutoCheckEnabled        bool `json:"auto_check_enabled"`
	ControlEnabled          bool `json:"control_enabled"`
	IntervalSeconds         int  `json:"interval_seconds"`
	JitterSeconds           int  `json:"jitter_seconds"`
	FailureThreshold        int  `json:"failure_threshold"`
	RecoveryThreshold       int  `json:"recovery_threshold"`
	CooldownSeconds         int  `json:"cooldown_seconds"`
	RecoveryIntervalSeconds int  `json:"recovery_interval_seconds"`
	EvidenceWindowSeconds   int  `json:"evidence_window_seconds"`
	TimeoutSeconds          int  `json:"timeout_seconds"`
	MaxOutputTokens         int  `json:"max_output_tokens"`
	DailyCallLimit          int  `json:"daily_call_limit"`
}

func DefaultRules(limit int) Rules {
	return Rules{IntervalSeconds: 1800, JitterSeconds: 60, FailureThreshold: 3, RecoveryThreshold: 2, CooldownSeconds: 1800, RecoveryIntervalSeconds: 600, EvidenceWindowSeconds: 7200, TimeoutSeconds: 180, MaxOutputTokens: 4096, DailyCallLimit: limit}
}
func (r Rules) Validate() error {
	limits := []struct {
		name          string
		value, lo, hi int
	}{
		{"interval_seconds", r.IntervalSeconds, 300, 86400}, {"recovery_interval_seconds", r.RecoveryIntervalSeconds, 300, 86400},
		{"failure_threshold", r.FailureThreshold, 1, 10}, {"recovery_threshold", r.RecoveryThreshold, 1, 10},
		{"cooldown_seconds", r.CooldownSeconds, 300, 86400}, {"evidence_window_seconds", r.EvidenceWindowSeconds, 600, 604800},
		{"timeout_seconds", r.TimeoutSeconds, 30, 600}, {"max_output_tokens", r.MaxOutputTokens, 512, 16384}, {"daily_call_limit", r.DailyCallLimit, 1, 1000},
	}
	for _, l := range limits {
		if l.value < l.lo || l.value > l.hi {
			return Invalid(l.name, fmt.Sprintf("%s必须为%d–%d", l.name, l.lo, l.hi))
		}
	}
	if r.JitterSeconds < 0 || r.JitterSeconds > min(300, r.IntervalSeconds/4) {
		return Invalid("jitter_seconds", "抖动超出周期允许范围")
	}
	if r.EvidenceWindowSeconds < max(r.IntervalSeconds, r.RecoveryIntervalSeconds, (r.FailureThreshold-1)*r.IntervalSeconds, (r.RecoveryThreshold-1)*r.RecoveryIntervalSeconds) {
		return Invalid("evidence_window_seconds", "证据窗口无法容纳阈值所需样本")
	}
	return nil
}

type State struct {
	Name              string     `json:"state"`
	Freshness         string     `json:"freshness"`
	FailureStreak     int        `json:"failure_streak"`
	RecoveryStreak    int        `json:"recovery_streak"`
	LastRunID         *int64     `json:"last_run_id"`
	LastVerdict       string     `json:"last_verdict"`
	LastCheckedAt     *time.Time `json:"last_checked_at"`
	LastValidAt       *time.Time `json:"last_valid_at"`
	CooldownUntil     *time.Time `json:"cooldown_until"`
	EvidenceExpiresAt *time.Time `json:"evidence_expires_at"`
	GlobalEpoch       int64      `json:"global_control_epoch"`
	PolicyEpoch       int64      `json:"policy_control_epoch"`
}

func NewState(global, policy int64) State {
	return State{Name: "untested", Freshness: "never", GlobalEpoch: global, PolicyEpoch: policy}
}
func (s State) Isolated() bool { return s.Name == "quarantined" || s.Name == "recovering" }
func (s State) Project(now time.Time, window int) State {
	s.Freshness = "never"
	if s.LastValidAt != nil {
		expires := s.LastValidAt.Add(time.Duration(window) * time.Second)
		s.EvidenceExpiresAt = &expires
		s.Freshness = "fresh"
		if !now.Before(expires) {
			s.Freshness = "stale"
		}
	}
	return s
}

type Policy struct {
	ID                       int64      `json:"id"`
	AccountID                int64      `json:"account_id"`
	AccountName              string     `json:"account_name"`
	SourceGroupID            int64      `json:"source_group_id"`
	SourceGroupName          string     `json:"source_group_name"`
	ControlSupported         *bool      `json:"control_supported,omitempty"`
	CompatibleSourceGroupIDs []int64    `json:"compatible_source_group_ids"`
	RoutingMode              string     `json:"routing_mode"`
	Condition                Condition  `json:"condition"`
	EffectiveModel           string     `json:"effective_upstream_model"`
	EffectiveEffort          string     `json:"effective_reasoning_effort"`
	EffectiveTier            string     `json:"effective_service_tier"`
	RoutingFingerprint       string     `json:"routing_fingerprint"`
	ControlScopeHash         string     `json:"control_scope_hash"`
	ScopeHash                string     `json:"scope_hash"`
	Revision                 int64      `json:"revision"`
	ControlEpoch             int64      `json:"policy_control_epoch"`
	Eligibility              string     `json:"eligibility"`
	Rules                    Rules      `json:"rules"`
	State                    State      `json:"state"`
	Control                  Control    `json:"control"`
	NextRunAt                *time.Time `json:"next_run_at"`
	ArchivedAt               *time.Time `json:"archived_at"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}
type Control struct {
	Effective bool   `json:"effective"`
	Reason    string `json:"reason"`
	Action    string `json:"action"`
}
type Target struct {
	AccountID                int64     `json:"account_id"`
	AccountName              string    `json:"account_name"`
	SourceGroupID            int64     `json:"source_group_id"`
	SourceGroupName          string    `json:"source_group_name"`
	PhysicalGroupID          int64     `json:"physical_group_id"`
	CompatibleSourceGroupIDs []int64   `json:"compatible_source_group_ids"`
	Eligibility              string    `json:"eligibility"`
	Condition                Condition `json:"normalized_condition"`
	EffectiveModel           string    `json:"effective_upstream_model"`
	Fingerprint              string    `json:"routing_fingerprint"`
	EffectiveEffort          string    `json:"effective_reasoning_effort"`
	EffectiveTier            string    `json:"effective_service_tier"`
	CanProbe                 bool      `json:"can_probe"`
	CanEnforce               bool      `json:"can_enforce"`
	SupportsOutputLimit      bool      `json:"supports_output_limit"`
	Reasons                  []string  `json:"reasons"`
}

func Hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (p *Policy) SetTarget(t Target) {
	p.AccountName = t.AccountName
	p.SourceGroupName = t.SourceGroupName
	p.ControlSupported = &t.CanEnforce
	p.CompatibleSourceGroupIDs = t.CompatibleSourceGroupIDs
	p.Condition = t.Condition
	p.EffectiveModel = t.EffectiveModel
	p.RoutingFingerprint = t.Fingerprint
	p.EffectiveEffort = t.EffectiveEffort
	p.EffectiveTier = t.EffectiveTier
	p.Eligibility = t.Eligibility
	c := p.Condition
	c.BenchmarkID = ""
	c.BenchmarkVersion = ""
	c.ScorerVersion = ""
	p.ControlScopeHash = Hash([]any{p.AccountID, t.EffectiveModel, t.Fingerprint, c.Protocol, c.Transport, t.EffectiveEffort, t.EffectiveTier})
	p.ScopeHash = Hash([]any{p.ControlScopeHash, p.Condition.BenchmarkID, p.Condition.BenchmarkVersion, p.Condition.ScorerVersion})
}

type Run struct {
	ID                   int64      `json:"id"`
	BatchID              string     `json:"batch_id"`
	PolicyID             int64      `json:"policy_id"`
	AccountID            int64      `json:"account_id"`
	Snapshot             Policy     `json:"input_snapshot"`
	ScopeHash            string     `json:"scope_hash"`
	TriggerSource        string     `json:"trigger_source"`
	Status               string     `json:"status"`
	Verdict              *string    `json:"verdict"`
	ReasonCode           string     `json:"reason_code"`
	ActorID              int64      `json:"actor_user_id"`
	LeaseToken           string     `json:"-"`
	LeaseExpiresAt       *time.Time `json:"-"`
	CancelRequested      bool       `json:"cancel_requested"`
	UpstreamDispatchedAt *time.Time `json:"upstream_dispatched_at"`
	ScheduledSlot        *time.Time `json:"scheduled_slot"`
	QueuedAt             time.Time  `json:"queued_at"`
	StartedAt            *time.Time `json:"started_at"`
	FinishedAt           *time.Time `json:"finished_at"`
	QueueExpiresAt       time.Time  `json:"queue_expires_at"`
	Result               Result     `json:"result"`
	ResponseText         *string    `json:"response_text"`
	ResponseExpiresAt    *time.Time `json:"response_expires_at"`
	StateApplied         bool       `json:"state_applied"`
	StateSkipReason      string     `json:"state_skip_reason"`
}
type Result struct {
	Verdict           string   `json:"verdict"`
	ReasonCode        string   `json:"reason_code"`
	ResponseText      string   `json:"-"`
	ResponseTruncated bool     `json:"response_truncated"`
	FinalAnswer       *int64   `json:"final_answer"`
	ExpectedAnswer    int64    `json:"expected_answer"`
	LatencyMS         int64    `json:"latency_ms"`
	InputTokens       *int64   `json:"input_tokens"`
	OutputTokens      *int64   `json:"output_tokens"`
	TotalTokens       *int64   `json:"total_tokens"`
	EstimatedCostUSD  *float64 `json:"estimated_cost_usd"`
}
type Event struct {
	ID          int64     `json:"id"`
	PolicyID    *int64    `json:"policy_id"`
	AccountID   *int64    `json:"account_id"`
	RunID       *int64    `json:"run_id"`
	Type        string    `json:"event_type"`
	OldState    string    `json:"old_state"`
	NewState    string    `json:"new_state"`
	Reason      string    `json:"reason_code"`
	ActorID     int64     `json:"actor_user_id"`
	Revision    int64     `json:"revision"`
	GlobalEpoch int64     `json:"global_control_epoch"`
	PolicyEpoch int64     `json:"policy_control_epoch"`
	CreatedAt   time.Time `json:"created_at"`
}
type Provider interface {
	Targets(context.Context) ([]Target, error)
	Resolve(context.Context, int64, int64, Condition) (Target, error)
	Probe(context.Context, Run, func() error) Result
}
type Sample struct {
	At      time.Time
	Verdict string
}
