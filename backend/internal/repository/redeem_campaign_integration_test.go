//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type redeemCampaignFixture struct {
	client   *dbent.Client
	repo     *redeemCodeRepository
	redeem   *service.RedeemService
	campaign *service.RedeemCampaign
	users    []int64
	sequence int
}

func newRedeemCampaignFixture(t *testing.T, limit int) *redeemCampaignFixture {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewRedeemCodeRepository(client).(*redeemCodeRepository)
	f := &redeemCampaignFixture{client: client, repo: repo}
	f.redeem = service.NewRedeemService(repo, NewUserRepository(client, integrationDB), nil, nil, nil, client, nil, nil)
	var err error
	f.campaign, err = f.redeem.CreateCampaign(ctx, service.RedeemCampaignInput{Name: t.Name(), PerUserLimit: limit})
	require.NoError(t, err)
	for i := range 2 {
		u, err := client.User.Create().SetEmail(fmt.Sprintf("campaign-%d-%d@example.com", f.campaign.ID, i)).SetPasswordHash("test").Save(ctx)
		require.NoError(t, err)
		f.users = append(f.users, u.ID)
	}
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM redeem_codes WHERE campaign_id = $1", f.campaign.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM redeem_campaign_claims WHERE campaign_id = $1", f.campaign.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM redeem_campaigns WHERE id = $1", f.campaign.ID)
		require.NoError(t, err)
		for _, id := range f.users {
			_, err = integrationDB.ExecContext(ctx, "DELETE FROM zenxiang_liyu_ticket_batches WHERE user_id = $1", id)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, "DELETE FROM zenxiang_liyu_ticket_wallets WHERE user_id = $1", id)
			require.NoError(t, err)
			require.NoError(t, client.User.DeleteOneID(id).Exec(ctx))
		}
	})
	return f
}

func (f *redeemCampaignFixture) code(t *testing.T, codeType string, value float64) *service.RedeemCode {
	t.Helper()
	f.sequence++
	code := &service.RedeemCode{Code: fmt.Sprintf("CAMP-%d-%d", f.campaign.ID, f.sequence), Type: codeType, Value: value, CampaignID: &f.campaign.ID, Status: service.StatusUnused}
	require.NoError(t, f.redeem.CreateCode(context.Background(), code))
	return code
}

func (f *redeemCampaignFixture) claimCount(t *testing.T, userID int64) int {
	t.Helper()
	var count int
	require.NoError(t, integrationDB.QueryRow("SELECT COALESCE(MAX(redeemed_count), 0) FROM redeem_campaign_claims WHERE campaign_id=$1 AND user_id=$2", f.campaign.ID, userID).Scan(&count))
	return count
}

func TestRedeemCampaignConcurrentCodesRespectLimit(t *testing.T) {
	f := newRedeemCampaignFixture(t, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var codes []string
	for range 20 {
		codes = append(codes, f.code(t, service.RedeemTypeBalance, 1).Code)
	}
	start := make(chan struct{})
	results := make(chan error, len(codes))
	var wg sync.WaitGroup
	for _, code := range codes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			<-start
			_, err := f.redeem.Redeem(ctx, f.users[0], code)
			results <- err
		}(code)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrRedeemCampaignLimit)
		}
	}
	require.Equal(t, 3, success)
	require.Equal(t, 3, f.claimCount(t, f.users[0]))
	u, err := f.client.User.Get(ctx, f.users[0])
	require.NoError(t, err)
	require.Equal(t, float64(3), u.Balance)
	unused, err := f.client.RedeemCode.Query().Where(redeemcode.CampaignIDEQ(f.campaign.ID), redeemcode.StatusEQ(service.StatusUnused)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 17, unused)
}

