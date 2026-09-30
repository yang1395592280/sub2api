//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCreateLock_ConcurrentRepositoriesRespectLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	owner, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("key-create-lock-%d@test.com", time.Now().UnixNano())).
		SetPasswordHash("test-password-hash").SetRole(service.RoleUser).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", owner.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", owner.ID)
	})
	repos := []*apiKeyRepository{
		newAPIKeyRepositoryWithSQL(integrationEntClient, integrationDB),
		newAPIKeyRepositoryWithSQL(integrationEntClient, integrationDB),
	}
	newKey := func(i int) *service.APIKey {
		return &service.APIKey{UserID: owner.ID, Key: fmt.Sprintf("sk-key-create-lock-%d-%d", owner.ID, i), Name: "concurrent", Status: service.StatusActive}
	}
	for i := 0; i < 2; i++ {
		require.NoError(t, repos[0].Create(ctx, newKey(i)))
	}
	const parallel = 8
	start := make(chan struct{})
	results := make(chan error, parallel)
	for i := 0; i < parallel; i++ {
		go func(i int) {
			<-start
			repo := repos[i%len(repos)]
			results <- repo.WithUserCreateLock(ctx, owner.ID, func(txCtx context.Context) error {
				count, err := repo.CountByUserID(txCtx, owner.ID)
				if err != nil {
					return err
				}
				if count >= 3 {
					return service.ErrAPIKeyCountExceeded
				}
				return repo.Create(txCtx, newKey(i+2))
			})
		}(i)
	}
	close(start)
	success := 0
	for i := 0; i < parallel; i++ {
		err := <-results
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrAPIKeyCountExceeded)
		}
	}
	count, err := repos[0].CountByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, 1, success, "两个仓库实例争抢同一用户的最后一个名额")
	require.Equal(t, int64(3), count)
}
