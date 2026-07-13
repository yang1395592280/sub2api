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
