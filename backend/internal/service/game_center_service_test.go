package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type gameCenterSettingRepoStub struct {
	values map[string]string
}

func (s *gameCenterSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *gameCenterSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *gameCenterSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *gameCenterSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *gameCenterSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *gameCenterSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *gameCenterSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type gameCenterRepoStub struct {
	assets                    *GameCenterAssets
	assetsErr                 error
	claimErr                  error
	lastClaimInput            ClaimPointsInput
	exchangeBalanceResult     *GameCenterExchangeResult
	exchangeBalanceErr        error
	lastExchangeBalanceInput  ExchangeBalanceToPointsInput
	exchangePointsResult      *GameCenterExchangeResult
	exchangePointsErr         error
	lastExchangePointsInput   ExchangePointsToBalanceInput
	catalogs                  []GameCatalog
	catalogsErr               error
	claimedBatchKeys          map[string]struct{}
	claimedBatchKeysErr       error
	ledgerItems               []GamePointsLedgerItem
	ledgerPagination          *pagination.PaginationResult
	ledgerErr                 error
	lastLedgerUserID          int64
	lastLedgerParams          pagination.PaginationParams
	adminLedgerItems          []GameCenterAdminLedgerItem
	adminLedgerPagination     *pagination.PaginationResult
	adminLedgerErr            error
	claimRecords              []GameCenterClaimRecord
	claimRecordsPagination    *pagination.PaginationResult
	claimRecordsErr           error
	exchangeRecords           []GameCenterExchangeRecord
	exchangeRecordsPagination *pagination.PaginationResult
	exchangeRecordsErr        error
	lastAdjustInput           AdminAdjustPointsInput
	adjustErr                 error
}

func (s *gameCenterRepoStub) GetUserAssets(_ context.Context, _ int64) (*GameCenterAssets, error) {
	if s.assetsErr != nil {
		return nil, s.assetsErr
	}
	if s.assets != nil {
		return s.assets, nil
	}
	return &GameCenterAssets{Balance: 12.5, Points: 50}, nil
}

func (s *gameCenterRepoStub) ClaimPoints(_ context.Context, input ClaimPointsInput) error {
	s.lastClaimInput = input
	return s.claimErr
}

func (s *gameCenterRepoStub) ExchangeBalanceToPoints(_ context.Context, input ExchangeBalanceToPointsInput) (*GameCenterExchangeResult, error) {
	s.lastExchangeBalanceInput = input
	if s.exchangeBalanceErr != nil {
		return nil, s.exchangeBalanceErr
	}
	if s.exchangeBalanceResult != nil {
		return s.exchangeBalanceResult, nil
	}
	return &GameCenterExchangeResult{
		Direction:    GameCenterExchangeDirectionBalanceToPoints,
		SourceAmount: input.Amount,
		TargetPoints: 200,
		Rate:         input.Rate,
	}, nil
}

func (s *gameCenterRepoStub) ExchangePointsToBalance(_ context.Context, input ExchangePointsToBalanceInput) (*GameCenterExchangeResult, error) {
	s.lastExchangePointsInput = input
	if s.exchangePointsErr != nil {
		return nil, s.exchangePointsErr
	}
	if s.exchangePointsResult != nil {
		return s.exchangePointsResult, nil
	}
	return &GameCenterExchangeResult{
		Direction:    GameCenterExchangeDirectionPointsToBalance,
		SourcePoints: input.Points,
		TargetAmount: 1.5,
		Rate:         input.Rate,
	}, nil
}

func (s *gameCenterRepoStub) ListCatalogs(_ context.Context) ([]GameCatalog, error) {
	if s.catalogsErr != nil {
		return nil, s.catalogsErr
	}
	return s.catalogs, nil
}

func (s *gameCenterRepoStub) UpdateCatalog(_ context.Context, _ string, _ UpdateGameCatalogRequest) error {
	return nil
}

func (s *gameCenterRepoStub) ListClaimedBatchKeys(_ context.Context, _ int64, _ string) (map[string]struct{}, error) {
	if s.claimedBatchKeysErr != nil {
		return nil, s.claimedBatchKeysErr
	}
	if s.claimedBatchKeys != nil {
		return s.claimedBatchKeys, nil
	}
	return map[string]struct{}{}, nil
}

func (s *gameCenterRepoStub) ListLedger(_ context.Context, userID int64, params pagination.PaginationParams) ([]GamePointsLedgerItem, *pagination.PaginationResult, error) {
	s.lastLedgerUserID = userID
	s.lastLedgerParams = params
	if s.ledgerErr != nil {
		return nil, nil, s.ledgerErr
	}
	if s.ledgerPagination != nil {
		return s.ledgerItems, s.ledgerPagination, nil
	}
	return s.ledgerItems, &pagination.PaginationResult{Total: int64(len(s.ledgerItems)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (s *gameCenterRepoStub) ListAdminLedger(_ context.Context, _ pagination.PaginationParams, _ *int64) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	if s.adminLedgerErr != nil {
		return nil, nil, s.adminLedgerErr
	}
	return s.adminLedgerItems, s.adminLedgerPagination, nil
}

func (s *gameCenterRepoStub) ListClaimRecords(_ context.Context, _ pagination.PaginationParams, _ *int64) ([]GameCenterClaimRecord, *pagination.PaginationResult, error) {
	if s.claimRecordsErr != nil {
		return nil, nil, s.claimRecordsErr
	}
	return s.claimRecords, s.claimRecordsPagination, nil
}

func (s *gameCenterRepoStub) ListExchangeRecords(_ context.Context, _ pagination.PaginationParams, _ *int64) ([]GameCenterExchangeRecord, *pagination.PaginationResult, error) {
	if s.exchangeRecordsErr != nil {
		return nil, nil, s.exchangeRecordsErr
	}
	return s.exchangeRecords, s.exchangeRecordsPagination, nil
}

func (s *gameCenterRepoStub) AdjustPoints(_ context.Context, input AdminAdjustPointsInput) error {
	s.lastAdjustInput = input
	return s.adjustErr
}

func TestGameCenterServiceClaimPoints(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{}
	settings := &gameCenterSettingRepoStub{
		values: map[string]string{
			SettingKeyGameCenterClaimEnabled:  "true",
			SettingKeyGameCenterClaimSchedule: `[{"batch_key":"night","name":"晚间积分","claim_time":"20:00","points_amount":100,"enabled":true}]`,
		},
	}
	svc := NewGameCenterService(repo, settings)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 25, 20, 1, 0, 0, time.FixedZone("CST", 8*3600))
	}

	err := svc.ClaimPoints(context.Background(), 7, "night")
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.lastClaimInput.UserID)
	require.Equal(t, "night", repo.lastClaimInput.BatchKey)
	require.Equal(t, int64(100), repo.lastClaimInput.PointsAmount)
}

