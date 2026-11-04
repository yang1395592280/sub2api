package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIAccountSelector func(map[int64]struct{}) (*service.APIKey, string, *service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error)

type openAIPendingAccountSelection struct {
	apiKey    *service.APIKey
	model     string
	selection *service.AccountSelectionResult
	decision  service.OpenAIAccountScheduleDecision
}

// Keep an acquired alternative for the next loop iteration so preferring another
// account neither selects twice nor releases and races to reacquire its slot.
type openAIFailoverSelection struct {
	pending *openAIPendingAccountSelection
}

func (s *openAIFailoverSelection) Select(excludedIDs map[int64]struct{}, selectAccount openAIAccountSelector) (*service.APIKey, string, *service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
	if s.pending == nil {
		return selectAccount(excludedIDs)
	}
	next := s.pending
	s.pending = nil
	return next.apiKey, next.model, next.selection, next.decision, nil
}

func (s *openAIFailoverSelection) Close() {
	if s.pending != nil && s.pending.selection != nil && s.pending.selection.ReleaseFunc != nil {
		s.pending.selection.ReleaseFunc()
	}
	s.pending = nil
}

// TryAlternative uses the same authorized selection path as the request. A miss
// leaves the original pool retry budget and failed-account exclusions intact.
func (s *openAIFailoverSelection) TryAlternative(
	ctx context.Context,
	apiKey *service.APIKey,
	account *service.Account,
	failoverErr *service.UpstreamFailoverError,
	excludedIDs map[int64]struct{},
	preserveAccountBinding bool,
	canSwitch bool,
	selectAccount openAIAccountSelector,
) bool {
	if !canSwitch || preserveAccountBinding || ctx.Err() != nil || selectAccount == nil ||
		apiKey == nil || apiKey.UsesOpenAIAutoCheapestGroup() || apiKey.GroupID == nil || apiKey.Group == nil ||
		apiKey.Group.Platform != service.PlatformOpenAI || account == nil || account.Platform != service.PlatformOpenAI ||
		failoverErr == nil || !failoverErr.ShouldRetryNextAccount() || failoverErr.RequestScopedTransient {
		return false
	}
	failed := make(map[int64]struct{}, len(excludedIDs)+1)
	for id := range excludedIDs {
		failed[id] = struct{}{}
	}
	failed[account.ID] = struct{}{}
	key, model, selection, decision, err := selectAccount(failed)
	if err != nil || ctx.Err() != nil || selection == nil || selection.Account == nil {
		if selection != nil && selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return false
	}
	// Defend against a stale/custom selector ignoring exclusions. Never consume a
	// switch or retain a slot for an account that already failed this request.
	if _, excluded := failed[selection.Account.ID]; excluded {
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return false
	}
	s.pending = &openAIPendingAccountSelection{apiKey: key, model: model, selection: selection, decision: decision}
	return true
}
