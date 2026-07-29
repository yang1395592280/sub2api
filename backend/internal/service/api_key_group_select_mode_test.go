package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeyEffectiveGroupRepoStub struct {
	lastAPIKeyID int64
	lastGroupID  int64
	lastAt       time.Time
	err          error
}

func (s *apiKeyEffectiveGroupRepoStub) Create(context.Context, *APIKey) error {
	panic("unexpected Create call")
}

func (s *apiKeyEffectiveGroupRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	panic("unexpected GetByID call")
}

func (s *apiKeyEffectiveGroupRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}

func (s *apiKeyEffectiveGroupRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}

func (s *apiKeyEffectiveGroupRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}

func (s *apiKeyEffectiveGroupRepoStub) Update(context.Context, *APIKey, APIKeyUpdateFields) error {
	panic("unexpected Update call")
}

func (s *apiKeyEffectiveGroupRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *apiKeyEffectiveGroupRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}

func (s *apiKeyEffectiveGroupRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}

func (s *apiKeyEffectiveGroupRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}

func (s *apiKeyEffectiveGroupRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}

func (s *apiKeyEffectiveGroupRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	panic("unexpected ExistsByKey call")
}

func (s *apiKeyEffectiveGroupRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *apiKeyEffectiveGroupRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}

func (s *apiKeyEffectiveGroupRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}

func (s *apiKeyEffectiveGroupRepoStub) ClearGroupIDByUserAndGroup(context.Context, int64, int64) (int64, error) {
	panic("unexpected ClearGroupIDByUserAndGroup call")
}

func (s *apiKeyEffectiveGroupRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}

func (s *apiKeyEffectiveGroupRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}

func (s *apiKeyEffectiveGroupRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}

func (s *apiKeyEffectiveGroupRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}

func (s *apiKeyEffectiveGroupRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *apiKeyEffectiveGroupRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}

func (s *apiKeyEffectiveGroupRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}

func (s *apiKeyEffectiveGroupRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}

func (s *apiKeyEffectiveGroupRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

func (s *apiKeyEffectiveGroupRepoStub) UpdateLastEffectiveGroup(ctx context.Context, apiKeyID int64, groupID int64, at time.Time) error {
	s.lastAPIKeyID = apiKeyID
	s.lastGroupID = groupID
	s.lastAt = at
	return s.err
}

func TestAPIKeyNilGroupSelectMode_DefaultsToFixed(t *testing.T) {
	require.Equal(t, APIKeyGroupSelectModeFixed, (*APIKey)(nil).NormalizedGroupSelectMode())
}

func TestAPIKeyEmptyGroupSelectMode_DefaultsToFixed(t *testing.T) {
	require.Equal(t, APIKeyGroupSelectModeFixed, (&APIKey{}).NormalizedGroupSelectMode())
}

func TestAPIKeyUnknownGroupSelectMode_DefaultsToFixed(t *testing.T) {
	require.Equal(t, APIKeyGroupSelectModeFixed, (&APIKey{GroupSelectMode: "manual"}).NormalizedGroupSelectMode())
}

func TestAPIKeyOpenAIAutoCheapestGroupSelectMode_IsPreserved(t *testing.T) {
	key := &APIKey{GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}

	require.Equal(t, APIKeyGroupSelectModeOpenAIAutoCheapest, key.NormalizedGroupSelectMode())
}

func TestAPIKeyNormalizedGroupSelectMode_DefaultsToFixed(t *testing.T) {
	testCases := []struct {
		name string
		key  *APIKey
	}{
		{
			name: "nil key returns fixed",
			key:  nil,
		},
		{
			name: "empty mode returns fixed",
			key:  &APIKey{},
		},
		{
			name: "unknown mode returns fixed",
			key:  &APIKey{GroupSelectMode: "manual"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, APIKeyGroupSelectModeFixed, tc.key.NormalizedGroupSelectMode())
		})
	}
}

func TestAPIKeyUsesOpenAIAutoCheapestGroup(t *testing.T) {
	require.False(t, (*APIKey)(nil).UsesOpenAIAutoCheapestGroup())
	require.False(t, (&APIKey{}).UsesOpenAIAutoCheapestGroup())
	require.False(t, (&APIKey{GroupSelectMode: "fixed"}).UsesOpenAIAutoCheapestGroup())
	require.False(t, (&APIKey{GroupSelectMode: "manual"}).UsesOpenAIAutoCheapestGroup())
	require.True(t, (&APIKey{GroupSelectMode: APIKeyGroupSelectModeOpenAIAutoCheapest}).UsesOpenAIAutoCheapestGroup())
}

func TestAPIKeyRepository_ExposesUpdateLastEffectiveGroup(t *testing.T) {
	var repo APIKeyRepository = &apiKeyEffectiveGroupRepoStub{}
	at := time.Unix(1710000000, 0).UTC()

	err := repo.UpdateLastEffectiveGroup(context.Background(), 101, 202, at)

	require.NoError(t, err)
	stub := repo.(*apiKeyEffectiveGroupRepoStub)
	require.Equal(t, int64(101), stub.lastAPIKeyID)
	require.Equal(t, int64(202), stub.lastGroupID)
	require.True(t, stub.lastAt.Equal(at))
}
