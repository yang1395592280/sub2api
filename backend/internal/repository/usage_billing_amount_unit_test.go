//go:build unit

package repository

import (
	"context"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingRejectsInvalidAmountsBeforeTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, set := range []func(*service.UsageBillingCommand){
			func(c *service.UsageBillingCommand) { c.BalanceCost = value },
			func(c *service.UsageBillingCommand) { c.SubscriptionCost = value },
			func(c *service.UsageBillingCommand) { c.APIKeyQuotaCost = value },
			func(c *service.UsageBillingCommand) { c.APIKeyRateLimitCost = value },
			func(c *service.UsageBillingCommand) { c.AccountQuotaCost = value },
		} {
			cmd := &service.UsageBillingCommand{RequestID: "invalid-amount"}
			set(cmd)
			_, err := repo.Apply(context.Background(), cmd)
			require.EqualError(t, err, "invalid billing amount")
		}
		for _, call := range []func(context.Context, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error){
			repo.ReserveBatchImageBalance, repo.CaptureBatchImageBalance, repo.ReleaseBatchImageBalance,
		} {
			for _, cmd := range []*service.BatchImageBalanceHoldCommand{
				{RequestID: "invalid-hold", HoldAmount: value},
				{RequestID: "invalid-capture", HoldAmount: 1, ActualAmount: value},
			} {
				_, err := call(context.Background(), cmd)
				require.EqualError(t, err, "invalid image billing amount")
			}
		}
	}
	// No transaction or SQL is expected for rejected billing commands.
	require.NoError(t, mock.ExpectationsWereMet())
}
