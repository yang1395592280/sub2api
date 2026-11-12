package service

import (
	"context"
	"encoding/json"
	"errors"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyWalletUsagePolicy = "wallet_usage_policy"

// WalletUsagePolicy is global; activities cannot choose their own subsidy rate.
type WalletUsagePolicy struct {
	PaidUnits    int64 `json:"paid_units"`
	WelfareUnits int64 `json:"welfare_units"`
}

func DefaultWalletUsagePolicy() WalletUsagePolicy {
	return WalletUsagePolicy{PaidUnits: 2, WelfareUnits: 1}
}

func (p WalletUsagePolicy) Validate() error {
	if p.PaidUnits < 1 || p.PaidUnits > 10000 || p.WelfareUnits < 0 || p.WelfareUnits > 10000 {
		return infraerrors.BadRequest("WALLET_INVALID_POLICY", "充值份额须为 1–10000，福利份额须为 0–10000 的整数")
	}
	return nil
}

func ParseWalletUsagePolicy(raw string) (WalletUsagePolicy, error) {
	var p WalletUsagePolicy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func (s *CheckinService) GetWalletUsagePolicy(ctx context.Context) (WalletUsagePolicy, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyWalletUsagePolicy)
	if errors.Is(err, ErrSettingNotFound) {
		return DefaultWalletUsagePolicy(), nil
	}
	if err != nil {
		return WalletUsagePolicy{}, err
	}
	return ParseWalletUsagePolicy(raw)
}

func (s *CheckinService) UpdateWalletUsagePolicy(ctx context.Context, p WalletUsagePolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKeyWalletUsagePolicy, string(raw))
}
