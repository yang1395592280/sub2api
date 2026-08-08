package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeyCreateUpdateRepoStub struct {
	created *APIKey
	updated *APIKey
	current *APIKey
}

func (s *apiKeyCreateUpdateRepoStub) Create(ctx context.Context, key *APIKey) error {
	cloned := *key
	s.created = &cloned
	key.ID = 101
	return nil
}

func (s *apiKeyCreateUpdateRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	if s.current == nil {
		return nil, ErrAPIKeyNotFound
	}
	cloned := *s.current
	return &cloned, nil
}

func (s *apiKeyCreateUpdateRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}

func (s *apiKeyCreateUpdateRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}

func (s *apiKeyCreateUpdateRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}

func (s *apiKeyCreateUpdateRepoStub) Update(ctx context.Context, key *APIKey, _ APIKeyUpdateFields) error {
	cloned := *key
	s.updated = &cloned
	return nil
}

func (s *apiKeyCreateUpdateRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *apiKeyCreateUpdateRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}

func (s *apiKeyCreateUpdateRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}

func (s *apiKeyCreateUpdateRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}

func (s *apiKeyCreateUpdateRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}

func (s *apiKeyCreateUpdateRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}

func (s *apiKeyCreateUpdateRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *apiKeyCreateUpdateRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}

func (s *apiKeyCreateUpdateRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}

func (s *apiKeyCreateUpdateRepoStub) ClearGroupIDByUserAndGroup(context.Context, int64, int64) (int64, error) {
	panic("unexpected ClearGroupIDByUserAndGroup call")
}

func (s *apiKeyCreateUpdateRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}

func (s *apiKeyCreateUpdateRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}

func (s *apiKeyCreateUpdateRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}

func (s *apiKeyCreateUpdateRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}

func (s *apiKeyCreateUpdateRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *apiKeyCreateUpdateRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}

func (s *apiKeyCreateUpdateRepoStub) UpdateLastEffectiveGroup(context.Context, int64, int64, time.Time) error {
	panic("unexpected UpdateLastEffectiveGroup call")
}

func (s *apiKeyCreateUpdateRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}

func (s *apiKeyCreateUpdateRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}

func (s *apiKeyCreateUpdateRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

type apiKeyServiceUserRepoStub struct {
	user *User
}

type apiKeyServiceGroupRepoStub struct {
	getByID func(ctx context.Context, id int64) (*Group, error)
}

func (s *apiKeyServiceGroupRepoStub) Create(context.Context, *Group) error {
	panic("unexpected Create call")
}
func (s *apiKeyServiceGroupRepoStub) GetByID(ctx context.Context, id int64) (*Group, error) {
	if s.getByID == nil {
		panic("unexpected GetByID call")
	}
	return s.getByID(ctx, id)
}
func (s *apiKeyServiceGroupRepoStub) GetByIDLite(context.Context, int64) (*Group, error) {
	panic("unexpected GetByIDLite call")
}
func (s *apiKeyServiceGroupRepoStub) Update(context.Context, *Group) error {
	panic("unexpected Update call")
}
func (s *apiKeyServiceGroupRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyServiceGroupRepoStub) DeleteCascade(context.Context, int64) ([]int64, error) {
	panic("unexpected DeleteCascade call")
}
func (s *apiKeyServiceGroupRepoStub) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyServiceGroupRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *apiKeyServiceGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	panic("unexpected ListActive call")
}
func (s *apiKeyServiceGroupRepoStub) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	panic("unexpected ListActiveByPlatform call")
}
func (s *apiKeyServiceGroupRepoStub) ExistsByName(context.Context, string) (bool, error) {
	panic("unexpected ExistsByName call")
}
func (s *apiKeyServiceGroupRepoStub) GetAccountCount(context.Context, int64) (int64, int64, error) {
	panic("unexpected GetAccountCount call")
}
func (s *apiKeyServiceGroupRepoStub) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected DeleteAccountGroupsByGroupID call")
}
func (s *apiKeyServiceGroupRepoStub) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	panic("unexpected GetAccountIDsByGroupIDs call")
}
func (s *apiKeyServiceGroupRepoStub) BindAccountsToGroup(context.Context, int64, []int64) error {
	panic("unexpected BindAccountsToGroup call")
}
func (s *apiKeyServiceGroupRepoStub) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	panic("unexpected UpdateSortOrders call")
}

