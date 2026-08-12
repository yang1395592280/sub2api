package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbaccountgroup "github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/openaiautoschedulerscoreevent"
	"github.com/Wei-Shaw/sub2api/ent/openaiautoschedulerscorestate"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"

	entsql "entgo.io/ent/dialect/sql"
)

const (
	openAIAutoSchedulerDefaultPageSize = 50
	openAIAutoSchedulerMaxPageSize     = 200
	openAIAutoSchedulerMaxMessageLen   = 1000
)

type openAIAutoSchedulerRepository struct {
	client *dbent.Client
}

func NewOpenAIAutoSchedulerRepository(client *dbent.Client) service.OpenAIAutoSchedulerRepository {
	return &openAIAutoSchedulerRepository{client: client}
}

func (r *openAIAutoSchedulerRepository) GetGroup(ctx context.Context, groupID int64) (*service.Group, error) {
	g, err := r.client.Group.Query().
		Where(group.IDEQ(groupID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return groupEntityToService(g), nil
}

func (r *openAIAutoSchedulerRepository) GetScoreState(ctx context.Context, accountID, groupID int64, model string) (*service.OpenAIAutoSchedulerScoreState, error) {
	state, err := r.client.OpenAIAutoSchedulerScoreState.Query().
		Where(
			openaiautoschedulerscorestate.AccountIDEQ(accountID),
			openaiautoschedulerscorestate.GroupIDEQ(groupID),
			openaiautoschedulerscorestate.ModelEQ(strings.TrimSpace(model)),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	out := openAIAutoSchedulerScoreStateEntityToService(state)
	return &out, nil
}

func (r *openAIAutoSchedulerRepository) HasOpenCircuitScoreState(ctx context.Context, accountID, groupID int64, model string) (bool, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return false, nil
	}
	now := time.Now()
	return r.client.OpenAIAutoSchedulerScoreState.Query().
		Where(
			openaiautoschedulerscorestate.AccountIDEQ(accountID),
			openaiautoschedulerscorestate.GroupIDEQ(groupID),
			openaiautoschedulerscorestate.ModelEQ(model),
			openaiautoschedulerscorestate.StateEQ(service.OpenAIAutoSchedulerStateOpen),
			openaiautoschedulerscorestate.Or(
				openaiautoschedulerscorestate.CooldownUntilIsNil(),
				openaiautoschedulerscorestate.CooldownUntilGT(now),
			),
		).
		Exist(ctx)
}

func (r *openAIAutoSchedulerRepository) ListScoreStatesForSummary(ctx context.Context, groupID int64, model string) ([]service.OpenAIAutoSchedulerScoreState, error) {
	model = strings.TrimSpace(model)
	if groupID <= 0 || model == "" {
		return nil, nil
	}
	entities, err := r.client.OpenAIAutoSchedulerScoreState.Query().
		Where(
			openaiautoschedulerscorestate.GroupIDEQ(groupID),
			openaiautoschedulerscorestate.ModelEQ(model),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.OpenAIAutoSchedulerScoreState, 0, len(entities))
	for _, entity := range entities {
		out = append(out, openAIAutoSchedulerScoreStateEntityToService(entity))
	}
	return out, nil
}

func (r *openAIAutoSchedulerRepository) UpsertScoreState(ctx context.Context, state service.OpenAIAutoSchedulerScoreState) error {
	model := strings.TrimSpace(state.Model)
	return r.client.OpenAIAutoSchedulerScoreState.Create().
		SetAccountID(state.AccountID).
		SetGroupID(state.GroupID).
		SetModel(model).
		SetFinalScore(clampOpenAIAutoSchedulerBasisPoints(state.FinalScore)).
		SetBaseScore(clampOpenAIAutoSchedulerBasisPoints(state.BaseScore)).
		SetLatencyScore(clampOpenAIAutoSchedulerSignedComponentScore(state.LatencyScore)).
		SetErrorScore(clampOpenAIAutoSchedulerSignedComponentScore(state.ErrorScore)).
		SetRecoveryScore(clampOpenAIAutoSchedulerSignedComponentScore(state.RecoveryScore)).
		SetCostScore(clampOpenAIAutoSchedulerSignedComponentScore(state.CostScore)).
		SetState(state.State).
		SetConsecutiveSlowCount(state.ConsecutiveSlowCount).
		SetConsecutiveErrorCount(state.ConsecutiveErrorCount).
		SetConsecutiveSuccessCount(state.ConsecutiveSuccessCount).
		SetRequestCount(state.RequestCount).
		SetTtfbSampleCount(state.TtfbSampleCount).
		SetSlowRate(state.SlowRate).
		SetErrorRate(state.ErrorRate).
		SetStuckRate(state.StuckRate).
		SetNillableCooldownUntil(state.CooldownUntil).
		SetNillableLastLatencyMs(state.LastLatencyMS).
		SetNillableLastTtfbMs(state.LastTtfbMS).
		SetNillableLastStatusCode(state.LastStatusCode).
		SetNillableLastError(state.LastError).
		SetReason(state.Reason).
		SetNillableLastCheckedAt(state.LastCheckedAt).
		OnConflictColumns(
			openaiautoschedulerscorestate.FieldAccountID,
			openaiautoschedulerscorestate.FieldGroupID,
			openaiautoschedulerscorestate.FieldModel,
		).
		UpdateNewValues().
		Exec(ctx)
}

func (r *openAIAutoSchedulerRepository) InsertScoreEvent(ctx context.Context, event service.OpenAIAutoSchedulerScoreEvent) error {
	builder := r.client.OpenAIAutoSchedulerScoreEvent.Create().
		SetAccountID(event.AccountID).
		SetGroupID(event.GroupID).
		SetModel(strings.TrimSpace(event.Model)).
		SetEventType(event.EventType).
		SetScoreBefore(clampOpenAIAutoSchedulerBasisPoints(event.ScoreBefore)).
		SetScoreAfter(clampOpenAIAutoSchedulerBasisPoints(event.ScoreAfter)).
		SetNillableLatencyMs(event.LatencyMS).
		SetNillableTtfbMs(event.TtfbMS).
		SetNillableStatusCode(event.StatusCode).
		SetMessage(truncateOpenAIAutoSchedulerMessage(event.Message))
	if !event.CreatedAt.IsZero() {
		builder.SetCreatedAt(event.CreatedAt)
	}
	return builder.Exec(ctx)
}

func (r *openAIAutoSchedulerRepository) ListScoreStates(ctx context.Context, params service.OpenAIAutoSchedulerListParams) ([]service.OpenAIAutoSchedulerScoreState, int64, error) {
	page, pageSize := normalizeOpenAIAutoSchedulerPage(params.Page, params.PageSize)
	query := r.client.OpenAIAutoSchedulerScoreState.Query()
	if params.AccountID > 0 {
		query = query.Where(openaiautoschedulerscorestate.AccountIDEQ(params.AccountID))
	}
	if params.GroupID > 0 {
		query = query.Where(openaiautoschedulerscorestate.GroupIDEQ(params.GroupID))
	}
	if strings.TrimSpace(params.Model) != "" {
		query = query.Where(openaiautoschedulerscorestate.ModelEQ(strings.TrimSpace(params.Model)))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := query.
		Order(
			openaiautoschedulerscorestate.ByFinalScore(entsql.OrderDesc()),
			openaiautoschedulerscorestate.ByUpdatedAt(entsql.OrderDesc()),
		).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]service.OpenAIAutoSchedulerScoreState, 0, len(entities))
	for _, entity := range entities {
		out = append(out, openAIAutoSchedulerScoreStateEntityToService(entity))
	}
	if err := r.fillOpenAIAutoSchedulerScoreAccountNames(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, int64(total), nil
}

func (r *openAIAutoSchedulerRepository) fillOpenAIAutoSchedulerScoreAccountNames(ctx context.Context, states []service.OpenAIAutoSchedulerScoreState) error {
	if len(states) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(states))
	seen := make(map[int64]struct{}, len(states))
	for _, state := range states {
		if state.AccountID <= 0 {
			continue
		}
		if _, ok := seen[state.AccountID]; ok {
			continue
		}
		seen[state.AccountID] = struct{}{}
		ids = append(ids, state.AccountID)
	}
	if len(ids) == 0 {
		return nil
	}
	accounts, err := r.client.Account.Query().
		Where(dbaccount.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return err
	}
	names := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		names[account.ID] = account.Name
	}
	for i := range states {
		if name := strings.TrimSpace(names[states[i].AccountID]); name != "" {
			states[i].AccountName = name
		}
	}
	return nil
}

