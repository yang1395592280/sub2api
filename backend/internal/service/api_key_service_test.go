package service

import (
	"context"
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

func (s *apiKeyCreateUpdateRepoStub) Update(ctx context.Context, key *APIKey) error {
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

func (s *apiKeyServiceUserRepoStub) Create(context.Context, *User) error {
	panic("unexpected Create call")
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
func (s *apiKeyServiceUserRepoStub) Update(context.Context, *User) error {
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
func (s *apiKeyServiceUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchSetConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected BatchAddConcurrency call")
}
func (s *apiKeyServiceUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
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
