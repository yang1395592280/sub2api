package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type groupCapacityAccountRepoStub struct {
	AccountRepository
	rows      []GroupAccountCapacityRow
	requested []int64
}

func (s *groupCapacityAccountRepoStub) ListSchedulableCapacityByGroupIDs(_ context.Context, groupIDs []int64) ([]GroupAccountCapacityRow, error) {
	s.requested = append([]int64(nil), groupIDs...)
	return append([]GroupAccountCapacityRow(nil), s.rows...), nil
}

type groupCapacityGroupRepoStub struct {
	GroupRepository
	groupIDs  []int64
	listCalls int
}

func (s *groupCapacityGroupRepoStub) ListActiveIDs(context.Context) ([]int64, error) {
	s.listCalls++
	return append([]int64(nil), s.groupIDs...), nil
}

type groupCapacityConcurrencyCacheStub struct {
	ConcurrencyCache
	counts    map[int64]int
	requested []int64
}

func (s *groupCapacityConcurrencyCacheStub) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), accountIDs...)
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = s.counts[id]
	}
	return out, nil
}

type groupCapacitySessionCacheStub struct {
	SessionLimitCache
	counts       map[int64]int
	requested    []int64
	idleTimeouts map[int64]time.Duration
}

func (s *groupCapacitySessionCacheStub) GetActiveSessionCountBatch(_ context.Context, accountIDs []int64, idleTimeouts map[int64]time.Duration) (map[int64]int, error) {
	s.requested = append([]int64(nil), accountIDs...)
	s.idleTimeouts = make(map[int64]time.Duration, len(idleTimeouts))
	for id, timeout := range idleTimeouts {
		s.idleTimeouts[id] = timeout
	}
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = s.counts[id]
	}
	return out, nil
}

type groupCapacityRPMCacheStub struct {
	RPMCache
	counts    map[int64]int
	requested []int64
}

func (s *groupCapacityRPMCacheStub) GetRPMBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), accountIDs...)
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = s.counts[id]
	}
	return out, nil
}

func TestGetAllGroupCapacityBatchAggregatesRuntimeAndLimits(t *testing.T) {
	accountRepo := &groupCapacityAccountRepoStub{
		rows: []GroupAccountCapacityRow{
			{
				GroupID:     10,
				AccountID:   1,
				Concurrency: 2,
				Extra: map[string]any{
					"max_sessions":                 3,
					"session_idle_timeout_minutes": 7,
					"base_rpm":                     11,
				},
			},
			{
				GroupID:     20,
				AccountID:   1,
				Concurrency: 2,
				Extra: map[string]any{
					"max_sessions":                 3,
					"session_idle_timeout_minutes": 7,
					"base_rpm":                     11,
				},
			},
			{
				GroupID:     20,
				AccountID:   2,
				Concurrency: 4,
				Extra: map[string]any{
					"max_sessions":                 1,
					"session_idle_timeout_minutes": 9,
					"base_rpm":                     13,
				},
			},
		},
	}
	groupRepo := &groupCapacityGroupRepoStub{groupIDs: []int64{10, 20}}
	concurrencyCache := &groupCapacityConcurrencyCacheStub{counts: map[int64]int{1: 1, 2: 2}}
	sessionCache := &groupCapacitySessionCacheStub{counts: map[int64]int{1: 2, 2: 1}}
	rpmCache := &groupCapacityRPMCacheStub{counts: map[int64]int{1: 5, 2: 7}}
	svc := NewGroupCapacityService(
		accountRepo,
		groupRepo,
		NewConcurrencyService(concurrencyCache),
		sessionCache,
		rpmCache,
		nil,
		nil,
		nil,
		nil,
	)

	results, err := svc.GetAllGroupCapacity(context.Background())
	require.NoError(t, err)

	require.Equal(t, 1, groupRepo.listCalls)
	require.Equal(t, []int64{10, 20}, accountRepo.requested)
	require.Equal(t, []int64{1, 2}, concurrencyCache.requested)
	require.ElementsMatch(t, []int64{1, 2}, sessionCache.requested)
	require.ElementsMatch(t, []int64{1, 2}, rpmCache.requested)
	require.Equal(t, 7*time.Minute, sessionCache.idleTimeouts[1])
	require.Equal(t, 9*time.Minute, sessionCache.idleTimeouts[2])

	require.Equal(t, []GroupCapacitySummary{
		{
			GroupID:         10,
			ConcurrencyUsed: 1,
			ConcurrencyMax:  2,
			SessionsUsed:    2,
			SessionsMax:     3,
			RPMUsed:         5,
			RPMMax:          11,
		},
		{
			GroupID:         20,
			ConcurrencyUsed: 3,
			ConcurrencyMax:  6,
			SessionsUsed:    3,
			SessionsMax:     4,
			RPMUsed:         12,
			RPMMax:          24,
		},
	}, results)
}

