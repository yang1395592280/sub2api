package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Sub2APICheckinStatusPending     = "pending"
	Sub2APICheckinStatusSuccess     = "success"
	Sub2APICheckinStatusError       = "error"
	Sub2APICheckinStatusUnsupported = "unsupported"

	sub2APICheckinDefaultURL     = "/api/v1/user/checkin"
	sub2APICheckinHTTPTimeout    = 15 * time.Second
	sub2APICheckinScanInterval   = time.Minute
	sub2APICheckinCandidateLimit = 200
	sub2APICheckinRetryMaxCount  = 3
	sub2APICheckinErrorMaxText   = 180
)

type Sub2APICheckinService struct {
	accountRepo            AccountRepository
	upstreamBalanceService *OpenAIUpstreamBalanceService
	client                 *http.Client
	clock                  func() time.Time
	loc                    *time.Location
	rng                    *rand.Rand
	rngMu                  sync.Mutex
	stopCh                 chan struct{}
	stopOnce               sync.Once
	wg                     sync.WaitGroup
}

func NewSub2APICheckinService(
	accountRepo AccountRepository,
	upstreamBalanceService *OpenAIUpstreamBalanceService,
	loc *time.Location,
) *Sub2APICheckinService {
	client := &http.Client{Timeout: sub2APICheckinHTTPTimeout}
	if upstreamBalanceService != nil && upstreamBalanceService.client != nil {
		client = upstreamBalanceService.client
	}
	if loc == nil {
		loc = time.Local
	}
	return &Sub2APICheckinService{
		accountRepo:            accountRepo,
		upstreamBalanceService: upstreamBalanceService,
		client:                 client,
		clock:                  time.Now,
		loc:                    loc,
		rng:                    rand.New(rand.NewSource(time.Now().UnixNano())),
		stopCh:                 make(chan struct{}),
	}
}

func (s *Sub2APICheckinService) Start() {
	if s == nil || s.accountRepo == nil {
		return
	}
	s.wg.Add(1)
	go s.scanLoop()
}

func (s *Sub2APICheckinService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *Sub2APICheckinService) RefreshNow(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("sub2api checkin service is not configured")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	now := s.now()
	return s.executeCheckin(ctx, account, now)
}

