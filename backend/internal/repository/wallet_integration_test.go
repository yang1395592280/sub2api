//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func walletFixture(t *testing.T, w service.WalletBalance) (int64, int64) {
	t.Helper()
	u := mustCreateUser(t, testEntClient(t), &service.User{Email: uuid.NewString() + "@wallet.test", PasswordHash: "hash", Balance: w.Paid, CheckinBalance: w.Checkin, LiyuBalance: w.Liyu, GiftBalance: w.Gift})
	key := mustCreateApiKey(t, testEntClient(t), &service.APIKey{UserID: u.ID, Key: "sk-" + uuid.NewString(), Name: "wallet"})
	return u.ID, key.ID
}
func readTestWallet(t *testing.T, id int64) service.WalletBalance {
	t.Helper()
	var w service.WalletBalance
	require.NoError(t, integrationDB.QueryRow(`SELECT balance,checkin_balance,liyu_balance,gift_balance FROM users WHERE id=$1`, id).Scan(&w.Paid, &w.Checkin, &w.Liyu, &w.Gift))
	return w
}
func TestWalletIntegrationUsageConcurrencyAndNegativeCredit(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Paid: 100, Checkin: 3, Liyu: -5, Gift: 3})
	repo := NewUsageBillingRepository(testEntClient(t), integrationDB)
	cmd := &service.UsageBillingCommand{UserID: id, APIKeyID: key, RequestID: uuid.NewString(), BalanceCost: 9}
	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 8, Checkin: 1}, *result.WalletDebit)
	repeat, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, repeat.Applied)
	require.Equal(t, service.WalletBalance{Paid: 92, Checkin: 2, Liyu: -5, Gift: 3}, readTestWallet(t, id))
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repo.Apply(ctx, &service.UsageBillingCommand{UserID: id, APIKeyID: key, RequestID: fmt.Sprintf("%s-%d", cmd.RequestID, i), BalanceCost: .3})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, service.WalletBalance{Paid: 86, Checkin: 2, Liyu: -5, Gift: 3}, readTestWallet(t, id))
	var paid, gift float64
	require.NoError(t, integrationDB.QueryRow(`SELECT -SUM(paid_delta),-SUM(gift_delta) FROM wallet_transactions WHERE user_id=$1 AND event_type='usage'`, id).Scan(&paid, &gift))
	require.Equal(t, 14., paid)
	require.Equal(t, 0., gift)
}
func TestWalletIntegrationCheckinLossAndRecovery(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10, Liyu: 4, Gift: 5})
	repo := NewCheckinRepository(integrationDB)
	_, err := repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-01", RewardAmount: 1})
	require.NoError(t, err)
	_, err = repo.ApplyBonusOutcome(ctx, id, "2026-11-01", "lose", -1.5)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 10, Checkin: -.5, Liyu: 4, Gift: 5}, readTestWallet(t, id))
	_, err = repo.ApplyBonusOutcome(ctx, id, "2026-11-01", "lose", -1.5)
	require.ErrorIs(t, err, service.ErrCheckinLuckyBonusAlreadyPlayed)
	_, err = repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-02", RewardAmount: .3})
	require.NoError(t, err)
	require.Equal(t, -.2, readTestWallet(t, id).Checkin)
	_, err = repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-03", RewardAmount: .3})
	require.NoError(t, err)
	require.Equal(t, .1, readTestWallet(t, id).Checkin)
	// Same date from concurrent requests credits once.
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-04", RewardAmount: 1})
			out <- err
		}()
	}
	wg.Wait()
	close(out)
	successes := 0
	for err := range out {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrCheckinAlreadyToday)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1.1, readTestWallet(t, id).Checkin)
}
func TestWalletIntegrationRiskAndFrozenRefund(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Paid: 20, Checkin: 4, Liyu: 2, Gift: 1})
	repo := NewUsageBillingRepository(testEntClient(t), integrationDB)
	batch := uuid.NewString()
	hold := &service.BatchImageBalanceHoldCommand{UserID: id, APIKeyID: key, BatchID: batch, RequestID: service.BatchImageHoldRequestID(batch), HoldAmount: 9}
	_, err := repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 14, Checkin: 1, Liyu: 2, Gift: 1}, readTestWallet(t, id))
	riskRepo := NewCheckinRepository(integrationDB).(service.CheckinRiskRepository)
	c, err := riskRepo.ConfirmCheckinRisk(ctx, id, 0, "已核实脚本", "多日验证绕过与自动表单证据")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(72*time.Hour), c.ObserveUntil, time.Minute)
	ids, err := riskRepo.ProcessDueCheckinRisks(ctx)
	require.NoError(t, err)
	require.NotContains(t, ids, id)
	_, err = integrationDB.Exec(`UPDATE checkin_risk_cases SET observe_until=NOW()-INTERVAL '1 second' WHERE id=$1`, c.ID)
	require.NoError(t, err)
	_, err = riskRepo.ProcessDueCheckinRisks(ctx)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 14, Liyu: 2, Gift: 1}, readTestWallet(t, id))
	release := *hold
	release.RequestID = service.BatchImageReleaseRequestID(batch)
	_, err = repo.ReleaseBatchImageBalance(ctx, &release)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 20, Liyu: 2, Gift: 1}, readTestWallet(t, id))
	_, err = repo.ReleaseBatchImageBalance(ctx, &release)
	require.NoError(t, err)
	c, err = riskRepo.GetCheckinRisk(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 4., c.ClearedAmount)
	_, err = NewCheckinRepository(integrationDB).CreateAndCredit(ctx, &service.CheckinRecord{UserID: id, CheckinDate: "2026-11-05", RewardAmount: 1})
	require.ErrorIs(t, err, service.ErrCheckinSuspended)
	_, err = riskRepo.ResolveCheckinRisk(ctx, c.ID, 0, "restore")
	require.NoError(t, err)
	_, err = riskRepo.ResolveCheckinRisk(ctx, c.ID, 0, "restore")
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 20, Checkin: 4, Liyu: 2, Gift: 1}, readTestWallet(t, id))
}
func TestWalletIntegrationImageCaptureReconcilesLedger(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Paid: 20, Checkin: 2, Gift: 2})
	repo := NewUsageBillingRepository(testEntClient(t), integrationDB)
	batch := uuid.NewString()
	cmd := &service.BatchImageBalanceHoldCommand{UserID: id, APIKeyID: key, BatchID: batch, RequestID: service.BatchImageHoldRequestID(batch), HoldAmount: 9}
	_, err := repo.ReserveBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	cmd.RequestID = service.BatchImageCaptureRequestID(batch)
	cmd.ActualAmount = 6
	_, err = repo.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 16, Gift: 2}, readTestWallet(t, id))
	var paid, checkin float64
	require.NoError(t, integrationDB.QueryRow(`SELECT SUM(paid_delta),SUM(checkin_delta) FROM wallet_transactions WHERE user_id=$1`, id).Scan(&paid, &checkin))
	require.Equal(t, -4., paid)
	require.Equal(t, -2., checkin)
	// Capture and cancellation cannot both settle one hold.
	cmd.RequestID = service.BatchImageReleaseRequestID(batch)
	_, err = repo.ReleaseBatchImageBalance(ctx, cmd)
	require.Error(t, err)
	require.Equal(t, service.WalletBalance{Paid: 16, Gift: 2}, readTestWallet(t, id))
}

