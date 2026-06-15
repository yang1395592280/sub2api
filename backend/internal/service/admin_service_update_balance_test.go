//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type balanceUserRepoStub struct {
	*userRepoStub
	updateErr error
	updated   []*User
}

func (s *balanceUserRepoStub) Update(ctx context.Context, user *User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if user == nil {
		return nil
	}
	clone := *user
	s.updated = append(s.updated, &clone)
	if s.userRepoStub != nil {
		s.userRepoStub.user = &clone
	}
	return nil
}

type batchBalanceUserRepoStub struct {
	*userRepoStub
	users   map[int64]*User
	updated []*User
}

func (s *batchBalanceUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	clone := *user
	return &clone, nil
}

func (s *batchBalanceUserRepoStub) Update(ctx context.Context, user *User) error {
	if user == nil {
		return nil
	}
	clone := *user
	s.updated = append(s.updated, &clone)
	s.users[user.ID] = &clone
	return nil
}

type balanceRedeemRepoStub struct {
	*redeemRepoStub
	created []*RedeemCode
}

func (s *balanceRedeemRepoStub) Create(ctx context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	clone := *code
	s.created = append(s.created, &clone)
	return nil
}

type authCacheInvalidatorStub struct {
	userIDs  []int64
	groupIDs []int64
	keys     []string
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	s.keys = append(s.keys, key)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	s.groupIDs = append(s.groupIDs, groupID)
}

func TestAdminService_UpdateUserBalance_InvalidatesAuthCache(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 5, "add", "")
	require.NoError(t, err)
	require.Equal(t, []int64{7}, invalidator.userIDs)
	require.Len(t, redeemRepo.created, 1)
}

func TestAdminService_UpdateUserBalance_NoChangeNoInvalidate(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 10, "set", "")
	require.NoError(t, err)
	require.Empty(t, invalidator.userIDs)
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_BatchAddBalanceToUsers_DeduplicatesAndRecordsAdjustments(t *testing.T) {
	repo := &batchBalanceUserRepoStub{
		userRepoStub: &userRepoStub{},
		users: map[int64]*User{
			7: {ID: 7, Balance: 10},
			8: {ID: 8, Balance: 20},
		},
	}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	affected, err := svc.BatchAddBalanceToUsers(context.Background(), []int64{7, 8, 7, 0, -2}, 5, "add", "bonus")
	require.NoError(t, err)
	require.Equal(t, 2, affected)
	require.Len(t, repo.updated, 2)
	require.Equal(t, 15.0, repo.users[7].Balance)
	require.Equal(t, 25.0, repo.users[8].Balance)
	require.Equal(t, []int64{7, 8}, invalidator.userIDs)
	require.Len(t, redeemRepo.created, 2)
	require.Equal(t, 5.0, redeemRepo.created[0].Value)
	require.Equal(t, "bonus", redeemRepo.created[0].Notes)
	require.Equal(t, 5.0, redeemRepo.created[1].Value)
	require.Equal(t, "bonus", redeemRepo.created[1].Notes)
}

func TestAdminService_BatchAddBalanceToUsers_SubtractsAndRecordsAdjustments(t *testing.T) {
	repo := &batchBalanceUserRepoStub{
		userRepoStub: &userRepoStub{},
		users: map[int64]*User{
			7: {ID: 7, Balance: 10},
			8: {ID: 8, Balance: 20},
		},
	}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	affected, err := svc.BatchAddBalanceToUsers(context.Background(), []int64{7, 8, 7}, 3, "subtract", "refund")
	require.NoError(t, err)
	require.Equal(t, 2, affected)
	require.Equal(t, 7.0, repo.users[7].Balance)
	require.Equal(t, 17.0, repo.users[8].Balance)
	require.Equal(t, []int64{7, 8}, invalidator.userIDs)
	require.Len(t, redeemRepo.created, 2)
	require.Equal(t, -3.0, redeemRepo.created[0].Value)
	require.Equal(t, "refund", redeemRepo.created[0].Notes)
	require.Equal(t, -3.0, redeemRepo.created[1].Value)
	require.Equal(t, "refund", redeemRepo.created[1].Notes)
}
