package service

import (
	"context"
	"time"
)

type OpenAISchedulerExplorationReservationOutcome string

const (
	OpenAISchedulerExplorationReservationAllowed         OpenAISchedulerExplorationReservationOutcome = "allowed"
	OpenAISchedulerExplorationReservationMinimumInterval OpenAISchedulerExplorationReservationOutcome = "minimum_interval"
	OpenAISchedulerExplorationReservationHourlyLimit     OpenAISchedulerExplorationReservationOutcome = "hourly_limit"
	OpenAISchedulerExplorationReservationDenied          OpenAISchedulerExplorationReservationOutcome = "denied"
)

type OpenAISchedulerExplorationCache interface {
	Reserve(
		context.Context,
		OpenAISchedulerHealthKey,
		time.Duration,
		int,
	) (bool, error)
}

type OpenAISchedulerDetailedExplorationCache interface {
	ReserveWithOutcome(
		context.Context,
		OpenAISchedulerHealthKey,
		time.Duration,
		int,
	) (OpenAISchedulerExplorationReservationOutcome, error)
}
