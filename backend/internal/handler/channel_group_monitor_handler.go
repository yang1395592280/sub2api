package handler

import (
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GroupMonitorHandler struct {
	svc      *service.GroupMonitorService
	settings *service.SettingService
}

func NewGroupMonitorHandler(svc *service.GroupMonitorService, settings *service.SettingService) *GroupMonitorHandler {
	return &GroupMonitorHandler{svc: svc, settings: settings}
}
func groupMonitorID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "Invalid monitor ID")
		return 0, false
	}
	return id, true
}
func groupMonitorError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrGroupMonitorBusy) {
		response.Error(c, 409, err.Error())
	} else if errors.Is(err, service.ErrGroupMonitorInvalid) {
		response.BadRequest(c, err.Error())
	} else {
		response.ErrorFrom(c, err)
	}
}
func (h *GroupMonitorHandler) AdminList(c *gin.Context) {
	plans, err := h.svc.List(c.Request.Context())
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, plans)
}
func (h *GroupMonitorHandler) Save(c *gin.Context) {
	var req struct {
		GroupIDs []int64                    `json:"group_ids"`
		Config   service.GroupMonitorConfig `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid configuration")
		return
	}
	if err := h.svc.Save(c.Request.Context(), req.GroupIDs, req.Config); err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, gin.H{"saved": len(req.GroupIDs)})
}
func (h *GroupMonitorHandler) Delete(c *gin.Context) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
func (h *GroupMonitorHandler) Run(c *gin.Context) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid check kind")
		return
	}
	if err := h.svc.StartRun(c.Request.Context(), id, req.Kind, true); err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, gin.H{"started": true})
}
func (h *GroupMonitorHandler) AdminHistory(c *gin.Context) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	items, err := h.svc.History(c.Request.Context(), id, c.Query("kind"), c.Query("model"))
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, items)
}
func (h *GroupMonitorHandler) Review(c *gin.Context) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil || runID <= 0 {
		response.BadRequest(c, "Invalid run ID")
		return
	}
	var req struct {
		Review string `json:"review"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "Invalid review")
		return
	}
	if err = h.svc.Review(c.Request.Context(), id, runID, req.Review); err != nil {
		groupMonitorError(c, err)
		return
	}
	response.Success(c, gin.H{"saved": true})
}

// Whitelist DTOs prevent accidental account/ID/error disclosure when the admin
// record grows new fields. Generated HTML is exposed only behind its own flag.
type publicGroupMonitorRun struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	CreatedAt any    `json:"created_at"`
	LatencyMs int64  `json:"latency_ms"`
	TTFTMs    *int64 `json:"ttft_ms,omitempty"`
	Review    string `json:"review"`
	HTML      string `json:"html,omitempty"`
}

func publicGroupRun(r service.GroupMonitorRun) publicGroupMonitorRun {
	return publicGroupMonitorRun{ID: r.ID, Kind: r.Kind, Model: r.Model, Status: r.Status, CreatedAt: r.CreatedAt, LatencyMs: r.Detail.LatencyMs, TTFTMs: r.Detail.TTFTMs, Review: r.Detail.Review, HTML: r.Detail.HTML}
}
func (h *GroupMonitorHandler) userEnabled(c *gin.Context) bool {
	runtime := h.settings.GetChannelMonitorRuntime(c.Request.Context())
	return runtime.Enabled && runtime.Mode == service.ChannelMonitorModeV1 && h.settings.CustomFeatureEnabled(c.Request.Context(), service.SettingKeyMonitorUserVisible)
}
func (h *GroupMonitorHandler) UserList(c *gin.Context) {
	if !h.userEnabled(c) {
		response.Success(c, []any{})
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}
	plans, err := h.svc.List(c.Request.Context())
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	plans, err = h.svc.FilterVisible(c.Request.Context(), subject.UserID, plans)
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	out := []gin.H{}
	pelican := h.settings.CustomFeatureEnabled(c.Request.Context(), service.SettingKeyMonitorPelicanUserVisible)
	for _, p := range plans {
		if p.Config.Enabled {
			latest := []publicGroupMonitorRun{}
			for _, r := range p.LatestRuns {
				if r.Kind == "probe" || pelican {
					latest = append(latest, publicGroupRun(r))
				}
			}
			out = append(out, gin.H{"id": p.ID, "group_id": p.GroupID, "group_name": p.GroupName, "platform": p.Platform, "models": p.Config.Models, "pelican_visible": pelican, "latest_runs": latest})
		}
	}
	response.Success(c, out)
}
func (h *GroupMonitorHandler) UserHistory(c *gin.Context) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	if !h.userEnabled(c) {
		response.NotFound(c, "Monitor not found")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}
	plans, err := h.svc.List(c.Request.Context())
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	visible := false
	for _, p := range plans {
		if p.ID == id {
			visible = h.svc.Visible(c.Request.Context(), subject.UserID, p)
			break
		}
	}
	if !visible {
		response.NotFound(c, "Monitor not found")
		return
	}
	pelican := h.settings.CustomFeatureEnabled(c.Request.Context(), service.SettingKeyMonitorPelicanUserVisible)
	if !pelican && c.Query("kind") == "pelican" {
		response.NotFound(c, "Monitor not found")
		return
	}
	items, err := h.svc.History(c.Request.Context(), id, c.Query("kind"), c.Query("model"))
	if err != nil {
		groupMonitorError(c, err)
		return
	}
	out := []publicGroupMonitorRun{}
	for _, r := range items {
		if r.Kind == "probe" || pelican {
			out = append(out, publicGroupRun(r))
		}
	}
	response.Success(c, out)
}

func (h *GroupMonitorHandler) getRun(c *gin.Context, admin bool) {
	id, ok := groupMonitorID(c)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil || runID <= 0 {
		response.BadRequest(c, "Invalid run ID")
		return
	}
	if !admin {
		if !h.userEnabled(c) {
			response.NotFound(c, "Monitor not found")
			return
		}
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "Authentication required")
			return
		}
		plans, e := h.svc.List(c.Request.Context())
		if e != nil {
			groupMonitorError(c, e)
			return
		}
		visible := false
		for _, p := range plans {
			if p.ID == id {
				visible = h.svc.Visible(c.Request.Context(), subject.UserID, p)
				break
			}
		}
		if !visible {
			response.NotFound(c, "Monitor not found")
			return
		}
	}
	run, err := h.svc.GetRun(c.Request.Context(), id, runID)
	if err != nil {
		response.NotFound(c, "Run not found")
		return
	}
	if admin {
		response.Success(c, run)
		return
	}
	if run.Kind == "pelican" && !h.settings.CustomFeatureEnabled(c.Request.Context(), service.SettingKeyMonitorPelicanUserVisible) {
		response.NotFound(c, "Run not found")
		return
	}
	response.Success(c, publicGroupRun(*run))
}
func (h *GroupMonitorHandler) AdminRun(c *gin.Context) { h.getRun(c, true) }
func (h *GroupMonitorHandler) UserRun(c *gin.Context)  { h.getRun(c, false) }
