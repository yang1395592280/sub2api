package service

import (
	"testing"
	"time"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/stretchr/testify/require"
)

func TestQualityScopeMatchesResolvedPolicyAndTracksRoutingChanges(t *testing.T) {
	account := &Account{
		ID:        42,
		CreatedAt: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC),
		Platform:  PlatformOpenAI,
		Type:      AccountTypeAPIKey,
		GroupIDs:  []int64{7, 9},
		Credentials: map[string]any{
			"api_key":       "never-store-this-key",
			"base_url":      "https://relay.example.test/v1?token=private",
			"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5-upstream"},
		},
		Extra: map[string]any{"quality_identity_version": "identity-v1"},
	}
	group := &Group{
		ID:                          11,
		Platform:                    PlatformOpenAI,
		GroupRole:                   GroupRoleStandard,
		AllowAutoCheapestScheduling: true,
		MaxReasoningEffort:          "high",
		ForceOpenAIFast:             true,
	}
	condition := quality.Condition{
		ModelID:          "gpt-5.5",
		Protocol:         "responses",
		Transport:        "http",
		ReasoningEffort:  "high",
		ServiceTier:      "priority",
		BenchmarkID:      "candy",
		BenchmarkVersion: "1",
		ScorerVersion:    "strict-json-v1",
	}
	target := quality.Target{
		Condition:       condition,
		EffectiveModel:  "gpt-5.5-upstream",
		EffectiveEffort: "high",
		EffectiveTier:   "priority",
		Fingerprint: quality.Hash([]any{
			qualityRoutingFingerprint(account, account),
			qualityGroupRoutingFingerprint(group),
		}),
	}
	policy := quality.Policy{AccountID: account.ID, Condition: condition}
	policy.SetTarget(target)

	got := qualityScope(account, target.EffectiveModel, condition.Protocol, condition.Transport, target.EffectiveEffort, target.EffectiveTier, group)
	require.Equal(t, policy.ControlScopeHash, got)
	require.NotContains(t, got, "never-store-this-key")
	require.NotContains(t, qualityEndpointIdentity("https://relay.example.test/v1?token=private"), "private")

	equivalentGroup := *group
	equivalentGroup.ID++
	require.Equal(t, qualityGroupRoutingFingerprint(group), qualityGroupRoutingFingerprint(&equivalentGroup))

	changedGroup := *group
	changedGroup.ForceOpenAIFast = false
	require.NotEqual(t, got, qualityScope(account, target.EffectiveModel, condition.Protocol, condition.Transport, target.EffectiveEffort, target.EffectiveTier, &changedGroup))

	changedAccount := *account
	changedAccount.Credentials = map[string]any{
		"api_key":       "rotated-key",
		"base_url":      "https://relay.example.test/v2",
		"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5-upstream"},
	}
	require.NotEqual(t, got, qualityScope(&changedAccount, target.EffectiveModel, condition.Protocol, condition.Transport, target.EffectiveEffort, target.EffectiveTier, group))
}