func (s *Sub2APICheckinService) scanLoop() {
	defer s.wg.Done()

	s.processDueCheckins()

	ticker := time.NewTicker(sub2APICheckinScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processDueCheckins()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Sub2APICheckinService) processDueCheckins() {
	ctx := context.Background()
	accounts, err := s.accountRepo.ListSub2APICheckinCandidates(ctx, sub2APICheckinCandidateLimit)
	if err != nil {
		slog.Error("sub2api.checkin_list_candidates_failed", "error", err)
		return
	}
	for i := range accounts {
		account := accounts[i]
		if err := s.reconcileAccount(ctx, &account, s.now()); err != nil {
			slog.Warn("sub2api.checkin_reconcile_failed", "account_id", account.ID, "error", err)
		}
	}
}

func (s *Sub2APICheckinService) reconcileAccount(ctx context.Context, account *Account, now time.Time) error {
	if !isSub2APICheckinEnabled(account) {
		return nil
	}

	localNow := now.In(s.location())
	today := localNow.Format("2006-01-02")
	startHHMM, endHHMM := sub2APICheckinWindow(account)
	start, end, err := checkinWindowForDate(localNow, startHHMM, endHHMM, s.location())
	if err != nil {
		_, persistErr := s.persistAccountUpdates(ctx, account, map[string]any{
			"upstream_checkin_status": Sub2APICheckinStatusUnsupported,
			"upstream_checkin_error":  truncate(err.Error(), sub2APICheckinErrorMaxText),
		}, nil)
		if persistErr != nil {
			return persistErr
		}
		return nil
	}

	successToday := account.GetExtraString("upstream_checkin_last_success_date") == today
	if successToday || reachedRetryCapForLocalDate(account, today) {
		return nil
	}

	nextRun := parseRFC3339String(account.GetExtraString("upstream_checkin_next_run_at"))
	plannedToday := sameLocalDate(nextRun, localNow, s.location())
	if !successToday && (!plannedToday || nextRun.Before(start) || nextRun.After(end)) {
		planned, planErr := s.planNextRunForDate(localNow, startHHMM, endHHMM)
		if planErr != nil {
			return planErr
		}
		nextRun = planned
		_, err = s.persistAccountUpdates(ctx, account, map[string]any{
			"upstream_checkin_status":      Sub2APICheckinStatusPending,
			"upstream_checkin_error":       "",
			"upstream_checkin_next_run_at": planned.Format(time.RFC3339),
		}, nil)
		if err != nil {
			return err
		}
	}

	if localNow.After(end) || (!nextRun.IsZero() && !localNow.Before(nextRun)) {
		_, err = s.executeCheckin(ctx, account, now)
		return err
	}
	return nil
}

func (s *Sub2APICheckinService) executeCheckin(ctx context.Context, account *Account, now time.Time) (*Account, error) {
	if account == nil {
		return nil, ErrAccountNotFound
	}
	baseURL := strings.TrimSpace(getUpstreamBalanceBaseURL(account))
	if baseURL == "" {
		return s.persistAccountUpdates(ctx, account, map[string]any{
			"upstream_checkin_status":      Sub2APICheckinStatusUnsupported,
			"upstream_checkin_error":       "base_url is required",
			"upstream_checkin_last_run_at": s.nowInLocation().Format(time.RFC3339),
		}, nil)
	}

	checkinURL, err := buildSub2APICheckinURL(baseURL, account.GetCredential("upstream_checkin_url"))
	if err != nil {
		return s.persistAccountUpdates(ctx, account, map[string]any{
			"upstream_checkin_status":      Sub2APICheckinStatusUnsupported,
			"upstream_checkin_error":       truncate(err.Error(), sub2APICheckinErrorMaxText),
			"upstream_checkin_last_run_at": now.In(s.location()).Format(time.RFC3339),
		}, nil)
	}

	authHeader, credentialUpdates, err := s.authResolver().resolveSub2APIAdminAuthorization(ctx, account, baseURL)
	if err != nil {
		updates := s.buildFailureUpdates(account, now, err.Error())
		return s.persistAccountUpdates(ctx, account, updates, credentialUpdates)
	}

	payload, err := s.postCheckin(ctx, checkinURL, authHeader)
	if err != nil {
		updates := s.buildFailureUpdates(account, now, err.Error())
		return s.persistAccountUpdates(ctx, account, updates, credentialUpdates)
	}

	status, rewardAmount, balance, _, errMsg, ok := classifySub2APICheckinResponse(payload)
	if !ok || status != Sub2APICheckinStatusSuccess {
		if errMsg == "" {
			errMsg = "unrecognized sub2api checkin response"
		}
		updates := s.buildFailureUpdates(account, now, errMsg)
		return s.persistAccountUpdates(ctx, account, updates, credentialUpdates)
	}

	updates := s.buildSuccessUpdates(now, rewardAmount, balance, account)
	return s.persistAccountUpdates(ctx, account, updates, credentialUpdates)
}

func (s *Sub2APICheckinService) persistAccountUpdates(ctx context.Context, account *Account, extraUpdates, credentialUpdates map[string]any) (*Account, error) {
	if len(credentialUpdates) == 0 {
		if err := s.accountRepo.UpdateExtra(ctx, account.ID, extraUpdates); err != nil {
			return nil, err
		}
	} else {
		rows, err := s.accountRepo.BulkUpdate(ctx, []int64{account.ID}, AccountBulkUpdate{
			Credentials: credentialUpdates,
			Extra:       extraUpdates,
		})
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			return nil, ErrAccountNotFound
		}
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	for k, v := range extraUpdates {
		account.Extra[k] = v
	}
	if len(credentialUpdates) > 0 {
		if account.Credentials == nil {
			account.Credentials = map[string]any{}
		}
		for k, v := range credentialUpdates {
			account.Credentials[k] = v
		}
	}
	return account, nil
}

func (s *Sub2APICheckinService) buildSuccessUpdates(now time.Time, rewardAmount, balance *float64, account *Account) map[string]any {
	localNow := now.In(s.location())
	today := localNow.Format("2006-01-02")
	startHHMM, endHHMM := sub2APICheckinWindow(account)
	updates := map[string]any{
		"upstream_checkin_status":            Sub2APICheckinStatusSuccess,
		"upstream_checkin_last_run_at":       localNow.Format(time.RFC3339),
		"upstream_checkin_last_success_date": today,
		"upstream_checkin_error":             "",
		"upstream_checkin_retry_date":        today,
		"upstream_checkin_retry_count":       0,
	}
	if rewardAmount != nil {
		updates["upstream_checkin_reward_amount"] = *rewardAmount
	}
	if balance != nil {
		updates["upstream_checkin_balance"] = *balance
	}
	if nextRun, err := s.planNextRunForDate(localNow.Add(24*time.Hour), startHHMM, endHHMM); err == nil {
		updates["upstream_checkin_next_run_at"] = nextRun.Format(time.RFC3339)
	}
	return updates
}

func (s *Sub2APICheckinService) buildFailureUpdates(account *Account, now time.Time, errMsg string) map[string]any {
	localNow := now.In(s.location())
	today := localNow.Format("2006-01-02")
	retryCount := 0
	if account.GetExtraString("upstream_checkin_retry_date") == today {
		retryCount = account.getExtraInt("upstream_checkin_retry_count")
	}
	updates := map[string]any{
		"upstream_checkin_status":      Sub2APICheckinStatusError,
		"upstream_checkin_last_run_at": localNow.Format(time.RFC3339),
		"upstream_checkin_error":       truncate(errMsg, sub2APICheckinErrorMaxText),
		"upstream_checkin_retry_date":  today,
	}
	if retryCount < sub2APICheckinRetryMaxCount {
		retryCount++
		updates["upstream_checkin_retry_count"] = retryCount
		retryAt := localNow.Add(s.randomRetryDelay())
		updates["upstream_checkin_next_run_at"] = retryAt.Format(time.RFC3339)
		return updates
	}
	updates["upstream_checkin_retry_count"] = retryCount
	updates["upstream_checkin_next_run_at"] = ""
	return updates
}

func (s *Sub2APICheckinService) planNextRunForDate(day time.Time, startHHMM, endHHMM string) (time.Time, error) {
	start, end, err := checkinWindowForDate(day, startHHMM, endHHMM, s.location())
	if err != nil {
		return time.Time{}, err
	}
	return randomTimeBetween(start, end, s.randomSource()), nil
}

func (s *Sub2APICheckinService) postCheckin(ctx context.Context, targetURL, authorization string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(nil))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authorization)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func (s *Sub2APICheckinService) authResolver() *OpenAIUpstreamBalanceService {
	if s.upstreamBalanceService != nil {
		return s.upstreamBalanceService
	}
	return NewOpenAIUpstreamBalanceService(s.accountRepo, s.client)
}

