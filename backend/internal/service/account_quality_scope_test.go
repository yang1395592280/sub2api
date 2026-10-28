package service

import (
	"context"
	"testing"
	"time"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/stretchr/testify/require"
)

type qualityScopeGroups struct {
	GroupRepository
	items   []Group
	members map[int64][]int64
}

func (r qualityScopeGroups) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	return r.items, nil
}
func (r qualityScopeGroups) GetAccountIDsByGroupIDs(_ context.Context, ids []int64) ([]int64, error) {
	out := []int64{}
	for _, id := range ids {
		out = append(out, r.members[id]...)
	}
	return out, nil
}

type qualityScopeAccounts struct {
	AccountRepository
	items []*Account
}

func (r qualityScopeAccounts) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return r.items, nil
}

func TestQualityOrdinaryGroupsCanProbeButCannotControl(t *testing.T) {
	a := &Account{ID: 42, Name: "Shared upstream", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: []int64{7, 9}, Credentials: map[string]any{"api_key": "test", "base_url": "https://example.test"}, Extra: map[string]any{"openai_passthrough_mode": "responses"}}
	provider := &accountQualityProvider{
		accounts: qualityScopeAccounts{items: []*Account{a}},
		groups: qualityScopeGroups{items: []Group{
			{ID: 7, Name: "0.1 普通组", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard},
			{ID: 9, Name: "不降智", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, AllowAutoCheapestScheduling: true},
			{ID: 10, Name: "内部池", Platform: PlatformOpenAI, GroupRole: GroupRoleSelfHostedPool},
		}, members: map[int64][]int64{7: {42}, 9: {42}, 10: {42}}},
	}
	targets, err := provider.Targets(context.Background())
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.ElementsMatch(t, []int64{7, 9}, targets[0].CompatibleSourceGroupIDs)
	require.True(t, targets[0].CanEnforce)
	condition := quality.Condition{ModelID: "gpt-5.5", Protocol: qualityProtocol(a), Transport: "http", ReasoningEffort: "upstream_default", ServiceTier: "upstream_default", BenchmarkID: "candy", BenchmarkVersion: "1", ScorerVersion: "strict-json-v1"}
	ordinary, err := provider.Resolve(context.Background(), 42, 7, condition)
	require.NoError(t, err)
	require.True(t, ordinary.CanProbe)
	require.False(t, ordinary.CanEnforce)
	require.Equal(t, "0.1 普通组", ordinary.SourceGroupName)
	auto, err := provider.Resolve(context.Background(), 42, 9, condition)
	require.NoError(t, err)
	require.True(t, auto.CanEnforce)
	settings := quality.DefaultSettings()
	settings.ControlEnabled = true
	policy := quality.Policy{AccountID: 42, Rules: quality.DefaultRules(100), State: quality.NewState(settings.ControlEpoch, 0)}
	policy.Rules.ControlEnabled = true
	policy.SetTarget(ordinary)
	policy.State.Name = "quarantined"
	require.False(t, quality.Present(policy, settings, time.Now()).Control.Effective)
	require.Equal(t, "source_observe_only", quality.Present(policy, settings, time.Now()).Control.Reason)
	policy.SetTarget(auto)
	require.Equal(t, "deny", quality.Present(policy, settings, time.Now()).Control.Action)
	_, err = provider.Resolve(context.Background(), 42, 999, condition)
	require.Error(t, err)
}

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
