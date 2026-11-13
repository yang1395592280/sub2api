package service

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"
)

// UpstreamDailyUsage keeps upstream debits separate from recharge-adjusted costs.
// A nil amount means unavailable; zero is reserved for a complete, successful query.
type UpstreamDailyUsage struct {
	Date        string   `json:"date"`
	Timezone    string   `json:"timezone"`
	KeyCost     *float64 `json:"key_cost"`
	AccountCost *float64 `json:"account_cost"`
	CashCost    *float64 `json:"cash_cost"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	Source      string   `json:"source"`
	UpdatedAt   string   `json:"updated_at"`
}

func newUpstreamDailyUsage() *UpstreamDailyUsage {
	now := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60))
	return &UpstreamDailyUsage{
		Date: now.Format("2006-01-02"), Timezone: "Asia/Shanghai",
		Status: "unavailable", Error: "unavailable", UpdatedAt: now.UTC().Format(time.RFC3339),
	}
}

func finishUpstreamDailyUsage(usage *UpstreamDailyUsage, rechargeRatio float64) {
	if usage.KeyCost == nil {
		return
	}
	usage.Status, usage.Error = "ok", ""
	cost := *usage.KeyCost
	if rechargeRatio > 0 {
		cost /= rechargeRatio
	}
	usage.CashCost = &cost
}

func (s *OpenAIUpstreamBalanceService) enrichSub2APIDailyUsage(ctx context.Context, account *Account, baseURL, apiKey string, payload map[string]any, snapshot *OpenAIUpstreamBalanceSnapshot) {
	usage := newUpstreamDailyUsage()
	snapshot.DailyUsage = usage
	usage.Source = "sub2api_daily_usage"
	// daily_usage honours the requested timezone; usage.today follows the upstream
	// server's timezone, so it cannot safely substitute for a Beijing calendar day.
	if points, ok := payload["daily_usage"].([]any); ok {
		cost := 0.0
		valid := true
		for _, raw := range points {
			point, ok := raw.(map[string]any)
			if !ok {
				valid = false
				break
			}
			if getString(point, "date") == usage.Date {
				amount, ok := getFloat64(point, "actual_cost")
				if !ok || !validUpstreamUsageAmount(amount) {
					valid = false
					break
				}
				cost += amount
			} else {
				valid = false
			}
		}
		if valid {
			usage.KeyCost = &cost
		}
	}
	defer func() { finishUpstreamDailyUsage(usage, account.EffectiveUpstreamRechargeRatio()) }()
	if _, configured := getSub2APIAdminAuth(account); !configured {
		usage.Error = "credentials"
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	authorization := snapshot.adminAuthorization
	if authorization == "" {
		var updates map[string]any
		var err error
		authorization, updates, err = s.resolveSub2APIAdminAuthorization(ctx, account, baseURL)
		if err != nil {
			return
		}
		if len(updates) > 0 {
			snapshot.CredentialUpdates = updates
		}
	}
	if cost, err := s.fetchSub2APIDailyCost(ctx, baseURL, authorization, usage.Date, nil); err == nil {
		usage.AccountCost = &cost
	}
	if usage.KeyCost != nil {
		return
	}
	if snapshot.UpstreamKeyID == nil {
		keys, err := s.fetchSub2APIAdminKeys(ctx, baseURL, authorization)
		if err != nil {
			return
		}
		for _, key := range keys {
			if key.Key == apiKey && key.ID > 0 {
				id := key.ID
				snapshot.UpstreamKeyID = &id
				break
			}
		}
	}
	if snapshot.UpstreamKeyID == nil {
		usage.Error = "key_unmatched"
		return
	}
	if cost, err := s.fetchSub2APIDailyCost(ctx, baseURL, authorization, usage.Date, snapshot.UpstreamKeyID); err == nil {
		usage.KeyCost = &cost
		usage.Source = "sub2api_usage_stats"
	}
}

func validUpstreamUsageAmount(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func (s *OpenAIUpstreamBalanceService) fetchSub2APIDailyCost(ctx context.Context, baseURL, authorization, date string, keyID *int64) (float64, error) {
	query := url.Values{"start_date": {date}, "end_date": {date}, "timezone": {"Asia/Shanghai"}}
	if keyID != nil {
		query.Set("api_key_id", strconv.FormatInt(*keyID, 10))
	}
	var payload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildUpstreamAdminURL(baseURL, "/api/v1/usage/stats?"+query.Encode()), map[string]string{"Authorization": authorization}, &payload); err != nil {
		return 0, err
	}
	data, _ := payload["data"].(map[string]any)
	if code, ok := getInt64(payload, "code"); ok && code != 0 {
		return 0, fmt.Errorf("upstream usage query failed")
	}
	amount, ok := getFloat64(data, "total_actual_cost")
	if !ok || !validUpstreamUsageAmount(amount) {
		return 0, fmt.Errorf("upstream usage amount unavailable")
	}
	return amount, nil
}

func (s *OpenAIUpstreamBalanceService) enrichNewAPIDailyUsage(ctx context.Context, baseURL string, auth newAPIUserBalanceAuth, snapshot *OpenAIUpstreamBalanceSnapshot) {
	usage := newUpstreamDailyUsage()
	snapshot.DailyUsage = usage
	usage.Source = "new_api_consume_logs"
	if snapshot.UpstreamKeyID == nil || *snapshot.UpstreamKeyID <= 0 {
		usage.Error = "key_unmatched"
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	quotaPerUSD := newAPIQuotaPerUSD
	var statusPayload map[string]any
	if err := s.doJSONGETWithHeaders(ctx, buildUpstreamAdminURL(baseURL, "/api/status"), nil, &statusPayload); err == nil {
		data, _ := statusPayload["data"].(map[string]any)
		if value, ok := getFloat64(data, "quota_per_unit"); ok && validUpstreamUsageAmount(value) && value > 0 {
			quotaPerUSD = value
		}
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	start, _ := time.ParseInLocation("2006-01-02", usage.Date, location)
	query := url.Values{
		"type": {"2"}, "start_timestamp": {strconv.FormatInt(start.Unix(), 10)},
		"end_timestamp": {strconv.FormatInt(time.Now().Unix(), 10)}, "page_size": {"100"},
	}
	var keyQuota, accountQuota float64
	scanned := 0
	expectedTotal := int64(-1)
	// New API's stat endpoint filters token_name rather than token_id. Reading
	// consume logs preserves attribution for renamed and duplicate-name tokens.
	// The extra page also detects deployments whose total count is capped.
	for page := 1; page <= 101; page++ {
		query.Set("p", strconv.Itoa(page))
		var payload map[string]any
		if err := s.doJSONGETWithHeaders(ctx, buildUpstreamAdminURL(baseURL, "/api/log/self?"+query.Encode()), newAPIUserSessionHeaders(auth, auth.SessionCookie), &payload); err != nil {
			return
		}
		if success, _ := payload["success"].(bool); !success {
			return
		}
		data, _ := payload["data"].(map[string]any)
		items, ok := data["items"].([]any)
		total, hasTotal := getInt64(data, "total")
		if !ok || !hasTotal || total < 0 {
			usage.Error = "incomplete"
			return
		}
		if total > 10000 || scanned+len(items) > 10000 {
			usage.Error = "limit"
			return
		}
		if expectedTotal >= 0 && total != expectedTotal {
			usage.Error = "incomplete"
			return
		}
		expectedTotal = total
		if int64(scanned+len(items)) > total {
			usage.Error = "incomplete"
			return
		}
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			tokenID, hasID := getInt64(item, "token_id")
			quota, hasQuota := getFloat64(item, "quota")
			logType, _ := getInt64(item, "type")
			if !hasID || !hasQuota || !validUpstreamUsageAmount(quota) || logType != 2 {
				usage.Error = "incomplete"
				return
			}
			accountQuota += quota
			if tokenID == *snapshot.UpstreamKeyID {
				keyQuota += quota
			}
		}
		scanned += len(items)
		if len(items) == 0 || (len(items) < 100 && int64(scanned) >= total) {
			if int64(scanned) < total {
				usage.Error = "incomplete"
				return
			}
			keyCost, accountCost := keyQuota/quotaPerUSD, accountQuota/quotaPerUSD
			usage.KeyCost, usage.AccountCost = &keyCost, &accountCost
			finishUpstreamDailyUsage(usage, 1)
			return
		}
	}
	usage.Error = "limit"
}
