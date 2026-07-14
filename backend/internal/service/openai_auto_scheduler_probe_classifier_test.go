package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyOpenAIAutoSchedulerProbeEvent(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.SlowThresholdMS = 6000
	settings.SevereSlowThresholdMS = 15000

	tests := []struct {
		name string
		in   OpenAIAutoSchedulerProbeResult
		want string
	}{
		{"fast success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: openAIAutoSchedulerProbeIntPtr(1200)}, OpenAIAutoSchedulerEventProbeSuccess},
		{"slow success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: openAIAutoSchedulerProbeIntPtr(7000)}, OpenAIAutoSchedulerEventSlow},
		{"severe success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: openAIAutoSchedulerProbeIntPtr(16000)}, OpenAIAutoSchedulerEventSevereSlow},
		{"slow success latency fallback", OpenAIAutoSchedulerProbeResult{Success: true, LatencyMS: openAIAutoSchedulerProbeIntPtr(7000)}, OpenAIAutoSchedulerEventSlow},
		{"non-positive latency success", OpenAIAutoSchedulerProbeResult{Success: true, LatencyMS: openAIAutoSchedulerProbeIntPtr(0)}, OpenAIAutoSchedulerEventProbeSuccess},
		{"probe error", OpenAIAutoSchedulerProbeResult{Success: false, Err: errors.New("timeout")}, OpenAIAutoSchedulerEventProbeError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, classifyOpenAIAutoSchedulerProbeEvent(tt.in, settings))
		})
	}
}

func openAIAutoSchedulerProbeIntPtr(value int) *int {
	return &value
}
