package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

const SettingKeyZenxiangLiyuEnabled = "zenxiang_liyu_enabled"

// Only owner-confirmed custom switches belong here. Other settings remain on
// their original pages; the Liyu default preserves existing upgrade behavior.
var customFeatureDefaults = map[string]bool{
	SettingKeyZenxiangLiyuEnabled:       true,
	SettingKeyRedeemCodePurchaseEnabled: false,
	SettingKeyJoinGroupEnabled:          false,
}

type CustomFeatureMenu struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}
type CustomFeatureSettings struct {
	Flags map[string]bool     `json:"flags"`
	Menus []CustomFeatureMenu `json:"menus"`
}

func CustomFeatureKeys() []string {
	keys := make([]string, 0, len(customFeatureDefaults))
	for k := range customFeatureDefaults {
		keys = append(keys, k)
	}
	return keys
}
func (s *SettingService) CustomFeatureEnabled(ctx context.Context, key string) bool {
	defaultValue, supported := customFeatureDefaults[key]
	if !supported {
		return false
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{key})
	if err != nil {
		return false
	}
	if v, ok := values[key]; ok && v != "" {
		return v == "true"
	}
	return defaultValue
}
func (s *SettingService) GetCustomFeatures(ctx context.Context) (*CustomFeatureSettings, error) {
	keys := append(CustomFeatureKeys(), SettingKeyCustomMenuItems)
	values, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := &CustomFeatureSettings{Flags: map[string]bool{}, Menus: []CustomFeatureMenu{}}
	for key, defaultValue := range customFeatureDefaults {
		v, ok := values[key]
		out.Flags[key] = defaultValue
		if ok && v != "" {
			out.Flags[key] = v == "true"
		}
	}
	var menus []struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(values[SettingKeyCustomMenuItems]), &menus); err == nil {
		for _, m := range menus {
			out.Menus = append(out.Menus, CustomFeatureMenu{ID: m.ID, Label: m.Label, Enabled: m.Enabled == nil || *m.Enabled})
		}
	}
	return out, nil
}
func (s *SettingService) PatchCustomFeatures(ctx context.Context, flags, menus map[string]bool) error {
	updates := map[string]string{}
	for key, value := range flags {
		if _, ok := customFeatureDefaults[key]; !ok {
			return fmt.Errorf("unsupported feature: %s", key)
		}
		updates[key] = strconv.FormatBool(value)
	}
	if len(menus) > 0 {
		raw, err := s.settingRepo.GetValue(ctx, SettingKeyCustomMenuItems)
		if err != nil {
			return err
		}
		var items []map[string]json.RawMessage
		if err = json.Unmarshal([]byte(raw), &items); err != nil {
			return err
		}
		found := map[string]bool{}
		for _, item := range items {
			var id string
			_ = json.Unmarshal(item["id"], &id)
			if value, ok := menus[id]; ok {
				item["enabled"] = json.RawMessage(strconv.FormatBool(value))
				found[id] = true
			}
		}
		if len(found) != len(menus) {
			return fmt.Errorf("unknown custom menu")
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			return err
		}
		updates[SettingKeyCustomMenuItems] = string(encoded)
	}
	if len(updates) == 0 {
		return nil
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	s.refreshCachedSettingsAfterWrite(ctx, nil, OmittedSettingKeys{"partial": {}})
	return nil
}
