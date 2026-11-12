package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Read in the billing transaction, never from a process-local settings cache.
func loadWalletUsagePolicy(ctx context.Context, q sqlExecutor) (service.WalletUsagePolicy, error) {
	rows, err := q.QueryContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.SettingKeyWalletUsagePolicy)
	if err != nil {
		return service.WalletUsagePolicy{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return service.DefaultWalletUsagePolicy(), rows.Err()
	}
	var raw string
	if err := rows.Scan(&raw); err != nil {
		return service.WalletUsagePolicy{}, err
	}
	return service.ParseWalletUsagePolicy(raw)
}

func walletPolicyNotes(policy service.WalletUsagePolicy) string {
	raw, _ := json.Marshal(policy)
	return string(raw)
}

func (r *userRepository) hydrateWalletPolicy(ctx context.Context, users ...*service.User) error {
	if len(users) == 0 {
		return nil
	}
	policy, err := loadWalletUsagePolicy(ctx, clientFromContext(ctx, r.client))
	if err != nil {
		return err
	}
	for _, u := range users {
		if u != nil {
			u.WalletUsagePolicy = &policy
		}
	}
	return nil
}
