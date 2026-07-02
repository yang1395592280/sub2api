//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestListSub2APICheckinCandidates_Integration(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "inactive",
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusDisabled,
		Priority: 1,
		Credentials: map[string]any{
			"api_key":                  "sk-disabled",
			"upstream_admin_type":      "sub2api",
			"upstream_checkin_enabled": true,
		},
	})
	mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "wrong-provider",
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Priority: 2,
		Credentials: map[string]any{
			"api_key":                  "sk-other",
			"upstream_admin_type":      "newapi",
			"upstream_checkin_enabled": true,
		},
	})
	mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "non-apikey",
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Priority: 3,
		Credentials: map[string]any{
			"refresh_token":            "rt",
			"upstream_admin_type":      "sub2api",
			"upstream_checkin_enabled": true,
		},
	})
	mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "checkin-disabled",
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Priority: 4,
		Credentials: map[string]any{
			"api_key":                  "sk-off",
			"upstream_admin_type":      "sub2api",
			"upstream_checkin_enabled": false,
		},
	})
	first := mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "candidate-priority-5",
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Priority: 5,
		Credentials: map[string]any{
			"api_key":                  "sk-5",
			"upstream_admin_type":      "sub2api",
			"upstream_checkin_enabled": true,
		},
	})
	second := mustCreateAccount(t, tx.Client(), &service.Account{
		Name:     "candidate-priority-20",
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Priority: 20,
		Credentials: map[string]any{
			"api_key":                  "sk-20",
			"upstream_admin_type":      "sub2api",
			"upstream_checkin_enabled": true,
		},
	})

	accounts, err := repo.ListSub2APICheckinCandidates(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID, second.ID}, []int64{accounts[0].ID, accounts[1].ID})

	limited, err := repo.ListSub2APICheckinCandidates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, first.ID, limited[0].ID)
}