func TestGetAllGroupCapacityBatchKeepsEmptyGroupRows(t *testing.T) {
	accountRepo := &groupCapacityAccountRepoStub{
		rows: []GroupAccountCapacityRow{
			{GroupID: 20, AccountID: 2, Concurrency: 4},
		},
	}
	groupRepo := &groupCapacityGroupRepoStub{groupIDs: []int64{10, 20}}
	svc := NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil, nil, nil, nil, nil)

	results, err := svc.GetAllGroupCapacity(context.Background())
	require.NoError(t, err)

	require.Equal(t, []GroupCapacitySummary{
		{GroupID: 10},
		{GroupID: 20, ConcurrencyMax: 4},
	}, results)
}

type groupCapacityUserRepoStub struct {
	UserRepository
	usersByGroup map[int64][]User
	allUsers     []User
}

func (s *groupCapacityUserRepoStub) ListAllowedUsersByGroupID(_ context.Context, groupID int64) ([]User, error) {
	return append([]User(nil), s.usersByGroup[groupID]...), nil
}

func (s *groupCapacityUserRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _ UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return append([]User(nil), s.allUsers...), &pagination.PaginationResult{Total: int64(len(s.allUsers))}, nil
}

type groupCapacityDetailGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s *groupCapacityDetailGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	return s.groups[id], nil
}

type groupCapacityUserLoadCacheStub struct {
	ConcurrencyCache
	loads map[int64]*UserLoadInfo
}

func (s *groupCapacityUserLoadCacheStub) GetUsersLoadBatch(_ context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	out := make(map[int64]*UserLoadInfo, len(users))
	for _, user := range users {
		if load := s.loads[user.ID]; load != nil {
			copied := *load
			out[user.ID] = &copied
		}
	}
	return out, nil
}

type groupCapacityUserGroupLoadCacheStub struct {
	groupCapacityUserLoadCacheStub
	groupLoads map[int64]int
}

func (s *groupCapacityUserGroupLoadCacheStub) GetUserGroupConcurrencyBatch(_ context.Context, _ int64, userIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(userIDs))
	for _, userID := range userIDs {
		out[userID] = s.groupLoads[userID]
	}
	return out, nil
}

type groupCapacityUserRPMCacheStub struct {
	UserRPMCache
	userCounts  map[int64]int
	groupCounts map[int64]map[int64]int
}

func (s *groupCapacityUserRPMCacheStub) GetUserGroupRPM(_ context.Context, userID, groupID int64) (int, error) {
	if s.groupCounts[userID] == nil {
		return 0, nil
	}
	return s.groupCounts[userID][groupID], nil
}

func (s *groupCapacityUserRPMCacheStub) GetUserRPM(_ context.Context, userID int64) (int, error) {
	return s.userCounts[userID], nil
}

type groupCapacityRateRepoStub struct {
	UserGroupRateRepository
	overrides map[int64]map[int64]*int
}

func (s *groupCapacityRateRepoStub) GetRPMOverrideByUserAndGroup(_ context.Context, userID, groupID int64) (*int, error) {
	if s.overrides[userID] == nil {
		return nil, nil
	}
	return s.overrides[userID][groupID], nil
}

