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

func (s *gameCenterSettingRepoStub) Get(context.Context, string) (*Setting, error) { panic("unexpected Get call") }
func (s *gameCenterSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}
func (s *gameCenterSettingRepoStub) Set(context.Context, string, string) error { panic("unexpected Set call") }
func (s *gameCenterSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}
func (s *gameCenterSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}
func (s *gameCenterSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}
func (s *gameCenterSettingRepoStub) Delete(context.Context, string) error { panic("unexpected Delete call") }

type gameCenterRepoStub struct {
	assets                *GameCenterAssets
	assetsErr             error
	catalogs              []GameCatalog
	catalogsErr           error
	ledgerItems           []GamePointsLedgerItem
	ledgerPagination      *pagination.PaginationResult
	ledgerErr             error
	lastLedgerUserID      int64
	lastLedgerParams      pagination.PaginationParams
	lastLedgerFilter      GamePointsLedgerFilter
	leaderboardItems      []GamePointsLeaderboardItem
	leaderboardPagination *pagination.PaginationResult
	leaderboardErr        error
	lastAdjustInput       AdminAdjustPointsInput
	adjustErr             error
}

func (s *gameCenterRepoStub) GetUserAssets(context.Context, int64) (*GameCenterAssets, error) {
	if s.assetsErr != nil {
		return nil, s.assetsErr
	}
	if s.assets != nil {
		return s.assets, nil
	}
	return &GameCenterAssets{Points: 50}, nil
}
func (s *gameCenterRepoStub) ListCatalogs(context.Context) ([]GameCatalog, error) {
	if s.catalogsErr != nil {
		return nil, s.catalogsErr
	}
	return s.catalogs, nil
}
func (s *gameCenterRepoStub) UpdateCatalog(context.Context, string, UpdateGameCatalogRequest) error {
	return nil
}
func (s *gameCenterRepoStub) ListLedger(_ context.Context, params pagination.PaginationParams, filter GamePointsLedgerFilter) ([]GamePointsLedgerItem, *pagination.PaginationResult, error) {
	if filter.UserID != nil {
		s.lastLedgerUserID = *filter.UserID
	}
	s.lastLedgerParams = params
	s.lastLedgerFilter = filter
	if s.ledgerErr != nil {
		return nil, nil, s.ledgerErr
	}
	if s.ledgerPagination != nil {
		return s.ledgerItems, s.ledgerPagination, nil
	}
	return s.ledgerItems, &pagination.PaginationResult{Total: int64(len(s.ledgerItems)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}
func (s *gameCenterRepoStub) ListPointsLeaderboard(context.Context, pagination.PaginationParams) ([]GamePointsLeaderboardItem, *pagination.PaginationResult, error) {
	if s.leaderboardErr != nil {
		return nil, nil, s.leaderboardErr
	}
	return s.leaderboardItems, s.leaderboardPagination, nil
}
func (s *gameCenterRepoStub) ListAdminLedger(context.Context, pagination.PaginationParams, GamePointsLedgerFilter) ([]GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	panic("unexpected ListAdminLedger call")
}
func (s *gameCenterRepoStub) ListClaimRecords(context.Context, pagination.PaginationParams, GamePointsLedgerFilter) ([]GameCenterClaimRecord, *pagination.PaginationResult, error) {
	panic("unexpected ListClaimRecords call")
}
func (s *gameCenterRepoStub) AdjustPoints(_ context.Context, input AdminAdjustPointsInput) error {
	s.lastAdjustInput = input
	return s.adjustErr
}

func TestGameCenterServiceGetOverviewReturnsPointsOnlyPayload(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{
		assets:      &GameCenterAssets{Points: 150},
		catalogs:    []GameCatalog{{GameKey: "checkin", Name: "Checkin"}},
		ledgerItems: []GamePointsLedgerItem{{ID: 1, EntryType: "checkin_reward", DeltaPoints: 10, PointsAfter: 150}},
	}
	settings := &gameCenterSettingRepoStub{values: map[string]string{SettingKeyGameCenterEnabled: "true"}}
	svc := NewGameCenterService(repo, settings, nil)

	result, err := svc.GetOverview(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 5}, "Asia/Shanghai")
	require.NoError(t, err)
	require.True(t, result.Enabled)
	require.Equal(t, int64(150), result.Points)
	require.Len(t, result.Catalogs, 1)
	require.Len(t, result.RecentLedger, 1)
	require.Equal(t, int64(7), repo.lastLedgerUserID)
}

func TestGameCenterServiceGetOverviewCarriesPointsOnlyCheckinStatus(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{assets: &GameCenterAssets{Points: 150}}
	checkinSvc := NewCheckinService(
		&checkinRepoStub{
			hasChecked: true,
			records: []CheckinRecord{
				{CheckinDate: "2026-04-02", RewardPoints: 12, BaseRewardPoints: 12, BonusStatus: CheckinBonusStatusNone},
			},
			getByDateRecord:   &CheckinRecord{CheckinDate: time.Now().Format("2006-01-02"), RewardPoints: 12, BaseRewardPoints: 12, BonusStatus: CheckinBonusStatusNone},
			totalCount:        1,
			totalRewardPoints: 12,
		},
		newCheckinSettings(true, "2", "20"),
		nil,
		nil,
	)
	svc := NewGameCenterService(repo, &gameCenterSettingRepoStub{values: map[string]string{SettingKeyGameCenterEnabled: "true"}}, checkinSvc)

	result, err := svc.GetOverview(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 5}, "Asia/Shanghai")
	require.NoError(t, err)
	require.NotNil(t, result.Checkin)
	require.Equal(t, int64(2), result.Checkin.MinRewardPoints)
	require.Equal(t, int64(20), result.Checkin.MaxRewardPoints)
}

func TestGameCenterServiceGetUserLedgerScopesByUserID(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{
		ledgerItems: []GamePointsLedgerItem{{ID: 1, EntryType: "checkin_reward", DeltaPoints: 10, PointsAfter: 30}},
	}
	svc := NewGameCenterService(repo, &gameCenterSettingRepoStub{}, nil)

	items, result, err := svc.GetUserLedger(context.Background(), 9, pagination.PaginationParams{Page: 2, PageSize: 3}, GamePointsLedgerFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotNil(t, result)
	require.Equal(t, int64(9), repo.lastLedgerUserID)
	require.Equal(t, 2, repo.lastLedgerParams.Page)
	require.Equal(t, 3, repo.lastLedgerParams.PageSize)
}

func TestGameCenterServiceAdjustPointsAllowsEmptyReason(t *testing.T) {
	t.Parallel()

	repo := &gameCenterRepoStub{}
	svc := NewGameCenterService(repo, &gameCenterSettingRepoStub{}, nil)
	err := svc.AdjustPoints(context.Background(), AdminAdjustPointsInput{UserID: 7, DeltaPoints: 10})
	require.NoError(t, err)
	require.Equal(t, int64(10), repo.lastAdjustInput.DeltaPoints)
	require.Equal(t, "", repo.lastAdjustInput.Reason)
}

func TestGameCenterServiceValidateRequiresDependencies(t *testing.T) {
	t.Parallel()

	svc := &GameCenterService{now: time.Now}
	require.Error(t, svc.Validate())
}
