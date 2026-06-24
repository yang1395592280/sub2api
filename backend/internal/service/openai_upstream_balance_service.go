package service

import (
	"bytes"
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

	UpstreamKeyID           *int64
	GroupRateMultiplier     *float64
	EffectiveRateMultiplier *float64
	RateSource              string
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
	}
	if group != "" {
		updates["upstream_group"] = group
	}
	if !snapshot.UpdatedAt.IsZero() {
		updates["upstream_balance_updated_at"] = snapshot.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if snapshot.GroupID != nil {
		updates["upstream_group_id"] = *snapshot.GroupID
	}
	if snapshot.UpstreamKeyID != nil {
		updates["upstream_key_id"] = *snapshot.UpstreamKeyID
	}
	if snapshot.GroupRateMultiplier != nil {
		updates["upstream_group_rate_multiplier"] = *snapshot.GroupRateMultiplier
	}
	if snapshot.EffectiveRateMultiplier != nil {
		updates["upstream_effective_rate_multiplier"] = *snapshot.EffectiveRateMultiplier
	}
	if strings.TrimSpace(snapshot.RateSource) != "" {
		updates["upstream_rate_source"] = strings.TrimSpace(snapshot.RateSource)
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
			snapshot, err = s.probeSub2API(ctx, account, baseURL, apiKey)
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

func (s *OpenAIUpstreamBalanceService) probeSub2API(ctx context.Context, account *Account, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	var payload map[string]any
	if err := s.doJSONGET(ctx, buildOpenAIEndpointURL(baseURL, "/v1/usage"), apiKey, &payload); err != nil {
		return OpenAIUpstreamBalanceSnapshot{}, err
	}

	remaining, ok := getFloat64(payload, "remaining")
	if !ok {
		return OpenAIUpstreamBalanceSnapshot{}, fmt.Errorf("sub2api response missing remaining")
	}
	snapshot := OpenAIUpstreamBalanceSnapshot{
		Remaining: remaining,
		Unit:      strings.TrimSpace(getString(payload, "unit")),
		Group:     getOpenAIUpstreamGroupName(payload),
		GroupID:   getOpenAIUpstreamGroupID(payload),
	}
	if strings.TrimSpace(snapshot.Group) == "" {
		s.enrichSub2APIAdminMetadata(ctx, account, baseURL, apiKey, &snapshot)
	}
	return snapshot, nil
}

func (s *OpenAIUpstreamBalanceService) probeNewAPI(ctx context.Context, account *Account, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	if auth, ok := getNewAPIUserBalanceAuth(account); ok {
		return s.probeNewAPIUserSelf(ctx, baseURL, apiKey, auth)
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

func (s *OpenAIUpstreamBalanceService) probeNewAPIUserSelf(ctx context.Context, baseURL, apiKey string, auth newAPIUserBalanceAuth) (OpenAIUpstreamBalanceSnapshot, error) {
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
	group := strings.TrimSpace(getString(data, "group"))
	if tokenGroup, ok := s.resolveNewAPITokenGroup(ctx, baseURL, apiKey, auth); ok {
		group = tokenGroup
	}
	snapshot := OpenAIUpstreamBalanceSnapshot{
		Remaining: nonNegativeBalance(quota / newAPIQuotaPerUSD),
		Unit:      "USD",
		Group:     group,
	}
	if rate, ok := s.resolveNewAPIGroupRate(ctx, baseURL, group, auth); ok {
		snapshot.GroupRateMultiplier = &rate
		snapshot.EffectiveRateMultiplier = &rate
		snapshot.RateSource = "group_rate"
	}
	return snapshot, nil
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

func (s *OpenAIUpstreamBalanceService) doJSONPOSTWithHeaders(ctx context.Context, targetURL string, headers map[string]string, body any, dest any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
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
	return decoder.Decode(dest)
}

type sub2APIAdminAuth struct {
	AccessToken string
	TokenType   string
	Email       string
	Password    string
}

type sub2APIAdminKey struct {
	ID                  int64
	Key                 string
	GroupID             *int64
	GroupName           string
	GroupRateMultiplier *float64
}

func (s *OpenAIUpstreamBalanceService) enrichSub2APIAdminMetadata(ctx context.Context, account *Account, baseURL, apiKey string, snapshot *OpenAIUpstreamBalanceSnapshot) {
	if snapshot == nil {
		return
	}
	auth, ok := getSub2APIAdminAuth(account)
	if !ok {
		return
	}

	token := strings.TrimSpace(auth.AccessToken)
	tokenType := strings.TrimSpace(auth.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	if token == "" && strings.TrimSpace(auth.Email) != "" && strings.TrimSpace(auth.Password) != "" {
		loginToken, loginTokenType, err := s.loginSub2APIAdmin(ctx, baseURL, auth.Email, auth.Password)
		if err != nil {
			return
		}
		token = loginToken
		if strings.TrimSpace(loginTokenType) != "" {
			tokenType = loginTokenType
		}
	}
	if token == "" {
		return
	}

	authHeader := token
	if !strings.Contains(token, " ") {
		authHeader = strings.TrimSpace(tokenType) + " " + token
	}
	keys, err := s.fetchSub2APIAdminKeys(ctx, baseURL, authHeader)
	if err != nil {
		return
	}

	var matched *sub2APIAdminKey
	for i := range keys {
		if strings.TrimSpace(keys[i].Key) == apiKey {
			matched = &keys[i]
			break
		}
	}
	if matched == nil {
		return
	}

	snapshot.UpstreamKeyID = &matched.ID
	snapshot.GroupID = matched.GroupID
	snapshot.Group = matched.GroupName
	snapshot.GroupRateMultiplier = matched.GroupRateMultiplier

	if matched.GroupID != nil {
		if rates, err := s.fetchSub2APIUserGroupRates(ctx, baseURL, authHeader); err == nil {
			if rate, ok := rates[*matched.GroupID]; ok {
				snapshot.EffectiveRateMultiplier = &rate
				snapshot.RateSource = "user_group_rate"
				return
			}
		}
	}
	if matched.GroupRateMultiplier != nil {
		snapshot.EffectiveRateMultiplier = matched.GroupRateMultiplier
		snapshot.RateSource = "group_rate"
	}
}

func getSub2APIAdminAuth(account *Account) (sub2APIAdminAuth, bool) {
	if account == nil {
		return sub2APIAdminAuth{}, false
	}
	if provider := strings.TrimSpace(account.GetCredential("upstream_admin_type")); provider != UpstreamBalanceProviderSub2API {
		return sub2APIAdminAuth{}, false
	}
	auth := sub2APIAdminAuth{
		AccessToken: strings.TrimSpace(account.GetCredential("upstream_admin_access_token")),
		TokenType:   strings.TrimSpace(account.GetCredential("upstream_admin_token_type")),
		Email:       strings.TrimSpace(firstNonEmpty(account.GetCredential("upstream_admin_email"), account.GetCredential("upstream_admin_username"))),
		Password:    strings.TrimSpace(account.GetCredential("upstream_admin_password")),
	}
	return auth, auth.AccessToken != "" || (auth.Email != "" && auth.Password != "")
}

func (s *OpenAIUpstreamBalanceService) loginSub2APIAdmin(ctx context.Context, baseURL, email, password string) (string, string, error) {
	var payload map[string]any
	err := s.doJSONPOSTWithHeaders(ctx, buildSub2APIAuthLoginURL(baseURL), nil, map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	}, &payload)
	if err != nil {
		return "", "", err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	token := strings.TrimSpace(getString(data, "access_token"))
	if token == "" {
		return "", "", fmt.Errorf("sub2api login response missing access_token")
	}
	return token, strings.TrimSpace(getString(data, "token_type")), nil
}

func (s *OpenAIUpstreamBalanceService) fetchSub2APIAdminKeys(ctx context.Context, baseURL, authorization string) ([]sub2APIAdminKey, error) {
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildSub2APIKeysURL(baseURL), map[string]string{
		"Authorization": authorization,
	}, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	rawItems, _ := data["items"].([]any)
	keys := make([]sub2APIAdminKey, 0, len(rawItems))
	for _, raw := range rawItems {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		key := sub2APIAdminKey{Key: strings.TrimSpace(getString(item, "key"))}
		if id, ok := getInt64(item, "id"); ok {
			key.ID = id
		}
		if groupID, ok := getInt64(item, "group_id"); ok {
			key.GroupID = &groupID
		}
		group, _ := item["group"].(map[string]any)
		key.GroupName = strings.TrimSpace(getString(group, "name"))
		if rate, ok := getFloat64(group, "rate_multiplier"); ok {
			key.GroupRateMultiplier = &rate
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *OpenAIUpstreamBalanceService) fetchSub2APIUserGroupRates(ctx context.Context, baseURL, authorization string) (map[int64]float64, error) {
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildSub2APIGroupRatesURL(baseURL), map[string]string{
		"Authorization": authorization,
	}, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	rates := make(map[int64]float64, len(data))
	for key, raw := range data {
		groupID, err := parseInt64String(key)
		if err != nil {
			continue
		}
		switch value := raw.(type) {
		case json.Number:
			if f, err := value.Float64(); err == nil {
				rates[groupID] = f
			}
		case float64:
			rates[groupID] = value
		}
	}
	return rates, nil
}

type newAPIUserBalanceAuth struct {
	UserID      string
	AccessToken string
}

type newAPIToken struct {
	Key   string
	Group string
}

func (s *OpenAIUpstreamBalanceService) resolveNewAPITokenGroup(ctx context.Context, baseURL, apiKey string, auth newAPIUserBalanceAuth) (string, bool) {
	tokens, err := s.fetchNewAPITokens(ctx, baseURL, auth)
	if err != nil {
		return "", false
	}
	for _, token := range tokens {
		if !matchesNewAPITokenKey(apiKey, token.Key) {
			continue
		}
		group := strings.TrimSpace(token.Group)
		return group, group != ""
	}
	if len(tokens) == 1 {
		group := strings.TrimSpace(tokens[0].Group)
		return group, group != ""
	}
	return "", false
}

func (s *OpenAIUpstreamBalanceService) fetchNewAPITokens(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth) ([]newAPIToken, error) {
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildNewAPITokensURL(baseURL), map[string]string{
		"Authorization": auth.AccessToken,
		"New-Api-User":  auth.UserID,
	}, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	rawItems, _ := data["items"].([]any)
	tokens := make([]newAPIToken, 0, len(rawItems))
	for _, raw := range rawItems {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		tokens = append(tokens, newAPIToken{
			Key:   strings.TrimSpace(getString(item, "key")),
			Group: strings.TrimSpace(getString(item, "group")),
		})
	}
	return tokens, nil
}

func (s *OpenAIUpstreamBalanceService) resolveNewAPIGroupRate(ctx context.Context, baseURL, group string, auth newAPIUserBalanceAuth) (float64, bool) {
	group = strings.TrimSpace(group)
	if group == "" {
		return 0, false
	}
	rates, err := s.fetchNewAPIGroupRates(ctx, baseURL, auth)
	if err != nil {
		return 0, false
	}
	rate, ok := rates[group]
	return rate, ok
}

func (s *OpenAIUpstreamBalanceService) fetchNewAPIGroupRates(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth) (map[string]float64, error) {
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildNewAPIUserGroupsURL(baseURL), map[string]string{
		"Authorization": auth.AccessToken,
		"New-Api-User":  auth.UserID,
	}, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	rates := make(map[string]float64, len(data))
	for group, raw := range data {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		if rate, ok := getFloat64(item, "ratio"); ok {
			rates[strings.TrimSpace(group)] = rate
		}
	}
	return rates, nil
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

func buildNewAPITokensURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/token/?p=1&size=100")
}

func buildNewAPIUserGroupsURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/user/self/groups")
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

func buildSub2APIAuthLoginURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/v1/auth/login")
}

func buildSub2APIKeysURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/v1/keys?page=1&page_size=100&sort_by=created_at&sort_order=desc")
}

func buildSub2APIGroupRatesURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/v1/groups/rates")
}

func buildUpstreamAdminURL(baseURL, endpoint string) string {
	trimmed := strings.TrimSpace(baseURL)
	endpointURL, endpointErr := url.Parse(endpoint)
	endpointPath := endpoint
	endpointQuery := ""
	if endpointErr == nil {
		endpointPath = endpointURL.Path
		endpointQuery = endpointURL.RawQuery
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return strings.TrimRight(strings.TrimSuffix(trimmed, "/v1"), "/") + endpoint
	}
	path := strings.TrimRight(parsed.Path, "/")
	path = strings.TrimSuffix(path, "/v1")
	if path == "" {
		path = endpointPath
	} else {
		path += endpointPath
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = endpointQuery
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

func matchesNewAPITokenKey(apiKey, maskedKey string) bool {
	apiKey = strings.TrimSpace(apiKey)
	maskedKey = strings.TrimSpace(maskedKey)
	if apiKey == "" || maskedKey == "" {
		return false
	}
	if apiKey == maskedKey {
		return true
	}
	if !strings.Contains(maskedKey, "*") {
		return false
	}
	parts := strings.Split(maskedKey, "*")
	prefix := strings.TrimSpace(parts[0])
	suffix := strings.TrimSpace(parts[len(parts)-1])
	if prefix == "" && suffix == "" {
		return false
	}
	if prefix != "" && !strings.Contains(apiKey, prefix) {
		return false
	}
	if suffix != "" && !strings.HasSuffix(apiKey, suffix) {
		return false
	}
	return true
}

func getInt64(m map[string]any, key string) (int64, bool) {
	value, ok := getFloat64(m, key)
	if !ok {
		return 0, false
	}
	return int64(value), true
}

func parseInt64String(value string) (int64, error) {
	var n int64
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("empty int64")
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid int64 %q", value)
		}
		n = n*10 + int64(r-'0')
	}
	return n, nil
}
