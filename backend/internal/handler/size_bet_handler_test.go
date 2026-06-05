package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type sizeBetHandlerStub struct {
	listRecentRoundsCalls int
	listRoundsCalls       int
	lastRecentLimit       int
}

func (s *sizeBetHandlerStub) GetCurrentRoundView(context.Context, int64, time.Time) (*service.SizeBetCurrentRoundView, error) {
	panic("unexpected GetCurrentRoundView call")
}

func (s *sizeBetHandlerStub) PlaceBet(context.Context, service.PlaceSizeBetRequest) (*service.SizeBet, error) {
	panic("unexpected PlaceBet call")
}

func (s *sizeBetHandlerStub) GetHistory(context.Context, int64, pagination.PaginationParams) ([]service.SizeBetUserHistoryItem, *pagination.PaginationResult, error) {
	panic("unexpected GetHistory call")
}

func (s *sizeBetHandlerStub) ListRecentRounds(_ context.Context, limit int) ([]service.SizeBetRound, error) {
	s.listRecentRoundsCalls++
	s.lastRecentLimit = limit
	return []service.SizeBetRound{
		{ID: 1, RoundNo: 1001, Status: service.SizeBetRoundStatusSettled, StartsAt: time.Now(), SettlesAt: time.Now()},
	}, nil
}

func (s *sizeBetHandlerStub) ListRounds(context.Context, pagination.PaginationParams) ([]service.SizeBetRound, *pagination.PaginationResult, error) {
	s.listRoundsCalls++
	panic("ListRounds should not be used by user-facing recent rounds handler")
}

func (s *sizeBetHandlerStub) GetStatsOverview(context.Context, string) (*service.SizeBetStatsOverview, error) {
	panic("unexpected GetStatsOverview call")
}

func (s *sizeBetHandlerStub) ListStatsUsers(context.Context, string, pagination.PaginationParams) ([]service.SizeBetStatsUserItem, *pagination.PaginationResult, error) {
	panic("unexpected ListStatsUsers call")
}

func (s *sizeBetHandlerStub) GetLeaderboard(context.Context, string, time.Time) (*service.SizeBetLeaderboardView, error) {
	panic("unexpected GetLeaderboard call")
}

func (s *sizeBetHandlerStub) GetRules(context.Context, time.Time) (*service.SizeBetRulesView, error) {
	panic("unexpected GetRules call")
}

func TestFormatTimeZero(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Fatalf("expected empty string")
	}
}

func TestSizeBetHandlerListRecentRoundsUsesRuntimeRecentRoundsService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &sizeBetHandlerStub{}
	h := &SizeBetHandler{service: stub}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/game/size-bet/rounds?page=1&page_size=5", nil)

	h.ListRecentRounds(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stub.listRecentRoundsCalls != 1 {
		t.Fatalf("expected ListRecentRounds to be called once, got %d", stub.listRecentRoundsCalls)
	}
	if stub.lastRecentLimit != 5 {
		t.Fatalf("expected ListRecentRounds limit=5, got %d", stub.lastRecentLimit)
	}
	if stub.listRoundsCalls != 0 {
		t.Fatalf("expected ListRounds not to be called, got %d", stub.listRoundsCalls)
	}
}

func TestSizeBetHandlerListRecentRoundsNormalizesRuntimePaginationMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &sizeBetHandlerStub{}
	h := &SizeBetHandler{service: stub}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/game/size-bet/rounds?page=2&page_size=3", nil)

	h.ListRecentRounds(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"page":1`) {
		t.Fatalf("expected runtime recent rounds endpoint to normalize page metadata to 1, got body: %s", body)
	}
	if !strings.Contains(body, `"page_size":3`) {
		t.Fatalf("expected runtime recent rounds endpoint to preserve requested page_size, got body: %s", body)
	}
}
