# OpenAI Score-Based Routing Design

## Background

The current OpenAI routing path already filters accounts by static availability:
`status=active`, `schedulable=true`, model support, capability support, quota pause,
`rate_limit_reset_at`, `overload_until`, and `temp_unschedulable_until`.

It also contains an optional advanced scheduler with runtime health signals
(`HealthScore`, error EWMA, TTFT EWMA, primary/standby/observe/degraded tiers),
but normal behavior still leaves several bad-channel cases too weak:

- OpenAI 429 is cooled down through `SetRateLimited`.
- OpenAI 529 is cooled down through `SetOverloaded`.
- OpenAI 503/502/504 and transport or timeout failures are mostly logged or treated
  as request-level failover, but they do not reliably remove the account from the
  next normal scheduling attempt.
- Sticky sessions can keep trying a channel unless the runtime health checks are
  active and strict enough.

The desired behavior is score-driven routing: channels with slow or failing
behavior quickly lose scheduling priority, hard failures trigger short cooldown,
and recovery happens through controlled probing or cooldown recovery rather than
through user requests repeatedly hitting bad channels.

## Goals

1. Make OpenAI routing primarily health-score driven for normal requests.
2. Treat 503/502/504, upstream timeouts, and transport errors as strong health
   failures that can remove an account from normal routing for a short period.
3. Penalize consecutive slow responses, especially TTFT or first response latency
   above 10 seconds.
4. Ensure sticky sessions escape when the bound account becomes slow, degraded, or
   cooling down.
5. Preserve existing hard-limit behavior for 429 and 529.
6. Keep the first implementation small and testable without adding a new
   persistent health table or a full probe scheduler.

## Non-Goals

- Do not redesign all platform routing. This spec only covers OpenAI routing.
- Do not add a management UI in the first implementation.
- Do not replace existing group, model mapping, pricing, capability, and
  concurrency checks.
- Do not persist every score sample to the database in the first implementation.
- Do not implement full scheduled probe recovery in the first implementation.
  The first phase should leave an extension point and use cooldown expiry plus
  runtime recovery as the minimum viable recovery behavior.

## Proposed Approach

Use the existing `OpenAIAccountScheduler` as the health-routing core instead of
creating a parallel scheduler.

The scheduler already has most of the required concepts:

- runtime stats per account;
- health score;
- error EWMA;
- TTFT EWMA;
- consecutive failure/success counters;
- cooldown and degraded tier;
- sticky-session escape checks;
- primary/standby/observe/degraded routing.

The implementation should tighten those semantics so normal routing only selects
healthy enough accounts, and error handling reports meaningful failure reasons to
the scheduler consistently.

## Health Model

Each OpenAI account starts with an effective healthy state:

- default success EWMA: healthy;
- default health score: high;
- default tier: primary or standby depending on ranking;
- no cooldown.

Runtime events update the state:

- Success with normal latency: increases success streak and gradually improves
  score.
- Success with TTFT greater than 10 seconds: counts as a slow sample. It should
  reduce health score and can move the account to observe after consecutive slow
  samples.
- 429: keep existing `SetRateLimited` behavior and report a rate-limited health
  failure.
- 529: keep existing `SetOverloaded` behavior and report an upstream overload
  health failure.
- 502/503/504: report an upstream 5xx health failure and apply short scheduler
  cooldown.
- Transport error or upstream timeout: report transport or timeout health failure
  and apply short scheduler cooldown.
- 401/403: keep existing auth policy. These errors may also report health failure,
  but the permanent or temporary auth-state handling remains authoritative.

Initial thresholds:

- Slow response threshold: 10 seconds.
- Consecutive slow threshold: 2 samples.
- Consecutive failure threshold: 2 samples for immediate degraded routing.
- Short cooldown for 5xx/timeout/transport: 60 seconds.
- Existing 429/529 reset/cooldown values remain authoritative.

These values should be constants or config-backed defaults using the existing
settings style where practical.

