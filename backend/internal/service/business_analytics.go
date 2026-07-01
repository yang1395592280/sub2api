package service

import (
	"context"
	"time"
)

// BusinessAnalyticsAggregationRepository 负责重建经营分析聚合表。
type BusinessAnalyticsAggregationRepository interface {
	RecomputeDaily(ctx context.Context, startDate, endDate time.Time) error
	RecomputeWeekly(ctx context.Context, weekStart time.Time) error
}

// BusinessAnalyticsRepository 提供经营分析读查询。
type BusinessAnalyticsRepository interface {
	GetOverview(ctx context.Context, filter BusinessAnalyticsFilter) (*BusinessOverviewData, error)
	GetTrend(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessTrendPoint, error)
	GetGroups(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessGroupRow, error)
	GetChannels(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessChannelRow, error)
	GetPriceChangeImpact(ctx context.Context, input PriceChangeImpactInput) (*PriceChangeImpactResponse, error)
	GetRecords(ctx context.Context, filter BusinessRecordsFilter) (*BusinessRecordsResponse, error)
}

// BusinessAnalyticsFilter 是经营分析汇总接口的公共过滤条件。
type BusinessAnalyticsFilter struct {
	StartDate   time.Time
	EndDate     time.Time
	Granularity string
	GroupID     int64
	AccountID   int64
	Platform    string
}

// BusinessRecordsFilter 是经营分析明细接口过滤条件。
type BusinessRecordsFilter struct {
	BusinessAnalyticsFilter
	Page     int
	PageSize int
}

type BusinessOverviewData struct {
	Requests      int64   `json:"requests"`
	ActiveUsers   int64   `json:"active_users"`
	ActiveAPIKeys int64   `json:"active_api_keys"`
	TotalTokens   int64   `json:"total_tokens"`
	Revenue       float64 `json:"revenue"`
	ChannelCost   float64 `json:"channel_cost"`
	GrossProfit   float64 `json:"gross_profit"`
	MissingPrice  int64   `json:"missing_channel_price_records"`
	Trend         []BusinessTrendPoint
}

type BusinessOverviewResponse struct {
	StartDate            string               `json:"start_date"`
	EndDate              string               `json:"end_date"`
	Requests             int64                `json:"requests"`
	ActiveUsers          int64                `json:"active_users"`
	ActiveAPIKeys        int64                `json:"active_api_keys"`
	TotalTokens          int64                `json:"total_tokens"`
	Revenue              float64              `json:"revenue"`
	ChannelCost          float64              `json:"channel_cost"`
	GrossProfit          float64              `json:"gross_profit"`
	ProfitMargin         *float64             `json:"profit_margin"`
	RevenuePerActiveUser *float64             `json:"revenue_per_active_user"`
	ProfitPerActiveUser  *float64             `json:"profit_per_active_user"`
	MissingPriceRecords  int64                `json:"missing_channel_price_records"`
	Trend                []BusinessTrendPoint `json:"trend"`
}

type BusinessTrendPoint struct {
	Date         string   `json:"date"`
	Requests     int64    `json:"requests"`
	ActiveUsers  int64    `json:"active_users"`
	Revenue      float64  `json:"revenue"`
	ChannelCost  float64  `json:"channel_cost"`
	GrossProfit  float64  `json:"gross_profit"`
	ProfitMargin *float64 `json:"profit_margin,omitempty"`
}

type BusinessGroupRow struct {
	GroupID               int64    `json:"group_id"`
	GroupName             string   `json:"group_name"`
	Platform              string   `json:"platform"`
	CurrentRateMultiplier *float64 `json:"current_rate_multiplier,omitempty"`
	AverageRateMultiplier *float64 `json:"avg_rate_multiplier,omitempty"`
	Requests              int64    `json:"requests"`
	ActiveUsers           int64    `json:"active_users"`
	ActiveAPIKeys         int64    `json:"active_api_keys"`
	TotalTokens           int64    `json:"total_tokens"`
	Revenue               float64  `json:"revenue"`
	ChannelCost           float64  `json:"channel_cost"`
	GrossProfit           float64  `json:"gross_profit"`
	ProfitMargin          *float64 `json:"profit_margin,omitempty"`
	PreviousRevenue       float64  `json:"previous_revenue"`
	PreviousGrossProfit   float64  `json:"previous_gross_profit"`
	RevenueChangeRate     *float64 `json:"revenue_change_rate,omitempty"`
	GrossProfitChangeRate *float64 `json:"gross_profit_change_rate,omitempty"`
}

