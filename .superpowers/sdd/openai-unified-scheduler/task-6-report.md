# Task 6 Report: Coordinated OpenAI Scheduler Probes

Date: 2026-07-14
Branch: `feature/openai-unified-scheduler`
Starting HEAD: `c7b2fc8c965f2c6bc0a192a8e2dc5e629b593c9a`

## Result

- The periodic runner now waits for `interval +/- probe_jitter_seconds`, including before the first cycle, so process startups do not immediately synchronize a full probe pass.
- Every enabled-group membership is collected into a plan keyed by the complete normalized physical health key: account ID, resolved actual upstream model, actual endpoint, and actual transport.
- One upstream checker call and one unified probe-health update are performed per physical key. Group IDs are retained only to fan the result out to every legacy group score/event path.
- A fresh real sample skips the upstream probe. The unified health key lock is reused to recheck freshness after the checker returns, preventing a real sample that arrives during a probe from being overwritten in the same process.
- Each enabled cycle acquires the existing singleton leader lock using exact key `openai-auto-scheduler-probe`, a UUID owner stable for the runner lifetime, a 45-second maximum cycle context, and a two-minute lease TTL. The lease is released after workers and legacy fan-out finish.
- Redis lock errors use the existing PostgreSQL advisory-lock fallback. A peer-held lock runs no checker.
- A health freshness batch-read error preserves the old probe behavior but only after leader gating and physical-key deduplication. Unified health write failures are logged while legacy group fan-out continues.
- Stop cancels the timer/cycle context, waits for the loop, and then drains the worker pool. Start/Stop ordering is protected so WaitGroup Add/Wait cannot race.

## TDD Evidence

### Initial RED

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service \
  -run 'OpenAIAutoSchedulerProbeRunner.*(Deduplicate|Leader|Jitter|Fresh)' -count=1
```

Exit: `1`.

The compiler reported the missing health/leader/DB constructor dependencies, physical plan item and merge helper, exact leader constants, and deterministic jitter helper. This was the expected feature-missing failure.

### Initial GREEN

The same focused command exited `0`; service completed in `0.768s`.

### Review Fallback RED/GREEN

The review-driven test changed health-repository failure from fail-closed to deduplicated legacy-compatible probing across two groups.

RED:

```text
checker calls got 0, want 1
```

Command exited `1` in `0.711s`.

After substituting an empty freshness map on batch-read failure, the same test exited `0` in `0.694s`: checker count was one and both legacy groups received the result.

## Verification

Wire generation succeeded after temporarily resolving the Wire CLI-only `github.com/google/subcommands` checksum. The generated `wire_gen.go` passes the health sink, leader cache, and SQL DB to the provider. Temporary `go.mod` and `go.sum` changes were restored and are not part of the task.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./cmd/server \
  -run 'OpenAIAutoSchedulerProbeRunner|WireGen' -count=1
```

Exit: `0`; service `0.814s`, cmd/server `0.450s`.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service \
  -run 'OpenAIAutoSchedulerProbeRunner.*(Deduplicate|Leader|Jitter|Fresh)|TryAcquireSingletonLeaderLock' -count=1
```

Exit: `0`; `2.197s`.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1 -timeout 2m
GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -count=1
```

Exit: `0`; service `58.680s`, cmd/server `0.593s`.

```bash
git diff --check
```

Exit: `0`.

Independent review found no remaining Critical or Important issues and assessed the change `Approved / Ready` after the health-error fallback fix.

## Concern

The accepted v1 design serializes real/probe load-apply-upsert by health key only inside one process. A real write from another instance can still race a probe write in a short window because the repository contract has no conditional upsert/CAS. The engine design explicitly accepts this multi-instance short-window eventual consistency for v1; the Task 6 leader lease prevents duplicate probe writers but does not serialize real writers. A future repository-level conditional update can close this window without changing this task's approved contract.
