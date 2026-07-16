package service

import (
	"context"
	"sync"
)

type openAIAutoCheapestFailureState struct {
	mu           sync.RWMutex
	failedGroups map[int64]string
	circuit      OpenAIAutoCheapestGroupCircuit
	model        string
	endpoint     string
	transport    string
}

type openAIAutoCheapestFailureStateKey struct{}

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

// MarkOpenAIAutoCheapestGroupFailed prevents the current request from trying
// another account in a group that already returned a transient upstream error.
func MarkOpenAIAutoCheapestGroupFailed(ctx context.Context, groupID int64, reason string) {
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
	state.mu.Unlock()

	// Count at most one failure per group in a request. Account-level retries
	// within the same group must not consume the cross-request circuit budget.
	if circuit != nil {
		_ = circuit.RecordFailure(ctx, OpenAIAutoCheapestGroupHealthKey{GroupID: groupID, Model: model, Endpoint: endpoint, Transport: transport}, reason)
	}
}

func setOpenAIAutoCheapestGroupHealthContext(ctx context.Context, model, endpoint, transport string) {
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil { return }
	state.mu.Lock()
	state.model = model
	state.endpoint = endpoint
	state.transport = transport
	state.mu.Unlock()
}

func openAIAutoCheapestGroupHealthKey(ctx context.Context, groupID int64) OpenAIAutoCheapestGroupHealthKey {
	key := OpenAIAutoCheapestGroupHealthKey{GroupID: groupID}
	state, _ := ctx.Value(openAIAutoCheapestFailureStateKey{}).(*openAIAutoCheapestFailureState)
	if state == nil { return key }
	state.mu.RLock()
	key.Model = state.model
	key.Endpoint = state.endpoint
	key.Transport = state.transport
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

func openAIAutoCheapestGroupFailureReason(ctx context.Context, groupID int64) (string, bool) {
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
