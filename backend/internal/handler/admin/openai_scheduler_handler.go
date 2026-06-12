package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OpenAISchedulerHandler struct {
	gatewayService *service.OpenAIGatewayService
}

func NewOpenAISchedulerHandler(gatewayService *service.OpenAIGatewayService) *OpenAISchedulerHandler {
	return &OpenAISchedulerHandler{gatewayService: gatewayService}
}

type openAISchedulerSettingsDTO struct {
	HealthRankingEnabled        bool    `json:"health_ranking_enabled"`
	PrimaryRatio                float64 `json:"primary_ratio"`
	PrimaryMinCount             int     `json:"primary_min_count"`
	TTFTDegradeMS               int     `json:"ttft_degrade_ms"`
	ErrorRateDegradeThreshold   float64 `json:"error_rate_degrade_threshold"`
	ConsecutiveFailureThreshold int     `json:"consecutive_failure_threshold"`
	RecoverSuccessThreshold     int     `json:"recover_success_threshold"`
	CooldownSeconds             int     `json:"cooldown_seconds"`
	ObserveProbeRatio           float64 `json:"observe_probe_ratio"`
}

type openAISchedulerActionRequest struct {
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	DurationSeconds int    `json:"duration_seconds"`
}

func openAISchedulerSettingsToDTO(settings service.OpenAISchedulerHealthSettings) openAISchedulerSettingsDTO {
	return openAISchedulerSettingsDTO{
		HealthRankingEnabled:        settings.HealthRankingEnabled,
		PrimaryRatio:                settings.PrimaryRatio,
		PrimaryMinCount:             settings.PrimaryMinCount,
		TTFTDegradeMS:               settings.TTFTDegradeMS,
		ErrorRateDegradeThreshold:   settings.ErrorRateDegradeThreshold,
		ConsecutiveFailureThreshold: settings.ConsecutiveFailureThreshold,
		RecoverSuccessThreshold:     settings.RecoverSuccessThreshold,
		CooldownSeconds:             int(settings.Cooldown.Seconds()),
		ObserveProbeRatio:           settings.ObserveProbeRatio,
	}
}

func openAISchedulerSettingsFromDTO(dto openAISchedulerSettingsDTO) service.OpenAISchedulerHealthSettings {
	return service.OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        dto.HealthRankingEnabled,
		PrimaryRatio:                dto.PrimaryRatio,
		PrimaryMinCount:             dto.PrimaryMinCount,
		TTFTDegradeMS:               dto.TTFTDegradeMS,
		ErrorRateDegradeThreshold:   dto.ErrorRateDegradeThreshold,
		ConsecutiveFailureThreshold: dto.ConsecutiveFailureThreshold,
		RecoverSuccessThreshold:     dto.RecoverSuccessThreshold,
		Cooldown:                    time.Duration(dto.CooldownSeconds) * time.Second,
		ObserveProbeRatio:           dto.ObserveProbeRatio,
	}
}

func (h *OpenAISchedulerHandler) GetOverview(c *gin.Context) {
	metrics := h.gatewayService.SnapshotOpenAIAccountSchedulerMetrics()
	settings := h.gatewayService.SnapshotOpenAISchedulerHealthSettings()
	items, err := h.gatewayService.ListOpenAISchedulerAccountSnapshots(c.Request.Context(), nil)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	tierCounts := openAISchedulerTierCounts(items)
	response.Success(c, gin.H{
		"settings":    openAISchedulerSettingsToDTO(settings),
		"metrics":     metrics,
		"tier_counts": tierCounts,
	})
}

func (h *OpenAISchedulerHandler) ListAccounts(c *gin.Context) {
	groupID, ok := parseOptionalQueryInt64(c, "group_id")
	if !ok {
		return
	}
	items, err := h.gatewayService.ListOpenAISchedulerAccountSnapshots(c.Request.Context(), groupID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	tier := strings.TrimSpace(c.Query("tier"))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	filtered := make([]service.OpenAISchedulerAccountSnapshot, 0, len(items))
	for _, item := range items {
		if tier != "" && item.Health.Tier != tier {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(item.AccountName), search) {
			continue
		}
		filtered = append(filtered, item)
	}

	page := parsePositiveQueryInt(c, "page", 1)
	pageSize := parsePositiveQueryInt(c, "page_size", 20)
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	response.Success(c, gin.H{
		"items":     filtered[start:end],
		"total":     len(filtered),
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *OpenAISchedulerHandler) GetAccount(c *gin.Context) {
	accountID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	snapshot, found := h.gatewayService.SnapshotOpenAIAccountHealth(c.Request.Context(), accountID)
	if !found {
		response.NotFound(c, "scheduler health snapshot not found")
		return
	}
	response.Success(c, snapshot)
}

func (h *OpenAISchedulerHandler) ApplyAction(c *gin.Context) {
	accountID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	var req openAISchedulerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	err := h.gatewayService.ApplyOpenAISchedulerHealthAction(accountID, service.OpenAISchedulerHealthAction{
		Action:   req.Action,
		Reason:   req.Reason,
		Duration: time.Duration(req.DurationSeconds) * time.Second,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *OpenAISchedulerHandler) GetSettings(c *gin.Context) {
	response.Success(c, openAISchedulerSettingsToDTO(h.gatewayService.SnapshotOpenAISchedulerHealthSettings()))
}

func (h *OpenAISchedulerHandler) UpdateSettings(c *gin.Context) {
	var req openAISchedulerSettingsDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.gatewayService.SaveOpenAISchedulerHealthSettings(c.Request.Context(), openAISchedulerSettingsFromDTO(req)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, openAISchedulerSettingsToDTO(h.gatewayService.SnapshotOpenAISchedulerHealthSettings()))
}

func parsePathInt64(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "Invalid id")
		return 0, false
	}
	return id, true
}

func parsePositiveQueryInt(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parseOptionalQueryInt64(c *gin.Context, name string) (*int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		response.Error(c, http.StatusBadRequest, "Invalid "+name)
		return nil, false
	}
	return &value, true
}

func openAISchedulerTierCounts(items []service.OpenAISchedulerAccountSnapshot) gin.H {
	counts := gin.H{
		service.OpenAISchedulerTierPrimary:  0,
		service.OpenAISchedulerTierStandby:  0,
		service.OpenAISchedulerTierObserve:  0,
		service.OpenAISchedulerTierDegraded: 0,
	}
	for _, item := range items {
		if _, ok := counts[item.Health.Tier]; ok {
			counts[item.Health.Tier] = counts[item.Health.Tier].(int) + 1
		}
	}
	return counts
}