func TestGetGroupCapacityUsersFiltersActiveAndPaginates(t *testing.T) {
	groupID := int64(10)
	override := 5
	users := []User{
		{ID: 1, Username: "alpha", Email: "a@example.com", Notes: "A", Status: StatusActive, Concurrency: 3, RPMLimit: 20},
		{ID: 2, Username: "beta", Email: "b@example.com", Status: StatusActive, Concurrency: 4, RPMLimit: 30},
		{ID: 3, Username: "quiet", Email: "q@example.com", Status: StatusActive, Concurrency: 2, RPMLimit: 40},
		{ID: 4, Username: "gamma", Email: "g@example.com", Status: StatusDisabled, Concurrency: 1, RPMLimit: 0},
	}
	svc := NewGroupCapacityService(
		nil,
		&groupCapacityDetailGroupRepoStub{groups: map[int64]*Group{
			groupID: {ID: groupID, Name: "exclusive", IsExclusive: true, RPMLimit: 10},
		}},
		NewConcurrencyService(&groupCapacityUserLoadCacheStub{loads: map[int64]*UserLoadInfo{
			1: {UserID: 1, CurrentConcurrency: 2},
			4: {UserID: 4, CurrentConcurrency: 1},
		}}),
		nil,
		nil,
		&groupCapacityUserRepoStub{usersByGroup: map[int64][]User{groupID: users}},
		nil,
		&groupCapacityRateRepoStub{overrides: map[int64]map[int64]*int{
			1: {groupID: &override},
		}},
		&groupCapacityUserRPMCacheStub{
			userCounts: map[int64]int{1: 7, 2: 1, 4: 2},
			groupCounts: map[int64]map[int64]int{
				1: {groupID: 4},
				2: {groupID: 8},
			},
		},
	)

	items, total, err := svc.GetGroupCapacityUsers(context.Background(), groupID, pagination.PaginationParams{Page: 1, PageSize: 2}, true)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, []GroupCapacityUserDetail{
		{
			UserID:             1,
			Username:           "alpha",
			Email:              "a@example.com",
			Notes:              "A",
			Status:             StatusActive,
			CurrentConcurrency: 2,
			ConcurrencyLimit:   3,
			CurrentRPM:         4,
			EffectiveRPMLimit:  5,
			RPMLimitSource:     "override",
			RPMOverride:        &override,
			GroupRPMLimit:      10,
			UserRPMLimit:       20,
		},
		{
			UserID:             4,
			Username:           "gamma",
			Email:              "g@example.com",
			Status:             StatusDisabled,
			CurrentConcurrency: 1,
			ConcurrencyLimit:   1,
			CurrentRPM:         0,
			EffectiveRPMLimit:  10,
			RPMLimitSource:     "group",
			GroupRPMLimit:      10,
		},
	}, items)

	items, total, err = svc.GetGroupCapacityUsers(context.Background(), groupID, pagination.PaginationParams{Page: 2, PageSize: 2}, true)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, []GroupCapacityUserDetail{
		{
			UserID:             2,
			Username:           "beta",
			Email:              "b@example.com",
			Status:             StatusActive,
			CurrentConcurrency: 0,
			ConcurrencyLimit:   4,
			CurrentRPM:         8,
			EffectiveRPMLimit:  10,
			RPMLimitSource:     "group",
			GroupRPMLimit:      10,
			UserRPMLimit:       30,
		},
	}, items)
}

func TestGetGroupCapacityUsersPrefersGroupConcurrency(t *testing.T) {
	groupID := int64(11)
	users := []User{
		{ID: 1, Username: "alpha", Email: "a@example.com", Status: StatusActive, Concurrency: 5, RPMLimit: 0},
	}
	svc := NewGroupCapacityService(
		nil,
		&groupCapacityDetailGroupRepoStub{groups: map[int64]*Group{
			groupID: {ID: groupID, Name: "exclusive", IsExclusive: true, RPMLimit: 0},
		}},
		NewConcurrencyService(&groupCapacityUserGroupLoadCacheStub{
			groupCapacityUserLoadCacheStub: groupCapacityUserLoadCacheStub{
				loads: map[int64]*UserLoadInfo{1: {UserID: 1, CurrentConcurrency: 4}},
			},
			groupLoads: map[int64]int{1: 1},
		}),
		nil,
		nil,
		&groupCapacityUserRepoStub{usersByGroup: map[int64][]User{groupID: users}},
		nil,
		nil,
		&groupCapacityUserRPMCacheStub{},
	)

	items, total, err := svc.GetGroupCapacityUsers(context.Background(), groupID, pagination.PaginationParams{Page: 1, PageSize: 20}, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 1, items[0].CurrentConcurrency)
}

func TestGetGroupCapacityUsersUsesUserRPMFallbackForPublicGroup(t *testing.T) {
	groupID := int64(20)
	users := []User{
		{ID: 1, Username: "alpha", Email: "a@example.com", Status: StatusActive, Concurrency: 3, RPMLimit: 12},
		{ID: 2, Username: "beta", Email: "b@example.com", Status: StatusActive, Concurrency: 4, RPMLimit: 0},
	}
	svc := NewGroupCapacityService(
		nil,
		&groupCapacityDetailGroupRepoStub{groups: map[int64]*Group{
			groupID: {ID: groupID, Name: "public", IsExclusive: false, RPMLimit: 0},
		}},
		NewConcurrencyService(&groupCapacityUserLoadCacheStub{loads: map[int64]*UserLoadInfo{}}),
		nil,
		nil,
		&groupCapacityUserRepoStub{allUsers: users},
		nil,
		nil,
		&groupCapacityUserRPMCacheStub{
			userCounts:  map[int64]int{1: 6},
			groupCounts: map[int64]map[int64]int{},
		},
	)

	items, total, err := svc.GetGroupCapacityUsers(context.Background(), groupID, pagination.PaginationParams{Page: 1, PageSize: 20}, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, []GroupCapacityUserDetail{
		{
			UserID:             1,
			Username:           "alpha",
			Email:              "a@example.com",
			Status:             StatusActive,
			CurrentConcurrency: 0,
			ConcurrencyLimit:   3,
			CurrentRPM:         6,
			EffectiveRPMLimit:  12,
			RPMLimitSource:     "user",
			GroupRPMLimit:      0,
			UserRPMLimit:       12,
		},
	}, items)
}
