//go:build unit

package service

import "testing"

func TestBetterProbeResultPrefersHealthyThenFast(t *testing.T) {
	fastDegraded := &CheckResult{Status: MonitorStatusDegraded, LatencyMs: probeIntPtr(20)}
	slowOperational := &CheckResult{Status: MonitorStatusOperational, LatencyMs: probeIntPtr(200)}
	if !betterProbeResult(slowOperational, fastDegraded) {
		t.Fatal("operational result should beat degraded result")
	}
	fastOperational := &CheckResult{Status: MonitorStatusOperational, LatencyMs: probeIntPtr(10)}
	if !betterProbeResult(fastOperational, slowOperational) {
		t.Fatal("faster result should win within the same status")
	}
}

func probeIntPtr(v int) *int { return &v }
