# Task 5 Report: Runtime Settings and Shadow Mode

Date: 2026-07-14
Branch: `feature/openai-unified-scheduler`
Starting HEAD: `057443590b1a523929862f705cdefe6d376b10b4`

## Result

- Added the additive runtime settings contract: `mode`, `shadow_mode`, `top_k`, `exploration_rate`, `session_escape_min_gap_ms`, `session_escape_ratio`, `health_ttl_seconds`, `real_sample_fresh_seconds`, and `probe_jitter_seconds`.
- `mode` accepts `legacy` and `balanced`; the default is `balanced`.
- Old persisted JSON without `shadow_mode` defaults to `true`, so existing installations keep the legacy order during rollout.
- Explicit `shadow_mode=false` survives decode, normalization, and persistence. Only `balanced` plus `shadow_mode=false` applies the balanced order.
- `legacy` skips balanced health loading and ordering. Shadow mode computes the balanced decision, logs the comparison, clears balanced-only circuit rejection from the returned result, and returns the complete supplied legacy order. Live mode returns the balanced order.
- Ordinary sticky shadow requests run the same pure balanced comparison without acquiring another slot or changing the selected legacy account. Runtime escape thresholds, including explicit zero values, feed both sticky and load-order policy paths.
- Runtime settings use a scheduler-local five-second cache with atomic reads and mutex-protected refresh, matching the existing advanced scheduler TTL pattern and avoiding a settings DB query per request.
- Runtime `health_ttl_seconds` and `real_sample_fresh_seconds` feed the existing unified health settings. `probe_jitter_seconds` is exposed and validated for Task 6 consumption.
- Frontend changes are limited to backward-compatible optional API type additions; no UI was implemented.

## TDD Evidence

### RED

Command:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin \
  -run 'OpenAIAutoSchedulerSettings|BalancedDefaults|Shadow|BalancedRanges|OldPayload|LegacyMode' -count=1
```

Exit: `1`

Expected failure: both packages failed to compile because `Mode`, `ShadowMode`, `TopK`, `ExplorationRate`, escape, TTL, freshness, and jitter fields did not exist. This established RED before production implementation.

### GREEN

Command:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin \
  -run 'OpenAIAutoSchedulerSettings|BalancedDefaults|Shadow|BalancedRanges|OldPayload|LegacyMode' -count=1
```

Exit: `0`

Result: service `1.113s`; handler/admin `0.446s`.

Additional runtime adapter focused test:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service \
  -run 'RuntimeSettingsMapBalancedControls|ShadowKeepsLegacyPlan|OpenAIAutoSchedulerSettings|OpenAIBalancedScheduler.*(Shadow|Live|Legacy)' -count=1
```

Exit: `0`; service `3.211s`.

Review-driven boundary tests were also written before their fixes. The first run exited `1` because the sticky shadow helper did not exist. After implementation, all five boundary cases passed: all-circuit shadow legacy restoration, sticky shadow comparison, runtime soft-sticky thresholds, explicit zero thresholds, and one settings repository read across repeated requests.

A second RED reproduced inaccurate health fallback telemetry:

```text
shadow_account_id=1 reason=same_account
FAIL: ShadowAccountID should be zero when balanced health is unavailable
```

After the fix, the focused six-case set exited `0` in `0.763s`. Health fallback now records `ShadowAccountID=0`, diff `0`, and reason `health_unavailable` while returning the full legacy order.

## Compatibility Details

`OpenAIAutoSchedulerSettings.UnmarshalJSON` starts from `DefaultOpenAIAutoSchedulerSettings` and tracks presence for the nine additive fields. This is required because Go's `bool` and numeric zero values cannot distinguish a missing field from explicit `false` or `0`. The handler also binds into a default settings value. Consequently:

- Missing `shadow_mode` remains `true`.
- Explicit `shadow_mode=false` remains false.
- Valid explicit zero values for exploration, escape gap, escape ratio, and jitter remain zero.
- Explicit out-of-range zero for `top_k` remains visible to handler validation instead of being silently defaulted.
- Existing JSON fields and the existing setting key remain unchanged.

## Shadow Observability

No existing dedicated shadow decision repository was present. The implementation reuses the balanced policy result and the repository's established `slog` pattern rather than creating a second store. Event `openai_balanced_scheduler_shadow_decision` records:

- `legacy_account_id`
- `shadow_account_id`
- `predicted_ttft_difference_ms` (`shadow - legacy`)
- `reason`

The same comparison is retained on `OpenAIBalancedSelectionResult` as `LegacyAccountID`, `ShadowAccountID`, `PredictedTTFTDifferenceMS`, and `ShadowReason` for tests and existing decision flow use.

If balanced health cannot be loaded or hydrated, no shadow account is fabricated. The comparison records `shadow_account_id=0` and `reason=health_unavailable`.

## Validation

All commands are fresh after the final implementation unless noted.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin \
  -run 'OpenAIAutoSchedulerSettings|Shadow' -count=1
```

