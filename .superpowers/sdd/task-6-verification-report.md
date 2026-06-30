### Task 6 Verification Report: OpenAI Auto Cheapest Group

Status: DONE_WITH_CONCERNS

Summary:
- Ran backend focused suites for API key, OpenAI, auto cheapest, gateway, and repository migration/API key coverage.
- Ran backend package regression sweep for service, handler, and repository packages.
- Ran frontend focused auto-group tests and TypeScript checking.

Tests run:
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'APIKey|OpenAI|AutoCheapest|Gateway' -count=1`
  - Passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'OpenAI|Gateway|APIKey' -count=1`
  - Passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'Migration|APIKey' -count=1`
  - Passed.
- `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - Passed, 2 files / 3 tests.
- `cd frontend && pnpm typecheck`
  - Passed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler ./internal/repository -count=1`
  - Passed.

Manual smoke:
- Not run. This requires a local runtime with at least two configured OpenAI groups and account pools. No such environment was started in this session.

Concerns:
- Existing unrelated dirty files remain outside this task: `frontend/pnpm-lock.yaml`, `.pnpm-store/`, and `frontend/pnpm-workspace.yaml`.