func TestWalletIntegrationWelfareAloneCannotFundImageHold(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Checkin: 20, Liyu: -1, Gift: 5})
	batch := uuid.NewString()
	_, err := NewUsageBillingRepository(testEntClient(t), integrationDB).ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		UserID: id, APIKeyID: key, BatchID: batch, RequestID: service.BatchImageHoldRequestID(batch), HoldAmount: 1,
	})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.Equal(t, service.WalletBalance{Checkin: 20, Liyu: -1, Gift: 5}, readTestWallet(t, id))
}

func TestWalletIntegrationAdminGiftIdempotencyAndHistory(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10, Checkin: -1})
	repo := NewUserRepository(testEntClient(t), integrationDB).(*userRepository)
	applied, err := repo.CreditWallet(ctx, id, 5, 2, 0, "赠送福利", "admin-request-1")
	require.NoError(t, err)
	require.True(t, applied)
	applied, err = repo.CreditWallet(ctx, id, 5, 2, 0, "赠送福利", "admin-request-1")
	require.NoError(t, err)
	require.False(t, applied)
	_, err = repo.CreditWallet(ctx, id, 5, 3, 0, "赠送福利", "admin-request-1")
	require.Error(t, err)
	require.Equal(t, service.WalletBalance{Paid: 15, Checkin: -1, Gift: 2}, readTestWallet(t, id))
	entries, total, recharged, err := repo.ListWalletBalanceHistory(ctx, id, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, 5., recharged)
	require.Contains(t, entries[0].SourceDetail, "赠送 +2.00")
	paidTotal, err := NewRedeemCodeRepository(testEntClient(t)).SumPositiveBalanceByUser(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 5., paidTotal)
	u, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 2., u.GiftBalance)
	require.Equal(t, -1., u.CheckinBalance)
	require.Equal(t, 5., u.TotalRecharged, "only paid credit counts toward cumulative recharge, once")
}