func (r *openAIAutoSchedulerRepository) ListSchedulableOpenAIAccountsByGroup(ctx context.Context, groupID int64) ([]service.Account, error) {
	now := time.Now()
	joins, err := r.client.AccountGroup.Query().
		Where(
			dbaccountgroup.GroupIDEQ(groupID),
			dbaccountgroup.HasAccountWith(
				dbaccount.PlatformEQ(service.PlatformOpenAI),
				dbaccount.StatusEQ(service.StatusActive),
				dbaccount.SchedulableEQ(true),
				dbaccount.DeletedAtIsNil(),
				tempUnschedulablePredicate(),
				notExpiredPredicate(now),
				dbaccount.Or(dbaccount.OverloadUntilIsNil(), dbaccount.OverloadUntilLTE(now)),
				dbaccount.Or(dbaccount.RateLimitResetAtIsNil(), dbaccount.RateLimitResetAtLTE(now)),
			),
		).
		Order(
			dbaccountgroup.ByPriority(),
			dbaccountgroup.ByAccountField(dbaccount.FieldPriority),
		).
		WithAccount().
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.Account, 0, len(joins))
	seen := make(map[int64]struct{}, len(joins))
	for _, join := range joins {
		if join.Edges.Account == nil {
			continue
		}
		if _, ok := seen[join.AccountID]; ok {
			continue
		}
		seen[join.AccountID] = struct{}{}
		out = append(out, *accountEntityToService(join.Edges.Account))
	}
	return out, nil
}