func TestRedeemCampaignSharedLimitAndEditableQuota(t *testing.T) {
	f := newRedeemCampaignFixture(t, 1)
	ctx := context.Background()
	balance := f.code(t, service.RedeemTypeBalance, 2)
	ticket := f.code(t, service.RedeemTypeLotteryTicket, 2)
	_, err := f.redeem.Redeem(ctx, f.users[0], balance.Code)
	require.NoError(t, err)
	_, err = f.redeem.Redeem(ctx, f.users[0], ticket.Code)
	require.ErrorIs(t, err, service.ErrRedeemCampaignLimit)
	require.Equal(t, 1, f.claimCount(t, f.users[0]))
	got, err := f.repo.GetByID(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, got.Status)
	// Deleting the consumed code must not reset the independent claim counter.
	require.NoError(t, f.repo.Delete(ctx, balance.ID))
	_, err = f.redeem.RedeemForAdminFulfillment(ctx, f.users[0], ticket.Code)
	require.ErrorIs(t, err, service.ErrRedeemCampaignLimit)
	_, err = f.redeem.UpdateCampaign(ctx, f.campaign.ID, service.RedeemCampaignInput{Name: "提高限额", PerUserLimit: 3})
	require.NoError(t, err)
	_, err = f.redeem.Redeem(ctx, f.users[0], ticket.Code)
	require.NoError(t, err)
	third := f.code(t, service.RedeemTypeBalance, 2)
	_, err = f.redeem.Redeem(ctx, f.users[0], third.Code)
	require.NoError(t, err)
	require.Equal(t, 3, f.claimCount(t, f.users[0]))
	var tickets int
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM zenxiang_liyu_ticket_wallets WHERE user_id=$1", f.users[0]).Scan(&tickets))
	require.Equal(t, 2, tickets, "a code granting two tickets counts as one redemption")
	_, err = f.redeem.UpdateCampaign(ctx, f.campaign.ID, service.RedeemCampaignInput{Name: "降低限额", PerUserLimit: 1})
	require.NoError(t, err)
	fourth := f.code(t, service.RedeemTypeBalance, 2)
	_, err = f.redeem.Redeem(ctx, f.users[0], fourth.Code)
	require.ErrorIs(t, err, service.ErrRedeemCampaignLimit)
	_, err = f.redeem.Redeem(ctx, f.users[1], fourth.Code)
	require.NoError(t, err, "another account has an independent quota")
	other := newRedeemCampaignFixture(t, 1)
	_, err = other.redeem.Redeem(ctx, f.users[0], other.code(t, service.RedeemTypeBalance, 1).Code)
	require.NoError(t, err, "another campaign has an independent quota")
	ordinary := &service.RedeemCode{Code: fmt.Sprintf("ORDINARY-%d", f.campaign.ID), Type: service.RedeemTypeBalance, Value: 1, Status: service.StatusUnused}
	require.NoError(t, f.redeem.CreateCode(ctx, ordinary))
	t.Cleanup(func() { require.NoError(t, f.repo.Delete(ctx, ordinary.ID)) })
	_, err = f.redeem.Redeem(ctx, f.users[0], ordinary.Code)
	require.NoError(t, err, "ordinary codes remain redeemable after a giveaway limit is reached")
}

