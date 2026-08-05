package repository

import (
	"testing"

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
