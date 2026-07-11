package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeOpenAIOverbrushThreshold(threshold int) int {
	if threshold < 1 {
		return 1
	}
	if threshold > 100 {
		return 100
	}
	return threshold
}

// GetOpenAIOverbrushSettings 获取 OpenAI OAuth 超刷配置。
func (s *SettingService) GetOpenAIOverbrushSettings(ctx context.Context) (*OpenAIOverbrushSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIOverbrushSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultOpenAIOverbrushSettings(), nil
		}
		return nil, fmt.Errorf("get openai overbrush settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultOpenAIOverbrushSettings(), nil
	}

	var settings OpenAIOverbrushSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultOpenAIOverbrushSettings(), nil
	}
	settings.Consecutive429Threshold = normalizeOpenAIOverbrushThreshold(settings.Consecutive429Threshold)
	return &settings, nil
}

// SetOpenAIOverbrushSettings 设置 OpenAI OAuth 超刷配置。
func (s *SettingService) SetOpenAIOverbrushSettings(ctx context.Context, settings *OpenAIOverbrushSettings) error {
	if settings == nil {
		settings = DefaultOpenAIOverbrushSettings()
	}
	if settings.Consecutive429Threshold < 1 || settings.Consecutive429Threshold > 100 {
		return infraerrors.BadRequest("OPENAI_OVERBRUSH_THRESHOLD_INVALID", "consecutive_429_threshold must be between 1-100")
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal openai overbrush settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyOpenAIOverbrushSettings, string(data))
}
