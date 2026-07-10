# Task 4 Report: Account Editor Switch

## Implementation Summary

- Added the OpenAI overbrush switch to `EditAccountModal.vue`.
- The switch is visible only for OpenAI API key accounts whose `credentials.upstream_admin_type` is empty after trimming.
- Account form hydration resets the switch and loads `extra.openai_overbrush_enabled === true` for OpenAI API key accounts.
- Saving writes `extra.openai_overbrush_enabled: true` only while the eligible account's switch is enabled; saving disabled or ineligible accounts removes the key.
- Added the exact required Chinese and English i18n copy.

## Tests

- RED: `cd frontend && pnpm test -- EditAccountModal.spec.ts --run`
  - Confirmed the two new tests failed before implementation because the switch and its test id were absent.
- GREEN: `cd frontend && pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`
  - Passed: 1 file, 37 tests.
- `cd frontend && pnpm typecheck`
  - Passed (`vue-tsc --noEmit`).
- `git diff --check`
  - Passed.

## Files Changed

- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `.superpowers/sdd/task-4-report.md`

## Self-Review

- Verified the eligibility check exactly excludes non-OpenAI, non-API-key, and accounts with any non-empty trimmed `upstream_admin_type`.
- Verified the saved payload preserves the existing OpenAI `extra` data and changes only the overbrush key according to the switch state.
- Verified the component tests exercise display removal after a prop update and persistence after a click.
- No backend or Settings page files were changed.

## Concerns

- The task-specified `pnpm test -- EditAccountModal.spec.ts --run` command causes this repository's Vitest setup to run the wider test suite. The Task 4 component file passes, but the full invocation exits non-zero because of an unrelated existing failure in `BulkEditAccountModal.spec.ts` and three existing Dashboard Pinia unhandled rejections. The direct single-file Vitest command above passes cleanly.

## Review Fix Report (2026-07-10)

### Changes

- Expanded OpenAI overbrush visibility coverage for `upstream_admin_type: 'new-api'`, `upstream_admin_type: 'sub2api'`, OAuth, setup-token, and whitespace-only `upstream_admin_type`.
- Extended the enabled persistence test to prove an unrelated existing `extra` field is retained.
- Added a disabled persistence test proving `openai_overbrush_enabled` is removed while unrelated `extra` data remains.
- No component or i18n implementation changes were needed; the existing eligibility and payload logic satisfies these cases.

### Verification

- `cd frontend && pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`
  - Passed: 1 file, 42 tests.
- `cd frontend && pnpm typecheck`
  - Passed: `vue-tsc --noEmit`.
