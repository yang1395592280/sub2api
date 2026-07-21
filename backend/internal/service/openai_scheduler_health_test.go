package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeOpenAISchedulerHealthRepository struct{}

func (*fakeOpenAISchedulerHealthRepository) GetBatch(context.Context, []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error) {
	return nil, nil
}

func (*fakeOpenAISchedulerHealthRepository) Upsert(context.Context, OpenAISchedulerHealthSnapshot) error {
	return nil
}

func TestOpenAISchedulerHealthContract(t *testing.T) {
	var _ OpenAISchedulerHealthRepository = (*fakeOpenAISchedulerHealthRepository)(nil)

	requireStructFieldTypes(t, reflect.TypeOf(OpenAISchedulerHealthKey{}), []structFieldType{
		{name: "AccountID", typ: reflect.TypeOf(int64(0))},
		{name: "ModelFamily", typ: reflect.TypeOf("")},
		{name: "Endpoint", typ: reflect.TypeOf("")},
		{name: "Transport", typ: reflect.TypeOf("")},
	})
	requireStructFieldTypes(t, reflect.TypeOf(OpenAISchedulerHealthSnapshot{}), []structFieldType{
		{name: "Key", typ: reflect.TypeOf(OpenAISchedulerHealthKey{})},
		{name: "State", typ: reflect.TypeOf("")},
		{name: "PredictedTTFTMS", typ: reflect.TypeOf(float64(0))},
		{name: "ErrorRate", typ: reflect.TypeOf(float64(0))},
		{name: "RateLimitedRate", typ: reflect.TypeOf(float64(0))},
		{name: "ServerErrorRate", typ: reflect.TypeOf(float64(0))},
		{name: "ConsecutiveSlow", typ: reflect.TypeOf(int(0))},
		{name: "ConsecutiveError", typ: reflect.TypeOf(int(0))},
		{name: "ConsecutiveSuccess", typ: reflect.TypeOf(int(0))},
		{name: "RealSampleCount", typ: reflect.TypeOf(int64(0))},
		{name: "ProbeSampleCount", typ: reflect.TypeOf(int64(0))},
		{name: "LastRealAt", typ: reflect.TypeOf((*time.Time)(nil))},
		{name: "LastProbeAt", typ: reflect.TypeOf((*time.Time)(nil))},
		{name: "CooldownUntil", typ: reflect.TypeOf((*time.Time)(nil))},
		{name: "ExpiresAt", typ: reflect.TypeOf(time.Time{})},
	})
}

func TestClassifyOpenAISchedulerHealthConfidence(t *testing.T) {
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	freshRealAt := now.Add(-time.Minute)
	staleRealAt := now.Add(-10 * time.Minute)
	freshExpiry := now.Add(time.Hour)

	tests := []struct {
		name         string
		state        string
		expiresAt    time.Time
		realSamples  int64
		probeSamples int64
		lastRealAt   *time.Time
		want         string
	}{
		{name: "fresh real running", state: OpenAIAutoSchedulerStateRunning, expiresAt: freshExpiry, realSamples: 1, lastRealAt: &freshRealAt, want: OpenAISchedulerHealthConfidenceHigh},
		{name: "probe only running", state: OpenAIAutoSchedulerStateRunning, expiresAt: freshExpiry, probeSamples: 1, want: OpenAISchedulerHealthConfidenceMedium},
		{name: "stale real running", state: OpenAIAutoSchedulerStateRunning, expiresAt: freshExpiry, realSamples: 1, lastRealAt: &staleRealAt, want: OpenAISchedulerHealthConfidenceMedium},
		{name: "observing stays low with fresh real", state: OpenAIAutoSchedulerStateObserving, expiresAt: freshExpiry, realSamples: 10, lastRealAt: &freshRealAt, want: OpenAISchedulerHealthConfidenceLow},
		{name: "open stays low", state: OpenAIAutoSchedulerStateOpen, expiresAt: freshExpiry, realSamples: 10, lastRealAt: &freshRealAt, want: OpenAISchedulerHealthConfidenceLow},
		{name: "expired stays low", state: OpenAIAutoSchedulerStateRunning, expiresAt: now.Add(-time.Second), realSamples: 10, lastRealAt: &freshRealAt, want: OpenAISchedulerHealthConfidenceLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOpenAISchedulerHealthConfidence(
				tt.state, tt.expiresAt, tt.realSamples, tt.probeSamples, tt.lastRealAt, now, 180,
			)
			require.Equal(t, tt.want, got)
		})
	}
}

type structFieldType struct {
	name string
	typ  reflect.Type
}

func requireStructFieldTypes(t *testing.T, typ reflect.Type, expected []structFieldType) {
	t.Helper()
	require.Equal(t, len(expected), typ.NumField())
	for index, want := range expected {
		field := typ.Field(index)
		require.Equal(t, want.name, field.Name)
		require.Equal(t, want.typ, field.Type)
	}
}