type apiKeyServiceUserSubRepoStub struct {
	getActiveByUserIDAndGroupID func(ctx context.Context, userID, groupID int64) (*UserSubscription, error)
}

func (s *apiKeyServiceUserSubRepoStub) Create(context.Context, *UserSubscription) error {
	panic("unexpected Create call")
}
func (s *apiKeyServiceUserSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}
func (s *apiKeyServiceUserSubRepoStub) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDForUpdate call")
}
func (s *apiKeyServiceUserSubRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (s *apiKeyServiceUserSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndGroupID call")
}
func (s *apiKeyServiceUserSubRepoStub) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	if s.getActiveByUserIDAndGroupID == nil {
		panic("unexpected GetActiveByUserIDAndGroupID call")
	}
	return s.getActiveByUserIDAndGroupID(ctx, userID, groupID)
}
func (s *apiKeyServiceUserSubRepoStub) Update(context.Context, *UserSubscription) error {
	panic("unexpected Update call")
}
func (s *apiKeyServiceUserSubRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyServiceUserSubRepoStub) Restore(context.Context, int64, string) (*UserSubscription, error) {
	panic("unexpected Restore call")
}
func (s *apiKeyServiceUserSubRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}
func (s *apiKeyServiceUserSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListActiveByUserID call")
}
func (s *apiKeyServiceUserSubRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *apiKeyServiceUserSubRepoStub) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyServiceUserSubRepoStub) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndGroupID call")
}
func (s *apiKeyServiceUserSubRepoStub) ExistsActiveByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsActiveByUserIDAndGroupID call")
}
func (s *apiKeyServiceUserSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error {
	panic("unexpected ExtendExpiry call")
}
func (s *apiKeyServiceUserSubRepoStub) UpdateStatus(context.Context, int64, string) error {
	panic("unexpected UpdateStatus call")
}
func (s *apiKeyServiceUserSubRepoStub) UpdateNotes(context.Context, int64, string) error {
	panic("unexpected UpdateNotes call")
}
func (s *apiKeyServiceUserSubRepoStub) ActivateWindows(context.Context, int64, time.Time, time.Time) error {
	panic("unexpected ActivateWindows call")
}
func (s *apiKeyServiceUserSubRepoStub) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time, time.Time) error {
	panic("unexpected ResetUsageWindows call")
}
func (s *apiKeyServiceUserSubRepoStub) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetDailyUsage call")
}
func (s *apiKeyServiceUserSubRepoStub) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}
func (s *apiKeyServiceUserSubRepoStub) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}
func (s *apiKeyServiceUserSubRepoStub) IncrementUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementUsage call")
}
func (s *apiKeyServiceUserSubRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

func (s *apiKeyServiceUserRepoStub) Create(context.Context, *User) error {
	panic("unexpected Create call")
}
func (s *apiKeyServiceUserRepoStub) CreateWithEmailAliasGuard(context.Context, *User) error {
	panic("unexpected CreateWithEmailAliasGuard call")
}
func (s *apiKeyServiceUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	cloned := *s.user
	return &cloned, nil
}
func (s *apiKeyServiceUserRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (s *apiKeyServiceUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected GetByEmail call")
}
func (s *apiKeyServiceUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}
func (s *apiKeyServiceUserRepoStub) Update(context.Context, *User, UserUpdateFields) error {
	panic("unexpected Update call")
}
func (s *apiKeyServiceUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *apiKeyServiceUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("unexpected GetUserAvatar call")
}
func (s *apiKeyServiceUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertUserAvatar call")
}
func (s *apiKeyServiceUserRepoStub) DeleteUserAvatar(context.Context, int64) error {
	panic("unexpected DeleteUserAvatar call")
}
func (s *apiKeyServiceUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *apiKeyServiceUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *apiKeyServiceUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserIDs call")
}
func (s *apiKeyServiceUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserID call")
}
func (s *apiKeyServiceUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("unexpected UpdateUserLastActiveAt call")
}
func (s *apiKeyServiceUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}
func (s *apiKeyServiceUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}
func (s *apiKeyServiceUserRepoStub) AdjustBalance(context.Context, int64, float64) (BalanceChange, error) {
	panic("unexpected AdjustBalance call")
}
func (s *apiKeyServiceUserRepoStub) SetBalance(context.Context, int64, float64) (BalanceChange, error) {
	panic("unexpected SetBalance call")
}
func (s *apiKeyServiceUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchSetConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchAddConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) BatchUpdateLimits(context.Context, []int64, *int, *int) (int, error) {
	panic("unexpected BatchUpdateLimits call")
}
func (s *apiKeyServiceUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}
func (s *apiKeyServiceUserRepoStub) ExistsByEmailAlias(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmailAlias call")
}
func (s *apiKeyServiceUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}
func (s *apiKeyServiceUserRepoStub) ListAllowedUsersByGroupID(context.Context, int64) ([]User, error) {
	panic("unexpected ListAllowedUsersByGroupID call")
}
func (s *apiKeyServiceUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}
func (s *apiKeyServiceUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}
func (s *apiKeyServiceUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected ListUserAuthIdentities call")
}
func (s *apiKeyServiceUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected UnbindUserAuthProvider call")
}
func (s *apiKeyServiceUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}
func (s *apiKeyServiceUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}
func (s *apiKeyServiceUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func newAPIKeyServiceCreateTestHarness(t *testing.T) (*APIKeyService, *apiKeyCreateUpdateRepoStub) {
	t.Helper()

	repo := &apiKeyCreateUpdateRepoStub{}
	userRepo := &apiKeyServiceUserRepoStub{
		user: &User{
			ID:     42,
			Status: StatusActive,
			Role:   RoleUser,
		},
	}
	svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})
	return svc, repo
}

func newAPIKeyServiceUpdateTestHarness(t *testing.T, current *APIKey) *APIKeyService {
	t.Helper()

	repo := &apiKeyCreateUpdateRepoStub{current: current}
	return NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})
}

