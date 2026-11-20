package service

import (
	"context"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type campaignValidationRepo struct {
	RedeemCodeRepository
	RedeemCampaignRepository
	campaignID       int64
	createdCampaigns []RedeemCampaign
	createdCodes     []RedeemCode
}

func (r *campaignValidationRepo) GetCampaign(_ context.Context, id int64) (*RedeemCampaign, error) {
	r.campaignID = id
	return &RedeemCampaign{ID: id}, nil
}

func (r *campaignValidationRepo) CreateCampaign(_ context.Context, campaign *RedeemCampaign) error {
	campaign.ID = int64(len(r.createdCampaigns) + 100)
	r.createdCampaigns = append(r.createdCampaigns, *campaign)
	return nil
}

func (r *campaignValidationRepo) Create(_ context.Context, code *RedeemCode) error {
	code.ID = int64(len(r.createdCodes) + 1)
	r.createdCodes = append(r.createdCodes, *code)
	return nil
}

func TestRedeemCampaignInput(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
		valid bool
	}{
		{"  群福利  ", 1, true}, {strings.Repeat("福", 100), 1000, true},
		{" ", 1, false}, {strings.Repeat("福", 101), 1, false},
		{"福利", 0, false}, {"福利", -1, false}, {"福利", 1001, false},
	} {
		input := RedeemCampaignInput{Name: tt.name, PerUserLimit: tt.limit}
		err := validateRedeemCampaignInput(&input)
		if tt.valid {
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tt.name), input.Name)
		} else {
			require.Equal(t, "REDEEM_CAMPAIGN_INVALID", infraerrors.Reason(err))
		}
	}
}

func TestRedeemCampaignCodeValidation(t *testing.T) {
	id := int64(42)
	for _, tt := range []struct {
		codeType string
		value    float64
		campaign *int64
		valid    bool
	}{
		{RedeemTypeBalance, 10, &id, true},
		{RedeemTypeLotteryTicket, 2, &id, true},
		{RedeemTypeGamePoints, 100, &id, true},
		{RedeemTypeGamePoints, 0, &id, false},
		{RedeemTypeGamePoints, -100, &id, false},
		{RedeemTypeBalance, -10, &id, false},
		{RedeemTypeBalance, 0, &id, false},
		{RedeemTypeSubscription, 10, &id, false},
		{RedeemTypeConcurrency, 1, &id, false},
		{RedeemTypeInvitation, 0, &id, false},
		{RedeemTypeBalance, -10, nil, true},
	} {
		repo := &campaignValidationRepo{}
		err := validateRedeemCampaignCode(context.Background(), repo, tt.campaign, tt.codeType, tt.value)
		if tt.valid {
			require.NoError(t, err)
			if tt.campaign != nil {
				require.Equal(t, id, repo.campaignID)
			}
		} else {
			require.Equal(t, "REDEEM_CAMPAIGN_CODE_INVALID", infraerrors.Reason(err))
			require.Zero(t, repo.campaignID)
		}
	}
}

func TestRedeemPerUserLimitValidation(t *testing.T) {
	limit := func(v int) *int { return &v }
	for _, tt := range []struct {
		codeType string
		value    float64
		limit    *int
		reason   string
	}{
		{RedeemTypeLotteryTicket, 1, limit(1), ""},
		{RedeemTypeLotteryTicket, 5, limit(1000), ""},
		{RedeemTypeGamePoints, 100, limit(1), ""},
		{RedeemTypeGamePoints, 500, limit(3), ""},
		{RedeemTypeLotteryTicket, 1, nil, ""},
		{RedeemTypeGamePoints, 100, nil, ""},
		{RedeemTypeBalance, 10, nil, ""},
		{RedeemTypeBalance, 10, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeSubscription, 1, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeConcurrency, 1, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeInvitation, 0, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeGamePoints, 0, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeGamePoints, -10, limit(1), "REDEEM_PER_USER_LIMIT_TYPE_INVALID"},
		{RedeemTypeGamePoints, 100, limit(0), "REDEEM_PER_USER_LIMIT_INVALID"},
		{RedeemTypeGamePoints, 100, limit(-2), "REDEEM_PER_USER_LIMIT_INVALID"},
		{RedeemTypeLotteryTicket, 1, limit(1001), "REDEEM_PER_USER_LIMIT_INVALID"},
	} {
		err := validateRedeemPerUserLimit(tt.codeType, tt.value, tt.limit)
		if tt.reason == "" {
			require.NoError(t, err)
		} else {
			require.Equal(t, tt.reason, infraerrors.Reason(err))
		}
	}
}

func TestAdminGenerateRedeemCodes_PerUserLimit(t *testing.T) {
	limit := func(v int) *int { return &v }
	cid := int64(42)

	t.Run("creates one internal batch campaign for lottery_ticket and game_points", func(t *testing.T) {
		for _, tc := range []struct {
			codeType string
			value    float64
			limit    int
		}{
			{RedeemTypeLotteryTicket, 1, 1},
			{RedeemTypeGamePoints, 200, 3},
		} {
			repo := &campaignValidationRepo{}
			svc := &adminServiceImpl{redeemCodeRepo: repo}
			codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
				Count:        3,
				Type:         tc.codeType,
				Value:        tc.value,
				PerUserLimit: limit(tc.limit),
			})
			require.NoError(t, err)
			require.Len(t, codes, 3)
			require.Len(t, repo.createdCampaigns, 1)
			require.Equal(t, tc.limit, repo.createdCampaigns[0].PerUserLimit)
			for _, code := range codes {
				require.NotNil(t, code.CampaignID)
				require.Equal(t, repo.createdCampaigns[0].ID, *code.CampaignID)
			}
		}
	})

	t.Run("does not create campaign when per_user_limit is nil", func(t *testing.T) {
		repo := &campaignValidationRepo{}
		svc := &adminServiceImpl{redeemCodeRepo: repo}
		codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
			Count: 2,
			Type:  RedeemTypeGamePoints,
			Value: 100,
		})
		require.NoError(t, err)
		require.Len(t, codes, 2)
		require.Empty(t, repo.createdCampaigns)
		for _, code := range codes {
			require.Nil(t, code.CampaignID)
		}
	})

	t.Run("rejects conflict and invalid values before creating campaign", func(t *testing.T) {
		repo := &campaignValidationRepo{}
		svc := &adminServiceImpl{redeemCodeRepo: repo}

		_, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
			Count:        1,
			Type:         RedeemTypeGamePoints,
			Value:        100,
			CampaignID:   &cid,
			PerUserLimit: limit(1),
		})
		require.Equal(t, "REDEEM_PER_USER_LIMIT_CONFLICT", infraerrors.Reason(err))
		require.Empty(t, repo.createdCampaigns)

		_, err = svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
			Count:        1,
			Type:         RedeemTypeLotteryTicket,
			Value:        99,
			PerUserLimit: limit(1),
		})
		require.Error(t, err)
		require.Empty(t, repo.createdCampaigns)

		_, err = svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
			Count:        1,
			Type:         RedeemTypeGamePoints,
			Value:        1.5,
			PerUserLimit: limit(1),
		})
		require.ErrorIs(t, err, ErrRedeemGamePointsInvalid)
		require.Empty(t, repo.createdCampaigns)
	})
}
