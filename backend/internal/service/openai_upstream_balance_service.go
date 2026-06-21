package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UpstreamBalanceProviderSub2API    = "sub2api"
	UpstreamBalanceProviderNewAPI     = "new-api"
	UpstreamBalanceStatusOK           = "ok"
	UpstreamBalanceStatusError        = "error"
	UpstreamBalanceStatusUnsupported  = "unsupported"
	openAIUpstreamBalanceHTTPTimeout  = 15 * time.Second
	openAIUpstreamBalanceErrorMaxText = 180
	newAPIQuotaPerUSD                 = 500000.0
)

type OpenAIUpstreamBalanceSnapshot struct {
	Provider  string
	Remaining float64
	Unit      string
	Status    string
	Error     string
	UpdatedAt time.Time
	Group     string
	GroupID   *int64
}

type OpenAIUpstreamBalanceService struct {
	accountRepo AccountRepository
	client      *http.Client
}

func NewOpenAIUpstreamBalanceService(accountRepo AccountRepository, client *http.Client) *OpenAIUpstreamBalanceService {
	if client == nil {
		client = &http.Client{Timeout: openAIUpstreamBalanceHTTPTimeout}
	}
	return &OpenAIUpstreamBalanceService{accountRepo: accountRepo, client: client}
}

func (s *OpenAIUpstreamBalanceService) Refresh(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "UPSTREAM_BALANCE_NOT_CONFIGURED", "upstream balance service is not configured")
	}

	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil || !account.IsOpenAIApiKey() {
		return nil, infraerrors.New(http.StatusBadRequest, "UPSTREAM_BALANCE_INVALID_ACCOUNT", "only OpenAI API Key accounts support upstream balance")
	}

	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if baseURL == "" || apiKey == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "UPSTREAM_BALANCE_MISSING_CREDENTIALS", "base_url and api_key are required")
	}

	snapshot, probeErr := s.probe(ctx, account, baseURL, apiKey)
	var updates map[string]any
	if probeErr != nil {
		snapshot = OpenAIUpstreamBalanceSnapshot{
			Status:    UpstreamBalanceStatusError,
			Error:     truncate(probeErr.Error(), openAIUpstreamBalanceErrorMaxText),
			UpdatedAt: time.Now().UTC(),
		}
		updates = buildOpenAIUpstreamBalanceErrorUpdates(snapshot)
	} else {
		updates = buildOpenAIUpstreamBalanceUpdates(snapshot)
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		return nil, err
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	for k, v := range updates {
		account.Extra[k] = v
	}
	return account, nil
}

func buildOpenAIUpstreamBalanceUpdates(snapshot OpenAIUpstreamBalanceSnapshot) map[string]any {
	group := strings.TrimSpace(snapshot.Group)
	updates := map[string]any{
		"upstream_balance_provider":   snapshot.Provider,
		"upstream_balance_remaining":  snapshot.Remaining,
		"upstream_balance_unit":       snapshot.Unit,
		"upstream_balance_status":     snapshot.Status,
		"upstream_balance_error":      snapshot.Error,
		"upstream_balance_updated_at": "",
		"upstream_group":              group,
	}
	if !snapshot.UpdatedAt.IsZero() {
		updates["upstream_balance_updated_at"] = snapshot.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if snapshot.GroupID != nil {
		updates["upstream_group_id"] = *snapshot.GroupID
	}
	return updates
}

func buildOpenAIUpstreamBalanceErrorUpdates(snapshot OpenAIUpstreamBalanceSnapshot) map[string]any {
	updates := map[string]any{
		"upstream_balance_status":     snapshot.Status,
		"upstream_balance_error":      snapshot.Error,
		"upstream_balance_updated_at": "",
	}
	if !snapshot.UpdatedAt.IsZero() {
		updates["upstream_balance_updated_at"] = snapshot.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return updates
}

func (s *OpenAIUpstreamBalanceService) probe(ctx context.Context, account *Account, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	providers := []string{UpstreamBalanceProviderSub2API, UpstreamBalanceProviderNewAPI}
	if account.GetExtraString("upstream_balance_provider") == UpstreamBalanceProviderNewAPI || hasNewAPIUserBalanceCredentials(account) {
		providers = []string{UpstreamBalanceProviderNewAPI, UpstreamBalanceProviderSub2API}
	}

	var lastErr error
	for _, provider := range providers {
		var (
			snapshot OpenAIUpstreamBalanceSnapshot
			err      error
		)
		switch provider {
		case UpstreamBalanceProviderSub2API:
			snapshot, err = s.probeSub2API(ctx, baseURL, apiKey)
		case UpstreamBalanceProviderNewAPI:
			snapshot, err = s.probeNewAPI(ctx, account, baseURL, apiKey)
		default:
			err = fmt.Errorf("unsupported provider %q", provider)
		}
		if err == nil {
			snapshot.Provider = provider
			snapshot.Status = UpstreamBalanceStatusOK
			snapshot.UpdatedAt = time.Now().UTC()
			return snapshot, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no upstream balance provider available")
	}
	return OpenAIUpstreamBalanceSnapshot{}, lastErr
}

func (s *OpenAIUpstreamBalanceService) probeSub2API(ctx context.Context, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	var payload map[string]any
	if err := s.doJSONGET(ctx, buildOpenAIEndpointURL(baseURL, "/v1/usage"), apiKey, &payload); err != nil {
		return OpenAIUpstreamBalanceSnapshot{}, err
	}

	remaining, ok := getFloat64(payload, "remaining")
	if !ok {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("sub2api response missing remaining")
	}
	return OpenAIUpstreamBalanceSnapshot{
		Remaining: remaining,
		Unit:      strings.TrimSpace(getString(payload, "unit")),
		Group:     getOpenAIUpstreamGroupName(payload),
		GroupID:   getOpenAIUpstreamGroupID(payload),
	}, nil
}

func (s *OpenAIUpstreamBalanceService) probeNewAPI(ctx context.Context, account *Account, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	if auth, ok := getNewAPIUserBalanceAuth(account); ok {
		return s.probeNewAPIUserSelf(ctx, baseURL, auth)
	}
	var payload map[string]any
	if err := s.doJSONGET(ctx, buildNewAPIUsageURL(baseURL), apiKey, &payload); err != nil {
		return OpenAIUpstreamBalanceSnapshot{}, err
	}

	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("new-api response missing data")
	}

	unit := strings.TrimSpace(getString(data, "unit"))
	if unit == "" {
		unit = "quota"
	}
	if remaining, ok := getFirstFloat64(data, "total_available", "available_quota", "remaining_quota", "remain_quota", "quota_remaining"); ok {
		return OpenAIUpstreamBalanceSnapshot{
			Remaining: nonNegativeBalance(remaining),
			Unit:      unit,
		}, nil
	}

	quota, ok := getFloat64(data, "quota")
	if !ok {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("new-api response missing quota")
	}
	used, ok := getFloat64(data, "used_quota")
	if !ok {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("new-api response missing or invalid used_quota")
	}

	return OpenAIUpstreamBalanceSnapshot{
		Remaining: nonNegativeBalance(quota - used),
		Unit:      unit,
	}, nil
}

func (s *OpenAIUpstreamBalanceService) probeNewAPIUserSelf(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth) (OpenAIUpstreamBalanceSnapshot, error) {
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildNewAPIUserSelfURL(baseURL), map[string]string{
		"Authorization": auth.AccessToken,
		"New-Api-User":  auth.UserID,
	}, &payload); err != nil {
		return OpenAIUpstreamBalanceSnapshot{}, err
	}

	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("new-api user self response missing data")
	}
	quota, ok := getFloat64(data, "quota")
	if !ok {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("new-api user self response missing quota")
	}
	return OpenAIUpstreamBalanceSnapshot{
		Remaining: nonNegativeBalance(quota / newAPIQuotaPerUSD),
		Unit:      "USD",
		Group:     strings.TrimSpace(getString(data, "group")),
	}, nil
}