func TestWalletIntegrationLiyuLossOnlyDebitsLiyu(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10, Checkin: 2, Liyu: .25, Gift: 3})
	_, err := integrationDB.Exec(`UPDATE zenxiang_liyu_settings SET lucky_coin_enabled=TRUE,lucky_coin_double_probability=0 WHERE id=1`)
	require.NoError(t, err)
	var recordID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO zenxiang_liyu_records
		(request_id,user_id,play_date,ticket_amount,reward_amount,user_net_amount,system_revenue,system_expense,system_profit,
		 prize_name_snapshot,probability_snapshot,balance_before,balance_after_ticket,balance_after_reward)
		VALUES($1,$2,CURRENT_DATE,1,1,1,0,1,-1,'wallet test',100,0,0,1) RETURNING id`, uuid.NewString(), id).Scan(&recordID))
	repo := NewZenxiangLiyuRepository(testEntClient(t), integrationDB)
	result, err := repo.PlayLuckyCoin(ctx, service.ZenxiangLiyuLuckyCoinCommand{UserID: id, RecordID: recordID, Roll: 99})
	require.NoError(t, err)
	require.Equal(t, -1.75, result.BalanceAfter)
	require.Equal(t, service.WalletBalance{Paid: 10, Checkin: 2, Liyu: -1.75, Gift: 3}, readTestWallet(t, id))
	_, err = repo.PlayLuckyCoin(ctx, service.ZenxiangLiyuLuckyCoinCommand{UserID: id, RecordID: recordID, Roll: 99})
	require.ErrorIs(t, err, service.ErrZenxiangLiyuLuckyCoinAlreadyPlayed)
	require.Equal(t, -1.75, readTestWallet(t, id).Liyu)
}
func TestWalletIntegrationUsageBreakdownQueries(t *testing.T) {
	ctx := context.Background()
	id, key := walletFixture(t, service.WalletBalance{Paid: 20, Checkin: 3})
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: uuid.NewString(), Type: service.AccountTypeAPIKey})
	repo := NewUsageLogRepository(testEntClient(t), integrationDB)
	requestID := uuid.NewString()
	log := &service.UsageLog{UserID: id, APIKeyID: key, AccountID: account.ID, RequestID: requestID, Model: "wallet-test", TotalCost: 9, ActualCost: 9, CreatedAt: time.Now()}
	_, err := repo.Create(ctx, log)
	require.NoError(t, err)
	_, err = NewUsageBillingRepository(testEntClient(t), integrationDB).Apply(ctx, &service.UsageBillingCommand{UserID: id, APIKeyID: key, RequestID: requestID, BalanceCost: 9})
	require.NoError(t, err)
	read, err := repo.GetByID(ctx, log.ID)
	require.NoError(t, err)
	require.NotNil(t, read.WalletUsage)
	require.Equal(t, 6., read.WalletUsage.PaidCost)
	require.Equal(t, 3., read.WalletUsage.WelfareCost)
	stats, err := repo.GetStatsWithFilters(ctx, UsageLogFilters{UserID: id})
	require.NoError(t, err)
	require.Equal(t, 6., stats.WalletPaidCost)
	require.Equal(t, 3., stats.WalletWelfareCost)
	require.Zero(t, stats.WalletUnclassifiedCost)
	old := &service.UsageLog{UserID: id, APIKeyID: key, AccountID: account.ID, RequestID: uuid.NewString(), Model: "wallet-test", TotalCost: 1, ActualCost: 1, CreatedAt: time.Now()}
	_, err = repo.Create(ctx, old)
	require.NoError(t, err)
	oldRead, err := repo.GetByID(ctx, old.ID)
	require.NoError(t, err)
	require.Nil(t, oldRead.WalletUsage)
	stats, err = repo.GetStatsWithFilters(ctx, UsageLogFilters{UserID: id})
	require.NoError(t, err)
	require.Equal(t, 1., stats.WalletUnclassifiedCost)
}
func TestWalletIntegrationSecurityEvidenceAndTokenOwnership(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 1, Checkin: -.5})
	other, _ := walletFixture(t, service.WalletBalance{})
	repo := NewCheckinRepository(integrationDB).(service.CheckinRiskRepository)
	require.NoError(t, repo.CreateCheckinPageToken(ctx, id, "token-hash"))
	valid, err := repo.ConsumeCheckinPageToken(ctx, other, "token-hash")
	require.NoError(t, err)
	require.False(t, valid)
	valid, err = repo.ConsumeCheckinPageToken(ctx, id, "token-hash")
	require.NoError(t, err)
	require.True(t, valid)
	valid, err = repo.ConsumeCheckinPageToken(ctx, id, "token-hash")
	require.NoError(t, err)
	require.False(t, valid)
	for day := 1; day <= 3; day++ {
		_, err = integrationDB.Exec(`INSERT INTO checkin_security_events(user_id,event_type,created_at) VALUES($1,'page_bypass',NOW()-$2*INTERVAL '1 day')`, id, day)
		require.NoError(t, err)
	}
	require.NoError(t, repo.RecordCheckinSecurityEvent(ctx, id, "page_bypass", "127.0.0.1", "test"))
	c, err := repo.GetCheckinRisk(ctx, id)
	require.NoError(t, err)
	require.Nil(t, c, "one weak signal alone must not confiscate credit")
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.RecordCheckinSecurityEvent(ctx, id, "honeypot", "127.0.0.1", "test"))
		require.NoError(t, repo.RecordCheckinSecurityEvent(ctx, id, "captcha_failure", "127.0.0.1", "test"))
	}
	c, err = repo.GetCheckinRisk(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, "observing", c.Status)
	_, err = integrationDB.Exec(`UPDATE checkin_risk_cases SET observe_until=NOW()-INTERVAL '1 second' WHERE id=$1`, c.ID)
	require.NoError(t, err)
	_, err = repo.ProcessDueCheckinRisks(ctx)
	require.NoError(t, err)
	require.Equal(t, service.WalletBalance{Paid: 1, Checkin: -.5}, readTestWallet(t, id), "clearing must retain negative check-in debt and paid credit")
}

func TestWalletIntegrationRiskSweepContinuesPastDeletedUser(t *testing.T) {
	ctx := context.Background()
	deletedID, _ := walletFixture(t, service.WalletBalance{Checkin: 1})
	activeID, _ := walletFixture(t, service.WalletBalance{Paid: 10, Checkin: 2})
	repo := NewCheckinRepository(integrationDB).(service.CheckinRiskRepository)
	deletedCase, err := repo.ConfirmCheckinRisk(ctx, deletedID, 0, "已核实脚本", "测试证据")
	require.NoError(t, err)
	activeCase, err := repo.ConfirmCheckinRisk(ctx, activeID, 0, "已核实脚本", "测试证据")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE checkin_risk_cases SET observe_until=NOW()-INTERVAL '1 second' WHERE id IN ($1,$2)`, deletedCase.ID, activeCase.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=$1`, deletedID)
	require.NoError(t, err)
	_, err = repo.ProcessDueCheckinRisks(ctx)
	require.NoError(t, err)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM checkin_risk_cases WHERE id=$1`, deletedCase.ID).Scan(&status))
	require.Equal(t, "revoked", status)
	c, err := repo.GetCheckinRisk(ctx, activeID)
	require.NoError(t, err)
	require.Equal(t, "cleared", c.Status)
	require.Equal(t, service.WalletBalance{Paid: 10}, readTestWallet(t, activeID))
}

