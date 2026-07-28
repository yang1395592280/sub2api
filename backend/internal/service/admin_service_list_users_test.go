//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type userRepoStubForListUsers struct {
	userRepoStub
	users                 []User
	err                   error
	listWithFiltersParams pagination.PaginationParams
	lastUsedByUserID      map[int64]*time.Time
	lastUsedErr           error
	summaryRole           string
	summaryBalance        float64
	summaryUserCount      int64
}

func (s *userRepoStubForListUsers) ListWithFilters(_ context.Context, params pagination.PaginationParams, _ UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listWithFiltersParams = params
	if s.err != nil {
		return nil, nil, s.err
	}
	out := make([]User, len(s.users))
	copy(out, s.users)
	return out, &pagination.PaginationResult{
		Total:    int64(len(out)),
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserIDs(_ context.Context, userIDs []int64) (map[int64]*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	result := make(map[int64]*time.Time, len(userIDs))
	for _, userID := range userIDs {
		if ts, ok := s.lastUsedByUserID[userID]; ok {
			result[userID] = ts
		}
	}
	return result, nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserID(_ context.Context, userID int64) (*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	return s.lastUsedByUserID[userID], nil
}

func (s *userRepoStubForListUsers) SumBalanceByRole(_ context.Context, role string) (float64, int64, error) {
	s.summaryRole = role
	return s.summaryBalance, s.summaryUserCount, nil
}

type userGroupRateRepoStubForListUsers struct {
	batchCalls int
	singleCall []int64

	batchErr  error
	batchData map[int64]map[int64]float64

	singleErr  map[int64]error
	singleData map[int64]map[int64]float64
}

func (s *userGroupRateRepoStubForListUsers) GetByUserIDs(_ context.Context, _ []int64) (map[int64]map[int64]float64, error) {
	s.batchCalls++
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	return s.batchData, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserID(_ context.Context, userID int64) (map[int64]float64, error) {
	s.singleCall = append(s.singleCall, userID)
	if err, ok := s.singleErr[userID]; ok {
		return nil, err
	}
	if rates, ok := s.singleData[userID]; ok {
		return rates, nil
	}
	return map[int64]float64{}, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserAndGroup(_ context.Context, userID, groupID int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) GetRPMOverrideByUserAndGroup(_ context.Context, _, _ int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	panic("unexpected SyncUserGroupRates call")
}

func (s *userGroupRateRepoStubForListUsers) GetByGroupID(_ context.Context, _ int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRateMultipliers(_ context.Context, _ int64, _ []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRPMOverrides(_ context.Context, _ int64, _ []GroupRPMOverrideInput) error {
	panic("unexpected SyncGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) ClearGroupRPMOverrides(_ context.Context, _ int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByGroupID(_ context.Context, _ int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByUserID(_ context.Context, userID int64) error {
	panic("unexpected DeleteByUserID call")
}

func TestAdminService_ListUsers_BatchRateFallbackToSingle(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 101, Username: "u1"},
			{ID: 202, Username: "u2"},
		},
	}
	rateRepo := &userGroupRateRepoStubForListUsers{
		batchErr: errors.New("batch unavailable"),
		singleData: map[int64]map[int64]float64{
			101: {11: 1.1},
			202: {22: 2.2},
		},
	}
	svc := &adminServiceImpl{
		userRepo:          userRepo,
		userGroupRateRepo: rateRepo,
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, users, 2)
	require.Equal(t, 1, rateRepo.batchCalls)
	require.ElementsMatch(t, []int64{101, 202}, rateRepo.singleCall)
	require.Equal(t, 1.1, users[0].GroupRates[11])
	require.Equal(t, 2.2, users[1].GroupRates[22])
}

func TestAdminService_ListUsers_PassesSortParams(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 1, Email: "a@example.com"}},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	_, _, err := svc.ListUsers(context.Background(), 2, 50, UserListFilters{}, "email", "ASC")
	require.NoError(t, err)
	require.Equal(t, pagination.PaginationParams{
		Page:      2,
		PageSize:  50,
		SortBy:    "email",
		SortOrder: "ASC",
	}, userRepo.listWithFiltersParams)
}

func TestAdminService_ListUsers_PopulatesLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 101, Email: "u@example.com"}},
		lastUsedByUserID: map[int64]*time.Time{
			101: &lastUsed,
		},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].LastUsedAt)
	require.WithinDuration(t, lastUsed, *users[0].LastUsedAt, time.Second)
}