func ptrString(value string) *string {
	return &value
}

func TestAPIKeyServiceCreate_OpenAIAutoCheapestAllowsNilGroup(t *testing.T) {
	req := CreateAPIKeyRequest{
		Name:            "auto",
		GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest,
		GroupID:         nil,
	}
	svc, repo := newAPIKeyServiceCreateTestHarness(t)

	got, err := svc.Create(context.Background(), 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, APIKeyGroupSelectModeOpenAIAutoCheapest, repo.created.GroupSelectMode)
	require.Nil(t, repo.created.GroupID)
	require.NotNil(t, repo.created.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, OpenAIAutoCheapestDefaultMaxRate, *repo.created.OpenAIAutoGroupMaxRateMultiplier)
}

func TestAPIKeyServiceCreate_OpenAIAutoCheapestZeroMaxRateUsesDefault(t *testing.T) {
	zeroRate := 0.0
	req := CreateAPIKeyRequest{
		Name:                             "auto-default-budget",
		GroupSelectMode:                  APIKeyGroupSelectModeOpenAIAutoCheapest,
		OpenAIAutoGroupMaxRateMultiplier: &zeroRate,
	}
	svc, repo := newAPIKeyServiceCreateTestHarness(t)

	got, err := svc.Create(context.Background(), 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.created.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, OpenAIAutoCheapestDefaultMaxRate, *repo.created.OpenAIAutoGroupMaxRateMultiplier)
}

func TestAPIKeyServiceCreate_OpenAIAutoCheapestStoresMaxRateMultiplier(t *testing.T) {
	maxRate := 0.8
	req := CreateAPIKeyRequest{
		Name:                             "auto-budget",
		GroupSelectMode:                  APIKeyGroupSelectModeOpenAIAutoCheapest,
		OpenAIAutoGroupMaxRateMultiplier: &maxRate,
	}
	svc, repo := newAPIKeyServiceCreateTestHarness(t)

	got, err := svc.Create(context.Background(), 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.created.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, maxRate, *repo.created.OpenAIAutoGroupMaxRateMultiplier)
}

func TestAPIKeyServiceUpdate_FixedRequiresGroupWhenSwitchingFromAuto(t *testing.T) {
	req := UpdateAPIKeyRequest{
		GroupSelectMode: ptrString(APIKeyGroupSelectModeFixed),
		GroupID:         nil,
	}
	svc := newAPIKeyServiceUpdateTestHarness(t, &APIKey{
		ID:              7,
		UserID:          42,
		GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest,
		GroupID:         nil,
		Status:          StatusActive,
	})

	_, err := svc.Update(context.Background(), 7, 42, req)

	require.ErrorIs(t, err, ErrGroupRequired)
}