func TestWalletIntegrationAdminRefundConcurrencyAndRollback(t *testing.T) {
	ctx := context.Background()
	id, _ := walletFixture(t, service.WalletBalance{Paid: 10, Checkin: -2, Liyu: 3, Gift: 4})
	repo := NewUserRepository(testEntClient(t), integrationDB).(*userRepository)
	require.NoError(t, repo.RefundWallet(ctx, id, 1, 2, "liyu", 0, "mixed", "first"))
	require.Equal(t, service.WalletBalance{Paid: 9, Checkin: -2, Liyu: 1, Gift: 4}, readTestWallet(t, id))
	require.NoError(t, repo.RefundWallet(ctx, id, 1, 2, "liyu", 0, "mixed", "first"))
	require.ErrorIs(t, repo.RefundWallet(ctx, id, 1, 2, "gift", 0, "mixed", "first"), service.ErrIdempotencyKeyConflict)
	require.Error(t, repo.RefundWallet(ctx, id, 1, 2, "liyu", 0, "too much", "fail"))
	require.Equal(t, service.WalletBalance{Paid: 9, Checkin: -2, Liyu: 1, Gift: 4}, readTestWallet(t, id))
	// A ledger insert failure must roll back the already issued balance UPDATE.
	_, err := integrationDB.Exec("ALTER TABLE wallet_transactions ADD CONSTRAINT refund_test_notes CHECK (notes <> 'force_refund_failure')")
	require.NoError(t, err)
	err = repo.RefundWallet(ctx, id, 1, 1, "gift", 0, "force_refund_failure", "rollback")
	_, dropErr := integrationDB.Exec("ALTER TABLE wallet_transactions DROP CONSTRAINT refund_test_notes")
	require.NoError(t, dropErr)
	require.Error(t, err)
	require.Equal(t, service.WalletBalance{Paid: 9, Checkin: -2, Liyu: 1, Gift: 4}, readTestWallet(t, id))
	// Concurrent identical operations charge once even after the selected source is exhausted.
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); out <- repo.RefundWallet(ctx, id, 0, 4, "gift", 0, "same", "same") }()
	}
	wg.Wait()
	close(out)
	for err := range out {
		require.NoError(t, err)
	}
	require.Equal(t, service.WalletBalance{Paid: 9, Checkin: -2, Liyu: 1}, readTestWallet(t, id))
	// Distinct concurrent requests compete for the locked source; it cannot go negative.
	out = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out <- repo.RefundWallet(ctx, id, 1, 1, "liyu", 0, "compete", fmt.Sprintf("compete-%d", i))
		}(i)
	}
	wg.Wait()
	close(out)
	success := 0
	for err := range out {
		if err == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, service.WalletBalance{Paid: 8, Checkin: -2}, readTestWallet(t, id))
	items, total, _, err := repo.ListWalletBalanceHistory(ctx, id, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	for _, item := range items {
		require.Equal(t, "管理员余额退款", item.SourceLabel)
	}
	var recharged float64
	require.NoError(t, integrationDB.QueryRow("SELECT total_recharged FROM users WHERE id=$1", id).Scan(&recharged))
	require.Zero(t, recharged)
}