func (r *openAIAutoSchedulerRepository) ListScoreEvents(ctx context.Context, params service.OpenAIAutoSchedulerListParams) ([]service.OpenAIAutoSchedulerScoreEvent, int64, error) {
	page, pageSize := normalizeOpenAIAutoSchedulerPage(params.Page, params.PageSize)
	query := r.client.OpenAIAutoSchedulerScoreEvent.Query()
	if params.AccountID > 0 {
		query = query.Where(openaiautoschedulerscoreevent.AccountIDEQ(params.AccountID))
	}
	if params.GroupID > 0 {
		query = query.Where(openaiautoschedulerscoreevent.GroupIDEQ(params.GroupID))
	}
	if strings.TrimSpace(params.Model) != "" {
		query = query.Where(openaiautoschedulerscoreevent.ModelEQ(strings.TrimSpace(params.Model)))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := query.
		Order(openaiautoschedulerscoreevent.ByCreatedAt(entsql.OrderDesc())).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]service.OpenAIAutoSchedulerScoreEvent, 0, len(entities))
	for _, entity := range entities {
		out = append(out, service.OpenAIAutoSchedulerScoreEvent{
			AccountID:   entity.AccountID,
			GroupID:     entity.GroupID,
			Model:       strings.TrimSpace(entity.Model),
			EventType:   entity.EventType,
			ScoreBefore: entity.ScoreBefore,
			ScoreAfter:  entity.ScoreAfter,
			LatencyMS:   entity.LatencyMs,
			TtfbMS:      entity.TtfbMs,
			StatusCode:  entity.StatusCode,
			Message:     entity.Message,
			CreatedAt:   entity.CreatedAt,
		})
	}
	return out, int64(total), nil
}

