//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gamePointsFixture struct {
	userID int64
	repo   *redeemCodeRepository
	svc    *service.RedeemService
}

func newGamePointsFixture(t *testing.T) *gamePointsFixture {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	u, err := client.User.Create().SetEmail(fmt.Sprintf("points-%d@example.com", time.Now().UnixNano())).SetPasswordHash("test").SetBalance(7).SetCheckinBalance(2).SetLiyuBalance(3).SetGiftBalance(4).Save(ctx)
	require.NoError(t, err)
	repo := NewRedeemCodeRepository(client).(*redeemCodeRepository)
	f := &gamePointsFixture{userID: u.ID, repo: repo}
	f.svc = service.NewRedeemService(repo, NewUserRepository(client, integrationDB), nil, nil, nil, client, nil, nil)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM game_points_ledger WHERE user_id=$1`, u.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM redeem_codes WHERE used_by=$1 OR notes=$2`, u.ID, fmt.Sprintf("points-fixture-%d", u.ID))
		require.NoError(t, err)
		require.NoError(t, client.User.DeleteOneID(u.ID).Exec(ctx))
	})
	return f
}

func (f *gamePointsFixture) code(t *testing.T, value float64) *service.RedeemCode {
	t.Helper()
	text, err := service.GenerateRedeemCode()
	require.NoError(t, err)
	code := &service.RedeemCode{Code: text, Type: service.RedeemTypeGamePoints, Value: value, Status: service.StatusUnused, Notes: fmt.Sprintf("points-fixture-%d", f.userID)}
	require.NoError(t, f.svc.CreateCode(context.Background(), code))
	return code
}

func (f *gamePointsFixture) assertAssets(t *testing.T, want int64, ledgerCount int) {
	t.Helper()
	var points int64
	var paid, checkin, liyu, gift float64
	require.NoError(t, integrationDB.QueryRow(`SELECT points, balance, checkin_balance, liyu_balance, gift_balance FROM users WHERE id=$1`, f.userID).Scan(&points, &paid, &checkin, &liyu, &gift))
	require.Equal(t, want, points)
	require.Equal(t, []float64{7, 2, 3, 4}, []float64{paid, checkin, liyu, gift})
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM game_points_ledger WHERE user_id=$1`, f.userID).Scan(&count))
	require.Equal(t, ledgerCount, count)
}

func TestGamePointsRedeemHTTPAndHistory(t *testing.T) {
	f := newGamePointsFixture(t)
	code := f.code(t, 250)
	h := handler.NewRedeemHandler(f.svc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: f.userID})
	})
	router.POST("/redeem", h.Redeem)
	router.GET("/redeem/history", h.GetHistory)
	body, err := json.Marshal(map[string]string{"code": code.Code})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/redeem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var response struct {
		Data struct {
			Type        string `json:"type"`
			PointsAdded int64  `json:"points_added"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, service.RedeemTypeGamePoints, response.Data.Type)
	require.Equal(t, int64(250), response.Data.PointsAdded)
	f.assertAssets(t, 250, 1)
	var before, after, delta int64
	var kind, reason string
	require.NoError(t, integrationDB.QueryRow(`SELECT points_before, points_after, delta_points, entry_type, reason FROM game_points_ledger WHERE user_id=$1`, f.userID).Scan(&before, &after, &delta, &kind, &reason))
	require.Equal(t, []int64{0, 250, 250}, []int64{before, after, delta})
	require.Equal(t, "redeem_code", kind)
	require.Contains(t, reason, code.Code)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/redeem/history?page=1&page_size=20", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"type":"game_points"`)
	require.NotContains(t, w.Body.String(), "points-fixture")
	_, err = f.svc.Redeem(context.Background(), f.userID, code.Code)
	require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
	f.assertAssets(t, 250, 1)
}

func TestGamePointsRedeemConcurrentSameAndDifferentCodes(t *testing.T) {
	f := newGamePointsFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	code := f.code(t, 20)
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.svc.Redeem(ctx, f.userID, code.Code); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
		}
	}
	require.Equal(t, 1, successes)
	f.assertAssets(t, 20, 1)
	results = make(chan error, 6)
	for range 6 {
		code := f.code(t, 30)
		wg.Add(1)
		go func(text string) { defer wg.Done(); _, err := f.svc.Redeem(ctx, f.userID, text); results <- err }(code.Code)
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	f.assertAssets(t, 200, 7)
}

