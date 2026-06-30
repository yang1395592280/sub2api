### Task 5 Report: Frontend Automatic Group Option and Submission

Status: DONE_WITH_CONCERNS

Changed files:
- `frontend/src/types/index.ts`
- `frontend/src/api/keys.ts`
- `frontend/src/views/user/keyGroupOptions.ts`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/views/user/__tests__/keyGroupOptions.spec.ts`
- `frontend/src/views/user/__tests__/KeysView.autoGroup.spec.ts`

Summary:
- Added frontend API key group selection mode types and last effective group fields.
- Added the `openai_auto_cheapest` sentinel option to the key group option builder.
- Updated key creation and update flows to send `group_select_mode`.
- Added a computed selected group value in `KeysView.vue` so automatic mode maps to `group_id: null`.
- Added automatic mode display in the key table with the latest effective group name when present.
- Kept inline row group switching fixed-group only and made it force `group_select_mode: fixed`.
- Treated automatic-mode keys as OpenAI when opening key usage/import flows that otherwise infer platform from `row.group`.

Tests run:
- `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts`
  - Passed, 2 tests.
- `cd frontend && pnpm test:run src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - First run failed because the test layout stub did not render named slots; fixed the test stub.
  - Passed, 1 test.
- `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - Passed, 2 files / 3 tests.
- `cd frontend && pnpm typecheck`
  - Passed.
- After local review fix, re-ran:
  - `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - `cd frontend && pnpm typecheck`
  - Both passed.
- `git diff --check -- frontend/src/api/keys.ts frontend/src/types/index.ts frontend/src/views/user/KeysView.vue frontend/src/views/user/keyGroupOptions.ts frontend/src/views/user/__tests__/keyGroupOptions.spec.ts frontend/src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - Passed with no output.

Concerns:
- The specified key group option test was already green when resuming because `keyGroupOptions.ts` had partial production changes before this handoff.
- Subagent review/explorer attempt failed with upstream `502 Bad Gateway`, so Task 5 review must be handled locally or retried later.

Review:
- Task reviewer returned Spec verdict: pass.
- Task reviewer returned Quality verdict: fail due to Minor i18n findings only.
- Fixed the i18n findings by adding `keys.openaiAutoCheapest.*` messages in zh/en and passing localized text into the group option builder.
- Re-ran:
  - `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - `cd frontend && pnpm typecheck`
  - Both passed.
