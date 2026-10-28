package accountquality

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScoreFinalAnswer(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"correct", `{"answer":21}`, "passed"}, {"wrong", `{"answer":29}`, "incorrect"}, {"negative", `{"answer":-1}`, "incorrect"},
		{"fenced", "```json\n{\"answer\":21}\n```", "passed"}, {"plain fence", "```\n{\"answer\":21}\n```", "passed"},
		{"numeric string", `{"answer":"21"}`, "unknown"}, {"duplicate", `{"answer":20,"answer":21}`, "unknown"}, {"extra", `{"answer":21,"why":"yes"}`, "unknown"},
		{"decimal", `{"answer":21.0}`, "unknown"}, {"exponent", `{"answer":21e0}`, "unknown"}, {"overflow", `{"answer":9223372036854775808}`, "unknown"},
		{"prose", `答案21 {"answer":21}`, "unknown"}, {"two objects", `{"answer":21}{"answer":21}`, "unknown"}, {"null", `{"answer":null}`, "unknown"},
		{"array", `[{"answer":21}]`, "unknown"}, {"empty", "", "unknown"}, {"partial", `{"answer":21`, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, Score(tc.text, true).Verdict) })
	}
	require.Equal(t, "incomplete_response", Score(`{"answer":21}`, false).ReasonCode)
	r := Score(strings.Repeat("汉", MaxResponseBytes), true)
	require.Equal(t, "response_too_large", r.ReasonCode)
	require.LessOrEqual(t, len(r.ResponseText), MaxResponseBytes)
}
func TestWindowDropsIndividualOldSamples(t *testing.T) {
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	rules := DefaultRules(100)
	state := NewState(1, 1)
	previous := []Sample{{now.Add(-121 * time.Minute), "incorrect"}, {now.Add(-60 * time.Minute), "incorrect"}}
	state = Reduce(state, rules, now, 3, Result{Verdict: "incorrect"}, previous)
	require.Equal(t, "observing", state.Name)
	require.Equal(t, 2, state.FailureStreak)
	state = Reduce(state, rules, now.Add(time.Minute), 4, Result{Verdict: "error"}, nil)
	require.Equal(t, 2, state.FailureStreak)
	state = Reduce(state, rules, now.Add(2*time.Minute), 5, Result{Verdict: "passed"}, previous)
	require.Equal(t, "normal", state.Name)
	require.Zero(t, state.FailureStreak)
}
func TestRecoveryRequiresCooldownAndAcceptedEvidence(t *testing.T) {
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	rules := DefaultRules(100)
	state := NewState(1, 1)
	state = Reduce(state, rules, now, 3, Result{Verdict: "incorrect"}, []Sample{{now.Add(-60 * time.Minute), "incorrect"}, {now.Add(-30 * time.Minute), "incorrect"}})
	require.Equal(t, "quarantined", state.Name)
	require.Equal(t, now.Add(30*time.Minute), *state.CooldownUntil)
	state = Reduce(state, rules, now.Add(10*time.Minute), 4, Result{Verdict: "passed"}, nil)
	require.Zero(t, state.RecoveryStreak)
	require.Equal(t, "quarantined", state.Name)
	state = Reduce(state, rules, now.Add(30*time.Minute), 5, Result{Verdict: "passed"}, nil)
	require.Equal(t, "recovering", state.Name)
	state = Reduce(state, rules, now.Add(35*time.Minute), 6, Result{Verdict: "unknown"}, nil)
	require.Equal(t, 1, state.RecoveryStreak)
	state = Reduce(state, rules, now.Add(40*time.Minute), 7, Result{Verdict: "passed"}, []Sample{{now.Add(10 * time.Minute), "passed"}, {now.Add(30 * time.Minute), "passed"}})
	require.Equal(t, "normal", state.Name)
	state.Name = "recovering"
	until := now.Add(30 * time.Minute)
	state.CooldownUntil = &until
	state = Reduce(state, rules, now.Add(40*time.Minute), 8, Result{Verdict: "incorrect"}, nil)
	require.Equal(t, "quarantined", state.Name)
	require.Equal(t, now.Add(70*time.Minute), *state.CooldownUntil)
}
func TestEpochProjectionAndStaleIsolation(t *testing.T) {
	now := time.Now()
	settings := DefaultSettings()
	settings.ControlEnabled = true
	settings.ControlEpoch = 2
	p := Policy{Eligibility: "eligible", Rules: DefaultRules(100), ControlEpoch: 1, State: NewState(1, 1)}
	p.Rules.ControlEnabled = true
	p.State.Name = "quarantined"
	projected := Present(p, settings, now)
	require.Equal(t, "untested", projected.State.Name)
	require.Equal(t, "baseline", projected.Control.Action)
	p.State.GlobalEpoch = 2
	p.State.LastValidAt = ptrTime(now.Add(-24 * time.Hour))
	projected = Present(p, settings, now)
	require.Equal(t, "deny", projected.Control.Action)
	p.State.Name = "observing"
	require.Equal(t, "baseline", Present(p, settings, now).Control.Action)
}
func ptrTime(v time.Time) *time.Time { return &v }
func TestThresholdWindowValidation(t *testing.T) {
	r := DefaultRules(100)
	require.NoError(t, r.Validate())
	r.FailureThreshold = 10
	require.Error(t, r.Validate())
	r.EvidenceWindowSeconds = 16200
	require.NoError(t, r.Validate())
	r.RecoveryThreshold = 10
	r.RecoveryIntervalSeconds = 3600
	require.Error(t, r.Validate())
}
func TestMonitorDoesNotUsePaginatedHistory(t *testing.T) {
	now := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	p := Policy{ID: 1, ScopeHash: "scope", Rules: DefaultRules(100), ControlEpoch: 1}
	settings := DefaultSettings()
	settings.ControlEpoch = 1
	runs := []Run{}
	for i := int64(1); i <= 150; i++ {
		verdict := "passed"
		if i%2 == 0 {
			verdict = "incorrect"
		}
		runs = append(runs, Run{ID: i, PolicyID: 1, ScopeHash: "scope", Status: "completed", Verdict: &verdict, QueuedAt: now.Add(-time.Minute), Snapshot: Policy{ControlEpoch: 1, State: NewState(1, 1)}})
	}
	m, err := BuildMonitor(p, settings, now, 300, "current_epoch", runs)
	require.NoError(t, err)
	require.Len(t, m.Buckets, 288)
	require.Equal(t, 150, m.Summary.Total)
	require.Equal(t, 50.0, *m.Summary.PassRate)
	require.Len(t, m.Buckets[287].SampleRunIDs, 5)
	require.True(t, m.Buckets[287].HasMore)
	runs[0].Snapshot.ControlEpoch = 0
	m, err = BuildMonitor(p, settings, now, 1800, "current_epoch", runs)
	require.NoError(t, err)
	require.Equal(t, 149, m.Summary.Total)
	m, err = BuildMonitor(p, settings, now, 1800, "current_scope", runs)
	require.NoError(t, err)
	require.Equal(t, 150, m.Summary.Total)
}
