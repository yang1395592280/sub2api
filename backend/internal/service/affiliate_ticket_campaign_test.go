package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type affiliateTicketCampaignSettingRepoStub struct {
	SettingRepository
	value string
	err   error
}

type affiliateTicketCampaignRepoStub struct {
	AffiliateTicketCampaignRepository
	registrationCalls int
	rechargeCalls     int
}

func (r *affiliateTicketCampaignRepoStub) ProcessInviteRegistration(context.Context, int64, int64, string, time.Time) (*AffiliateTicketCampaignEvent, error) {
	r.registrationCalls++
	return nil, nil
}

func (r *affiliateTicketCampaignRepoStub) ProcessInviteRecharge(context.Context, int64, int64, float64, time.Time) (*AffiliateTicketCampaignEvent, error) {
	r.rechargeCalls++
	return nil, nil
}

func (r *affiliateTicketCampaignSettingRepoStub) GetValue(context.Context, string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.value, nil
}

func TestAffiliateTicketCampaignSettingDefaultsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		err   error
		want  bool
	}{
		{name: "missing setting", err: errors.New("not found"), want: true},
		{name: "explicitly enabled", value: "true", want: true},
		{name: "explicitly disabled", value: "false", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &SettingService{settingRepo: &affiliateTicketCampaignSettingRepoStub{value: tt.value, err: tt.err}}
			require.Equal(t, tt.want, svc.IsAffiliateTicketCampaignEnabled(context.Background()))
		})
	}
}

func TestAffiliateTicketCampaignSwitchGuardsRewardEvents(t *testing.T) {
	ctx := context.Background()
	settingRepo := &affiliateTicketCampaignSettingRepoStub{value: "false"}
	repo := &affiliateTicketCampaignRepoStub{}
	svc := NewAffiliateTicketCampaignService(repo, &SettingService{settingRepo: settingRepo})

	require.NoError(t, svc.OnInviteRegistration(ctx, 1, 2))
	require.NoError(t, svc.OnInviteRecharge(ctx, 2, 10, AffiliateTicketCampaignRechargeFloor))
	require.Zero(t, repo.registrationCalls)
	require.Zero(t, repo.rechargeCalls)

	settingRepo.value = "true"
	require.NoError(t, svc.OnInviteRegistration(ctx, 1, 2))
	require.NoError(t, svc.OnInviteRecharge(ctx, 2, 10, AffiliateTicketCampaignRechargeFloor))
	require.Equal(t, 1, repo.registrationCalls)
	require.Equal(t, 1, repo.rechargeCalls)
}