type BusinessChannelRow struct {
	AccountID           int64    `json:"account_id"`
	AccountName         string   `json:"account_name"`
	ChannelID           int64    `json:"channel_id"`
	Platform            string   `json:"platform"`
	Status              string   `json:"status"`
	CurrentChannelPrice *float64 `json:"current_channel_price,omitempty"`
	AverageChannelPrice *float64 `json:"avg_channel_price,omitempty"`
	BalanceStatus       string   `json:"balance_status,omitempty"`
	Requests            int64    `json:"requests"`
	ActiveUsers         int64    `json:"active_users"`
	ActiveAPIKeys       int64    `json:"active_api_keys"`
	TotalTokens         int64    `json:"total_tokens"`
	Revenue             float64  `json:"revenue"`
	ChannelCost         float64  `json:"channel_cost"`
	GrossProfit         float64  `json:"gross_profit"`
	ProfitMargin        *float64 `json:"profit_margin,omitempty"`
	MissingPriceRecords int64    `json:"missing_channel_price_records"`
}

type PriceChangeImpactInput struct {
	GroupID    int64
	ChangeDate time.Time
	Days       int
}

type PriceChangeImpactResponse struct {
	GroupID                 int64     `json:"group_id"`
	ChangeDate              string    `json:"change_date"`
	BeforeRequests          int64     `json:"before_requests"`
	AfterRequests           int64     `json:"after_requests"`
	BeforeActiveUsers       int64     `json:"before_active_users"`
	AfterActiveUsers        int64     `json:"after_active_users"`
	BeforeRevenue           float64   `json:"before_revenue"`
	AfterRevenue            float64   `json:"after_revenue"`
	RevenueDelta            float64   `json:"revenue_delta"`
	BeforeChannelCost       float64   `json:"before_channel_cost"`
	AfterChannelCost        float64   `json:"after_channel_cost"`
	BeforeGrossProfit       float64   `json:"before_gross_profit"`
	AfterGrossProfit        float64   `json:"after_gross_profit"`
	GrossProfitDelta        float64   `json:"gross_profit_delta"`
	BeforeProfitMargin      *float64  `json:"before_profit_margin,omitempty"`
	AfterProfitMargin       *float64  `json:"after_profit_margin,omitempty"`
	BeforeAvgRateMultiplier *float64  `json:"before_avg_rate_multiplier,omitempty"`
	AfterAvgRateMultiplier  *float64  `json:"after_avg_rate_multiplier,omitempty"`
	NewUsers                int64     `json:"new_users"`
	LostUsers               int64     `json:"lost_users"`
	ChangeAt                time.Time `json:"change_at,omitempty"`
}

type BusinessRecordRow struct {
	ID                          int64     `json:"id"`
	CreatedAt                   time.Time `json:"created_at"`
	UserID                      int64     `json:"user_id"`
	UserEmail                   string    `json:"user_email"`
	APIKeyID                    int64     `json:"api_key_id"`
	APIKeyName                  string    `json:"api_key_name"`
	GroupID                     int64     `json:"group_id"`
	GroupName                   string    `json:"group_name"`
	AccountID                   int64     `json:"account_id"`
	AccountName                 string    `json:"account_name"`
	Model                       string    `json:"model"`
	Requests                    int64     `json:"requests"`
	TotalTokens                 int64     `json:"total_tokens"`
	Revenue                     float64   `json:"revenue"`
	ChannelCost                 float64   `json:"channel_cost"`
	GrossProfit                 float64   `json:"gross_profit"`
	RateMultiplier              *float64  `json:"rate_multiplier,omitempty"`
	ChannelPriceSnapshot        *float64  `json:"channel_price_snapshot,omitempty"`
	ChannelPriceSnapshotMissing bool      `json:"channel_price_snapshot_missing"`
}

