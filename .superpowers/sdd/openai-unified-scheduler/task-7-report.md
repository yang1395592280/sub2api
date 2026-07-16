# Task 7 Report: Deduplicate Group Price Refresh Runs

## Status

DONE

## Implementation

- Builds one due-cycle plan keyed by physical account ID and preserves first-seen account/group order.
- Refreshes each physical account once, then applies every due membership's price guard to the same refreshed snapshot.
- Preserves each group's configured interval and existing list-error/refresh-error behavior; panicked groups/accounts remain retryable.
- Coordinates the complete list/refresh/guard cycle under `group-upstream-balance-refresh` using the shared Redis lock with PostgreSQL advisory fallback and a stable runner owner.
- Bounds a cycle to 30 minutes with a 35-minute lease, so the lease covers refresh and guard work.
- Replaces the fixed scan ticker with a deterministic-in-tests +/-10% jittered timer.
- Makes `Stop` cancel in-flight refresh work and serializes Start/Stop lifecycle transitions.
- Wires `LeaderLockCache` and `*sql.DB` into the production provider; `wire_gen.go` was regenerated, not edited manually.

## TDD Evidence

- RED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestGroupUpstreamBalanceRefreshRunner_RefreshesSharedAccountOnceAndFansOutPriceGuards -count=1`
  - Expected `[42]`, actual `[42, 42]`.
- GREEN: shared account refreshed once and group 10/group 20 price guards both recorded.
- RED: leader test failed to compile because the coordinated constructor and singleton key did not exist; GREEN after runner-level leader acquisition/release.
- RED: jitter test failed to compile because `nextGroupUpstreamBalanceRefreshDelay` did not exist; GREEN at 54s/60s/66s boundaries.
- RED: list-account panic escaped and Stop did not cancel an in-flight refresh; GREEN after per-group recovery and parent-context cancellation.
- RED: provider rejected lock/DB arguments and Wire source still contained the old call; GREEN after provider update and Wire generation.

## Verification

- Focused: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'GroupUpstreamBalanceRefreshRunner|UpstreamPriceGuard|TestProvideGroupUpstreamBalanceRefreshRunner' -count=1`
- Race: `GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'GroupUpstreamBalanceRefreshRunner|UpstreamPriceGuard|TestProvideGroupUpstreamBalanceRefreshRunner' -count=1`
- Wire: `GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -run TestWireGenInjectsGroupUpstreamBalanceRefreshRunnerIntoStartupAndCleanup -count=1`
- Full service: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1 -timeout=2m`
- Full server: `GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -count=1`
- Diff: `git diff --check`
- Dependency files: `git diff --exit-code -- backend/go.mod backend/go.sum`

All commands completed successfully. The first Wire generation attempt was blocked by the sandboxed Go checksum cache; the approved `GOFLAGS=-mod=mod` generation succeeded, and its temporary `github.com/google/subcommands` checksum lines were removed so `go.mod`/`go.sum` remain unchanged.

## Concerns

- Per-group last-run timestamps remain in process memory, matching the existing runner behavior. The leader lease prevents concurrent multi-instance cycles, but does not persist group timestamps across restarts.
- The 30-minute cycle timeout is intentionally below the 35-minute lease. Very large deployments that cannot finish all sequential 15-second refreshes inside that window will retry unfinished due groups on a later cycle.

## Reviewer Fix Wave

- I1: Reloads the real account temp-unschedulable state with `GetByID` after each group guard terminal. This prevents a later membership from clearing a newer group's reason and handles conditional `SetTempUnschedulable` zero-row success without assuming the requested state was stored.
- I2: Narrows panic recovery to each price-guard membership. A panicked group remains retryable while later memberships for the same refreshed account continue; upstream refresh remains once per physical account.
- I3: Tracks pending and failed memberships per group. Empty/list-error groups and groups whose memberships reach terminal state commit immediately; context cancellation preserves already completed groups and leaves unfinished groups due.
- I4: Persists UTC Unix last-run values under `group-upstream-balance-refresh:last-run:<groupID>`. Each leader cycle loads all group keys with one `GetMultiple`; completion updates local state immediately and best-effort writes the distributed state. Read failures fall back to local state, while write failures retain local anti-repeat behavior without pretending another runner can observe the commit.
- Minor: Added an explicit TTL > max-cycle assertion and concurrent Start/Stop race coverage.

### Fix TDD Evidence

- I1 RED: shared snapshot caused group 20 to clear group 10's newly stored reason; zero-row conditional Set left the snapshot falsely empty. Both are GREEN after real-state reload.
- I2 RED: group 10 guard panic aborted group 20, leaving zero guard-history entries. GREEN with group 20 executed and only group 10 due next cycle.
- I3 RED: cancellation after group 10 completed retried both groups (`[10,20]`) instead of only group 20. GREEN with immediate membership completion.
- I4 RED: coordinated state constructor/key were absent. GREEN with two alternating runners issuing one physical refresh across a 600-second interval.

### Fix Verification

- Focused runner/compat/provider: PASS (`2.819s`).
- Focused price guard: PASS (`2.161s`).
- Focused Wire: PASS (`1.739s`).
- Focused race: PASS (`3.234s`).
- Full service: PASS (`58.429s`).
- Full cmd/server: PASS (`0.621s`).
- Wire generator completed successfully; temporary generator-only checksum changes were removed and `go.mod`/`go.sum` have no diff.

## Final Reviewer Fix

- Treats `GetByID` error, nil, or panic after a price-guard membership as an untrusted guard state.
- Marks the current and remaining memberships for that physical account failed and due, then stops only that account's fanout so no stale snapshot reaches later groups.
- Continues processing independent account plans in the same cycle.

### Final Fix TDD Evidence

- RED was captured before the paused implementation: reload error/nil allowed later memberships to consume stale state, and reload panic escaped the membership boundary.
- GREEN: `UntrustedReloadStopsOnlyCurrentAccountFanout/{error,nil,panic}` and `ReloadPanicIsContainedByMembership` all pass.

### Final Fix Verification

- Targeted untrusted reload and panic: PASS (`0.730s`).
- Focused runner/provider: PASS (`2.879s`).
- Focused price guard: PASS (`2.196s`).
- Focused upstream compatibility: PASS (`1.467s`).
- Focused Wire: PASS (`0.475s`).
- Focused race: PASS (`2.192s`).
- Full service: PASS (`58.343s`).
- Full cmd/server: PASS (`0.459s`).
- Wire generator completed successfully with no `wire_gen.go` diff; temporary generator-only checksum changes were removed.