Exit: `0`; service `0.707s`; handler/admin `0.636s`.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

Exit: `0`; `55.759s` after the last fallback fix.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -count=1
```

Exit: `0`; `1.072s`.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service \
  -run 'RuntimeSettingsMapBalancedControls|ShadowReturnsLegacyWhenAll|ComputesStickyShadow|SoftStickyUsesRuntime' -count=1
```

Exit: `0`; `2.383s`.

```bash
cd frontend
pnpm typecheck
pnpm exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts
```

Exit: `0`; typecheck passed; API tests `4/4` passed.

The first typecheck run exited `2` because required new fields broke the existing UI settings object. The API additions were made optional, consistent with the Task 5 generated-compatible boundary, and the fresh rerun passed without UI changes.

```bash
git diff --check
```

Exit: `0`.

## Risk Notes

- Shadow comparison logging covers balanced load ordering and ordinary sticky selection. Non-movable `previous_response_id` continues through the existing strong-sticky path without being moved.
- Runtime settings can take up to five seconds to become visible to a scheduler instance because of the hot-path cache. No separate rollout store was added.
- Task 6 must consume `probe_jitter_seconds`; this task only exposes and validates it.

## Post-Commit Reviewer Fix Wave

Starting HEAD: `2fffa86c3eb74d7e84e8b5e3f08da2b9a8dbc3c2`

The follow-up review found three settings-cache and shadow metadata issues. Each behavior received a failing test before its production fix.

### Shared Settings Cache RED/GREEN

Two RED tests showed that consecutive health outcomes and a selection followed by an outcome each performed two underlying settings reads (`expected 1, actual 2`). The five-second cache was moved from `defaultOpenAIAccountScheduler` to the singleton `SettingService`; selection and health outcome now share the same `atomic.Value` and `singleflight.Group`.

Focused GREEN: exit `0`, `1.048s`.

### Error, Timeout, and Last-Good RED/GREEN

The slow-read RED took `3.000251s` and failed the `<2.8s` bound. The loader now uses an independent `context.WithTimeout(context.WithoutCancel(ctx), 2s)` context.

The cache behavior is:

- Successful reads start their five-second TTL after DB and JSON work completes.
- A failed refresh returns stale last-known-good settings when available.
- Without last-known-good, failure returns safe `balanced + shadow=true` defaults but does not cache them as success.
- Concurrent expired reads share one refresh.
- Underlying DB/JSON errors remain distinguishable in the internal loader.

Four-case GREEN: exit `0`, `2.802s`.

### All-Rejected Shadow Metadata RED/GREEN

The RED observed `PredictedTTFTDifferenceMS=-2000` when all balanced candidates were circuit rejected. The result and structured log now retain the complete legacy order while recording `ShadowAccountID=0`, diff `0`, and reason `all_rejected`.

Combined focused GREEN: exit `0`, `2.809s`.

### Refresh/Update Race RED/GREEN

A deterministic interleaving test reproduced an old in-flight refresh overwriting a successful settings update (`expected balanced, actual legacy`). `SettingService` now captures a revision before DB refresh and commits cache values under a short mutex. Successful Set increments the revision and stores the new cache under the same mutex; DB reads do not hold the mutex.

Targeted GREEN: exit `0`, `3.425s`.

Race GREEN: exit `0`, `4.489s`.

### Final Fresh Verification

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1 -timeout 2m
```

Exit: `0`; `60.449s`.

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -count=1
```

Exit: `0`; `0.492s`.

```bash
cd frontend
pnpm typecheck
pnpm exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts
```

Exit: `0`; typecheck passed; API tests `4/4` passed.

Final independent review: Approved, with no remaining Critical or Important findings.
