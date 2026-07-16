# Task 4 Report: Balanced Selection Policy

## Status

Implemented and verified against base `9e78c47cb`.

## Scope

- Added `OpenAIBalancedScheduler.Order` with open/half-open exclusion, 1000ms + 25% ordinary-session TTFT escape, queue/429/5xx/error-rate hard escape, latency primary pool, group priority, price/quota ordering, Top-K 3 rank-weighted selection, and seeded 3% exploration.
- Kept strong `previous_response_id` selection ahead of the policy. Strong previous and non-escaped ordinary sticky candidates remain fixed first inside the policy.
- Preserved all latency-ineligible candidates as a failover tail; they never enter Top-K or exploration.
- Changed scheduler priority lookup to use the current `AccountGroup.Priority`, falling back to account priority only when that membership is absent.
- Added exact Task 3-compatible candidate health keys using physical account ID, actual mapped upstream model, resolved upstream endpoint, and actual transport.
- Propagated canonical requested endpoints through Responses, Messages, WS, Chat Completions, Embeddings, and parsed Images operations. Non-canonical alpha-search/count-tokens/Grok media paths pass an incomplete dimension and safely retain legacy order.
- Preloaded health once per external Select after hard filters and Compact gating, before subscription-pool partitioning. Subscription, regular, and fresh-load retry paths share the same request-scoped snapshot map.
- Injected the balanced scheduler from `OpenAISchedulerHealthRepository`; `wire_gen.go` was updated only by Wire generation.

## Preserved Contracts

- Existing price guard, account/model/endpoint/transport capability checks, quota/rate/account/parent gates, excluded IDs, Compact tiers/retry, overbrush, DB recheck, slot acquisition, wait plan, failover, automatic cheapest group, model mapping, SSE/WS, shadow accounts, and billing remain outside the policy.
- Health repository error, incomplete key, missing snapshot, or expired snapshot returns the original legacy order without interrupting the request.
- `GetBatch` is one call per external Select, including subscription-to-regular fallback and fresh load retry; no per-account health reads were added.
- No runtime mode, shadow settings, API, UI, migration, or Task 5 configuration was added.

## TDD Evidence

- Initial policy RED: compile failed on missing `OpenAIBalancedSelectionInput`, `OpenAIBalancedCandidate`, settings, and constructor.
- Endpoint/repository RED: compile failed on missing `RequiredEndpoint`, key resolver, request time, and health hydration.
- Adapter RED: missing balanced scheduler injection; after adapter work, hard-filter/slot tests exposed absent endpoint propagation.
- Wire RED: generated source lacked `ProvideOpenAIBalancedScheduler` and gateway injection.
- Additional REDs caught escaped queued sticky returning first, Top-K dropping failover candidates, ineligible exploration, strong sticky exploration override, repeated fresh/subscription pool health reads, missing 429/5xx rank penalties, missing weighted Top-K order, and missing error-rate escape reason.
- Full-suite failure was isolated to `TestSelectTopKOpenAICandidates`: its direct fixture did not populate the newly precomputed candidate priority. The fixture now mirrors both production constructors; comparator fallback was deliberately not added because group priority `0` is valid.