func (s *Sub2APICheckinService) now() time.Time {
	if s.clock == nil {
		return time.Now()
	}
	return s.clock()
}

func (s *Sub2APICheckinService) nowInLocation() time.Time {
	return s.now().In(s.location())
}

func (s *Sub2APICheckinService) location() *time.Location {
	if s.loc == nil {
		return time.Local
	}
	return s.loc
}

func (s *Sub2APICheckinService) randomSource() *rand.Rand {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng
}

func (s *Sub2APICheckinService) randomRetryDelay() time.Duration {
	start := time.Unix(0, 0)
	end := start.Add(20 * time.Minute)
	random := randomTimeBetween(start, end, s.randomSource())
	return 10*time.Minute + random.Sub(start)
}

func isSub2APICheckinEnabled(account *Account) bool {
	if account == nil || account.Status != StatusActive || account.Type != AccountTypeAPIKey {
		return false
	}
	if strings.TrimSpace(account.GetCredential("upstream_admin_type")) != UpstreamBalanceProviderSub2API {
		return false
	}
	if account.Credentials == nil {
		return false
	}
	raw, ok := account.Credentials["upstream_checkin_enabled"]
	if !ok || raw == nil {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		return err == nil && parsed
	default:
		return false
	}
}

func checkinWindowForDate(now time.Time, startHHMM, endHHMM string, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	hourStart, minuteStart, ok := parseHHMM(startHHMM)
	if !ok {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time %q", startHHMM)
	}
	hourEnd, minuteEnd, ok := parseHHMM(endHHMM)
	if !ok {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time %q", endHHMM)
	}
	localNow := now.In(loc)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hourStart, minuteStart, 0, 0, loc)
	end := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hourEnd, minuteEnd, 0, 0, loc)
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("checkin end time must be after start time")
	}
	return start, end, nil
}

