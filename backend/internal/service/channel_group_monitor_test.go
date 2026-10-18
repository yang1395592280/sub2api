package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type groupMonitorTestRepo struct {
	GroupMonitorRepository
	results  []GroupMonitorRun
	released bool
}

func (r *groupMonitorTestRepo) FinishRun(_ context.Context, _ int64, _ string, run GroupMonitorRun) error {
	r.results = append(r.results, run)
	return nil
}
func (r *groupMonitorTestRepo) Release(context.Context, int64, string, int) error {
	r.released = true
	return nil
}

type monitorGroupsStub struct {
	GroupRepository
	group *Group
}

func (r monitorGroupsStub) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

type monitorUsersStub struct {
	UserRepository
	user *User
}

func (r monitorUsersStub) GetByID(context.Context, int64) (*User, error) { return r.user, nil }

type monitorSubscriptionsStub struct {
	UserSubscriptionRepository
	sub *UserSubscription
}

func (r monitorSubscriptionsStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return r.sub, nil
}
func TestGroupMonitorRoundLocksAccountAndPreservesFailure(t *testing.T) {
	repo := &groupMonitorTestRepo{}
	selected := 0
	called := []int64{}
	account := &Account{ID: 42, Name: "sampled", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	s := &GroupMonitorService{repo: repo, groups: monitorGroupsStub{group: &Group{ID: 9, Status: StatusActive}}, ctx: context.Background()}
	s.selectAccount = func(context.Context, *Group, string) (*Account, error) { selected++; return account, nil }
	s.execute = func(_ context.Context, id int64, model, prompt string) (*ScheduledTestResult, error) {
		called = append(called, id)
		if model == "model-a" {
			return &ScheduledTestResult{Status: "failed", ErrorMessage: "upstream failed", UpstreamRequestID: "req-a"}, nil
		}
		return &ScheduledTestResult{Status: "success", ResponseText: "OK", UpstreamMessageID: "msg-b"}, nil
	}
	s.run(&GroupMonitorClaim{Monitor: GroupMonitor{ID: 1, GroupID: 9, Config: GroupMonitorConfig{Models: []string{"model-a", "model-b"}, KeepHistory: 100}}, Token: "batch", Runs: []GroupMonitorRun{{ID: 1, Kind: "probe", Model: "model-a"}, {ID: 2, Kind: "probe", Model: "model-b"}}})
	require.Equal(t, 1, selected)
	require.Equal(t, []int64{42, 42}, called)
	require.Len(t, repo.results, 2)
	require.Equal(t, "failed", repo.results[0].Status)
	require.Equal(t, "req-a", repo.results[0].Detail.UpstreamRequestID)
	require.Equal(t, "success", repo.results[1].Status)
	require.Equal(t, "msg-b", repo.results[1].Detail.UpstreamMessageID)
	require.NotEqual(t, repo.results[0].Detail.TraceID, repo.results[1].Detail.TraceID)
	require.True(t, repo.released)
}
func TestGroupMonitorSelectionFailureHasHistory(t *testing.T) {
	repo := &groupMonitorTestRepo{}
	s := &GroupMonitorService{repo: repo, groups: monitorGroupsStub{group: &Group{Status: StatusActive}}, ctx: context.Background(), selectAccount: func(context.Context, *Group, string) (*Account, error) { return nil, errors.New("unavailable") }}
	s.run(&GroupMonitorClaim{Monitor: GroupMonitor{Config: GroupMonitorConfig{Models: []string{"m"}}}, Runs: []GroupMonitorRun{{ID: 1, Kind: "probe", Model: "m"}}})
	require.Len(t, repo.results, 1)
	require.Equal(t, "failed", repo.results[0].Status)
	require.Zero(t, repo.results[0].Detail.AccountID)
	require.NotEmpty(t, repo.results[0].Detail.TraceID)
}
func TestGroupMonitorVisibilityUsesCurrentGroupAndSubscription(t *testing.T) {
	group := &Group{ID: 9, Status: StatusActive, IsExclusive: true}
	user := &User{ID: 1, Status: StatusActive}
	s := &GroupMonitorService{groups: monitorGroupsStub{group: group}, users: monitorUsersStub{user: user}}
	p := GroupMonitor{GroupID: 9, Config: GroupMonitorConfig{Enabled: true}}
	require.False(t, s.Visible(context.Background(), 1, p))
	user.AllowedGroups = []int64{9}
	require.True(t, s.Visible(context.Background(), 1, p))
	group.SubscriptionType = SubscriptionTypeSubscription
	s.subscriptions = monitorSubscriptionsStub{}
	require.False(t, s.Visible(context.Background(), 1, p))
	sub := &UserSubscription{Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
	s.subscriptions = monitorSubscriptionsStub{sub: sub}
	require.True(t, s.Visible(context.Background(), 1, p))
	sub.ExpiresAt = time.Now().Add(-time.Hour)
	require.False(t, s.Visible(context.Background(), 1, p))
	group.Status = "inactive"
	require.False(t, s.Visible(context.Background(), 1, p))
}
func TestGroupMonitorRejectsUnboundedOrDuplicatePlans(t *testing.T) {
	c := GroupMonitorConfig{Models: []string{"m"}, ProbeInterval: 300, PelicanInterval: 3600, DailyLimit: 500, KeepHistory: 100}
	require.NoError(t, validateGroupMonitorConfig(&c))
	c.Models = []string{"m", "m"}
	require.Error(t, validateGroupMonitorConfig(&c))
	c.Models = []string{"m"}
	c.ProbeInterval = 0
	require.Error(t, validateGroupMonitorConfig(&c))
}

type monitorSelectionGroups struct {
	GroupRepository
	groups map[int64]*Group
}

func (r monitorSelectionGroups) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	return r.groups[id], nil
}

type monitorSelectionAccounts struct {
	AccountRepository
	groupID int64
}

func (r *monitorSelectionAccounts) ListSchedulableByGroupIDAndPlatforms(_ context.Context, id int64, _ []string) ([]Account, error) {
	r.groupID = id
	return nil, nil
}

func (r *monitorSelectionAccounts) ListSchedulableByGroupIDAndPlatform(_ context.Context, id int64, _ string) ([]Account, error) {
	r.groupID = id
	return nil, nil
}

func TestGroupMonitorSelectionStaysInConfiguredGroup(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformGemini, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			fallbackID := int64(10)
			group := &Group{ID: 9, Platform: platform, Status: StatusActive, ClaudeCodeOnly: true, FallbackGroupID: &fallbackID}
			groups := monitorSelectionGroups{groups: map[int64]*Group{
				9:  group,
				10: {ID: 10, Platform: platform, Status: StatusActive},
			}}
			accounts := &monitorSelectionAccounts{}
			gateway := &GatewayService{groupRepo: groups, accountRepo: accounts}
			openai := &OpenAIGatewayService{accountRepo: accounts}
			s := NewGroupMonitorService(nil, groups, nil, nil, nil, gateway, openai, nil)
			defer s.Stop()
			_, err := s.selectAccount(context.Background(), group, "test-model")
			require.Error(t, err) // An empty monitored group must remain unavailable.
			require.Equal(t, group.ID, accounts.groupID, "must not sample the fallback group")
		})
	}
}

func TestGroupMonitorDoesNotFollowGroupChangedToComposite(t *testing.T) {
	repo := &groupMonitorTestRepo{}
	s := &GroupMonitorService{repo: repo, groups: monitorGroupsStub{group: &Group{ID: 9, Status: StatusActive, Platform: PlatformComposite}}, ctx: context.Background()}
	s.selectAccount = func(context.Context, *Group, string) (*Account, error) {
		t.Fatal("a group changed to composite must not resolve a different account source")
		return nil, nil
	}
	s.run(&GroupMonitorClaim{Monitor: GroupMonitor{GroupID: 9, Config: GroupMonitorConfig{Models: []string{"m"}}}, Runs: []GroupMonitorRun{{ID: 1, Kind: "probe", Model: "m"}}})
	require.Len(t, repo.results, 1)
	require.Equal(t, "failed", repo.results[0].Status)
	require.Zero(t, repo.results[0].Detail.AccountID)
}