func TestAPIKeyServiceUpdate_FixedGroupClearsMaxRateMultiplier(t *testing.T) {
	groupID := int64(12)
	existingMaxRate := 0.8
	req := UpdateAPIKeyRequest{
		GroupSelectMode: ptrString(APIKeyGroupSelectModeFixed),
		GroupID:         &groupID,
	}
	repo := &apiKeyCreateUpdateRepoStub{
		current: &APIKey{
			ID:                               7,
			UserID:                           42,
			GroupSelectMode:                  APIKeyGroupSelectModeOpenAIAutoCheapest,
			OpenAIAutoGroupMaxRateMultiplier: &existingMaxRate,
			Status:                           StatusActive,
			Key:                              "sk-test",
		},
	}
	svc := NewAPIKeyService(
		repo,
		&apiKeyServiceUserRepoStub{
			user: &User{ID: 42, Status: StatusActive, Role: RoleUser},
		},
		&apiKeyServiceGroupRepoStub{
			getByID: func(context.Context, int64) (*Group, error) {
				return &Group{ID: groupID, Status: StatusActive}, nil
			},
		},
		&apiKeyServiceUserSubRepoStub{
			getActiveByUserIDAndGroupID: func(context.Context, int64, int64) (*UserSubscription, error) {
				return nil, errors.New("unexpected subscription lookup")
			},
		},
		nil,
		nil,
		&config.Config{},
	)

	got, err := svc.Update(context.Background(), 7, 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Nil(t, repo.updated.OpenAIAutoGroupMaxRateMultiplier)
}

func TestAPIKeyServiceUpdate_OpenAIAutoCheapestStoresMaxRateMultiplier(t *testing.T) {
	maxRate := 0.5
	req := UpdateAPIKeyRequest{
		GroupSelectMode:                  ptrString(APIKeyGroupSelectModeOpenAIAutoCheapest),
		OpenAIAutoGroupMaxRateMultiplier: &maxRate,
	}
	svc := newAPIKeyServiceUpdateTestHarness(t, &APIKey{
		ID:              7,
		UserID:          42,
		GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest,
		Status:          StatusActive,
		Key:             "sk-test",
	})

	got, err := svc.Update(context.Background(), 7, 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, maxRate, *got.OpenAIAutoGroupMaxRateMultiplier)
}

func TestAPIKeyServiceUpdate_OpenAIAutoCheapestIgnoresRequestedInvalidGroup(t *testing.T) {
	req := UpdateAPIKeyRequest{
		GroupSelectMode: ptrString(APIKeyGroupSelectModeOpenAIAutoCheapest),
		GroupID:         ptrInt64(999),
	}
	repo := &apiKeyCreateUpdateRepoStub{
		current: &APIKey{
			ID:              7,
			UserID:          42,
			GroupSelectMode: APIKeyGroupSelectModeFixed,
			GroupID:         ptrInt64(12),
			Group:           &Group{ID: 12, Name: "fixed"},
			Status:          StatusActive,
			Key:             "sk-test",
		},
	}
	svc := NewAPIKeyService(
		repo,
		&apiKeyServiceUserRepoStub{
			user: &User{ID: 42, Status: StatusActive, Role: RoleUser},
		},
		&apiKeyServiceGroupRepoStub{
			getByID: func(context.Context, int64) (*Group, error) {
				return nil, ErrGroupNotFound
			},
		},
		&apiKeyServiceUserSubRepoStub{
			getActiveByUserIDAndGroupID: func(context.Context, int64, int64) (*UserSubscription, error) {
				return nil, errors.New("unexpected subscription lookup")
			},
		},
		nil,
		nil,
		&config.Config{},
	)

	got, err := svc.Update(context.Background(), 7, 42, req)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.updated)
	require.Equal(t, APIKeyGroupSelectModeOpenAIAutoCheapest, repo.updated.GroupSelectMode)
	require.Nil(t, repo.updated.GroupID)
	require.Nil(t, repo.updated.Group)
	require.NotNil(t, repo.updated.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, OpenAIAutoCheapestDefaultMaxRate, *repo.updated.OpenAIAutoGroupMaxRateMultiplier)
}

func ptrInt64(value int64) *int64 {
	return &value
}
