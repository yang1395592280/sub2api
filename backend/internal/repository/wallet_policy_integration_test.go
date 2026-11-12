//go:build integration

package repository

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setTestWalletPolicy(t *testing.T, p service.WalletUsagePolicy) {
	t.Helper()
	_, err := integrationDB.Exec(`INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, service.SettingKeyWalletUsagePolicy, walletPolicyNotes(p))
	require.NoError(t, err)
}

func TestWalletIntegrationConfiguredPolicyDebtAndFrozenSnapshot(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM settings WHERE key=$1`, service.SettingKeyWalletUsagePolicy)
		require.NoError(t, err)
	})
	setTestWalletPolicy(t, service.WalletUsagePolicy{PaidUnits: 3, WelfareUnits: 1})
	id, key := walletFixture(t, service.WalletBalance{Paid: 100, Checkin: -5, Gift: 7})
	repo := NewUsageBillingRepository(testEntClient(t), integrationDB)
	user, err := NewUserRepository(testEntClient(t), integrationDB).GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(3), user.UsagePolicy().PaidUnits)
	require.Equal(t, 102., user.Wallet().AvailableWithPolicy(user.UsagePolicy()))
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repo.Apply(ctx, &service.UsageBillingCommand{UserID: id, APIKeyID: key, RequestID: fmt.Sprintf("%s-%d", uuid.NewString(), i), BalanceCost: 1})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, service.WalletBalance{Paid: 82, Checkin: -5, Gift: 5}, readTestWallet(t, id))
	result, err := repo.Apply(ctx, &service.UsageBillingCommand{UserID: id, APIKeyID: key, RequestID: uuid.NewString(), BalanceCost: 9})
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 9}, *result.WalletDebit)
	require.Equal(t, service.WalletBalance{Paid: 73, Checkin: -5, Gift: 5}, readTestWallet(t, id))
	// A later negative activity changes debt, never an ordinary usage delta.
	batch := uuid.NewString()
	imageID, imageKey := walletFixture(t, service.WalletBalance{Paid: 20, Checkin: 4, Gift: 2})
	hold := &service.BatchImageBalanceHoldCommand{UserID: imageID, APIKeyID: imageKey, BatchID: batch, RequestID: service.BatchImageHoldRequestID(batch), HoldAmount: 8}
	_, err = repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 14, Checkin: 2, Gift: 2}, readTestWallet(t, imageID))
	setTestWalletPolicy(t, service.WalletUsagePolicy{PaidUnits: 1, WelfareUnits: 0})
	capture := *hold
	capture.RequestID = service.BatchImageCaptureRequestID(batch)
	capture.ActualAmount = 4
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 17, Checkin: 3, Gift: 2}, readTestWallet(t, imageID))
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 17, Checkin: 3, Gift: 2}, readTestWallet(t, imageID))
	result, err = repo.Apply(ctx, &service.UsageBillingCommand{UserID: imageID, APIKeyID: imageKey, RequestID: uuid.NewString(), BalanceCost: 3})
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 3}, *result.WalletDebit)
	// Invalid persistent policy fails the whole transaction, without consuming the retry key.
	_, err = integrationDB.Exec(`UPDATE settings SET value='{}' WHERE key=$1`, service.SettingKeyWalletUsagePolicy)
	require.NoError(t, err)
	invalid := &service.UsageBillingCommand{UserID: imageID, APIKeyID: imageKey, RequestID: uuid.NewString(), BalanceCost: 3}
	_, err = repo.Apply(ctx, invalid)
	require.Error(t, err)
	require.Equal(t, 14., readTestWallet(t, imageID).Paid)
	setTestWalletPolicy(t, service.WalletUsagePolicy{PaidUnits: 2, WelfareUnits: 1})
	result, err = repo.Apply(ctx, invalid)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestWalletIntegrationBonusTamperingAndTokenConcurrency(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10, Checkin: -1})
	repo := NewCheckinRepository(integrationDB).(*checkinRepository)
	for _, reward := range []float64{-1, 10001, math.NaN(), math.Inf(1)} {
		_, err := repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-10", RewardAmount: reward})
		require.Error(t, err)
	}
	_, err := repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-10", RewardAmount: 1})
	require.NoError(t, err)
	_, err = repo.ApplyBonusOutcome(ctx, id, "2026-11-10", "win", 100)
	require.Error(t, err)
	require.Zero(t, readTestWallet(t, id).Checkin)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.ApplyBonusOutcome(ctx, id, "2026-11-10", "lose", -1.5)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrCheckinLuckyBonusAlreadyPlayed)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, service.WalletBalance{Paid: 10, Checkin: -1.5}, readTestWallet(t, id))
	require.NoError(t, repo.CreateCheckinPageToken(ctx, id, "concurrent-token"))
	tokens := make(chan bool, 10)
	tokenErrs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			valid, err := repo.ConsumeCheckinPageToken(ctx, id, "concurrent-token")
			tokens <- valid
			tokenErrs <- err
		}()
	}
	wg.Wait()
	close(tokens)
	close(tokenErrs)
	success = 0
	for valid := range tokens {
		if valid {
			success++
		}
	}
	for err := range tokenErrs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, success)
	require.NoError(t, repo.CreateCheckinPageToken(ctx, id, "expired-token"))
	_, err = integrationDB.Exec(`UPDATE checkin_page_tokens SET expires_at=NOW()-INTERVAL '1 second' WHERE token_hash='expired-token'`)
	require.NoError(t, err)
	valid, err := repo.ConsumeCheckinPageToken(ctx, id, "expired-token")
	require.NoError(t, err)
	require.False(t, valid)
}