func (s *OpenAIUpstreamBalanceService) doJSONGET(ctx context.Context, targetURL, apiKey string, dest any) error {
	return s.doJSONGETWithHeaders(ctx, targetURL, map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, dest)
}

func (s *OpenAIUpstreamBalanceService) doJSONGETWithHeaders(ctx context.Context, targetURL string, headers map[string]string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

type newAPIUserBalanceAuth struct {
	UserID      string
	AccessToken string
}

func getNewAPIUserBalanceAuth(account *Account) (newAPIUserBalanceAuth, bool) {
	if account == nil {
		return newAPIUserBalanceAuth{}, false
	}
	auth := newAPIUserBalanceAuth{
		UserID:      strings.TrimSpace(account.GetCredential("new_api_user_id")),
		AccessToken: strings.TrimSpace(account.GetCredential("new_api_user_access_token")),
	}
	return auth, auth.UserID != "" && auth.AccessToken != ""
}

func hasNewAPIUserBalanceCredentials(account *Account) bool {
	_, ok := getNewAPIUserBalanceAuth(account)
	return ok
}

func buildNewAPIUsageURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimRight(strings.TrimSuffix(trimmed, "/v1"), "/") + "/api/usage/token/"
	}

	path := strings.TrimRight(parsed.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	if path == "" {
		path = "/api/usage/token/"
	} else {
		path = path + "/api/usage/token/"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String()
}

func buildNewAPIUserSelfURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimRight(strings.TrimSuffix(trimmed, "/v1"), "/") + "/api/user/self"
	}

	path := strings.TrimRight(parsed.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	if path == "" {
		path = "/api/user/self"
	} else {
		path = path + "/api/user/self"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String()
}

func getFloat64(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

func getFirstFloat64(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if v, ok := getFloat64(m, key); ok {
			return v, true
		}
	}
	return 0, false
}

func nonNegativeBalance(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func getOpenAIUpstreamGroupName(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"upstream_group", "group_name"} {
		if value := strings.TrimSpace(getString(payload, key)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(getString(payload, "group")); value != "" {
		return value
	}
	group, _ := payload["group"].(map[string]any)
	if value := strings.TrimSpace(getString(group, "name")); value != "" {
		return value
	}
	return ""
}

func getOpenAIUpstreamGroupID(payload map[string]any) *int64 {
	if payload == nil {
		return nil
	}
	if id, ok := getInt64(payload, "upstream_group_id"); ok {
		return &id
	}
	if id, ok := getInt64(payload, "group_id"); ok {
		return &id
	}
	group, _ := payload["group"].(map[string]any)
	if id, ok := getInt64(group, "id"); ok {
		return &id
	}
	return nil
}

func getInt64(m map[string]any, key string) (int64, bool) {
	value, ok := getFloat64(m, key)
	if !ok {
		return 0, false
	}
	return int64(value), true
}
