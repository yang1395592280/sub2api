package admin

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type businessAnalyticsService interface {
	GetOverview(ctx context.Context, filter service.BusinessAnalyticsFilter) (*service.BusinessOverviewResponse, error)
	GetGroups(ctx context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessGroupRow, error)
	GetChannels(ctx context.Context, filter service.BusinessAnalyticsFilter) ([]service.BusinessChannelRow, error)
	GetPriceChangeImpact(ctx context.Context, input service.PriceChangeImpactInput) (*service.PriceChangeImpactResponse, error)
	GetRecords(ctx context.Context, filter service.BusinessRecordsFilter) (*service.BusinessRecordsResponse, error)
}

type BusinessAnalyticsHandler struct {
	service businessAnalyticsService
}

func NewBusinessAnalyticsHandler(service businessAnalyticsService) *BusinessAnalyticsHandler {
	return &BusinessAnalyticsHandler{service: service}
}

func (h *BusinessAnalyticsHandler) GetOverview(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	result, err := h.service.GetOverview(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BusinessAnalyticsHandler) GetGroups(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	rows, err := h.service.GetGroups(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *BusinessAnalyticsHandler) GetGroupChannels(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	groupID, ok := parsePathID(c, "id", "分组 ID 无效")
	if !ok {
		return
	}
	filter.GroupID = groupID
	rows, err := h.service.GetChannels(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *BusinessAnalyticsHandler) GetChannels(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	rows, err := h.service.GetChannels(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *BusinessAnalyticsHandler) GetChannelGroups(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	accountID, ok := parsePathID(c, "id", "渠道账号 ID 无效")
	if !ok {
		return
	}
	// The channels view is keyed by account rows, so :id is the channel account id.
	filter.AccountID = accountID
	rows, err := h.service.GetGroups(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *BusinessAnalyticsHandler) GetPriceChangeImpact(c *gin.Context) {
	groupID, ok := parseRequiredInt64Query(c, "group_id", "group_id 无效")
	if !ok {
		return
	}
	changeDateRaw := strings.TrimSpace(c.Query("change_date"))
	if changeDateRaw == "" {
		response.BadRequest(c, "change_date 为必填项")
		return
	}
	changeDate, err := timezone.ParseInUserLocation("2006-01-02", changeDateRaw, c.Query("timezone"))
	if err != nil {
		response.BadRequest(c, "change_date 格式无效，请使用 YYYY-MM-DD")
		return
	}
	days := 7
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 90 {
			response.BadRequest(c, "days 无效")
			return
		}
		days = parsed
	}
	result, err := h.service.GetPriceChangeImpact(c.Request.Context(), service.PriceChangeImpactInput{
		GroupID:    groupID,
		ChangeDate: changeDate,
		Days:       days,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BusinessAnalyticsHandler) GetRecords(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.GetRecords(c.Request.Context(), service.BusinessRecordsFilter{
		BusinessAnalyticsFilter: filter,
		Page:                    page,
		PageSize:                pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *BusinessAnalyticsHandler) Export(c *gin.Context) {
	filter, ok := parseBusinessAnalyticsFilter(c)
	if !ok {
		return
	}
	result, err := h.service.GetRecords(c.Request.Context(), service.BusinessRecordsFilter{
		BusinessAnalyticsFilter: filter,
		Page:                    1,
		PageSize:                10000,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=business-analytics.csv")
	c.Status(http.StatusOK)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"created_at", "user_id", "user_email", "api_key_id", "api_key_name", "group_id", "group_name", "account_id", "account_name", "model", "revenue", "channel_cost", "gross_profit", "rate_multiplier", "channel_price_snapshot", "channel_price_snapshot_missing"})
	for _, row := range result.Items {
		rateMultiplier := ""
		if row.RateMultiplier != nil {
			rateMultiplier = fmt.Sprintf("%.10f", *row.RateMultiplier)
		}
		channelPriceSnapshot := ""
		if row.ChannelPriceSnapshot != nil {
			channelPriceSnapshot = fmt.Sprintf("%.10f", *row.ChannelPriceSnapshot)
		}
		_ = w.Write([]string{
			row.CreatedAt.Format(time.RFC3339),
			strconv.FormatInt(row.UserID, 10),
			row.UserEmail,
			strconv.FormatInt(row.APIKeyID, 10),
			row.APIKeyName,
			strconv.FormatInt(row.GroupID, 10),
			row.GroupName,
			strconv.FormatInt(row.AccountID, 10),
			row.AccountName,
			row.Model,
			fmt.Sprintf("%.10f", row.Revenue),
			fmt.Sprintf("%.10f", row.ChannelCost),
			fmt.Sprintf("%.10f", row.GrossProfit),
			rateMultiplier,
			channelPriceSnapshot,
			strconv.FormatBool(row.ChannelPriceSnapshotMissing),
		})
	}
	w.Flush()
}

func parseBusinessAnalyticsFilter(c *gin.Context) (service.BusinessAnalyticsFilter, bool) {
	startRaw := strings.TrimSpace(c.Query("start_date"))
	endRaw := strings.TrimSpace(c.Query("end_date"))
	if startRaw == "" || endRaw == "" {
		response.BadRequest(c, "start_date 和 end_date 为必填项")
		return service.BusinessAnalyticsFilter{}, false
	}
	userTZ := c.Query("timezone")
	start, err := timezone.ParseInUserLocation("2006-01-02", startRaw, userTZ)
	if err != nil {
		response.BadRequest(c, "start_date 格式无效，请使用 YYYY-MM-DD")
		return service.BusinessAnalyticsFilter{}, false
	}
	end, err := timezone.ParseInUserLocation("2006-01-02", endRaw, userTZ)
	if err != nil {
		response.BadRequest(c, "end_date 格式无效，请使用 YYYY-MM-DD")
		return service.BusinessAnalyticsFilter{}, false
	}
	end = end.AddDate(0, 0, 1)
	if !end.After(start) {
		response.BadRequest(c, "end_date 必须大于或等于 start_date")
		return service.BusinessAnalyticsFilter{}, false
	}
	granularity := strings.TrimSpace(c.Query("granularity"))
	if granularity != "" && granularity != "day" && granularity != "week" {
		response.BadRequest(c, "granularity 无效，仅支持 day 或 week")
		return service.BusinessAnalyticsFilter{}, false
	}
	filter := service.BusinessAnalyticsFilter{
		StartDate:   start,
		EndDate:     end,
		Granularity: granularity,
		Platform:    strings.TrimSpace(c.Query("platform")),
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			response.BadRequest(c, "group_id 无效")
			return service.BusinessAnalyticsFilter{}, false
		}
		filter.GroupID = id
	}
	if raw := strings.TrimSpace(c.Query("account_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			response.BadRequest(c, "account_id 无效")
			return service.BusinessAnalyticsFilter{}, false
		}
		filter.AccountID = id
	}
	return filter, true
}

func parseRequiredInt64Query(c *gin.Context, key, invalidMessage string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		response.BadRequest(c, key+" 为必填项")
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, invalidMessage)
		return 0, false
	}
	return id, true
}

func parsePathID(c *gin.Context, key, invalidMessage string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, invalidMessage)
		return 0, false
	}
	return id, true
}
