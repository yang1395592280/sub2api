package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type selfHostedPoolGroupRepo struct {
	groups         map[int64]*Group
	nextID         int64
	referenceCount int64
}

func (r *selfHostedPoolGroupRepo) clone(id int64) (*Group, error) {
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cloned := *group
	if group.SelfHostedPoolGroupID != nil {
		poolID := *group.SelfHostedPoolGroupID
		cloned.SelfHostedPoolGroupID = &poolID
	}
	return &cloned, nil
}

func (r *selfHostedPoolGroupRepo) Create(_ context.Context, group *Group) error {
	if r.nextID == 0 {
		r.nextID = 100
	}
	group.ID = r.nextID
	r.nextID++
	cloned := *group
	r.groups[group.ID] = &cloned
	return nil
}

func (r *selfHostedPoolGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	return r.clone(id)
}

func (r *selfHostedPoolGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	return r.clone(id)
}

func (r *selfHostedPoolGroupRepo) Update(_ context.Context, group *Group) error {
	cloned := *group
	r.groups[group.ID] = &cloned
	return nil
}

func (r *selfHostedPoolGroupRepo) Delete(context.Context, int64) error { return nil }

func (r *selfHostedPoolGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) {
	return nil, nil
}

func (r *selfHostedPoolGroupRepo) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *selfHostedPoolGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *selfHostedPoolGroupRepo) ListActive(context.Context) ([]Group, error) {
	return nil, nil
}

func (r *selfHostedPoolGroupRepo) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	return nil, nil
}

func (r *selfHostedPoolGroupRepo) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	return nil, nil
}

func (r *selfHostedPoolGroupRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}

func (r *selfHostedPoolGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}

func (r *selfHostedPoolGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}

func (r *selfHostedPoolGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}

func (r *selfHostedPoolGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error {
	return nil
}

func (r *selfHostedPoolGroupRepo) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	return nil
}

func (r *selfHostedPoolGroupRepo) CountSelfHostedPoolReferences(context.Context, int64) (int64, error) {
	return r.referenceCount, nil
}

func newSelfHostedPoolGroupRepo() *selfHostedPoolGroupRepo {
	return &selfHostedPoolGroupRepo{
		groups: map[int64]*Group{
			99: {
				ID:        99,
				Name:      "shared-pool",
				Platform:  PlatformOpenAI,
				GroupRole: GroupRoleSelfHostedPool,
				Status:    StatusActive,
			},
		},
	}
}

func TestAdminServiceSelfHostedPool_AllowsMultipleGroupsToSharePool(t *testing.T) {
	repo := newSelfHostedPoolGroupRepo()
	svc := &adminServiceImpl{groupRepo: repo}
	poolID := int64(99)

	groupB, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "rate-015", Platform: PlatformOpenAI, RateMultiplier: 0.15,
		SelfHostedPoolGroupID: &poolID,
	})
	require.NoError(t, err)
	groupC, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "rate-020", Platform: PlatformOpenAI, RateMultiplier: 0.20,
		SelfHostedPoolGroupID: &poolID,
	})
	require.NoError(t, err)
	require.Equal(t, poolID, *groupB.SelfHostedPoolGroupID)
	require.Equal(t, poolID, *groupC.SelfHostedPoolGroupID)
}

func TestAdminServiceSelfHostedPool_UpdateAssociationUsesTriState(t *testing.T) {
	repo := newSelfHostedPoolGroupRepo()
	poolID := int64(99)
	repo.groups[15] = &Group{ID: 15, Name: "rate-015", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, SelfHostedPoolGroupID: &poolID}
	repo.groups[20] = &Group{ID: 20, Name: "rate-020", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard, SelfHostedPoolGroupID: &poolID}
	svc := &adminServiceImpl{groupRepo: repo}

	preserved, err := svc.UpdateGroup(context.Background(), 15, &UpdateGroupInput{})
	require.NoError(t, err)
	require.Equal(t, poolID, *preserved.SelfHostedPoolGroupID)

	cleared, err := svc.UpdateGroup(context.Background(), 15, &UpdateGroupInput{SelfHostedPoolGroupIDSet: true})
	require.NoError(t, err)
	require.Nil(t, cleared.SelfHostedPoolGroupID)
	require.Equal(t, poolID, *repo.groups[20].SelfHostedPoolGroupID)
}

func TestAdminServiceSelfHostedPool_RejectsInvalidAssociations(t *testing.T) {
	repo := newSelfHostedPoolGroupRepo()
	svc := &adminServiceImpl{groupRepo: repo}
	poolID := int64(99)

	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "anthropic", Platform: PlatformAnthropic, RateMultiplier: 1,
		SelfHostedPoolGroupID: &poolID,
	})
	require.ErrorContains(t, err, "only standard openai groups")

	_, err = svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "nested", Platform: PlatformOpenAI, GroupRole: GroupRoleSelfHostedPool, RateMultiplier: 1,
		SelfHostedPoolGroupID: &poolID,
	})
	require.ErrorContains(t, err, "cannot reference another pool")

	repo.groups[15] = &Group{ID: 15, Name: "rate-015", Platform: PlatformOpenAI, GroupRole: GroupRoleStandard}
	selfID := int64(15)
	_, err = svc.UpdateGroup(context.Background(), 15, &UpdateGroupInput{
		SelfHostedPoolGroupID: &selfID, SelfHostedPoolGroupIDSet: true,
	})
	require.ErrorContains(t, err, "cannot reference itself")

	poolRole := GroupRoleSelfHostedPool
	_, err = svc.UpdateGroup(context.Background(), 15, &UpdateGroupInput{GroupRole: &poolRole})
	require.ErrorContains(t, err, "cannot be changed")
}

func TestAdminServiceSelfHostedPool_RejectsDeletingReferencedPool(t *testing.T) {
	repo := newSelfHostedPoolGroupRepo()
	repo.referenceCount = 2
	svc := &adminServiceImpl{groupRepo: repo}

	err := svc.DeleteGroup(context.Background(), 99)
	require.ErrorContains(t, err, "referenced by 2 group")
}

func TestAdminServiceSelfHostedPool_RejectsDirectBusinessAssignments(t *testing.T) {
	repo := newSelfHostedPoolGroupRepo()
	adminSvc := &adminServiceImpl{groupRepo: repo}
	poolID := int64(99)

	_, err := adminSvc.CreateUser(context.Background(), &CreateUserInput{AllowedGroups: []int64{poolID}})
	require.ErrorIs(t, err, ErrSelfHostedPoolNotAssignable)

	_, err = adminSvc.UpdateUser(context.Background(), 1, &UpdateUserInput{AllowedGroups: &[]int64{poolID}})
	require.ErrorIs(t, err, ErrSelfHostedPoolNotAssignable)

	_, err = adminSvc.BatchAddUsersToGroup(context.Background(), []int64{1}, poolID)
	require.ErrorIs(t, err, ErrSelfHostedPoolNotAssignable)

	_, err = adminSvc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1, Type: RedeemTypeSubscription, Value: 1, GroupID: &poolID,
	})
	require.ErrorIs(t, err, ErrSelfHostedPoolNotAssignable)

	subscriptionSvc := &SubscriptionService{groupRepo: repo}
	_, err = subscriptionSvc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 1, GroupID: poolID, ValidityDays: 30,
	})
	require.ErrorIs(t, err, ErrSelfHostedPoolNotAssignable)
}