func (r *openAIAutoSchedulerRepository) ListAccountReliability(ctx context.Context, accountIDs []int64, since time.Time) (map[int64]service.OpenAIAutoSchedulerAccountReliability, error) {
	out := make(map[int64]service.OpenAIAutoSchedulerAccountReliability, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	driver, ok := r.client.Driver().(*entsql.Driver)
	if !ok {
		return out, nil
	}
	rows, err := driver.DB().QueryContext(ctx, `
		SELECT account_id, COUNT(*),
		       COUNT(*) FILTER (WHERE event_type IN ('success', 'probe_success')),
		       COUNT(*) FILTER (WHERE event_type IN ('slow', 'severe_slow')),
		       COUNT(*) FILTER (WHERE event_type IN ('error', 'request_error', 'rate_limited', 'probe_error')),
		       COUNT(DISTINCT (created_at AT TIME ZONE 'UTC')::date),
		       AVG(NULLIF(COALESCE(ttfb_ms, latency_ms), 0)), MAX(created_at)
		FROM openai_auto_scheduler_score_events
		WHERE account_id = ANY($1) AND created_at >= $2
		GROUP BY account_id`, pq.Array(accountIDs), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID int64
		var item service.OpenAIAutoSchedulerAccountReliability
		var avg sql.NullFloat64
		var last sql.NullTime
		if err := rows.Scan(&accountID, &item.SampleCount, &item.SuccessCount, &item.SlowCount, &item.ErrorCount, &item.ActiveDays, &avg, &last); err != nil {
			return nil, err
		}
		if avg.Valid {
			item.AvgTTFBMS = &avg.Float64
		}
		if last.Valid {
			item.LastEventAt = &last.Time
		}
		item.Recommendation = accountReliabilityRecommendation(item)
		out[accountID] = item
	}
	return out, rows.Err()
}

func accountReliabilityRecommendation(item service.OpenAIAutoSchedulerAccountReliability) string {
	if item.SampleCount < 5 {
		return "insufficient_data"
	}
	successRate := float64(item.SuccessCount) / float64(item.SampleCount)
	slowRate := float64(item.SlowCount) / float64(item.SampleCount)
	avg := float64(0)
	if item.AvgTTFBMS != nil {
		avg = *item.AvgTTFBMS
	}
	if successRate >= 0.97 && slowRate <= 0.08 && avg <= 2500 {
		return "stable"
	}
	if successRate >= 0.90 && slowRate <= 0.20 && avg <= 5000 {
		return "observe"
	}
	return "avoid"
}

func (r *openAIAutoSchedulerRepository) ListScoreDailySamples(ctx context.Context, params service.OpenAIAutoSchedulerListParams, since time.Time) (map[int64]service.OpenAIAutoSchedulerDailySample, error) {
	query := r.client.OpenAIAutoSchedulerScoreEvent.Query().
		Where(openaiautoschedulerscoreevent.CreatedAtGTE(since))
	if params.AccountID > 0 {
		query = query.Where(openaiautoschedulerscoreevent.AccountIDEQ(params.AccountID))
	}
	if params.GroupID > 0 {
		query = query.Where(openaiautoschedulerscoreevent.GroupIDEQ(params.GroupID))
	}
	if strings.TrimSpace(params.Model) != "" {
		query = query.Where(openaiautoschedulerscoreevent.ModelEQ(strings.TrimSpace(params.Model)))
	}

	entities, err := query.
		Order(openaiautoschedulerscoreevent.ByCreatedAt(entsql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]service.OpenAIAutoSchedulerDailySample)
	for _, entity := range entities {
		if !isOpenAIAutoSchedulerDailySampleEvent(entity.EventType) {
			continue
		}
		sample := out[entity.AccountID]
		sample.AccountID = entity.AccountID
		sample.RequestCount++
		if entity.TtfbMs != nil && *entity.TtfbMs > 0 {
			sample.TtfbSampleCount++
			if sample.LastTtfbMS == nil {
				sample.LastTtfbMS = entity.TtfbMs
			}
		}
		out[entity.AccountID] = sample
	}
	return out, nil
}

func (r *openAIAutoSchedulerRepository) ListEnabledOpenAIGroups(ctx context.Context) ([]service.Group, error) {
	entities, err := r.client.Group.Query().
		Where(
			group.PlatformEQ(service.PlatformOpenAI),
			group.StatusEQ(service.StatusActive),
			group.OpenaiAutoSchedulerEnabledEQ(true),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.Group, 0, len(entities))
	for _, entity := range entities {
		out = append(out, *groupEntityToService(entity))
	}
	return out, nil
}

func isOpenAIAutoSchedulerDailySampleEvent(eventType string) bool {
	switch eventType {
	case service.OpenAIAutoSchedulerEventSuccess,
		service.OpenAIAutoSchedulerEventSlow,
		service.OpenAIAutoSchedulerEventSevereSlow,
		service.OpenAIAutoSchedulerEventError,
		service.OpenAIAutoSchedulerEventRateLimited,
		service.OpenAIAutoSchedulerEventProbeSuccess,
		service.OpenAIAutoSchedulerEventProbeError:
		return true
	default:
		return false
	}
}

func openAIAutoSchedulerScoreStateEntityToService(entity *dbent.OpenAIAutoSchedulerScoreState) service.OpenAIAutoSchedulerScoreState {
	return service.OpenAIAutoSchedulerScoreState{
		AccountID:               entity.AccountID,
		GroupID:                 entity.GroupID,
		Model:                   strings.TrimSpace(entity.Model),
		BaseScore:               entity.BaseScore,
		FinalScore:              entity.FinalScore,
		LatencyScore:            entity.LatencyScore,
		ErrorScore:              entity.ErrorScore,
		RecoveryScore:           entity.RecoveryScore,
		CostScore:               entity.CostScore,
		State:                   entity.State,
		ConsecutiveSlowCount:    entity.ConsecutiveSlowCount,
		ConsecutiveErrorCount:   entity.ConsecutiveErrorCount,
		ConsecutiveSuccessCount: entity.ConsecutiveSuccessCount,
		RequestCount:            entity.RequestCount,
		TtfbSampleCount:         entity.TtfbSampleCount,
		SlowRate:                entity.SlowRate,
		ErrorRate:               entity.ErrorRate,
		StuckRate:               entity.StuckRate,
		CooldownUntil:           entity.CooldownUntil,
		LastLatencyMS:           entity.LastLatencyMs,
		LastTtfbMS:              entity.LastTtfbMs,
		LastStatusCode:          entity.LastStatusCode,
		LastError:               entity.LastError,
		Reason:                  entity.Reason,
		LastCheckedAt:           entity.LastCheckedAt,
	}
}

func normalizeOpenAIAutoSchedulerPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = openAIAutoSchedulerDefaultPageSize
	}
	if pageSize > openAIAutoSchedulerMaxPageSize {
		pageSize = openAIAutoSchedulerMaxPageSize
	}
	return page, pageSize
}

func truncateOpenAIAutoSchedulerMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= openAIAutoSchedulerMaxMessageLen {
		return message
	}
	message = message[:openAIAutoSchedulerMaxMessageLen]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}
	return message
}

func clampOpenAIAutoSchedulerBasisPoints(score int) int {
	if score < 0 {
		return 0
	}
	if score > 10000 {
		return 10000
	}
	return score
}

func clampOpenAIAutoSchedulerSignedComponentScore(score int) int {
	if score < -10000 {
		return -10000
	}
	if score > 10000 {
		return 10000
	}
	return score
}