type userRepoStubForBatchAddGroup struct {
	userRepoStub
	addCalls []struct {
		userID  int64
		groupID int64
	}
}

func (s *userRepoStubForBatchAddGroup) AddGroupToAllowedGroups(_ context.Context, userID int64, groupID int64) error {
	s.addCalls = append(s.addCalls, struct {
		userID  int64
		groupID int64
	}{userID: userID, groupID: groupID})
	return nil
}

type authCacheInvalidatorStubForBatchAddGroup struct {
	userIDs []int64
}

func (s *authCacheInvalidatorStubForBatchAddGroup) InvalidateAuthCacheByKey(context.Context, string) {}
func (s *authCacheInvalidatorStubForBatchAddGroup) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}
func (s *authCacheInvalidatorStubForBatchAddGroup) InvalidateAuthCacheByGroupID(context.Context, int64) {}

type groupRepoStubForBatchAddGroup struct {
	group *Group
}

func (s *groupRepoStubForBatchAddGroup) GetByID(context.Context, int64) (*Group, error) {
	return s.group, nil
}

func (s *groupRepoStubForBatchAddGroup) Create(context.Context, *Group) error { panic("unexpected") }
func (s *groupRepoStubForBatchAddGroup) GetByIDLite(context.Context, int64) (*Group, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) Update(context.Context, *Group) error { panic("unexpected") }
func (s *groupRepoStubForBatchAddGroup) Delete(context.Context, int64) error  { panic("unexpected") }
func (s *groupRepoStubForBatchAddGroup) DeleteCascade(context.Context, int64) ([]int64, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) ListActive(context.Context) ([]Group, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) ExistsByName(context.Context, string) (bool, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) GetAccountCount(context.Context, int64) (int64, int64, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) BindAccountsToGroup(context.Context, int64, []int64) error {
	panic("unexpected")
}
func (s *groupRepoStubForBatchAddGroup) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	panic("unexpected")
}

func TestAdminService_GetUserBalanceSummary_UsesRegularUserRoleAggregate(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		summaryBalance:   123.45,
		summaryUserCount: 3,
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	summary, err := svc.GetUserBalanceSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, RoleUser, userRepo.summaryRole)
	require.Equal(t, 123.45, summary.TotalBalance)
	require.Equal(t, int64(3), summary.UserCount)
}

func TestAdminService_BatchAddUsersToGroup_DeduplicatesUsersAndKeepsExistingAccess(t *testing.T) {
	userRepo := &userRepoStubForBatchAddGroup{}
	groupRepo := &groupRepoStubForBatchAddGroup{
		group: &Group{
			ID:               9,
			Name:             "vip",
			Status:           StatusActive,
			IsExclusive:      true,
			SubscriptionType: SubscriptionTypeStandard,
		},
	}
	authInvalidator := &authCacheInvalidatorStubForBatchAddGroup{}
	svc := &adminServiceImpl{
		userRepo:              userRepo,
		groupRepo:             groupRepo,
		authCacheInvalidator:  authInvalidator,
	}

	result, err := svc.BatchAddUsersToGroup(context.Background(), []int64{7, 7, 8, 0}, 9)
	require.NoError(t, err)
	require.Equal(t, int64(9), result.GroupID)
	require.Equal(t, int64(2), result.ProcessedUsers)
	require.Len(t, userRepo.addCalls, 2)
	require.Equal(t, []int64{7, 8}, authInvalidator.userIDs)
}
