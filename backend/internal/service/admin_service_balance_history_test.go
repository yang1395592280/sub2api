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
	listCalls        int
	lastCodeTypes    []string
	lastParams       []pagination.PaginationParams
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

func (s *adminBalanceHistoryRedeemRepoStub) ListByUserPaginated(_ context.Context, _ int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	if s.listByUserErr != nil {
		return nil, nil, s.listByUserErr
	}

	s.listCalls++
	s.lastCodeTypes = append(s.lastCodeTypes, codeType)
	s.lastParams = append(s.lastParams, params)

	filtered := make([]RedeemCode, 0, len(s.items))
	for _, item := range s.items {
		if codeType == "" || item.Type == codeType {
			filtered = append(filtered, item)
		}
	}

	start := params.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + params.Limit()
	if end > len(filtered) {
		end = len(filtered)
	}

	return append([]RedeemCode(nil), filtered[start:end]...), &pagination.PaginationResult{
		Total:    int64(len(filtered)),
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
	pointsAfter := int64(60)

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
				PointsAfter:     &pointsAfter,
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

func TestNewAdminServiceGetUserBalanceHistoryUsesInjectedRepos(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	usedAt := now.Add(-2 * time.Hour)
	redeemRepo := &adminBalanceHistoryRedeemRepoStub{
		items: []RedeemCode{
			{
				ID:        1,
				Type:      RedeemTypeBalance,
				Value:     12,
				Status:    StatusUsed,
				CreatedAt: usedAt,
				UsedAt:    &usedAt,
			},
		},
		totalRecharged: 12,
	}
	checkinRepo := &checkinRepoStub{
		timelineItems: []UserActivityTimelineItem{
			{
				ID:        "checkin-2",
				Type:      "checkin_reward",
				Value:     0.02,
				CreatedAt: now.Add(-time.Hour),
			},
		},
	}
	sizeBetRepo := &sizeBetQueryRepoStub{
		historyItems: []SizeBetUserHistoryItem{
			{
				BetID:           3,
				RoundID:         9,
				RoundNo:         1001,
				NetResultAmount: 5,
				Status:          SizeBetStatusWon,
				PlacedAt:        now.Add(-30 * time.Minute),
			},
		},
	}

	svc := NewAdminService(
		nil,
		nil,
		nil,
		nil,
		nil,
		redeemRepo,
		checkinRepo,
		sizeBetRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	items, total, totalRecharged, err := svc.GetUserBalanceHistory(context.Background(), 7, 1, 10, "")

	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, int64(3), total)
	require.Equal(t, 12.0, totalRecharged)
	require.Equal(t, 1, redeemRepo.listCalls)
	require.Len(t, checkinRepo.timelineCallParams, 1)
	require.Equal(t, 1, sizeBetRepo.historyCalls)
}

func TestGetUserBalanceHistoryFiltersBalanceTypesOnly(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	redeemRepo := &adminBalanceHistoryRedeemRepoStub{
		items: []RedeemCode{
			{ID: 1, Type: RedeemTypeBalance, Value: 10, Status: StatusUsed, CreatedAt: now.Add(-time.Minute)},
			{ID: 2, Type: AdjustmentTypeAdminBalance, Value: -4, Status: StatusUsed, CreatedAt: now.Add(-2 * time.Minute)},
			{ID: 3, Type: RedeemTypeConcurrency, Value: 1, Status: StatusUsed, CreatedAt: now.Add(-3 * time.Minute)},
		},
		totalRecharged: 10,
	}
	checkinRepo := &checkinRepoStub{
		timelineItems: []UserActivityTimelineItem{
			{ID: "checkin-9", Type: "checkin_reward", Value: 0.02, CreatedAt: now},
		},
	}
	sizeBetRepo := &sizeBetQueryRepoStub{
		historyItems: []SizeBetUserHistoryItem{
			{BetID: 8, RoundID: 11, RoundNo: 1002, NetResultAmount: 5, Status: SizeBetStatusWon, PlacedAt: now},
		},
	}

	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		checkinRepo:    checkinRepo,
		sizeBetRepo:    sizeBetRepo,
	}

	items, total, _, err := svc.GetUserBalanceHistory(context.Background(), 7, 1, 10, "balance")

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	require.Equal(t, []string{RedeemTypeBalance, AdjustmentTypeAdminBalance}, []string{items[0].Type, items[1].Type})
	require.Equal(t, []string{RedeemTypeBalance, AdjustmentTypeAdminBalance}, redeemRepo.lastCodeTypes)
	require.Empty(t, checkinRepo.timelineCallParams)
	require.Zero(t, sizeBetRepo.historyCalls)
}

func TestGetUserBalanceHistoryPaginationBoundaryReturnsLastItem(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	redeemRepo := &adminBalanceHistoryRedeemRepoStub{
		items: []RedeemCode{
			{ID: 1, Type: RedeemTypeBalance, Value: 10, Status: StatusUsed, CreatedAt: now.Add(-time.Minute)},
			{ID: 2, Type: RedeemTypeBalance, Value: 9, Status: StatusUsed, CreatedAt: now.Add(-2 * time.Minute)},
		},
		totalRecharged: 19,
	}
	checkinRepo := &checkinRepoStub{
		timelineItems: []UserActivityTimelineItem{
			{ID: "checkin-3", Type: "checkin_reward", Value: 0.02, CreatedAt: now.Add(-3 * time.Minute)},
			{ID: "checkin-4", Type: "checkin_bonus", Value: 0.02, CreatedAt: now.Add(-4 * time.Minute)},
			{ID: "checkin-5", Type: "checkin_reward", Value: 0.02, CreatedAt: now.Add(-5 * time.Minute)},
		},
	}

	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		checkinRepo:    checkinRepo,
		sizeBetRepo:    &sizeBetQueryRepoStub{},
	}

	items, total, _, err := svc.GetUserBalanceHistory(context.Background(), 7, 3, 2, "")

	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, items, 1)
	require.Equal(t, "checkin-5", items[0].ID)
	require.Len(t, checkinRepo.timelineCallParams, 1)
	require.Equal(t, 6, checkinRepo.timelineCallParams[0].PageSize)
}

func TestMergeUserActivityTimelineItemsOrdersSameTimestampByNumericID(t *testing.T) {
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	items := mergeUserActivityTimelineItems(
		[]UserActivityTimelineItem{
			{ID: "game-9", Type: "game_net", CreatedAt: now},
			{ID: "game-10", Type: "game_net", CreatedAt: now},
		},
	)

	require.Equal(t, []string{"game-10", "game-9"}, []string{items[0].ID, items[1].ID})
}
