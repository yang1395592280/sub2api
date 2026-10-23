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
	campaignID int64
}

func (r *campaignValidationRepo) GetCampaign(_ context.Context, id int64) (*RedeemCampaign, error) {
	r.campaignID = id
	return &RedeemCampaign{ID: id}, nil
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
