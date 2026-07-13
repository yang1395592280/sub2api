package repository

import (
	"context"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/openaischedulerhealthstate"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAISchedulerHealthRepository struct {
	client *dbent.Client
}

func NewOpenAISchedulerHealthRepository(client *dbent.Client) service.OpenAISchedulerHealthRepository {
	return &openAISchedulerHealthRepository{client: client}
}

func (r *openAISchedulerHealthRepository) GetBatch(
	ctx context.Context,
	keys []service.OpenAISchedulerHealthKey,
) (map[service.OpenAISchedulerHealthKey]service.OpenAISchedulerHealthSnapshot, error) {
	keys = normalizeUniqueOpenAIHealthKeys(keys)
	result := make(map[service.OpenAISchedulerHealthKey]service.OpenAISchedulerHealthSnapshot, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	predicates := make([]predicate.OpenAISchedulerHealthState, 0, len(keys))
	for _, key := range keys {
		predicates = append(predicates, openaischedulerhealthstate.And(
			openaischedulerhealthstate.AccountIDEQ(key.AccountID),
			openaischedulerhealthstate.ModelFamilyEQ(key.ModelFamily),
			openaischedulerhealthstate.EndpointEQ(key.Endpoint),
			openaischedulerhealthstate.TransportEQ(key.Transport),
		))
	}

	entities, err := r.client.OpenAISchedulerHealthState.Query().
		Where(openaischedulerhealthstate.Or(predicates...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, entity := range entities {
		snapshot := openAISchedulerHealthEntityToService(entity)
		result[snapshot.Key] = snapshot
	}
	return result, nil
}

func (r *openAISchedulerHealthRepository) Upsert(ctx context.Context, snapshot service.OpenAISchedulerHealthSnapshot) error {
	snapshot.Key = normalizeOpenAIHealthKey(snapshot.Key)
	upsert := r.client.OpenAISchedulerHealthState.Create().
		SetAccountID(snapshot.Key.AccountID).
		SetModelFamily(snapshot.Key.ModelFamily).
		SetEndpoint(snapshot.Key.Endpoint).
		SetTransport(snapshot.Key.Transport).
		SetState(snapshot.State).
		SetPredictedTtftMs(snapshot.PredictedTTFTMS).
		SetErrorRate(snapshot.ErrorRate).
		SetRateLimitedRate(snapshot.RateLimitedRate).
		SetServerErrorRate(snapshot.ServerErrorRate).
		SetConsecutiveSlow(snapshot.ConsecutiveSlow).
		SetConsecutiveError(snapshot.ConsecutiveError).
		SetConsecutiveSuccess(snapshot.ConsecutiveSuccess).
		SetRealSampleCount(snapshot.RealSampleCount).
		SetProbeSampleCount(snapshot.ProbeSampleCount).
		SetNillableLastRealAt(snapshot.LastRealAt).
		SetNillableLastProbeAt(snapshot.LastProbeAt).
		SetNillableCooldownUntil(snapshot.CooldownUntil).
		SetExpiresAt(snapshot.ExpiresAt).
		OnConflictColumns(
			openaischedulerhealthstate.FieldAccountID,
			openaischedulerhealthstate.FieldModelFamily,
			openaischedulerhealthstate.FieldEndpoint,
			openaischedulerhealthstate.FieldTransport,
		).
		UpdateNewValues()

	if snapshot.LastRealAt == nil || snapshot.LastProbeAt == nil || snapshot.CooldownUntil == nil {
		upsert.Update(func(update *dbent.OpenAISchedulerHealthStateUpsert) {
			if snapshot.LastRealAt == nil {
				update.ClearLastRealAt()
			}
			if snapshot.LastProbeAt == nil {
				update.ClearLastProbeAt()
			}
			if snapshot.CooldownUntil == nil {
				update.ClearCooldownUntil()
			}
		})
	}

	return upsert.Exec(ctx)
}

func normalizeUniqueOpenAIHealthKeys(keys []service.OpenAISchedulerHealthKey) []service.OpenAISchedulerHealthKey {
	unique := make([]service.OpenAISchedulerHealthKey, 0, len(keys))
	seen := make(map[service.OpenAISchedulerHealthKey]struct{}, len(keys))
	for _, key := range keys {
		key = normalizeOpenAIHealthKey(key)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	return unique
}

func normalizeOpenAIHealthKey(key service.OpenAISchedulerHealthKey) service.OpenAISchedulerHealthKey {
	key.ModelFamily = strings.ToLower(strings.TrimSpace(key.ModelFamily))
	key.Endpoint = strings.ToLower(strings.TrimSpace(key.Endpoint))
	key.Transport = strings.ToLower(strings.TrimSpace(key.Transport))
	return key
}

func openAISchedulerHealthEntityToService(entity *dbent.OpenAISchedulerHealthState) service.OpenAISchedulerHealthSnapshot {
	return service.OpenAISchedulerHealthSnapshot{
		Key: normalizeOpenAIHealthKey(service.OpenAISchedulerHealthKey{
			AccountID:   entity.AccountID,
			ModelFamily: entity.ModelFamily,
			Endpoint:    entity.Endpoint,
			Transport:   entity.Transport,
		}),
		State:              entity.State,
		PredictedTTFTMS:    entity.PredictedTtftMs,
		ErrorRate:          entity.ErrorRate,
		RateLimitedRate:    entity.RateLimitedRate,
		ServerErrorRate:    entity.ServerErrorRate,
		ConsecutiveSlow:    entity.ConsecutiveSlow,
		ConsecutiveError:   entity.ConsecutiveError,
		ConsecutiveSuccess: entity.ConsecutiveSuccess,
		RealSampleCount:    entity.RealSampleCount,
		ProbeSampleCount:   entity.ProbeSampleCount,
		LastRealAt:         entity.LastRealAt,
		LastProbeAt:        entity.LastProbeAt,
		CooldownUntil:      entity.CooldownUntil,
		ExpiresAt:          entity.ExpiresAt,
	}
}

var _ service.OpenAISchedulerHealthRepository = (*openAISchedulerHealthRepository)(nil)