func TestWalletIntegrationSmallCheckinBonusPrecision(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(testEntClient(t))
	keys := []string{service.SettingKeyCheckinEnabled, service.SettingKeyCheckinMinReward, service.SettingKeyCheckinMaxReward, service.SettingKeyCheckinLuckyBonusEnabled, service.SettingKeyCheckinLuckyBonusSuccessRate}
	t.Cleanup(func() {
		for _, key := range keys {
			require.NoError(t, settings.Delete(ctx, key))
		}
	})
	for key, value := range map[string]string{
		service.SettingKeyCheckinEnabled: "true", service.SettingKeyCheckinMinReward: "0.00000001",
		service.SettingKeyCheckinMaxReward: "0.00000001", service.SettingKeyCheckinLuckyBonusEnabled: "true",
		service.SettingKeyCheckinLuckyBonusSuccessRate: "0",
	} {
		require.NoError(t, settings.Set(ctx, key, value))
	}
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10})
	svc := service.NewCheckinService(NewCheckinRepository(integrationDB), settings, nil, nil)
	_, err := svc.Checkin(ctx, id, "Asia/Shanghai")
	require.NoError(t, err)
	record, err := svc.PlayLuckyBonus(ctx, id, "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, -0.00000002, record.BonusDeltaAmount)
	require.Equal(t, -0.00000001, record.RewardAmount)
	require.Equal(t, service.WalletBalance{Paid: 10, Checkin: -0.00000001}, readTestWallet(t, id))
}

func TestWalletIntegrationImageSnapshotOwnershipAndLegacyNotes(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Paid: 20, Checkin: 4})
	_, otherKey := walletFixture(t, service.WalletBalance{Paid: 20})
	repo := NewUsageBillingRepository(testEntClient(t), integrationDB)
	batch := uuid.NewString()
	cmd := &service.BatchImageBalanceHoldCommand{UserID: id, APIKeyID: key, BatchID: batch, RequestID: service.BatchImageHoldRequestID(batch), HoldAmount: 9}
	_, err := repo.ReserveBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	// Historical holds had empty notes and retain the default 2:1 policy.
	_, err = integrationDB.Exec(`UPDATE wallet_transactions SET notes='' WHERE user_id=$1 AND event_type='image_hold' AND operation_id=$2`, id, batch)
	require.NoError(t, err)
	capture := *cmd
	capture.RequestID, capture.ActualAmount = service.BatchImageCaptureRequestID(batch), 6
	capture.APIKeyID = otherKey
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.Error(t, err)
	require.Equal(t, service.WalletBalance{Paid: 14, Checkin: 1}, readTestWallet(t, id))
	capture.APIKeyID = key
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 16, Checkin: 2}, readTestWallet(t, id))
	// A late cancel after capture cannot refund the same hold again.
	release := *cmd
	release.RequestID = service.BatchImageReleaseRequestID(batch)
	_, err = repo.ReleaseBatchImageBalance(ctx, &release)
	require.Error(t, err)
	require.Equal(t, service.WalletBalance{Paid: 16, Checkin: 2}, readTestWallet(t, id))
	var frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT frozen_balance FROM users WHERE id=$1`, id).Scan(&frozen))
	require.Zero(t, frozen)
}
