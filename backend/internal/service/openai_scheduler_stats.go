package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type OpenAISchedulerAccountDailyStat struct {
	AccountID      int64      `json:"account_id"`
	SelectCount    int64      `json:"select_count"`
	SelectRatio    float64    `json:"select_ratio"`
	LastSelectedAt *time.Time `json:"last_selected_at,omitempty"`
}

type OpenAISchedulerDailyStats struct {
	Date         string                            `json:"date"`
	GroupID      int64                             `json:"group_id"`
	TotalSelects int64                             `json:"total_selects"`
	Accounts     []OpenAISchedulerAccountDailyStat `json:"accounts"`
}

type OpenAISchedulerStatsRepository interface {
	IncrementDailySelection(ctx context.Context, statDate time.Time, groupID int64, accountID int64, selectedAt time.Time) error
	GetDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*OpenAISchedulerDailyStats, error)
	RecomputeDailyStatsFromUsageLogs(ctx context.Context, statDate time.Time, start time.Time, end time.Time, groupID int64) (*OpenAISchedulerDailyStats, error)
}

func (s *OpenAIGatewayService) recordOpenAISchedulerDailySelection(ctx context.Context, groupID *int64, accountID int64, selectedAt time.Time) {
	if s == nil || s.openAISchedulerStatsRepo == nil || groupID == nil || *groupID <= 0 || accountID <= 0 {
		return
	}
	if selectedAt.IsZero() {
		selectedAt = time.Now()
	}
	statDate := truncateOpenAISchedulerStatDate(selectedAt)
	_ = s.openAISchedulerStatsRepo.IncrementDailySelection(ctx, statDate, *groupID, accountID, selectedAt)
}

func (s *OpenAIGatewayService) GetOpenAISchedulerDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*OpenAISchedulerDailyStats, error) {
	if s == nil || s.openAISchedulerStatsRepo == nil || groupID <= 0 {
		return NewOpenAISchedulerDailyStats(statDate, groupID), nil
	}
	return s.openAISchedulerStatsRepo.GetDailyStats(ctx, truncateOpenAISchedulerStatDate(statDate), groupID)
}

func (s *OpenAIGatewayService) RecomputeOpenAISchedulerDailyStats(ctx context.Context, statDate time.Time, groupID int64) (*OpenAISchedulerDailyStats, error) {
	if groupID <= 0 {
		return NewOpenAISchedulerDailyStats(statDate, groupID), nil
	}
	statDate = truncateOpenAISchedulerStatDate(statDate)
	if s == nil || s.openAISchedulerStatsRepo == nil {
		return NewOpenAISchedulerDailyStats(statDate, groupID), nil
	}
	return s.openAISchedulerStatsRepo.RecomputeDailyStatsFromUsageLogs(ctx, statDate, statDate, statDate.Add(24*time.Hour), groupID)
}

func truncateOpenAISchedulerStatDate(t time.Time) time.Time {
	loc := timezone.Location()
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func NewOpenAISchedulerDailyStats(statDate time.Time, groupID int64) *OpenAISchedulerDailyStats {
	statDate = truncateOpenAISchedulerStatDate(statDate)
	return &OpenAISchedulerDailyStats{
		Date:    statDate.Format("2006-01-02"),
		GroupID: groupID,
	}
}