func randomTimeBetween(start, end time.Time, rng *rand.Rand) time.Time {
	if !end.After(start) {
		return start
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	window := end.Sub(start)
	if window <= 0 {
		return start
	}
	return start.Add(time.Duration(rng.Int63n(window.Nanoseconds())))
}

func parseHHMM(value string) (hour, minute int, ok bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

func buildSub2APICheckinURL(baseURL, configuredURL string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		return "", fmt.Errorf("base_url is required")
	}
	target := strings.TrimSpace(configuredURL)
	if target == "" {
		target = sub2APICheckinDefaultURL
	}
	parsedTarget, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	if parsedTarget.IsAbs() {
		baseParsed, baseErr := url.Parse(buildUpstreamAdminURL(base, "/"))
		if baseErr != nil {
			return "", baseErr
		}
		if !sameOrigin(baseParsed, parsedTarget) {
			return "", fmt.Errorf("checkin URL origin must match base_url origin")
		}
		return parsedTarget.String(), nil
	}
	return buildUpstreamAdminURL(base, target), nil
}

func classifySub2APICheckinResponse(body []byte) (status string, rewardAmount *float64, balance *float64, checkedInAt *time.Time, errMsg string, ok bool) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return Sub2APICheckinStatusError, nil, nil, nil, err.Error(), false
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		data = payload
	}
	if reward, exists := getFloat64(data, "reward_amount"); exists {
		rewardAmount = &reward
	}
	if value, exists := getFloat64(data, "balance"); exists {
		balance = &value
	}
	if ts := parseRFC3339String(strings.TrimSpace(getString(data, "checked_in_at"))); !ts.IsZero() {
		checkedInAt = &ts
	}

	message := strings.TrimSpace(getString(payload, "message"))
	lowerMsg := strings.ToLower(message)
	code, hasCode := getFloat64(payload, "code")
	checkedIn := false
	if rawCheckedIn, exists := data["checked_in"]; exists {
		if parsed, ok := rawCheckedIn.(bool); ok {
			checkedIn = parsed
		}
	}
	success := (hasCode && code == 0) || strings.Contains(lowerMsg, "success")
	if checkedIn || checkedInAt != nil || strings.Contains(lowerMsg, "already") {
		success = true
	}
	if success {
		return Sub2APICheckinStatusSuccess, rewardAmount, balance, checkedInAt, "", true
	}
	if message == "" {
		message = "unrecognized sub2api checkin response"
	}
	return Sub2APICheckinStatusError, rewardAmount, balance, checkedInAt, message, false
}

func getSub2APIAdminAuth(account *Account) (sub2APIAdminAuth, bool) {
	if account == nil {
		return sub2APIAdminAuth{}, false
	}
	if provider := strings.TrimSpace(account.GetCredential("upstream_admin_type")); provider != UpstreamBalanceProviderSub2API {
		return sub2APIAdminAuth{}, false
	}
	auth := sub2APIAdminAuth{
		AccessToken:  strings.TrimSpace(account.GetCredential("upstream_admin_access_token")),
		RefreshToken: strings.TrimSpace(account.GetCredential("upstream_admin_refresh_token")),
		TokenType:    strings.TrimSpace(account.GetCredential("upstream_admin_token_type")),
		Email:        strings.TrimSpace(firstNonEmpty(account.GetCredential("upstream_admin_email"), account.GetCredential("upstream_admin_username"))),
		Password:     strings.TrimSpace(account.GetCredential("upstream_admin_password")),
	}
	return auth, auth.AccessToken != "" || auth.RefreshToken != "" || (auth.Email != "" && auth.Password != "")
}

func sub2APICheckinWindow(account *Account) (string, string) {
	start := strings.TrimSpace(account.GetCredential("upstream_checkin_start_time"))
	end := strings.TrimSpace(account.GetCredential("upstream_checkin_end_time"))
	if start == "" {
		start = "08:00"
	}
	if end == "" {
		end = "10:30"
	}
	return start, end
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func sameLocalDate(value, ref time.Time, loc *time.Location) bool {
	if value.IsZero() || ref.IsZero() {
		return false
	}
	if loc == nil {
		loc = time.Local
	}
	v := value.In(loc)
	r := ref.In(loc)
	return v.Year() == r.Year() && v.YearDay() == r.YearDay()
}

func parseRFC3339String(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func reachedRetryCapForLocalDate(account *Account, date string) bool {
	if account == nil || strings.TrimSpace(date) == "" {
		return false
	}
	if account.GetExtraString("upstream_checkin_retry_date") != date {
		return false
	}
	return account.getExtraInt("upstream_checkin_retry_count") >= sub2APICheckinRetryMaxCount
}