## Routing Semantics

Normal request routing should behave as follows:

1. Build the static candidate set exactly as today.
2. Exclude accounts blocked by persistent state:
   `RateLimitResetAt`, `OverloadUntil`, `TempUnschedulableUntil`, expiry, quota,
   model/capability mismatch, group restrictions.
3. Exclude accounts blocked by scheduler runtime health:
   cooldown active or tier `degraded`.
4. Prefer `primary`, then `standby`.
5. Allow `observe` only for explicit probing or a small configured probe ratio.
   For the first implementation, normal requests should not rely on observe
   accounts when healthy accounts exist.
6. Never send normal user requests to `degraded` accounts.

If all candidates are degraded or cooling down, return no available account rather
than repeatedly hitting known bad channels.

## Sticky Session Behavior

Sticky routing remains useful for conversation continuity, but health must win.

When a sticky account is selected:

- recheck static schedulability;
- recheck runtime block/cooldown;
- check health tier;
- check slow-response escape thresholds.

If the account is degraded, cooling down, or over the slow/error escape threshold,
clear or bypass the sticky binding and select from healthy candidates.

The existing sticky escape mechanism should be reused and tightened instead of
adding a separate sticky store.

## Error Handling Integration

All OpenAI forwarding paths that can observe upstream failures should report to
the scheduler:

- passthrough `/responses`;
- OpenAI chat completions;
- OpenAI messages compatibility;
- OpenAI images;
- embeddings where applicable;
- transport error handling.

The first implementation should audit these paths and call the existing exported
methods:

- `ReportOpenAIAccountScheduleResult`
- `ReportOpenAIAccountScheduleResultWithReason`
- `ApplyOpenAISchedulerHealthAction` where cooldown is needed

For 503 passthrough specifically, the behavior should change from "mostly
passthrough/log" to "failover plus short cooldown" for normal gateway requests.
The upstream error may still be returned after failover exhaustion.

## Recovery

First implementation:

- cooldown expiry moves an account out of hard runtime block;
- successful later requests improve EWMA and success streak;
- degraded accounts should not be selected for normal traffic before cooldown
  expires or health moves back to observe/standby.

Future implementation:

- add a scheduled OpenAI health probe worker;
- probe degraded/observe accounts with a cheap model/request template;
- restore score gradually after consecutive probe success;
- expose score, cooldown, slow/error counters, and last probe result in admin UI.

## Testing Plan

Add focused service tests for the first phase:

1. OpenAI scheduler does not select an account in runtime cooldown.
2. Session sticky degraded account falls back to a fresh candidate.
3. Load-balance path skips degraded accounts.
4. OpenAI passthrough 503 triggers failover and health cooldown.
5. Consecutive slow TTFT reports move account out of normal sticky routing.
6. 429 still writes `RateLimitResetAt` and is excluded by static schedulability.
7. 529 still writes `OverloadUntil` and is excluded by static schedulability.

## Risks

- Over-aggressive cooldown can reduce available capacity during provider-wide
  incidents. Keep first cooldown short and configurable.
- In-memory health state is lost on process restart. This is acceptable for phase
  one because persistent 429/529/temp-unsched states still cover hard blocks.
- If advanced scheduler is disabled, health scoring may not apply. The
  implementation must either enable it for OpenAI routing by default or make the
  fallback path honor runtime cooldown consistently.
- Probe recovery is intentionally deferred, so a degraded account may recover more
  slowly until phase two.

## Acceptance Criteria

- A channel returning 503/502/504 or transport timeout is not immediately reused
  for the next normal OpenAI request while another healthy channel exists.
- A channel with consecutive TTFT above 10 seconds loses sticky priority and
  normal routing moves to a healthier channel.
- 429 and 529 behavior remains compatible with existing persistent cooldown
  fields.
- Existing model, group, capability, concurrency, and pricing restrictions remain
  effective.
- Tests demonstrate the new routing behavior without requiring external OpenAI
  calls.
