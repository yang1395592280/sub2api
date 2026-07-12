# OpenAI OAuth Overbrush Bulk Edit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an eligible-only bulk-edit control that can enable or disable OpenAI OAuth overbrush without sending or changing `schedulable`.

**Architecture:** Extend the existing `BulkEditAccountModal` field-gating pattern with a strict OpenAI OAuth eligibility computed value, one outer “modify this field” flag, and one inner boolean value. Reuse the existing bulk-update `extra` merge path and existing overbrush translation keys; no backend or API changes are required.

**Tech Stack:** Vue 3 Composition API, TypeScript, Vue Test Utils, Vitest, pnpm

## Global Constraints

- Show the control only when every target account has `platform === 'openai'` and `type === 'oauth'`.
- The outer checkbox controls whether this operation changes overbrush; unchecked means no overbrush key is submitted.
- Submit `extra.openai_overbrush_enabled: true` to enable and `false` to disable.
- Never add `schedulable` to the bulk-edit payload.
- Reuse `admin.accounts.openai.overbrush` and `admin.accounts.openai.overbrushDesc`.
- Do not change backend APIs, database fields, overbrush runtime behavior, or scheduler behavior.

---

### Task 1: OpenAI OAuth Overbrush Bulk Edit Control

**Files:**
- Modify: `frontend/src/components/account/BulkEditAccountModal.vue`
- Test: `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`

**Interfaces:**
- Consumes: existing target summaries `targetSelectedPlatforms` and `targetSelectedTypes`, `buildUpdatePayload(): Record<string, unknown> | null`, and `adminAPI.accounts.bulkUpdate(...)`.
- Produces: `allOpenAIOverbrushEligible: ComputedRef<boolean>`; template controls `#bulk-edit-openai-overbrush-enabled` and `#bulk-edit-openai-overbrush-toggle`; payload key `extra.openai_overbrush_enabled: boolean`.

- [ ] **Step 1: Write failing eligibility tests**

Add tests that mount the modal with eligible and ineligible target summaries:

```ts
it('仅全部为 OpenAI OAuth 时显示超刷批量编辑项', () => {
  const eligible = mountModal({
    selectedPlatforms: ['openai'],
    selectedTypes: ['oauth']
  })
  expect(eligible.find('#bulk-edit-openai-overbrush-enabled').exists()).toBe(true)

  const apiKey = mountModal({
    selectedPlatforms: ['openai'],
    selectedTypes: ['apikey']
  })
  expect(apiKey.find('#bulk-edit-openai-overbrush-enabled').exists()).toBe(false)

  const mixedTypes = mountModal({
    selectedPlatforms: ['openai'],
    selectedTypes: ['oauth', 'setup-token']
  })
  expect(mixedTypes.find('#bulk-edit-openai-overbrush-enabled').exists()).toBe(false)

  const otherPlatform = mountModal({
    selectedPlatforms: ['anthropic'],
    selectedTypes: ['oauth']
  })
  expect(otherPlatform.find('#bulk-edit-openai-overbrush-enabled').exists()).toBe(false)
})
```

- [ ] **Step 2: Run the eligibility test and verify RED**

Run:

```bash
pnpm --dir frontend vitest run src/components/account/__tests__/BulkEditAccountModal.spec.ts -t '仅全部为 OpenAI OAuth 时显示超刷批量编辑项'
```

Expected: FAIL because `#bulk-edit-openai-overbrush-enabled` does not exist for eligible targets.

- [ ] **Step 3: Write failing payload tests**

Add two focused tests for enabling and disabling; both assertions intentionally compare the entire update object so an accidental `schedulable: false` fails the test:

```ts
it.each([
  ['开启', true],
  ['关闭', false]
])('OpenAI OAuth 批量编辑可%s超刷且不修改调度', async (_label, enabled) => {
  const wrapper = mountModal({
    selectedPlatforms: ['openai'],
    selectedTypes: ['oauth']
  })

  await wrapper.get('#bulk-edit-openai-overbrush-enabled').setValue(true)
  if (enabled) {
    await wrapper.get('#bulk-edit-openai-overbrush-toggle').trigger('click')
  }
  await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
  await flushPromises()

  expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
  expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
    extra: {
      openai_overbrush_enabled: enabled
    }
  })
})

it('未勾选修改超刷时不提交更新', async () => {
  const wrapper = mountModal({
    selectedPlatforms: ['openai'],
    selectedTypes: ['oauth']
  })

  await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
  await flushPromises()

  expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
})
```

