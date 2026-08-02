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
	CredentialUpdates       map[string]any
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
	if account == nil || !accountSupportsUpstreamBalance(account) {
		return nil, infraerrors.New(http.StatusBadRequest, "UPSTREAM_BALANCE_INVALID_ACCOUNT", "only OpenAI and Anthropic API Key accounts support upstream balance")
	}

	baseURL := strings.TrimSpace(getUpstreamBalanceBaseURL(account))
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
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
		normalizeOpenAIUpstreamBalanceSnapshot(&snapshot, account.EffectiveUpstreamRechargeRatio())
		updates = buildOpenAIUpstreamBalanceUpdates(snapshot)
	}
	if err := s.persistRefreshUpdates(ctx, account, updates, snapshot); err != nil {
		return nil, err
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	for k, v := range updates {
		account.Extra[k] = v
	}
	if len(snapshot.CredentialUpdates) > 0 {
		if account.Credentials == nil {
			account.Credentials = map[string]any{}
		}
		for k, v := range snapshot.CredentialUpdates {
			account.Credentials[k] = v
		}
	}
	if channelPrice := openAIUpstreamBalanceChannelPrice(snapshot); channelPrice != nil {
		account.ChannelPrice = channelPrice
	}
	return account, nil
}

func normalizeOpenAIUpstreamBalanceSnapshot(snapshot *OpenAIUpstreamBalanceSnapshot, rechargeRatio float64) {
	if snapshot == nil || rechargeRatio <= 0 || rechargeRatio == 1 {
		return
	}
	snapshot.Remaining /= rechargeRatio
	if snapshot.GroupRateMultiplier != nil {
		rate := *snapshot.GroupRateMultiplier / rechargeRatio
		snapshot.GroupRateMultiplier = &rate
	}
	if snapshot.EffectiveRateMultiplier != nil {
		rate := *snapshot.EffectiveRateMultiplier / rechargeRatio
		snapshot.EffectiveRateMultiplier = &rate
	}
}

