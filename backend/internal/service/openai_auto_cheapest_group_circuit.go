package service

import (
	"context"
	"strings"
	"time"
)

// OpenAIAutoCheapestGroupCircuit tracks short-lived health for auto-selected groups.
// Implementations should fail open when their backing store is unavailable.
type OpenAIAutoCheapestGroupCircuit interface {
	Allow(ctx context.Context, key OpenAIAutoCheapestGroupHealthKey) (bool, error)
	RecordFailure(ctx context.Context, key OpenAIAutoCheapestGroupHealthKey, reason string) error
	RecordSuccess(ctx context.Context, key OpenAIAutoCheapestGroupHealthKey) error
}

type OpenAIAutoCheapestGroupHealthKey struct {
	GroupID int64
	Model   string
	Endpoint string
	Transport string
}

func NormalizeOpenAIAutoCheapestHealthModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return "unknown"
	}
	model = strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(model)
	if len(model) > 96 {
		model = model[:96]
	}
	return model
}

func NormalizeOpenAIAutoCheapestHealthPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" { return "unknown" }
	value = strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(value)
	if len(value) > 64 { return value[:64] }
	return value
}

func (k OpenAIAutoCheapestGroupHealthKey) Valid() bool { return k.GroupID > 0 }

const (
	OpenAIAutoCheapestFailureWindow = 60 * time.Second
	OpenAIAutoCheapestCooldown      = 60 * time.Second
	OpenAIAutoCheapestFailureLimit  = int64(1)
)
