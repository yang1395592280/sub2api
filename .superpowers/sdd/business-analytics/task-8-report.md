Task 8: End-to-End Verification and Documentation

Status: DONE

Verification run:
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ChannelPriceRefresh|RecordUsage|OpenAIUpstreamBalanceServiceRefresh'` from `backend`: PASS.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics|UsageLog.*ChannelPriceSnapshot'` from `backend`: PASS.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'` from `backend`: PASS.
- `pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts src/views/admin/__tests__/BusinessAnalyticsView.spec.ts src/router/__tests__/guards.spec.ts` from `frontend`: PASS; output included existing localStorage/Browserslist warnings.
- `gofmt -w internal/service/business_analytics.go internal/service/business_analytics_aggregation.go internal/service/business_analytics_aggregation_test.go internal/service/channel_price_refresh_job.go internal/repository/business_analytics_repo.go internal/repository/business_analytics_repo_test.go internal/repository/business_analytics_aggregation_repo.go internal/repository/business_analytics_aggregation_repo_test.go internal/handler/admin/business_analytics_handler.go internal/handler/admin/business_analytics_handler_test.go` from `backend`: PASS/no business diff left.
- `pnpm exec vue-tsc --noEmit` from `frontend`: PASS.
- `pnpm build` from `frontend`: PASS; output included existing Vite dynamic-import/chunk-size warnings and Browserslist warning.
- `git diff --check 665738ac..HEAD`: PASS.
- `git diff --check`: PASS.

Diff/status check:
- `git status --short --branch` shows only `.superpowers/sdd/*` scratch files modified/staged.
- `git diff --stat` shows only `.superpowers` scratch changes.
- `git diff --name-only 665738ac..HEAD` is business analytics related backend/frontend files.

Manual/visual verification:
- Attempted to open `http://127.0.0.1:3000/admin/business-analytics` with the in-app browser while frontend dev server was running.
- Browser plugin security policy rejected access to `127.0.0.1:3000`; no workaround was attempted.
- Manual admin-session visual verification remains required outside this browser-restricted environment.

Docs:
- No docs/spec commit needed. Implementation differences are already reflected in user-facing UI copy:
  - interval average group rate/channel price are shown as "接口暂未返回" because the current API does not expose those fields.
  - row-level historical approximation is not inferred without row-level snapshot fields; only aggregate missing snapshot hints are shown.

Concerns:
- Existing `.superpowers/sdd/task-1-report.md` remains staged/modified from prior workflow state and was intentionally not cleaned because previous restore attempts hit sandbox/approval issues.
- Git continues to report background gc loose-object warnings during commits; no repository cleanup was performed.
