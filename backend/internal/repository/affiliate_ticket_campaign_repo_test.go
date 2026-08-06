package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateTicketCampaignGrantDelta(t *testing.T) {
	tests := []struct {
		name           string
		registered     int
		recharge       int
		currentTickets int
		want           int
	}{
		{name: "one registration is not enough", registered: 1, want: 0},
		{name: "two registrations grant one", registered: 2, want: 1},
		{name: "each qualifying recharge grants one", recharge: 1, want: 1},
		{name: "registration and recharge rewards accumulate", registered: 4, recharge: 2, currentTickets: 1, want: 3},
		{name: "daily cap leaves only one grant", registered: 20, currentTickets: 9, want: 1},
		{name: "daily cap blocks further grants", registered: 20, recharge: 20, currentTickets: 10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, affiliateTicketCampaignGrantDelta(tt.registered, tt.recharge, tt.currentTickets))
		})
	}
}

func TestCampaignIPUsable(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "public ipv4", ip: "8.8.8.8", want: true},
		{name: "public ipv6", ip: "2001:4860:4860::8888", want: true},
		{name: "private proxy address", ip: "10.0.0.5", want: false},
		{name: "loopback", ip: "127.0.0.1", want: false},
		{name: "invalid", ip: "not-an-ip", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, campaignIPUsable(tt.ip))
		})
	}
}

func TestAffiliateTicketCampaignEligibilityRequiresStrictThresholds(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		hasUsage bool
		usage    float64
		balance  float64
		want     bool
	}{
		{name: "eligible", status: service.StatusActive, hasUsage: true, usage: 20.01, balance: 10.01, want: true},
		{name: "usage threshold is strict", status: service.StatusActive, hasUsage: true, usage: 20, balance: 11},
		{name: "balance threshold is strict", status: service.StatusActive, hasUsage: true, usage: 21, balance: 10},
		{name: "usage record required", status: service.StatusActive, usage: 21, balance: 11},
		{name: "active account required", status: service.StatusDisabled, hasUsage: true, usage: 21, balance: 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, affiliateTicketCampaignEligible(tt.status, tt.hasUsage, tt.usage, tt.balance))
		})
	}
}

func TestCampaignRegistrationRiskRequiresSameNetworkAndDevice(t *testing.T) {
	tests := []struct {
		name           string
		inviterIP      string
		inviteeIP      string
		inviterDevice  string
		inviteeDevice  string
		wantSameIP     bool
		wantSameDevice bool
	}{
		{name: "shared company IP different devices", inviterIP: "8.8.8.8", inviteeIP: "8.8.8.8", inviterDevice: "device-a", inviteeDevice: "device-b", wantSameIP: true},
		{name: "same network and same device", inviterIP: "8.8.8.8", inviteeIP: "8.8.8.8", inviterDevice: "device-a", inviteeDevice: "device-a", wantSameIP: true, wantSameDevice: true},
		{name: "same network missing device evidence", inviterIP: "8.8.8.8", inviteeIP: "8.8.8.8", inviterDevice: "", inviteeDevice: "", wantSameIP: true},
		{name: "different networks", inviterIP: "8.8.8.8", inviteeIP: "1.1.1.1", inviterDevice: "device-a", inviteeDevice: "device-a"},
		{name: "private addresses are not trusted risk evidence", inviterIP: "192.168.1.10", inviteeIP: "192.168.1.10", inviterDevice: "device-a", inviteeDevice: "device-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sameIP, sameDevice := campaignRegistrationRisk(tt.inviterIP, tt.inviteeIP, tt.inviterDevice, tt.inviteeDevice)
			require.Equal(t, tt.wantSameIP, sameIP)
			require.Equal(t, tt.wantSameDevice, sameDevice)
		})
	}
}

func TestProcessInviteRegistrationCreditsInviteeBonusAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &affiliateTicketCampaignRepository{db: db}
	playDate := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT e\.id, e\.event_type.*FROM affiliate_ticket_campaign_events e.*WHERE event_key = \$1`).
		WithArgs("invite_register:202").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT inviter\.registration_ip, invitee\.registration_ip`).
		WithArgs(int64(101), int64(202)).
		WillReturnRows(sqlmock.NewRows([]string{"inviter_ip", "invitee_ip", "inviter_device_hash", "invitee_device_hash", "inviter_status", "invitee_status"}).
			AddRow("8.8.8.8", "1.1.1.1", "", "", service.StatusActive, service.StatusActive))
	mock.ExpectQuery(`SELECT u\.status, u\.balance::double precision, EXISTS`).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "balance", "has_usage", "historical_usage"}).
			AddRow(service.StatusActive, 11.0, true, 21.0))
	mock.ExpectQuery(`INSERT INTO affiliate_ticket_campaign_events`).
		WithArgs("invite_register:202", int64(101), int64(202), playDate, 1.0, "granted", "", "8.8.8.8", "1.1.1.1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_type", "inviter_id", "invitee_id", "play_date", "amount", "ticket_count",
			"status", "risk_reason", "inviter_ip", "invitee_ip", "created_at",
		}).AddRow(301, "invite_register", 101, 202, playDate, 1.0, 0, "granted", "", "8.8.8.8", "1.1.1.1", createdAt))
	mock.ExpectExec(`UPDATE users SET balance = balance \+ \$1`).
		WithArgs(1.0, int64(202), service.StatusActive).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO affiliate_ticket_campaign_daily`).
		WithArgs(int64(101), playDate).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT registered_count, recharge_count, ticket_count`).
		WithArgs(int64(101), playDate).
		WillReturnRows(sqlmock.NewRows([]string{"registered_count", "recharge_count", "ticket_count"}).AddRow(0, 0, 0))
	mock.ExpectExec(`UPDATE affiliate_ticket_campaign_daily`).
		WithArgs(1, 0, 0, int64(101), playDate).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE affiliate_ticket_campaign_events`).
		WithArgs(0, "granted", int64(301)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	event, err := repo.ProcessInviteRegistration(context.Background(), 101, 202, "1.1.1.1", "", playDate)
	require.NoError(t, err)
	require.Equal(t, 1.0, event.Amount)
	require.Equal(t, "granted", event.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
