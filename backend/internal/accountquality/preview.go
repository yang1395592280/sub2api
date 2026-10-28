package accountquality

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
)

func (s *Service) previewHTTP(c *gin.Context) {
	raw, err := body(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var req struct {
		Mode     string            `json:"mode"`
		Items    []json.RawMessage `json:"items"`
		Settings json.RawMessage   `json:"settings"`
	}
	if err = decode(raw, &req); err != nil {
		writeError(c, err)
		return
	}
	if req.Mode != "create" && req.Mode != "update" && req.Mode != "settings" && req.Mode != "enable_global" {
		writeError(c, Invalid("mode", "预览模式错误"))
		return
	}
	if (req.Mode == "create" || req.Mode == "update") && (len(req.Settings) > 0 || len(req.Items) == 0 || len(req.Items) > 100) {
		writeError(c, Invalid("items", "预览需要1–100项且不能混入全局设置"))
		return
	}
	if (req.Mode == "settings" || req.Mode == "enable_global") && (len(req.Items) > 0 || len(req.Settings) == 0) {
		writeError(c, Invalid("settings", "请提供完整全局设置"))
		return
	}
	s.readHTTP(c, func(t *Tx) (any, error) {
		all, e := t.Policies(false)
		if e != nil {
			return nil, e
		}
		proposed := append([]Policy(nil), all...)
		settings := t.Settings
		problems := []map[string]any{}
		conflicts := []map[string]any{}
		normalized := []Target{}
		problem := func(index int, err error) {
			var qe *Error
			if errors.As(err, &qe) {
				problems = append(problems, map[string]any{"code": qe.Code, "item_index": index, "field": qe.Field, "message": qe.Message})
			} else {
				problems = append(problems, map[string]any{"code": "QUALITY_PERSISTENCE_UNAVAILABLE", "item_index": index, "message": "无法核验当前范围"})
			}
		}
		switch req.Mode {
		case "create":
			payload, _ := json.Marshal(map[string]any{"items": req.Items})
			items, e := parseCreate(payload, t.Settings.DefaultPolicyDailyCallLimit)
			if e != nil {
				problem(0, e)
				break
			}
			for i, item := range items {
				if e = item.Rules.Validate(); e != nil {
					problem(i, e)
					continue
				}
				target, e := s.Resolve(c.Request.Context(), item)
				if e != nil {
					problem(i, e)
					continue
				}
				normalized = append(normalized, target)
				if !target.CanProbe || (item.Rules.ControlEnabled && !target.CanEnforce) {
					problem(i, Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "该条件不可执行或控制"))
					continue
				}
				p := Policy{ID: -int64(i) - 1, AccountID: item.AccountID, SourceGroupID: item.SourceGroupID, Condition: item.Condition, Rules: item.Rules, RoutingMode: RoutingMode}
				p.SetTarget(target)
				if item.ReplacesPolicyID > 0 {
					found := false
					for j, old := range proposed {
						if old.ID == item.ReplacesPolicyID {
							found = true
							if old.Revision != item.ReplacesExpectedRevision || old.AccountID != item.AccountID {
								problem(i, revisionConflict())
							}
							proposed = append(proposed[:j], proposed[j+1:]...)
							break
						}
					}
					if !found {
						problem(i, Fail(404, "QUALITY_POLICY_NOT_FOUND", "待替换策略不存在"))
					}
				}
				for _, old := range proposed {
					if old.AccountID == p.AccountID && old.ControlScopeHash == p.ControlScopeHash {
						conflicts = append(conflicts, map[string]any{"item_index": i, "policy_id": old.ID})
						problem(i, Fail(409, "QUALITY_SCOPE_CONFLICT", "相同有效范围已有策略"))
					}
				}
				proposed = append(proposed, p)
			}
		case "update":
			payload, _ := json.Marshal(map[string]any{"items": req.Items})
			items, e := parseUpdates(payload, 0)
			if e != nil {
				problem(0, e)
				break
			}
			seen := map[int64]bool{}
			for i, item := range items {
				if seen[item.PolicyID] {
					problem(i, Invalid("policy_id", "重复策略"))
					continue
				}
				seen[item.PolicyID] = true
				if e = item.Rules.Validate(); e != nil {
					problem(i, e)
					continue
				}
				found := false
				for j, p := range proposed {
					if p.ID != item.PolicyID {
						continue
					}
					found = true
					if p.Revision != item.ExpectedRevision {
						problem(i, revisionConflict())
						break
					}
					target, e := s.resolveTarget(c.Request.Context(), p.AccountID, p.SourceGroupID, p.Condition)
					if e != nil {
						problem(i, e)
						break
					}
					normalized = append(normalized, target)
					candidate := p
					candidate.SetTarget(target)
					if candidate.ScopeHash != p.ScopeHash {
						problem(i, Fail(409, "QUALITY_SCOPE_CONFLICT", "路由变化，请替换策略"))
						break
					}
					if item.Rules.ControlEnabled && !target.CanEnforce {
						problem(i, Fail(400, "QUALITY_UNSUPPORTED_CONDITION", "该条件不可控制"))
						break
					}
					proposed[j].Rules = item.Rules
					break
				}
				if !found {
					problem(i, Fail(404, "QUALITY_POLICY_NOT_FOUND", "策略不存在"))
				}
			}
		case "settings", "enable_global":
			value, expected, e := parseSettings(req.Settings)
			if e != nil {
				problem(0, e)
				break
			}
			if expected != t.Settings.Revision {
				problem(0, revisionConflict())
			}
			if req.Mode == "enable_global" && !value.ControlEnabled {
				problem(0, Invalid("control_enabled", "启控预览必须打开控制开关"))
			}
			settings = value
		}
		capacity := Capacity(proposed, settings)
		fresh := 0
		oldIsolated := 0
		for _, p := range all {
			p = Present(p, t.Settings, t.Now)
			if p.Eligibility == "eligible" && p.State.Freshness == "fresh" {
				fresh++
			}
			if p.State.Isolated() {
				oldIsolated++
			}
		}
		var coverage *float64
		if len(all) > 0 {
			v := 100 * float64(fresh) / float64(len(all))
			coverage = &v
		}
		return map[string]any{"valid": len(problems) == 0, "validation_errors": problems, "conflicts": conflicts, "warnings": capacity["warnings"], "capacity_preview": capacity, "daily_calls_estimate": capacity["configured_demand"], "global_budget": settings.GlobalDailyCallLimit, "estimated_detection_coverage": coverage, "normalized_conditions": normalized, "observed_risk": map[string]any{"isolated_policy_count": oldIsolated}, "activation_effect": map[string]any{"state": "untested", "immediate_quality_exclusions": 0, "reason": "启控建立新代次，只使用新证据"}, "proposed_policy_count": len(proposed), "as_of": t.Now}, nil
	})
}
