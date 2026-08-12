package service

import (
	"context"
	"sync"
)

type openAIAutoCheapestFailureState struct {
	mu            sync.RWMutex
	failedGroups  map[int64]string
	circuit       OpenAIAutoCheapestGroupCircuit
	model         string
	endpoint      string
	transport     string
	userID        int64
	strictQuality bool
}

type openAIAutoCheapestFailureStateKey struct{}
type openAIAutoCheapestQualifiedOnlyKey struct{}

type openAIAutoCheapestChannelPricePriorityKey struct{}

// OpenAIAutoCheapestRoutingReason returns a safe, request-local explanation
// for an automatic-cheapest selection failure. It intentionally exposes only
// a stable operational category, never account credentials or internal IDs.
func OpenAIAutoCheapestRoutingReason(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return ""
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	var hasNoAvailable, hasNoEligible bool
	for _, reason := range state.failedGroups {
		switch reason {
		case "circuit_open":
			return "符合条件的自动调度分组已触发熔断"
		case "no_available_accounts":
			hasNoAvailable = true
		case "no_eligible_groups":
			hasNoEligible = true
		}
	}
	if hasNoAvailable {
		return "当前没有可用于该模型或接口的账号"
	}
	if hasNoEligible {
		return "没有符合自动调度最高倍率限制的分组"
	}
	return ""
}

func noteOpenAIAutoCheapestGroupSkipped(ctx context.Context, groupID int64, reason string) {
	if ctx == nil {
		return
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	if groupID <= 0 {
		for id := range state.failedGroups {
			if id <= 0 {
				state.mu.Unlock()
				return
			}
		}
		state.failedGroups[0] = reason
		state.mu.Unlock()
		return
	}
	if _, exists := state.failedGroups[groupID]; !exists {
		state.failedGroups[groupID] = reason
	}
	state.mu.Unlock()
}

// PrepareOpenAIAutoCheapestRequestContext attaches request-local group failure
// state. Fixed-group API keys do not need this state and keep their old path.
func PrepareOpenAIAutoCheapestRequestContext(ctx context.Context, enabled bool, circuit ...OpenAIAutoCheapestGroupCircuit) context.Context {
	if ctx == nil || !enabled {
		return ctx
	}
	if _, ok := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState); ok {
		return ctx
	}
	var groupCircuit OpenAIAutoCheapestGroupCircuit
	if len(circuit) > 0 {
		groupCircuit = circuit[0]
	}
	return context.WithValue(ctx, openAIAutoCheapestFailureStateKey{}, &openAIAutoCheapestFailureState{
		failedGroups: make(map[int64]string),
		circuit:      groupCircuit,
	})
}

// markOpenAIAutoCheapestGroupExhausted prevents the current request from
// repeatedly scanning a group only after account selection has confirmed that
// the group has no usable candidates. A single account failure stays at account
// scope so other accounts in the same, cheaper group can still run.
func markOpenAIAutoCheapestGroupExhausted(ctx context.Context, groupID int64, reason string) {
	if ctx == nil || groupID <= 0 {
		return
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	if _, exists := state.failedGroups[groupID]; exists {
		state.mu.Unlock()
		return
	}
	state.failedGroups[groupID] = reason
	circuit := state.circuit
	model := state.model
	endpoint := state.endpoint
	transport := state.transport
	userID := state.userID
	state.mu.Unlock()

	// Count at most one failure per group in a request. Account-level retries
	// within the same group must not consume the cross-request circuit budget.
	if circuit != nil {
		_ = circuit.RecordFailure(ctx, OpenAIAutoCheapestGroupHealthKey{GroupID: groupID, UserID: userID, Model: model, Endpoint: endpoint, Transport: transport}, reason)
	}
}

func setOpenAIAutoCheapestGroupFailureUserContext(ctx context.Context, userID int64) {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	state.userID = userID
	state.mu.Unlock()
}

func setOpenAIAutoCheapestGroupHealthContext(ctx context.Context, model, endpoint, transport string) {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	state.model = model
	state.endpoint = endpoint
	state.transport = transport
	state.mu.Unlock()
}

func openAIAutoCheapestGroupHealthKey(ctx context.Context, groupID int64) OpenAIAutoCheapestGroupHealthKey {
	key := OpenAIAutoCheapestGroupHealthKey{GroupID: groupID}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return key
	}
	state.mu.RLock()
	key.Model = state.model
	key.Endpoint = state.endpoint
	key.Transport = state.transport
	key.UserID = state.userID
	state.mu.RUnlock()
	return key
}

func OpenAIAutoCheapestGroupCircuitFromContext(ctx context.Context) OpenAIAutoCheapestGroupCircuit {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return nil
	}
	return state.circuit
}

// RequireOpenAIAutoCheapestQualifiedFailover disables the availability
// fallback for later account selections in the current auto-cheapest request.
// It is used after a first-output timeout so the next attempt cannot spend the
// remaining latency budget on another low-confidence or slow account.
func RequireOpenAIAutoCheapestQualifiedFailover(ctx context.Context) {
	if ctx == nil {
		return
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return
	}
	state.mu.Lock()
	state.strictQuality = true
	state.mu.Unlock()
}

func openAIAutoCheapestRequiresQualifiedFailover(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.strictQuality
}

func withOpenAIAutoCheapestQualifiedOnly(ctx context.Context) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, openAIAutoCheapestQualifiedOnlyKey{}, true)
}

func openAIAutoCheapestQualifiedOnly(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	qualifiedOnly, _ := ctx.Value(openAIAutoCheapestQualifiedOnlyKey{}).(bool)
	return qualifiedOnly
}

// withOpenAIAutoCheapestChannelPricePriority marks account selections made for
// an auto-cheapest API key. Health and capability filtering still run first;
// this marker only makes channel price the primary order within the surviving
// accounts of the current user-price group.
func withOpenAIAutoCheapestChannelPricePriority(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openAIAutoCheapestChannelPricePriorityKey{}, true)
}

func openAIAutoCheapestChannelPricePriority(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(openAIAutoCheapestChannelPricePriorityKey{}).(bool)
	return enabled
}

func openAIAutoCheapestGroupExhaustionReason(ctx context.Context, groupID int64) (string, bool) {
	if ctx == nil || groupID <= 0 {
		return "", false
	}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil {
		return "", false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	reason, ok := state.failedGroups[groupID]
	return reason, ok
}
