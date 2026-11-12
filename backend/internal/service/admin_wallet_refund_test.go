//go:build unit

package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

type refundUserRepoStub struct {
	UserRepository
	calls                      int
	paid, welfare              float64
	source, notes, operationID string
	actorID                    int64
	err                        error
}

func (r *refundUserRepoStub) RefundWallet(_ context.Context, _ int64, paid, welfare float64, source string, actorID int64, notes, operationID string) error {
	r.calls++
	r.paid = paid
	r.welfare = welfare
	r.source = source
	r.actorID = actorID
	r.notes = notes
	r.operationID = operationID
	return r.err
}
func (r *refundUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Balance: 10}, nil
}
func TestAdminWalletRefundServiceValidationAndForwarding(t *testing.T) {
	for _, source := range []string{"checkin", "liyu", "gift"} {
		t.Run(source, func(t *testing.T) {
			repo := &refundUserRepoStub{}
			cache := &authCacheInvalidatorStub{}
			svc := &adminServiceImpl{userRepo: repo, authCacheInvalidator: cache}
			u, err := svc.RefundUserWalletBalance(context.Background(), 7, 1.123456789, 2.123456789, source, 9, "refund", "op")
			require.NoError(t, err)
			require.Equal(t, int64(7), u.ID)
			require.Equal(t, 1, repo.calls)
			require.Equal(t, 1.12345679, repo.paid)
			require.Equal(t, 2.12345679, repo.welfare)
			require.Equal(t, source, repo.source)
			require.Equal(t, int64(9), repo.actorID)
			require.Equal(t, "refund", repo.notes)
			require.Equal(t, "op", repo.operationID)
			require.Equal(t, []int64{7}, cache.userIDs)
		})
	}
	for _, tc := range []struct {
		paid, welfare float64
		source        string
	}{{-1, 1, "gift"}, {1, -1, "gift"}, {math.NaN(), 1, "gift"}, {1, math.Inf(1), "gift"}, {0, 0, "gift"}, {0, 1, ""}, {0, 1, "other"}, {.000000001, 0, "gift"}} {
		repo := &refundUserRepoStub{}
		svc := &adminServiceImpl{userRepo: repo}
		_, err := svc.RefundUserWalletBalance(context.Background(), 7, tc.paid, tc.welfare, tc.source, 0, "", "op")
		require.Error(t, err)
		require.Zero(t, repo.calls)
	}
}
func TestAdminWalletRefundServiceRepositoryError(t *testing.T) {
	repo := &refundUserRepoStub{err: errors.New("insufficient balance")}
	svc := &adminServiceImpl{userRepo: repo}
	_, err := svc.RefundUserWalletBalance(context.Background(), 7, 1, 2, "liyu", 0, "", "op")
	require.ErrorIs(t, err, repo.err)
}