func TestRedeemCampaignFailureRollsBackQuotaAndCode(t *testing.T) {
	f := newRedeemCampaignFixture(t, 1)
	ctx := context.Background()
	_, err := integrationDB.Exec("INSERT INTO zenxiang_liyu_ticket_wallets (user_id, balance) VALUES ($1, 5)", f.users[0])
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO zenxiang_liyu_ticket_batches (user_id, source_type, source_key, granted_count, remaining_count, expires_at) VALUES ($1, 'redeem', 'initial', 5, 5, NOW() + INTERVAL '1 day')`, f.users[0])
	require.NoError(t, err)
	code := f.code(t, service.RedeemTypeLotteryTicket, 1)
	_, err = f.redeem.Redeem(ctx, f.users[0], code.Code)
	require.ErrorIs(t, err, service.ErrRedeemTicketCapacity)
	require.Zero(t, f.claimCount(t, f.users[0]))
	got, err := f.repo.GetByID(ctx, code.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, got.Status)
	_, err = f.redeem.Redeem(ctx, f.users[0], f.code(t, service.RedeemTypeBalance, 1).Code)
	require.NoError(t, err, "failed ticket delivery leaves the quota available")
}

func TestRedeemCampaignLimitUpdateWaitsForInFlightClaim(t *testing.T) {
	f := newRedeemCampaignFixture(t, 3)
	ctx := context.Background()
	require.Error(t, f.repo.ClaimCampaign(ctx, f.campaign.ID, f.users[0]), "claiming without the redemption transaction must fail closed")
	tx, err := f.client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, f.repo.ClaimCampaign(dbent.NewTxContext(ctx, tx), f.campaign.ID, f.users[0]))
	updateCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	_, err = f.redeem.UpdateCampaign(updateCtx, f.campaign.ID, service.RedeemCampaignInput{Name: "调整额度", PerUserLimit: 1})
	require.Error(t, err, "the update must wait while a claim still holds the campaign configuration lock")
	require.ErrorIs(t, updateCtx.Err(), context.DeadlineExceeded)
	require.NoError(t, tx.Commit())
	_, err = f.redeem.UpdateCampaign(ctx, f.campaign.ID, service.RedeemCampaignInput{Name: "调整额度", PerUserLimit: 1})
	require.NoError(t, err)
	_, err = f.redeem.Redeem(ctx, f.users[0], f.code(t, service.RedeemTypeBalance, 1).Code)
	require.ErrorIs(t, err, service.ErrRedeemCampaignLimit)
}

func TestRedeemCampaignHTTPAndMigration(t *testing.T) {
	f := newRedeemCampaignFixture(t, 1)
	ctx := context.Background()
	migration, err := migrations.FS.ReadFile("247_redeem_campaign_limits.sql")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, string(migration))
	require.NoError(t, err, "migration can be applied again")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.GetHeader("X-Test-User"), 10, 64)
		if id != 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		}
	})
	h := handler.NewRedeemHandler(f.redeem)
	adminService := service.NewAdminService(nil, nil, nil, nil, nil, nil, f.repo, nil, nil, nil, nil, nil, nil, f.client, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	admin := adminhandler.NewRedeemHandler(adminService, f.redeem)
	router.POST("/redeem", h.Redeem)
	router.GET("/campaigns", admin.ListCampaigns)
	router.POST("/campaigns", admin.CreateCampaign)
	router.POST("/generate", admin.Generate)
	router.PUT("/campaigns/:id", admin.UpdateCampaign)
	post := func(userID int64, code string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"code": code, "user_id": f.users[1]})
		req := httptest.NewRequest(http.MethodPost, "/redeem", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", strconv.FormatInt(userID, 10))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	code := f.code(t, service.RedeemTypeBalance, 1)
	require.Equal(t, http.StatusUnauthorized, post(0, code.Code).Code)
	w := post(f.users[0], code.Code)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "campaign_id", "campaign metadata is admin-only")
	require.Equal(t, 1, f.claimCount(t, f.users[0]), "authenticated identity wins over body user_id")
	require.Zero(t, f.claimCount(t, f.users[1]))
	second := f.code(t, service.RedeemTypeLotteryTicket, 1)
	w = post(f.users[0], second.Code)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "REDEEM_CAMPAIGN_LIMIT_REACHED")
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/campaigns/%d", f.campaign.ID), bytes.NewBufferString(`{"name":"Updated","per_user_limit":2}`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, http.StatusOK, post(f.users[0], second.Code).Code)
	got, total, err := f.repo.ListCampaigns(ctx, pagination.PaginationParams{Page: 1, PageSize: 1}, "Updated")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, got, 1)
	require.Equal(t, 2, got[0].PerUserLimit)
	for _, body := range []string{`{"name":"","per_user_limit":1}`, `{"name":"Invalid","per_user_limit":0}`, `{"name":"Invalid","per_user_limit":1.5}`} {
		request = httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, request)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(fmt.Sprintf(`{"count":2,"type":"balance","value":1,"campaign_id":%d}`, f.campaign.ID)))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var generated struct {
		Data []struct {
			CampaignID int64 `json:"campaign_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &generated))
	require.Len(t, generated.Data, 2)
	for _, code := range generated.Data {
		require.Equal(t, f.campaign.ID, code.CampaignID)
	}
	for _, body := range []string{
		fmt.Sprintf(`{"count":1,"type":"concurrency","value":1,"campaign_id":%d}`, f.campaign.ID),
		fmt.Sprintf(`{"count":1,"type":"balance","value":-1,"campaign_id":%d}`, f.campaign.ID),
	} {
		request = httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, request)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "REDEEM_CAMPAIGN_CODE_INVALID")
	}
	request = httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBufferString(`{"name":"New HTTP campaign","per_user_limit":4}`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct {
		Data service.RedeemCampaign `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, 4, created.Data.PerUserLimit)
	t.Cleanup(func() {
		_, err := integrationDB.Exec("DELETE FROM redeem_campaigns WHERE id=$1", created.Data.ID)
		require.NoError(t, err)
	})
	request = httptest.NewRequest(http.MethodGet, "/campaigns?page=1&page_size=200&search=New%20HTTP", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"page_size":100`)
	require.Contains(t, w.Body.String(), `"total":1`)
	_, err = f.redeem.GetCampaign(ctx, -1)
	require.Equal(t, "REDEEM_CAMPAIGN_NOT_FOUND", infraerrors.Reason(err))
}
