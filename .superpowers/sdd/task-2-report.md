# Task 2 Report: zenxiang_liyu Service Domain, Validation, Probability, and Simulation

## Status

DONE

## Commit

- `cbe121d61 feat: add zenxiang liyu service logic`

## Implemented

- Added `ZenxiangLiyuService` domain types, public service methods, repository port, settings/prize validation, and play command/result contracts.
- Added prize validation requiring at least one enabled prize and an enabled probability total of 100 within `0.000001`.
- Added deterministic probability selection with inclusive tier-boundary behavior.
- Added simulator support for daily play limits, minimum-balance checks, revenue/expense/profit aggregation, user outcome distribution, and prize hit/actual-rate metrics.
- Added target-profit recommendation support, including lower/higher reward interpolation and all-above/all-below fallback policies.
- Added the production `ProvideZenxiangLiyuService` Wire provider with `time.Now` and a time-seeded RNG.

## Tests

Executed from `backend`:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyuValidatePrizes' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyuRecommend' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyu|TestPickZenxiangLiyuPrize' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

All commands passed. The validation and recommendation additions were first executed in a failing state before their corresponding implementations.

## Self-review

- Confirmed no Task 2 edit touches `backend/ent`, migrations, repositories, handlers, frontend, or generated files.
- Added an exact-target duplicate-reward test after self-review exposed a zero-denominator interpolation edge case; the implementation now assigns 100% to the exact tier.
- The repository interface is intentionally limited to the service-facing contract; the transactional implementation remains owned by Task 3.

## Concerns

- Wire generation is intentionally deferred until Task 3 supplies `NewZenxiangLiyuRepository`; the registered provider currently depends on that future repository binding.

## Review Fixes (2026-07-10)

- Corrected `GetStatus` and `Play` authorization: global enable now permits all eligible users; when global enable is off, an explicit user grant is required.
- `DeletePrize` now reads the current prizes, validates the configuration after excluding the target, and refuses deletions that leave no valid enabled probability configuration.
- Protected all shared RNG reads in `Play` and `Simulate` with a service mutex. `Recommend` does not use the RNG.
- Added focused coverage for global enable, global disable with and without a grant, invalid remaining prize configuration on delete, and concurrent simulation RNG access.

### Verification

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyu(Authorization|DeletePrizeRejectsInvalidRemainingConfiguration|SimulateSupportsConcurrentRandomUse)' -count=1
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestZenxiangLiyuSimulateSupportsConcurrentRandomUse' -count=1
```

Both commands passed.

## Second Review Fixes (2026-07-10)

- Added `SavePrizes`, a full-set prize configuration replacement operation. It validates the submitted enabled probabilities as a complete 100% configuration before delegating exactly once to `ZenxiangLiyuRepository.SavePrizes`, allowing a valid 50/50 configuration to change atomically to 60/40.
- Kept `SavePrize` for individual writes; normal multi-row probability edits now have a transaction-capable bulk repository port.
- Prize validation now explicitly rejects `NaN`, positive infinity, and negative infinity reward amounts and probabilities.
- Added focused tests for atomic replacement, invalid-total rejection without repository mutation, and non-finite numeric values.

### Verification

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyu(SavePrizes|ValidatePrizesRejectsNonFiniteRewardAndProbability)' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyu|TestPickZenxiangLiyuPrize' -count=1
```

Both commands passed.