func TestGameCenterServiceExchangeBalanceToPoints(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{assets: &GameCenterAssets{Balance: 12.5, Points: 50}}
	settings := &gameCenterSettingRepoStub{
		values: map[string]string{
			SettingKeyGameCenterExchangeBalanceToPointsEnabled: "true",
			SettingKeyGameCenterExchangeBalanceToPointsRate:    "100",
			SettingKeyGameCenterExchangeMinBalanceAmount:       "1",
		},
	}
	svc := NewGameCenterService(repo, settings)

	result, err := svc.ExchangeBalanceToPoints(context.Background(), 7, 2)
	require.NoError(t, err)
	require.Equal(t, GameCenterExchangeDirectionBalanceToPoints, result.Direction)
	require.Equal(t, int64(200), result.TargetPoints)
	require.Equal(t, 2.0, repo.lastExchangeBalanceInput.Amount)
	require.Equal(t, 100.0, repo.lastExchangeBalanceInput.Rate)
}

func TestGameCenterServiceGetOverviewMarksClaimedBatch(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{
		assets: &GameCenterAssets{Balance: 12.5, Points: 150},
		claimedBatchKeys: map[string]struct{}{
			"night": {},
		},
	}
	settings := &gameCenterSettingRepoStub{
		values: map[string]string{
			SettingKeyGameCenterClaimSchedule: `[{"batch_key":"night","name":"晚间积分","claim_time":"20:00","points_amount":100,"enabled":true}]`,
		},
	}
	svc := NewGameCenterService(repo, settings)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 25, 20, 30, 0, 0, time.FixedZone("CST", 8*3600))
	}

	result, err := svc.GetOverview(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 5})
	require.NoError(t, err)
	require.Len(t, result.ClaimBatches, 1)
	require.Equal(t, "claimed", result.ClaimBatches[0].Status)
}

func TestGameCenterServiceAdjustPointsRequiresReason(t *testing.T) {
	t.Parallel()

	svc := NewGameCenterService(&gameCenterRepoStub{}, &gameCenterSettingRepoStub{})
	err := svc.AdjustPoints(context.Background(), AdminAdjustPointsInput{UserID: 7, DeltaPoints: 10})
	require.Error(t, err)
}
