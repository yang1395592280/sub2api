package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type cnBalanceResponseUpstream struct {
	statusCode int
	body       string
}

func (u *cnBalanceResponseUpstream) Do(
	_ *http.Request,
	_ string,
	_ int64,
	_ int,
) (*http.Response, error) {
	return &http.Response{
		StatusCode: u.statusCode,
		Body:       io.NopCloser(strings.NewReader(u.body)),
		Header:     make(http.Header),
	}, nil
}

func (u *cnBalanceResponseUpstream) DoWithTLS(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
	_ *tlsfingerprint.Profile,
) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

type cnBalanceProbeRepo struct {
	AccountRepository
	account     *Account
	extraWrites []map[string]any
}

func (r *cnBalanceProbeRepo) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

func (r *cnBalanceProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.extraWrites = append(r.extraWrites, updates)
	return nil
}

func (r *cnBalanceProbeRepo) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	if r.account == nil || len(ids) != 1 || ids[0] != r.account.ID {
		return 0, nil
	}
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for key, value := range updates.Extra {
		r.account.Extra[key] = value
	}
	if updates.ChannelPrice != nil {
		value := *updates.ChannelPrice
		r.account.ChannelPrice = &value
	}
	return 1, nil
}

type cnMetadataUpstream struct{}

func (u *cnMetadataUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	statusCode := http.StatusOK
	body := `{"remaining":12.5,"unit":"USD","group_name":"DeepSeek Pro","group_id":3,"group_rate_multiplier":0.2,"effective_rate_multiplier":0.16}`
	if req.URL.Path == "/user/balance" {
		statusCode = http.StatusNotFound
		body = `{}`
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func (u *cnMetadataUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestCNProviderBalanceService_RefreshAccountPersistsUpstreamMetadata(t *testing.T) {
	account := newDeepSeekBalanceProbeAccount()
	account.UpstreamRechargeRatio = 2
	repo := &cnBalanceProbeRepo{account: account}
	svc := NewCNProviderBalanceService(repo, nil, &cnMetadataUpstream{}, nil)

	refreshed, err := svc.RefreshAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.Same(t, account, refreshed)
	require.Equal(t, "DeepSeek Pro", account.Extra["upstream_group"])
	require.Equal(t, int64(3), account.Extra["upstream_group_id"])
	require.Equal(t, 0.1, account.Extra["upstream_group_rate_multiplier"])
	require.Equal(t, 0.08, account.Extra["upstream_effective_rate_multiplier"])
	require.Equal(t, 6.25, account.Extra["upstream_balance_remaining"])
	require.Equal(t, "USD", account.Extra["upstream_balance_unit"])
	require.Equal(t, UpstreamBalanceStatusOK, account.Extra["upstream_balance_status"])
	require.NotNil(t, account.ChannelPrice)
	require.Equal(t, 0.08, *account.ChannelPrice)
}

func newDeepSeekBalanceProbeAccount() *Account {
	return &Account{
		ID:       42,
		Platform: PlatformDeepseek,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"account_mode": AccountModePayG,
			"api_key":      "sk-test",
			"base_url":     "https://relay.example.com",
		},
	}
}

func TestCNProviderBalanceService_DeepSeekInvalidBalancePayloadDoesNotBecomeZero(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "missing balance infos",
			body:      `{"data":{"models":["deepseek-v4-flash"]}}`,
			wantError: "missing balance_infos",
		},
		{
			name:      "empty balance infos",
			body:      `{"is_available":true,"balance_infos":[]}`,
			wantError: "no valid balance entries",
		},
		{
			name:      "invalid balance value",
			body:      `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"not-a-number"}]}`,
			wantError: "no valid balance entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &cnBalanceProbeRepo{account: newDeepSeekBalanceProbeAccount()}
			upstream := &cnBalanceResponseUpstream{statusCode: http.StatusOK, body: tt.body}
			svc := NewCNProviderBalanceService(repo, nil, upstream, nil)

			result, err := svc.QueryBalance(context.Background(), repo.account.ID)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.Success)
			require.Contains(t, result.Error, tt.wantError)
			require.Empty(t, result.Balances)
			require.Empty(t, repo.extraWrites, "invalid relay balance payload must not persist a synthetic zero balance")
		})
	}
}

func TestCNProviderBalanceService_DeepSeekValidZeroBalanceRemainsSuccessful(t *testing.T) {
	repo := &cnBalanceProbeRepo{account: newDeepSeekBalanceProbeAccount()}
	upstream := &cnBalanceResponseUpstream{
		statusCode: http.StatusOK,
		body:       `{"is_available":false,"balance_infos":[{"currency":"CNY","total_balance":"0"}]}`,
	}
	svc := NewCNProviderBalanceService(repo, nil, upstream, nil)

	result, err := svc.QueryBalance(context.Background(), repo.account.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.False(t, result.Available)
	require.Equal(t, "CNY", result.Currency)
	require.Zero(t, result.Balance)
	require.Len(t, result.Balances, 1)
	require.Len(t, repo.extraWrites, 1, "a valid upstream zero balance must still be persisted")
}