## Verification

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAIBalancedScheduler|OpenAIGatewayService_SelectAccountWithScheduler|OpenAIAccountSchedulingPriority' -count=1` -> PASS (`0.687s`).
- `GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'OpenAIBalancedScheduler|BalancedHealthBatch|BalancedPreservesHardFilters|BalancedHealthFallback' -count=1` -> PASS (`2.084s`).
- Isolated full `internal/service` JSON run -> package PASS, exit 0 (`55.881s`).
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -count=1` -> PASS (`21.923s`).
- `GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go test ./cmd/server -count=1` -> PASS (`0.443s`).
- `git diff --check` -> PASS; `backend/go.sum` and `backend/go.mod` have no diff.

## Wire Generation

The first standard generator run failed on the known missing `github.com/google/subcommands` checksum. `GOFLAGS=-mod=mod GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server` succeeded. The temporary `go.sum` additions were removed.

## Concerns

- Task 5 still needs runtime mode/shadow controls. Task 4 intentionally activates the injected balanced policy under the existing advanced scheduler boundary without adding new settings.

## Reviewer Fix Wave (2026-07-14)

- Channel mapping now resolves per fixed or auto-effective API key before scheduling for Responses, Chat Completions, Embeddings, Images, and WS. The candidate health model then applies account mapping, so the key matches the final upstream model.
- WS ingress health transport now follows the actual route: mode-router passthrough uses WSv2, HTTP bridge uses HTTP SSE, and dynamic context-pool modes emit an incomplete key to trigger whole-request legacy fallback. With mode-router disabled, the existing protocol resolver remains authoritative.
- Balanced results now expose circuit-rejected account IDs. The adapter preserves that rejection through legacy-tail fill, fresh-load retry, slot failover, and wait-plan construction while retaining running latency-ineligible candidates.
- The latency-ineligible tail uses a price-neutral comparator and falls back to legacy order. Price remains active inside the latency-eligible pool.

### Reviewer Fix TDD Evidence

- Channel mapping RED: all five handler entry points still called selectors without a model resolver; Images had no model-resolver wrapper. GREEN covers fixed and auto-effective groups across all five endpoint types.
- WS ingress RED: HTTP bridge produced a WSv2 health key and context-pool mode issued a health batch. GREEN covers passthrough, HTTP bridge, context pool, and legacy resolver mode.
- Circuit RED: open and half-open candidates were re-appended first by the adapter and then by fresh-load retry. GREEN proves they never enter acquisition or wait fallback, while a running latency tail remains eligible for slot failover.
- Tail ordering RED: equivalent slow candidates changed from legacy `[2, 3]` to price order `[3, 2]`. GREEN restores legacy order without changing the existing eligible-pool price test.

### Reviewer Fix Verification

- Focused service tests -> PASS (`0.817s`).
- Handler channel-mapping contract -> PASS (`0.630s`).
- Focused service race tests -> PASS (`2.337s`).
- Full `internal/service` -> PASS (`55.807s`).
- Full `internal/handler` -> PASS (`22.682s`).
- `cmd/server` -> PASS (`0.458s`).
- `git diff --check` -> PASS.

## Reviewer Fix Wave 2 (2026-07-14)

### Investigation And RED

- The first sticky-boundary fixture used `sticky-circuit`, while the real session path reads `openai:sticky-circuit` through `openAISessionCacheKey`; ordinary and weighted sticky therefore never exercised their bindings.
- The previous-response fixtures wrote bindings but did not enable global/API-key WSv2 or the account WSv2 flag. `resolveAccountByPreviousResponseIDForCapability` correctly rejected those bindings because the protocol resolver selected HTTP.
- After correcting only those fixtures, existing production failed exactly at the intended boundaries: ordinary session selected open account `37121`, weighted fallback reacquired `37121`, movable previous selected `37121`, while non-movable previous stayed strongly bound and passed. The strict latency tail returned `[1,3,2]` instead of legacy `[1,2,3]`.

### GREEN

- Non-movable `previous_response_id` remains a hard sticky path and returns before any unified-health read.
- Requests with ordinary session sticky or movable previous bindings prepare the same hard-filtered candidate set and one request-scoped health batch before sticky selection. The prepared account/load request is reused by load balancing.
- Open/half-open snapshots reject ordinary soft sticky and weighted fallback reacquisition. Repository error, incomplete key, missing/expired snapshot, or changed key retains legacy sticky behavior.
- Movable previous bindings are resolved for unified policy even when sticky-weighted mode is disabled; healthy candidates retain policy preference, while circuit-rejected candidates yield.
- The latency-ineligible running tail is stable-sorted only by `LegacyOrderPosition`; latency, errors, queue, priority, quota, load, and price cannot reorder it.

### Verification

- Corrected RED command before production changes: three sticky/circuit subtests failed, non-movable strong control passed, strict tail failed.
- Focused sticky/circuit and strict-tail GREEN -> PASS.
- Expanded balanced/select-account coverage -> PASS.
- Expanded previous-response/session/weighted sticky coverage -> PASS (`5.060s`).
- Focused race coverage -> PASS (`2.707s`).
- Fresh full `internal/service` after the final privacy-side-effect timing check -> PASS (`55.919s`).
- `git diff --check` -> PASS; `backend/go.mod` and `backend/go.sum` have no diff.

## Reviewer Fix Wave 3 (2026-07-14)

### Root Causes And RED

- OAuth health keys stopped after account mapping, while forwarding calls `normalizeOpenAIModelForUpstream` after mapping. Responses HTTP and WS fixtures both returned `gpt-5.1` instead of the actual `gpt-5.4` upstream model.
- Ordinary session sticky returned before unified policy and only checked circuit state. Movable previous was passed as `PreviousResponseAccountID`, which the pure policy intentionally treats as strong. Both selected the 2600ms sticky account instead of the 1200ms candidate; the non-movable previous control passed.
- Compact health preload skipped cached tier0 accounts. A real stale retry fixture used scheduler cache tier0 and DB fresh tier2 for the same physical account; `buildOpenAISelectionOrder -> tryAcquireOpenAISelectionOrder` acquired the open account after DB recheck.
- Adapter `LegacyOrderPosition` used repository candidate indices. With candidates `[A,B,C]`, legacy selection `[C,B,A]`, eligible `A`, and tail `B/C`, the adapter returned `[A,B,C]` instead of `[A,C,B]`.
- Additional hard-escape integration RED showed queue/429/5xx/error-rate correctly produced `StickyEscapeReason`, but the non-exploration rank-weighted shuffle could move escaped sticky back to the front.

### GREEN

- Candidate keys now reuse the forwarding normalization helper after account mapping. Explicit Compact mappings retain the exact forwarding target, and API-key mappings remain unchanged.
- Sticky requests preload and reuse one candidate load batch plus one health batch. Ordinary session and movable previous use the unified absolute/relative TTFT and queue/429/5xx/error-rate escape rules. Non-movable previous still returns before health reads.
- Escaped sticky remains available as a capacity tail but is excluded from the rank-weighted Top-K shuffle that could otherwise select it first again.
- When snapshot/DB stale retry is possible, cached Compact tier0 accounts join the same health batch and open/half-open accounts are removed before retry. Health repository failure or incomplete metadata keeps the legacy stale retry behavior. Tier0 accounts cannot disable balanced selection when no snapshot retry path exists.
- Adapter positions come from the existing legacy selection order. Candidates outside legacy Top-K receive tail positions without being added to `LegacyOrderedAccountIDs`, preserving health-failure fallback exactly.

### Verification

- Initial four-item RED: OAuth HTTP/WS model mismatch; ordinary and movable sticky chose slow account; non-movable control passed; Compact stale account acquired; adapter tail order mismatched.
- All four targeted regressions and queue/429/5xx/error-rate adapter cases -> PASS.
- Expanded balanced/sticky/previous/Compact/model-mapping coverage -> PASS (`2.496s`).
- Final focused legacy-fallback coverage -> PASS (`0.743s`).
- Fresh focused race -> PASS (`2.192s`).
- Fresh full `internal/service` -> PASS (`55.714s`).

## Reviewer Fix Wave 4 (2026-07-14)

### RED

- The deterministic queue-escape fixture returned `TopK=2` instead of `1`. Its order was `[healthy, escaped sticky, slow]`, but escaped sticky still counted as latency eligible.
- A full WS passthrough success outcome used an API-key mapping from `gpt-5.1` to `mapped-ws-upstream`. Legacy `Model` correctly remained `gpt-5.1`, while health `ModelFamily` incorrectly remained `gpt-5.1` instead of the scheduler read key target.
- An OAuth control verifies the required ordering independently: custom alias account mapping to `gpt-5.1`, then OAuth alias normalization to `gpt-5.4`.

### GREEN

- Escaped sticky is removed from the latency-eligible pool and inserted into the failover tail using `LegacyOrderPosition`, with original candidate order as the stable tie-break. The deterministic result is `TopK=1`, order `[healthy, escaped sticky, slow]`; open/half-open candidates remain fully rejected.
- Passthrough dial, first-write, unfinished-turn, and completed-turn outcome metadata now reuse `openAIWSIngressAttemptMetadata`. That helper delegates model resolution to `resolveOpenAIAccountUpstreamModelForRequest`, matching channel-resolved input -> account mapping -> OAuth normalization.
- Completed-turn metadata no longer lets `OpenAIForwardResult.UpstreamModel` overwrite the exact scheduler health key. WS payload forwarding, forward-result fields, streaming, usage, and billing were not changed.

### Verification

- Corrected targeted RED: escaped sticky expected `TopK=1`, actual `2`; passthrough expected `mapped-ws-upstream`, actual `gpt-5.1`.
- Targeted GREEN -> PASS (`3.183s`).
- Expanded Task 3 outcome, passthrough, balanced scheduler, and soft-sticky coverage -> PASS (`5.790s`).
- Fresh focused race -> PASS (`2.336s`).
- Fresh full `internal/service` -> PASS (`56.432s`).
- `git diff --check` -> PASS; `backend/go.mod` and `backend/go.sum` have no diff.
