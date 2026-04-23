package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type adminBalanceHistoryRedeemRepoStub struct {
	items            []RedeemCode
	totalRecharged   float64
	listByUserErr    error
	totalRechargeErr error
}

func (s *adminBalanceHistoryRedeemRepoStub) Create(context.Context, *RedeemCode) error {
	panic("unexpected Create call")
}

func (s *adminBalanceHistoryRedeemRepoStub) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (s *adminBalanceHistoryRedeemRepoStub) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (s *adminBalanceHistoryRedeemRepoStub) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}

func (s *adminBalanceHistoryRedeemRepoStub) Update(context.Context, *RedeemCode) error {
	panic("unexpected Update call")
}

func (s *adminBalanceHistoryRedeemRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *adminBalanceHistoryRedeemRepoStub) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}

func (s *adminBalanceHistoryRedeemRepoStub) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *adminBalanceHistoryRedeemRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *adminBalanceHistoryRedeemRepoStub) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (s *adminBalanceHistoryRedeemRepoStub) ListByUserPaginated(_ context.Context, _ int64, params pagination.PaginationParams, _ string) ([]RedeemCode, *pagination.PaginationResult, error) {
	if s.listByUserErr != nil {
		return nil, nil, s.listByUserErr
	}

	start := params.Offset()
	if start > len(s.items) {
		start = len(s.items)
	}
	end := start + params.Limit()
	if end > len(s.items) {
		end = len(s.items)
	}

	return append([]RedeemCode(nil), s.items[start:end]...), &pagination.PaginationResult{
		Total:    int64(len(s.items)),
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

func (s *adminBalanceHistoryRedeemRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	if s.totalRechargeErr != nil {
		return 0, s.totalRechargeErr
	}
	return s.totalRecharged, nil
}

func TestGetUserBalanceHistoryIncludesGameAndCheckinActivity(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	usedAt := now.Add(-3 * time.Hour)
	settledAt := now.Add(-30 * time.Minute)
	balanceAfter := 60.0

	redeemRepo := &adminBalanceHistoryRedeemRepoStub{
		items: []RedeemCode{
			{
				ID:        1,
				Code:      "BAL-1",
				Type:      RedeemTypeBalance,
				Value:     50,
				Status:    StatusUsed,
				CreatedAt: usedAt,
				UsedAt:    &usedAt,
			},
		},
		totalRecharged: 50,
	}
	checkinRepo := &checkinRepoStub{
		records: []CheckinRecord{
			{
				ID:               2,
				UserID:           7,
				CheckinDate:      "2026-04-23",
				RewardAmount:     0.02,
				BaseRewardAmount: 0.02,
				BonusStatus:      CheckinBonusStatusNone,
				CreatedAt:        now.Add(-2 * time.Hour),
			},
		},
	}
	sizeBetRepo := &sizeBetQueryRepoStub{
		historyItems: []SizeBetUserHistoryItem{
			{
				BetID:           3,
				RoundID:         10,
				RoundNo:         1001,
				StakeAmount:     10,
				PayoutAmount:    20,
				NetResultAmount: 10,
				Status:          SizeBetStatusWon,
				BalanceAfter:    &balanceAfter,
				PlacedAt:        now.Add(-1 * time.Hour),
				SettledAt:       &settledAt,
			},
		},
		historyPagination: &pagination.PaginationResult{
			Total:    1,
			Page:     1,
			PageSize: 50,
		},
	}

	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		checkinRepo:    checkinRepo,
		sizeBetRepo:    sizeBetRepo,
	}

	items, total, totalRecharged, err := svc.GetUserBalanceHistory(context.Background(), 7, 1, 20, "")
	require.NoError(t, err)
	require.Equal(t, 50.0, totalRecharged)
	require.GreaterOrEqual(t, total, int64(3))
	require.Contains(t, items[0].Type, "game")

	sawCheckin := false
	for _, item := range items {
		if item.Type == "checkin_reward" {
			sawCheckin = true
			break
		}
	}
	require.True(t, sawCheckin, "expected merged check-in activity in balance history")
}