- [ ] **Step 4: Run the payload tests and verify RED**

Run:

```bash
pnpm --dir frontend vitest run src/components/account/__tests__/BulkEditAccountModal.spec.ts -t '超刷|未勾选修改超刷'
```

Expected: FAIL because the bulk overbrush controls and payload builder branch do not exist.

- [ ] **Step 5: Add the minimal modal state and payload branch**

Near the other target eligibility computed values add a strict OAuth-only check. Do not reuse `allOpenAIOAuth`, because that existing value intentionally includes setup-token accounts:

```ts
const allOpenAIOverbrushEligible = computed(() => {
  return (
    targetSelectedPlatforms.value.length === 1 &&
    targetSelectedPlatforms.value[0] === 'openai' &&
    targetSelectedTypes.value.length === 1 &&
    targetSelectedTypes.value[0] === 'oauth'
  )
})
```

In the field-enable state block add:

```ts
const enableOpenAIOverbrush = ref(false)
```

In the field-value state block add:

```ts
const openAIOverbrushEnabled = ref(false)
```

In `buildUpdatePayload`, near the other OpenAI `extra` fields, add:

```ts
if (enableOpenAIOverbrush.value) {
  const extra = ensureExtra()
  extra.openai_overbrush_enabled = openAIOverbrushEnabled.value
}
```

Do not assign `updates.schedulable` anywhere.

- [ ] **Step 6: Add the eligible-only template control**

Place the new block near the existing OpenAI OAuth-specific controls:

```vue
<!-- OpenAI OAuth overbrush -->
<div v-if="allOpenAIOverbrushEligible" class="border-t border-gray-200 pt-4 dark:border-dark-600">
  <div class="mb-3 flex items-center justify-between">
    <div class="flex-1 pr-4">
      <label
        id="bulk-edit-openai-overbrush-label"
        class="input-label mb-0"
        for="bulk-edit-openai-overbrush-enabled"
      >
        {{ t('admin.accounts.openai.overbrush') }}
      </label>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.openai.overbrushDesc') }}
      </p>
    </div>
    <input
      v-model="enableOpenAIOverbrush"
      id="bulk-edit-openai-overbrush-enabled"
      type="checkbox"
      aria-controls="bulk-edit-openai-overbrush-body"
      class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
    />
  </div>
  <div
    id="bulk-edit-openai-overbrush-body"
    :class="!enableOpenAIOverbrush && 'pointer-events-none opacity-50'"
    role="group"
    aria-labelledby="bulk-edit-openai-overbrush-label"
  >
    <button
      id="bulk-edit-openai-overbrush-toggle"
      type="button"
      :disabled="!enableOpenAIOverbrush"
      :class="[
        'relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
        enableOpenAIOverbrush ? 'cursor-pointer' : 'cursor-not-allowed',
        openAIOverbrushEnabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
      ]"
      @click="openAIOverbrushEnabled = !openAIOverbrushEnabled"
    >
      <span
        :class="[
          'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
          openAIOverbrushEnabled ? 'translate-x-5' : 'translate-x-0'
        ]"
      />
    </button>
  </div>
</div>
```

- [ ] **Step 7: Run the focused component tests and verify GREEN**

Run:

```bash
pnpm --dir frontend vitest run src/components/account/__tests__/BulkEditAccountModal.spec.ts
```

Expected: all tests in `BulkEditAccountModal.spec.ts` PASS with zero failures.

- [ ] **Step 8: Run frontend static and build verification**

Run:

```bash
pnpm --dir frontend typecheck
pnpm --dir frontend build
```

Expected: both commands exit 0.

- [ ] **Step 9: Review the final diff and commit**

Run:

```bash
git diff --check
git diff -- frontend/src/components/account/BulkEditAccountModal.vue frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts
git add frontend/src/components/account/BulkEditAccountModal.vue frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts
git commit -m "feat: add OpenAI overbrush bulk edit"
```

Expected: the diff contains only the eligible-only control, its two state values, one payload branch, and focused tests; commit succeeds.