type failingPointsGrantRepo struct{ *redeemCodeRepository }

func (r *failingPointsGrantRepo) GrantRedeemPoints(ctx context.Context, userID int64, sourceKey string, points int64) error {
	if err := r.redeemCodeRepository.GrantRedeemPoints(ctx, userID, sourceKey, points); err != nil {
		return err
	}
	return errors.New("injected failure after ledger insert")
}

func TestGamePointsRedeemRollsBackAllWritesAndAllowsRetry(t *testing.T) {
	f := newGamePointsFixture(t)
	code := f.code(t, 100)
	client := testEntClient(t)
	svc := service.NewRedeemService(&failingPointsGrantRepo{f.repo}, NewUserRepository(client, integrationDB), nil, nil, nil, client, nil, nil)
	_, err := svc.Redeem(context.Background(), f.userID, code.Code)
	require.ErrorContains(t, err, "injected failure")
	f.assertAssets(t, 0, 0)
	got, err := f.repo.GetByID(context.Background(), code.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, got.Status)
	require.Nil(t, got.UsedBy)
	_, err = f.svc.Redeem(context.Background(), f.userID, code.Code)
	require.NoError(t, err)
	f.assertAssets(t, 100, 1)
}

func TestGamePointsRedeemOverflowAndExpiryKeepCodeUnused(t *testing.T) {
	f := newGamePointsFixture(t)
	code := f.code(t, 100)
	_, err := integrationDB.Exec(`UPDATE users SET points=$2 WHERE id=$1`, f.userID, int64(math.MaxInt64-10))
	require.NoError(t, err)
	_, err = f.svc.Redeem(context.Background(), f.userID, code.Code)
	require.ErrorContains(t, err, "overflow")
	f.assertAssets(t, math.MaxInt64-10, 0)
	got, err := f.repo.GetByID(context.Background(), code.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, got.Status)
	_, err = integrationDB.Exec(`UPDATE redeem_codes SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, code.ID)
	require.NoError(t, err)
	_, err = f.svc.Redeem(context.Background(), f.userID, code.Code)
	require.ErrorIs(t, err, service.ErrRedeemCodeExpired)
}

func TestGameWelfareAssetsAndUserSort(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	var ids []int64
	for i, balances := range [][4]float64{{900, 0.1, 0.2, 0.3}, {1, 2, -1, 0}, {0, 0, 0, 1}} {
		u, err := client.User.Create().SetEmail(fmt.Sprintf("welfare-sort-%d@example.com", i)).SetPasswordHash("test").SetBalance(balances[0]).SetCheckinBalance(balances[1]).SetLiyuBalance(balances[2]).SetGiftBalance(balances[3]).Save(txCtx)
		require.NoError(t, err)
		ids = append(ids, u.ID)
	}
	repo := newUserRepositoryWithSQL(client, integrationDB)
	for _, order := range []string{"asc", "desc"} {
		users, _, err := repo.ListWithFilters(txCtx, pagination.PaginationParams{Page: 1, PageSize: 10, SortBy: "welfare_balance", SortOrder: order}, service.UserListFilters{Search: "welfare-sort-"})
		require.NoError(t, err)
		require.Len(t, users, 3)
		want := ids
		if order == "desc" {
			want = []int64{ids[2], ids[1], ids[0]}
		}
		require.Equal(t, want, []int64{users[0].ID, users[1].ID, users[2].ID})
	}
	// The game wallet counts only spendable positive buckets. A debt in one
	// bucket must not hide the amount the exchange repository can actually debit.
	require.NoError(t, tx.Commit())
	t.Cleanup(func() {
		for _, id := range ids {
			_, err := integrationDB.Exec(`DELETE FROM users WHERE id=$1`, id)
			require.NoError(t, err)
		}
	})
	assets, err := NewGameCenterRepository(client, integrationDB).GetUserAssets(ctx, ids[1])
	require.NoError(t, err)
	require.Equal(t, float64(2), assets.WelfareBalance)
}
