# Task 4 Report

## Zenxiang Liyu Status

Zenxiang Liyu currently has its transactional repository and user-facing status/play service methods available. User records and daily summary service methods are not yet implemented.

## Task 4A

### Scope

- Added authenticated user APIs for Zenxiang Liyu status and play.
- Added the handler to the application dependency graph and regenerated Wire output.
- Did not add admin APIs, routes, service methods, repository changes, Ent changes, migrations, frontend changes, or progress updates.

### User APIs

- `GET /api/v1/zenxiang-liyu/status`
- `POST /api/v1/zenxiang-liyu/play`

`POST /play` accepts `request_id` only and passes the authenticated user ID plus request ID to `ZenxiangLiyuService.Play`. Rewards, ticket amounts, balances, and probabilities remain server-controlled.

### Error Mapping

- `ErrZenxiangLiyuRequestIDRequired`, `ErrZenxiangLiyuInsufficientBalance`, and `ErrZenxiangLiyuDailyLimitReached`: HTTP 400.
- `ErrZenxiangLiyuDisabled` and `ErrZenxiangLiyuUnauthorized`: HTTP 403.
- Other service errors: HTTP 500.

### Task 4B Dependency

`GET /api/v1/zenxiang-liyu/records` and `GET /api/v1/zenxiang-liyu/daily-summary` are intentionally not registered in Task 4A. The current `ZenxiangLiyuService` exposes neither user-record listing nor daily-summary methods; Task 4B must add those service contracts and implementations before HTTP exposure.

### Verification

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestZenxiangLiyuHandler' -count=1
```

Result: passed.