func (s *OpenAIUpstreamBalanceService) persistRefreshUpdates(ctx context.Context, account *Account, updates map[string]any, snapshot OpenAIUpstreamBalanceSnapshot) error {
	channelPrice := openAIUpstreamBalanceChannelPrice(snapshot)
	if channelPrice == nil && len(snapshot.CredentialUpdates) == 0 {
		return s.accountRepo.UpdateExtra(ctx, account.ID, updates)
	}

	rows, err := s.accountRepo.BulkUpdate(ctx, []int64{account.ID}, AccountBulkUpdate{
		ChannelPrice: channelPrice,
		Credentials:  snapshot.CredentialUpdates,
		Extra:        updates,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func openAIUpstreamBalanceChannelPrice(snapshot OpenAIUpstreamBalanceSnapshot) *float64 {
	if snapshot.EffectiveRateMultiplier != nil && *snapshot.EffectiveRateMultiplier > 0 {
		price := *snapshot.EffectiveRateMultiplier
		return &price
	}
	if snapshot.GroupRateMultiplier != nil && *snapshot.GroupRateMultiplier > 0 {
		price := *snapshot.GroupRateMultiplier
		return &price
	}
	return nil
}

func accountSupportsUpstreamBalance(account *Account) bool {
	return account != nil &&
		account.Type == AccountTypeAPIKey &&
		(account.Platform == PlatformOpenAI || account.Platform == PlatformAnthropic)
}

func getUpstreamBalanceBaseURL(account *Account) string {
	if account == nil {
		return ""
	}
	if account.Platform == PlatformOpenAI {
		return account.GetOpenAIBaseURL()
	}
	return account.GetBaseURL()
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
		Remaining:           remaining,
		Unit:                strings.TrimSpace(getString(payload, "unit")),
		Group:               getOpenAIUpstreamGroupName(payload),
		GroupID:             getOpenAIUpstreamGroupID(payload),
		GroupRateMultiplier: getOpenAIUpstreamGroupRateMultiplier(payload),
	}
	if strings.TrimSpace(snapshot.Group) == "" {
		s.enrichSub2APIAdminMetadata(ctx, account, baseURL, apiKey, &snapshot)
	}
	// 部分上游的用量接口不返回分组/倍率，管理端凭据又可能受会话绑定或
	// 人机验证限制。保留原有识别顺序，仅在仍无法得到价格时使用 API Key
	// 自省当前实际计费倍率，避免余额刷新成功但渠道价格一直为空。
	if openAIUpstreamBalanceChannelPrice(snapshot) == nil {
		s.enrichSub2APIBillingRate(ctx, baseURL, apiKey, &snapshot)
	}
	return snapshot, nil
}

func (s *OpenAIUpstreamBalanceService) enrichSub2APIBillingRate(ctx context.Context, baseURL, apiKey string, snapshot *OpenAIUpstreamBalanceSnapshot) {
	if snapshot == nil || openAIUpstreamBalanceChannelPrice(*snapshot) != nil {
		return
	}

	var body json.RawMessage
	if err := s.doJSONGET(ctx, buildOpenAIEndpointURL(baseURL, "/v1/sub2api/billing"), apiKey, &body); err != nil {
		return
	}
	data, err := parseUpstreamBillingProbeResponse(body)
	if err != nil {
		return
	}

	if groupRate, ok := getFloat64(data, "group_rate_multiplier"); ok && groupRate > 0 {
		snapshot.GroupRateMultiplier = &groupRate
	}
	effectiveRate, ok := getFloat64(data, "effective_rate_multiplier")
	if !ok || effectiveRate <= 0 {
		return
	}
	snapshot.EffectiveRateMultiplier = &effectiveRate
	snapshot.RateSource = "sub2api_billing"
}

func (s *OpenAIUpstreamBalanceService) probeNewAPI(ctx context.Context, account *Account, baseURL, apiKey string) (OpenAIUpstreamBalanceSnapshot, error) {
	if auth, ok := getNewAPIUserBalanceAuth(account); ok {
		snapshot, err := s.probeNewAPIUserSelf(ctx, baseURL, auth)
		if err == nil {
			s.enrichNewAPIUserMetadata(ctx, baseURL, apiKey, auth, &snapshot)
		}
		return snapshot, err
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

func (s *OpenAIUpstreamBalanceService) doJSONPOSTWithHeaders(ctx context.Context, targetURL string, headers map[string]string, body any, dest any) error {
	_, err := s.doJSONPOSTWithHeadersAndCookies(ctx, targetURL, headers, body, dest)
	return err
}

func (s *OpenAIUpstreamBalanceService) doJSONPOSTForCookies(ctx context.Context, targetURL string, headers map[string]string, body any) (map[string]any, []*http.Cookie, error) {
	payload := map[string]any{}
	cookies, err := s.doJSONPOSTWithHeadersAndCookies(ctx, targetURL, headers, body, &payload)
	return payload, cookies, err
}

func (s *OpenAIUpstreamBalanceService) doJSONPOSTWithHeadersAndCookies(ctx context.Context, targetURL string, headers map[string]string, body any, dest any) ([]*http.Cookie, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(dest); err != nil {
		return nil, err
	}
	return resp.Cookies(), nil
}

type sub2APIAdminAuth struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Email        string
	Password     string
}

type sub2APIAdminToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
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
	authHeader, credentialUpdates, err := s.resolveSub2APIAdminAuthorization(ctx, account, baseURL)
	if err != nil {
		return
	}
	if len(credentialUpdates) > 0 {
		snapshot.CredentialUpdates = credentialUpdates
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

func (s *OpenAIUpstreamBalanceService) resolveSub2APIAdminAuthorization(ctx context.Context, account *Account, baseURL string) (string, map[string]any, error) {
	auth, ok := getSub2APIAdminAuth(account)
	if !ok {
		return "", nil, fmt.Errorf("sub2api admin credentials are required")
	}

	tokenType := strings.TrimSpace(auth.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	if strings.TrimSpace(auth.RefreshToken) != "" {
		refreshed, err := s.refreshSub2APIAdminToken(ctx, baseURL, auth.RefreshToken)
		if err == nil && strings.TrimSpace(refreshed.AccessToken) != "" {
			if strings.TrimSpace(refreshed.TokenType) != "" {
				tokenType = refreshed.TokenType
			}
			return formatSub2APIAuthorizationHeader(tokenType, refreshed.AccessToken), buildSub2APIAdminTokenCredentialUpdates(refreshed), nil
		}
	}
	if strings.TrimSpace(auth.AccessToken) != "" {
		return formatSub2APIAuthorizationHeader(tokenType, auth.AccessToken), nil, nil
	}
	if strings.TrimSpace(auth.Email) != "" && strings.TrimSpace(auth.Password) != "" {
		loginToken, loginTokenType, err := s.loginSub2APIAdmin(ctx, baseURL, auth.Email, auth.Password)
		if err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(loginTokenType) != "" {
			tokenType = loginTokenType
		}
		return formatSub2APIAuthorizationHeader(tokenType, loginToken), nil, nil
	}
	return "", nil, fmt.Errorf("sub2api admin credentials are required")
}

func formatSub2APIAuthorizationHeader(tokenType, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if strings.Contains(token, " ") {
		return token
	}
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return tokenType + " " + token
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

func (s *OpenAIUpstreamBalanceService) refreshSub2APIAdminToken(ctx context.Context, baseURL, refreshToken string) (sub2APIAdminToken, error) {
	var payload map[string]any
	err := s.doJSONPOSTWithHeaders(ctx, buildSub2APIAuthRefreshURL(baseURL), nil, map[string]string{
		"refresh_token": strings.TrimSpace(refreshToken),
	}, &payload)
	if err != nil {
		return sub2APIAdminToken{}, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	token := strings.TrimSpace(getString(data, "access_token"))
	if token == "" {
		return sub2APIAdminToken{}, fmt.Errorf("sub2api refresh response missing access_token")
	}
	return sub2APIAdminToken{
		AccessToken:  token,
		RefreshToken: strings.TrimSpace(getString(data, "refresh_token")),
		TokenType:    strings.TrimSpace(getString(data, "token_type")),
	}, nil
}

func buildSub2APIAdminTokenCredentialUpdates(token sub2APIAdminToken) map[string]any {
	updates := map[string]any{}
	if token.AccessToken != "" {
		updates["upstream_admin_access_token"] = token.AccessToken
	}
	if token.RefreshToken != "" {
		updates["upstream_admin_refresh_token"] = token.RefreshToken
	}
	if token.TokenType != "" {
		updates["upstream_admin_token_type"] = token.TokenType
	}
	return updates
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

type newAPIUserToken struct {
	ID    int64
	Key   string
	Name  string
	Group string
}

type newAPIUserBalanceAuth struct {
	UserID        string
	AccessToken   string
	SessionCookie string
	LoginUsername string
	LoginPassword string
}

func (s *OpenAIUpstreamBalanceService) enrichNewAPIUserMetadata(ctx context.Context, baseURL, apiKey string, auth newAPIUserBalanceAuth, snapshot *OpenAIUpstreamBalanceSnapshot) {
	if snapshot == nil {
		return
	}
	token, err := s.fetchNewAPIUserToken(ctx, baseURL, apiKey, auth)
	if err != nil || token == nil {
		return
	}
	if token.ID != 0 {
		snapshot.UpstreamKeyID = &token.ID
	}
	if strings.TrimSpace(token.Group) != "" {
		snapshot.Group = strings.TrimSpace(token.Group)
	}
	if strings.TrimSpace(snapshot.Group) == "" {
		return
	}
	rates, err := s.fetchNewAPIUserSelfGroupRates(ctx, baseURL, auth)
	if err != nil {
		return
	}
	if rate, ok := rates[strings.TrimSpace(snapshot.Group)]; ok {
		snapshot.EffectiveRateMultiplier = &rate
		snapshot.RateSource = "user_group_rate"
	}
}

func (s *OpenAIUpstreamBalanceService) fetchNewAPIUserToken(ctx context.Context, baseURL, apiKey string, auth newAPIUserBalanceAuth) (*newAPIUserToken, error) {
	queryKey := strings.TrimSpace(apiKey)
	if strings.HasPrefix(queryKey, "sk-") {
		queryKey = strings.TrimPrefix(queryKey, "sk-")
	}
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildNewAPITokenSearchURL(baseURL, queryKey), newAPIUserHeaders(auth), &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	rawItems, _ := data["items"].([]any)
	for _, raw := range rawItems {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		token := parseNewAPIUserToken(item)
		if token == nil {
			continue
		}
		if newAPIKeyMatchesAPIKey(token.Key, apiKey) || len(rawItems) == 1 {
			return token, nil
		}
	}
	return nil, fmt.Errorf("new-api token not found")
}

func parseNewAPIUserToken(item map[string]any) *newAPIUserToken {
	token := &newAPIUserToken{
		Key:   strings.TrimSpace(getString(item, "key")),
		Name:  strings.TrimSpace(getString(item, "name")),
		Group: strings.TrimSpace(getString(item, "group")),
	}
	if id, ok := getInt64(item, "id"); ok {
		token.ID = id
	}
	if token.Key == "" && token.Name == "" && token.Group == "" && token.ID == 0 {
		return nil
	}
	return token
}

func newAPIKeyMatchesAPIKey(tokenKey, apiKey string) bool {
	normalizedTokenKey := strings.TrimSpace(tokenKey)
	normalizedAPIKey := strings.TrimSpace(apiKey)
	if normalizedTokenKey == "" || normalizedAPIKey == "" {
		return false
	}
	if normalizedTokenKey == normalizedAPIKey {
		return true
	}
	if !strings.HasPrefix(normalizedTokenKey, "sk-") && "sk-"+normalizedTokenKey == normalizedAPIKey {
		return true
	}
	if strings.Contains(normalizedTokenKey, "*") {
		prefix, suffix, ok := strings.Cut(normalizedTokenKey, "*")
		if ok {
			return strings.HasPrefix(strings.TrimPrefix(normalizedAPIKey, "sk-"), strings.TrimPrefix(prefix, "sk-")) &&
				strings.HasSuffix(normalizedAPIKey, suffix)
		}
	}
	return false
}

func (s *OpenAIUpstreamBalanceService) fetchNewAPIUserSelfGroupRates(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth) (map[string]float64, error) {
	sessionCookie := strings.TrimSpace(auth.SessionCookie)
	if sessionCookie == "" {
		var err error
		sessionCookie, err = s.loginNewAPIUserSession(ctx, baseURL, auth)
		if err != nil {
			return nil, err
		}
	}
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildNewAPIUserSelfGroupsURL(baseURL), newAPIUserSessionHeaders(auth, sessionCookie), &payload); err != nil {
		return nil, err
	}
	rawRates, _ := payload["data"].(map[string]any)
	if rawRates == nil {
		rawRates = payload
	}
	rates := make(map[string]float64, len(rawRates))
	for group, raw := range rawRates {
		if rate, ok := anyToFloat64(raw); ok {
			rates[strings.TrimSpace(group)] = rate
			continue
		}
		entry, _ := raw.(map[string]any)
		if rate, ok := getFloat64(entry, "ratio"); ok {
			rates[strings.TrimSpace(group)] = rate
		}
	}
	return rates, nil
}

func (s *OpenAIUpstreamBalanceService) loginNewAPIUserSession(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth) (string, error) {
	username := strings.TrimSpace(auth.LoginUsername)
	password := auth.LoginPassword
	if username == "" || strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("new-api session cookie or login credentials are required for user group rates")
	}

	payload, cookies, err := s.doJSONPOSTForCookies(ctx, buildNewAPIUserLoginURL(baseURL), nil, map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	if success, ok := payload["success"].(bool); ok && !success {
		return "", fmt.Errorf("new-api login failed")
	}
	cookieHeader := buildCookieHeader(cookies)
	if cookieHeader == "" {
		return "", fmt.Errorf("new-api login response missing session cookie")
	}
	return cookieHeader, nil
}

func newAPIUserHeaders(auth newAPIUserBalanceAuth) map[string]string {
	return map[string]string{
		"Authorization": auth.AccessToken,
		"New-Api-User":  auth.UserID,
	}
}

func newAPIUserSessionHeaders(auth newAPIUserBalanceAuth, sessionCookie string) map[string]string {
	headers := map[string]string{
		"New-Api-User": auth.UserID,
		"Cookie":       sessionCookie,
	}
	if strings.TrimSpace(auth.AccessToken) != "" {
		headers["Authorization"] = auth.AccessToken
	}
	return headers
}

func getNewAPIUserBalanceAuth(account *Account) (newAPIUserBalanceAuth, bool) {
	if account == nil {
		return newAPIUserBalanceAuth{}, false
	}
	auth := newAPIUserBalanceAuth{
		UserID:        strings.TrimSpace(account.GetCredential("new_api_user_id")),
		AccessToken:   strings.TrimSpace(account.GetCredential("new_api_user_access_token")),
		SessionCookie: strings.TrimSpace(account.GetCredential("new_api_session_cookie")),
		LoginUsername: strings.TrimSpace(firstNonEmpty(account.GetCredential("new_api_login_username"), account.GetCredential("new_api_login_email"))),
		LoginPassword: account.GetCredential("new_api_login_password"),
	}
	return auth, auth.UserID != "" && auth.AccessToken != ""
}

func hasNewAPIUserBalanceCredentials(account *Account) bool {
	_, ok := getNewAPIUserBalanceAuth(account)
	return ok
}

func buildNewAPITokenSearchURL(baseURL, token string) string {
	return buildUpstreamAdminURL(baseURL, "/api/token/search?token="+url.QueryEscape(strings.TrimSpace(token)))
}

func buildNewAPIUserSelfGroupsURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/user/self/groups")
}

func buildNewAPIUserLoginURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/user/login")
}

func buildCookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(cookie.Name)+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
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

func buildSub2APIAuthLoginURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/v1/auth/login")
}

func buildSub2APIAuthRefreshURL(baseURL string) string {
	return buildUpstreamAdminURL(baseURL, "/api/v1/auth/refresh")
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
	return anyToFloat64(raw)
}

func anyToFloat64(raw any) (float64, bool) {
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
	// 部分 Sub2API 渠道把当前密钥及其分组放在 api_key 下，而余额仍在顶层。
	// 兼容该结构，避免余额刷新成功但分组与渠道价格无法同步。
	apiKey, _ := payload["api_key"].(map[string]any)
	if value := strings.TrimSpace(getString(apiKey, "group_name")); value != "" {
		return value
	}
	group, _ = apiKey["group"].(map[string]any)
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
	apiKey, _ := payload["api_key"].(map[string]any)
	if id, ok := getInt64(apiKey, "group_id"); ok {
		return &id
	}
	group, _ = apiKey["group"].(map[string]any)
	if id, ok := getInt64(group, "id"); ok {
		return &id
	}
	return nil
}

func getOpenAIUpstreamGroupRateMultiplier(payload map[string]any) *float64 {
	if payload == nil {
		return nil
	}
	for _, key := range []string{"upstream_group_rate_multiplier", "group_rate_multiplier", "rate_multiplier"} {
		if rate, ok := getFloat64(payload, key); ok {
			return &rate
		}
	}
	group, _ := payload["group"].(map[string]any)
	if rate, ok := getFloat64(group, "rate_multiplier"); ok {
		return &rate
	}
	apiKey, _ := payload["api_key"].(map[string]any)
	for _, key := range []string{"group_rate_multiplier", "rate_multiplier"} {
		if rate, ok := getFloat64(apiKey, key); ok {
			return &rate
		}
	}
	group, _ = apiKey["group"].(map[string]any)
	if rate, ok := getFloat64(group, "rate_multiplier"); ok {
		return &rate
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
