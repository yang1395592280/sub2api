package repository

import (
	"context"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/openaiautoschedulerscorestate"
	"github.com/Wei-Shaw/sub2api/internal/service"

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

func (r *openAIAutoSchedulerRepository) UpsertScoreState(ctx context.Context, state service.OpenAIAutoSchedulerScoreState) error {
	model := strings.TrimSpace(state.Model)
	return r.client.OpenAIAutoSchedulerScoreState.Create().
		SetAccountID(state.AccountID).
		SetGroupID(state.GroupID).
		SetModel(model).
		SetFinalScore(clampOpenAIAutoSchedulerBasisPoints(state.FinalScore)).
		SetBaseScore(clampOpenAIAutoSchedulerBasisPoints(state.BaseScore)).
		SetLatencyScore(clampOpenAIAutoSchedulerBasisPoints(state.LatencyScore)).
		SetErrorScore(clampOpenAIAutoSchedulerBasisPoints(state.ErrorScore)).
		SetRecoveryScore(clampOpenAIAutoSchedulerBasisPoints(state.RecoveryScore)).
		SetCostScore(clampOpenAIAutoSchedulerBasisPoints(state.CostScore)).
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
	return out, int64(total), nil
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
	return message[:openAIAutoSchedulerMaxMessageLen]
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
