package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminCheckinRepoStub struct {
	items     []service.AdminCheckinRecord
	total     int64
	lastQuery struct {
		page      int
		pageSize  int
		search    string
		date      string
		sortBy    string
		sortOrder string
	}
}

func (s *adminCheckinRepoStub) HasCheckedInOnDate(_ context.Context, _ int64, _ string) (bool, error) {
	panic("unexpected HasCheckedInOnDate call")
}

func (s *adminCheckinRepoStub) CreateAndCredit(_ context.Context, _ *service.CheckinRecord) (*service.CheckinRecord, error) {
	panic("unexpected CreateAndCredit call")
}

func (s *adminCheckinRepoStub) ListByUserAndDateRange(_ context.Context, _ int64, _, _ string) ([]service.CheckinRecord, error) {
	panic("unexpected ListByUserAndDateRange call")
}

func (s *adminCheckinRepoStub) GetUserTotals(_ context.Context, _ int64) (int64, float64, error) {
	panic("unexpected GetUserTotals call")
}

func (s *adminCheckinRepoStub) ListAdminRecords(_ context.Context, page, pageSize int, search, date, sortBy, sortOrder string) ([]service.AdminCheckinRecord, int64, error) {
	s.lastQuery.page = page
	s.lastQuery.pageSize = pageSize
	s.lastQuery.search = search
	s.lastQuery.date = date
	s.lastQuery.sortBy = sortBy
	s.lastQuery.sortOrder = sortOrder
	return s.items, s.total, nil
}

func TestAdminCheckinHandlerListPassesFiltersAndReturnsData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &adminCheckinRepoStub{
		items: []service.AdminCheckinRecord{
			{UserID: 1, UserEmail: "alice@example.com", UserName: "alice", CheckinDate: "2026-04-21", RewardAmount: 18.5},
		},
		total: 1,
	}
	checkinService := service.NewCheckinService(repo, nil, nil, nil)
	handler := NewCheckinHandler(checkinService)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/checkins?page=1&page_size=20&search=alice&date=2026-04-21&sort_by=reward_amount&sort_order=asc", nil)

	handler.List(c)

	require.Equal(t, 1, repo.lastQuery.page)
	require.Equal(t, 20, repo.lastQuery.pageSize)
	require.Equal(t, "alice", repo.lastQuery.search)
	require.Equal(t, "2026-04-21", repo.lastQuery.date)
	require.Equal(t, "reward_amount", repo.lastQuery.sortBy)
	require.Equal(t, "asc", repo.lastQuery.sortOrder)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []service.AdminCheckinRecord `json:"items"`
			Total int64                        `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, int64(1), resp.Data.Total)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, "alice@example.com", resp.Data.Items[0].UserEmail)
}