type BusinessRecordsResponse struct {
	Items    []BusinessRecordRow `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type BusinessAnalyticsService struct {
	repo BusinessAnalyticsRepository
}

func NewBusinessAnalyticsService(repo BusinessAnalyticsRepository) *BusinessAnalyticsService {
	return &BusinessAnalyticsService{repo: repo}
}

func (s *BusinessAnalyticsService) GetOverview(ctx context.Context, filter BusinessAnalyticsFilter) (*BusinessOverviewResponse, error) {
	data, err := s.repo.GetOverview(ctx, filter)
	if err != nil {
		return nil, err
	}
	trend, err := s.repo.GetTrend(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range trend {
		trend[i].ProfitMargin = ProfitMargin(trend[i].Revenue, trend[i].GrossProfit)
	}
	return &BusinessOverviewResponse{
		StartDate:            filter.StartDate.Format("2006-01-02"),
		EndDate:              filter.EndDate.AddDate(0, 0, -1).Format("2006-01-02"),
		Requests:             data.Requests,
		ActiveUsers:          data.ActiveUsers,
		ActiveAPIKeys:        data.ActiveAPIKeys,
		TotalTokens:          data.TotalTokens,
		Revenue:              data.Revenue,
		ChannelCost:          data.ChannelCost,
		GrossProfit:          data.GrossProfit,
		ProfitMargin:         ProfitMargin(data.Revenue, data.GrossProfit),
		RevenuePerActiveUser: perActiveUser(data.Revenue, data.ActiveUsers),
		ProfitPerActiveUser:  perActiveUser(data.GrossProfit, data.ActiveUsers),
		MissingPriceRecords:  data.MissingPrice,
		Trend:                trend,
	}, nil
}

func (s *BusinessAnalyticsService) GetGroups(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessGroupRow, error) {
	rows, err := s.repo.GetGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ProfitMargin = ProfitMargin(rows[i].Revenue, rows[i].GrossProfit)
		rows[i].RevenueChangeRate = changeRate(rows[i].PreviousRevenue, rows[i].Revenue)
		rows[i].GrossProfitChangeRate = changeRate(rows[i].PreviousGrossProfit, rows[i].GrossProfit)
	}
	return rows, nil
}

func (s *BusinessAnalyticsService) GetChannels(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessChannelRow, error) {
	rows, err := s.repo.GetChannels(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ProfitMargin = ProfitMargin(rows[i].Revenue, rows[i].GrossProfit)
	}
	return rows, nil
}

func (s *BusinessAnalyticsService) GetPriceChangeImpact(ctx context.Context, input PriceChangeImpactInput) (*PriceChangeImpactResponse, error) {
	resp, err := s.repo.GetPriceChangeImpact(ctx, input)
	if err != nil {
		return nil, err
	}
	resp.BeforeProfitMargin = ProfitMargin(resp.BeforeRevenue, resp.BeforeGrossProfit)
	resp.AfterProfitMargin = ProfitMargin(resp.AfterRevenue, resp.AfterGrossProfit)
	return resp, nil
}

func (s *BusinessAnalyticsService) GetRecords(ctx context.Context, filter BusinessRecordsFilter) (*BusinessRecordsResponse, error) {
	return s.repo.GetRecords(ctx, filter)
}

func ProfitMargin(revenue, grossProfit float64) *float64 {
	if revenue == 0 {
		return nil
	}
	v := grossProfit / revenue
	return &v
}

func perActiveUser(value float64, activeUsers int64) *float64 {
	if activeUsers == 0 {
		return nil
	}
	v := value / float64(activeUsers)
	return &v
}

func changeRate(previous, current float64) *float64 {
	if previous == 0 {
		return nil
	}
	v := (current - previous) / previous
	return &v
}
