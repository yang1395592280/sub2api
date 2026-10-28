package accountquality

import "time"

// Reduce uses accepted samples from the current scope and epoch, rather than
// incrementing a counter whose older members might have expired individually.
func Reduce(state State, rules Rules, now time.Time, runID int64, result Result, previous []Sample) State {
	state.LastRunID = &runID
	state.LastVerdict = result.Verdict
	state.LastCheckedAt = &now
	if result.Verdict != "passed" && result.Verdict != "incorrect" {
		return state.Project(now, rules.EvidenceWindowSeconds)
	}
	if state.Isolated() && state.CooldownUntil != nil && now.Before(*state.CooldownUntil) {
		return state.Project(now, rules.EvidenceWindowSeconds)
	}
	cutoff := now.Add(-time.Duration(rules.EvidenceWindowSeconds) * time.Second)
	streak := 1
	for i := len(previous) - 1; i >= 0; i-- {
		v := previous[i]
		if v.At.Before(cutoff) {
			break
		}
		if state.Isolated() && state.CooldownUntil != nil && v.At.Before(*state.CooldownUntil) {
			break
		}
		if v.Verdict == "passed" || v.Verdict == "incorrect" {
			if v.Verdict != result.Verdict {
				break
			}
			streak++
		}
	}
	state.LastValidAt = &now
	if state.Isolated() {
		state.FailureStreak = 0
		if result.Verdict == "passed" {
			state.RecoveryStreak = streak
			state.Name = "recovering"
			if streak >= rules.RecoveryThreshold {
				state.Name = "normal"
				state.RecoveryStreak = 0
				state.CooldownUntil = nil
			}
		} else {
			state.RecoveryStreak = 0
			state.Name = "quarantined"
			until := now.Add(time.Duration(rules.CooldownSeconds) * time.Second)
			state.CooldownUntil = &until
		}
	} else if result.Verdict == "passed" {
		state.Name = "normal"
		state.FailureStreak = 0
		state.RecoveryStreak = 0
	} else {
		state.FailureStreak = streak
		state.RecoveryStreak = 0
		state.Name = "observing"
		if streak >= rules.FailureThreshold {
			state.Name = "quarantined"
			until := now.Add(time.Duration(rules.CooldownSeconds) * time.Second)
			state.CooldownUntil = &until
		}
	}
	return state.Project(now, rules.EvidenceWindowSeconds)
}

func Present(p Policy, settings Settings, now time.Time) Policy {
	if p.State.GlobalEpoch != settings.ControlEpoch || p.State.PolicyEpoch != p.ControlEpoch {
		p.State = NewState(settings.ControlEpoch, p.ControlEpoch)
	}
	p.State = p.State.Project(now, p.Rules.EvidenceWindowSeconds)
	p.Control = Control{Reason: "global_control_disabled", Action: "observe_only"}
	if p.ControlSupported != nil && !*p.ControlSupported {
		p.Control.Reason = "source_observe_only"
		return p
	}
	if !settings.ControlEnabled {
		return p
	}
	p.Control.Reason = "policy_control_disabled"
	if !p.Rules.ControlEnabled {
		return p
	}
	p.Control.Reason = p.Eligibility
	if p.Eligibility != "eligible" || p.ArchivedAt != nil {
		return p
	}
	p.Control = Control{Effective: true, Reason: "enabled", Action: "baseline"}
	if p.State.Isolated() {
		p.Control.Action = "deny"
	} else if p.State.Name == "observing" && p.State.Freshness == "fresh" {
		p.Control.Action = "fallback"
	}
	return p
}
