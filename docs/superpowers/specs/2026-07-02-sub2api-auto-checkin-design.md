# sub2api Auto Check-in Design

Date: 2026-07-02

## Goal

Add a sub2api-specific auto check-in feature to upstream account management. Admins can enable daily check-in per upstream account, configure a check-in URL and a daily random execution window, manually test check-in, and see the latest check-in result.

The first version intentionally supports only sub2api upstream management credentials. It does not implement a generic HTTP task runner.

## Scope

In scope:

- Add check-in configuration to the existing account create/edit UI under the `sub2api` upstream admin credentials section.
- Support a default sub2api check-in endpoint: `POST /api/v1/user/checkin`.
- Support a configurable relative path or same-origin full URL.
- Execute one daily check-in per enabled account at a random time inside a configured local-time range.
- Use server local time for scheduling. The current deployment expectation is `Asia/Shanghai`.
- If the service misses the scheduled window and the account has not checked in successfully that day, run a same-day make-up check-in after service recovery.
- Retry transient failures up to 3 times per day with a random 10-30 minute delay.
- Treat an upstream "already checked in today" response as success.
- Store latest result status on the account for UI display.
- Provide a manual "test check-in" action from the account edit dialog.

Out of scope:

- Generic HTTP task configuration with arbitrary methods, headers, request bodies, or success expressions.
- Dedicated check-in history tables.
- Monthly reward analytics.
- Multi-timezone per-account configuration.
- Browser-cookie-based check-in.

## Existing Context

Accounts store upstream credentials in `Account.Credentials` JSONB and runtime/display metadata in `Account.Extra` JSONB. The existing sub2api upstream admin settings already use these credential keys:

- `upstream_admin_type`
- `upstream_admin_email`
- `upstream_admin_password`
- `upstream_admin_access_token`
- `upstream_admin_refresh_token`
- `upstream_admin_token_type`

Sensitive credential merging and redaction already preserve token/password values when the frontend leaves them blank.

The HAR for `ai.clol.site` shows the sub2api check-in endpoint:

- Method: `POST`
- Path: `/api/v1/user/checkin`
- Empty body
- Example response fields: `checked_in`, `eligible`, `reward_amount`, `balance`, `checked_in_at`

## Recommended Approach

Use account-embedded configuration and a lightweight backend runner.

This avoids a new table in the first version while keeping the feature close to the existing upstream admin credentials. Runtime status is written to `extra`, so the account list and edit dialog can show the latest result without joining another resource.

## Frontend Design

Show the check-in controls only when upstream admin settings are supported and `upstream_admin_type` is `sub2api`.

Add controls below the existing `sub2api 上游管理凭据` block:

- Toggle: enable auto check-in.
- Input: check-in URL, default `/api/v1/user/checkin`.
- Time input: random start time, default `08:00`.
- Time input: random end time, default `10:30`.
- Button: test check-in now.
- Status summary: latest status, latest run time, latest reward, latest balance, next planned run time, and latest error.

Saving the account writes non-sensitive check-in config into `credentials`. Existing sensitive credential preservation behavior remains unchanged.

The UI should validate:

- When auto check-in is enabled, both time inputs are required.
- Times use `HH:mm`.
- End time must be after start time for the first version.
- The check-in URL must be either a relative path or a full URL with the same origin as the account base URL.

## Backend Design

Add a `Sub2APICheckinService` with `Start()` and `Stop()` lifecycle methods, wired with the server cleanup lifecycle like existing background services.

The service scans enabled candidate accounts at a short interval such as 1 minute. Candidates are active API key accounts with:

- `credentials.upstream_admin_type = "sub2api"`
- `credentials.upstream_checkin_enabled = true`

For each candidate, the service:

1. Reads the check-in config and current runtime state.
2. Generates a daily random `next_run_at` if none exists for today.
3. Runs when current time reaches `next_run_at`.
4. If the configured window has already passed and there is no successful check-in for today, runs a make-up check-in immediately.
5. Acquires sub2api admin authorization using this fallback order:
   - refresh token
   - existing access token
   - email/password login
6. Executes `POST` to the configured check-in endpoint with no body.
7. Parses the response and persists status.
8. On transient failure, schedules another attempt after a random 10-30 minutes while the daily retry count is below 3.
9. After success, optionally invokes the existing upstream balance refresh flow. Balance refresh failure does not turn the check-in result into failure.

The check-in implementation should share or extract common sub2api admin auth helpers from the existing upstream balance service instead of duplicating token refresh and login behavior.

## Data Model

Do not add tables for the first version.

Store user-controlled config in `credentials`:

```json
{
  "upstream_checkin_enabled": true,
  "upstream_checkin_url": "/api/v1/user/checkin",
  "upstream_checkin_start_time": "08:00",
  "upstream_checkin_end_time": "10:30"
}
```

Store runtime status in `extra`:

```json
{
  "upstream_checkin_status": "success",
  "upstream_checkin_last_run_at": "2026-07-02T08:37:12+08:00",
  "upstream_checkin_last_success_date": "2026-07-02",
  "upstream_checkin_next_run_at": "2026-07-03T09:18:00+08:00",
  "upstream_checkin_reward_amount": 10,
  "upstream_checkin_balance": 89.51263582,
  "upstream_checkin_error": "",
  "upstream_checkin_retry_date": "2026-07-02",
  "upstream_checkin_retry_count": 0
}
```

Status values:

- `pending`: enabled but not yet run today.
- `success`: latest run succeeded or upstream reported already checked in.
- `error`: latest run failed and retry may or may not be pending.
- `unsupported`: config is present but the account is not usable for sub2api check-in.

The check-in config keys are not sensitive. Existing token and password keys remain sensitive and must continue to be redacted and merge-preserved.

## Scheduling Rules

Server local date defines "today".

Daily plan generation:

- If enabled and no valid `upstream_checkin_next_run_at` exists for the current local date, generate a random time between start and end.
- Persist that value to `extra`.
- If start/end config changes, regenerate the next run if the existing planned time is outside the new range and today's check-in has not succeeded.

Execution:

- Run when `now >= next_run_at`.
- If `now` is after the configured end time and today's success date is not today, run immediately as a make-up.
- After success, generate the next day's plan on the next scan cycle or immediately after success.

Retry:

- Retry at most 3 times per local day.
- `upstream_checkin_retry_count` is scoped by `upstream_checkin_retry_date`; a new local day resets the count.
- Each retry delay is a random duration from 10 to 30 minutes.
- A response meaning "already checked in today" is success and clears retry state.
- After retry exhaustion, keep status `error` until the next local day.

## HTTP and Security Rules

The default target URL is built from the account base URL plus `/api/v1/user/checkin`.

Configured URL rules:

- Relative paths are resolved against the account base URL.
- Full URLs are accepted only when origin matches the account base URL origin.
- Other origins are rejected during validation and skipped defensively in the runner.

Request rules:

- Method is always `POST`.
- Body is empty.
- `Accept: application/json` is sent.
- `Authorization` uses the sub2api admin token with token type defaulting to `Bearer`.

Sensitive data must not be written to logs or `extra`.

## Manual Test Action

Add an admin endpoint to run check-in for one account immediately. The endpoint:

- Requires admin auth, following existing admin account endpoint conventions.
- Uses the same validation, auth, request, and parse flow as scheduled check-in.
- Persists the result to `extra`.
- Returns the updated account or a structured check-in result for UI refresh.

Manual test should not require auto check-in to be enabled, but the account must be sub2api-configured and have a valid check-in URL.

## Error Handling

- Disabled check-in: skip without error.
- Missing sub2api admin credentials: persist `error` with a concise reason.
- Refresh token failure: fall back to access token, then email/password login.
- 401/403: persist auth failure and schedule retry if retries remain.
- 5xx or network timeout: persist transient error and schedule retry if retries remain.
- Unexpected JSON: persist parse failure and schedule retry if retries remain.
- `code = 0` with `data.checked_in = true` or `data.checked_in_at` present: persist `success`.
- Already checked in today, even if represented as a message instead of a fresh reward: persist `success`.
- Balance refresh failure after check-in success: keep check-in `success` and record only a non-fatal warning if needed.

## Testing Plan

Backend unit tests:

- Candidate filtering for enabled sub2api accounts.
- Time-window parsing and validation.
- Random time generation within range.
- Cross-day reset and next-run generation.
- Missed-window make-up behavior.
- Retry count and randomized retry delay bounds.
- Retry exhaustion.
- "Already checked in" response treated as success.
- Response parsing for `reward_amount`, `balance`, and `checked_in_at`.
- Auth fallback order: refresh token, access token, email/password.
- Credential updates from refreshed token are persisted.
- Runtime status writes to `extra` and does not leak tokens.

Frontend tests:

- Edit modal loads existing check-in config.
- Edit modal saves check-in config.
- Controls show only for `sub2api` upstream admin type.
- Defaults are applied for new sub2api config.
- Validation rejects invalid time ranges and cross-origin URLs.
- Manual test action displays success and failure summaries.

Manual verification:

- Configure an account with sub2api upstream admin credentials.
- Enable auto check-in and save.
- Click test check-in and verify latest status.
- Verify latest result is visible after reopening the edit dialog.
- Optionally shorten the scheduled window in a local environment and confirm scheduled execution updates `extra`.

## Risks and Mitigations

Risk: sub2api variants may return slightly different check-in payloads.

Mitigation: Treat HTTP 2xx plus explicit already-checked-in or success-shaped response as success, and persist raw concise error text only when parsing fails.

Risk: multiple app instances may run the same check-in.

Mitigation: First version can tolerate duplicate calls because upstream already-check-in is success. If deployment is multi-instance and duplicate requests become noisy, add a Redis or DB lock in a later iteration.

Risk: generated random time changes unexpectedly after account edits.

Mitigation: Preserve today's `next_run_at` unless config changed enough to make the planned time invalid or today's check-in has already completed.

Risk: token refresh logic diverges from upstream balance logic.

Mitigation: Extract common sub2api admin auth helpers and reuse them from both balance refresh and check-in.
