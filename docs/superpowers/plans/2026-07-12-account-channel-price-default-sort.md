# Account Channel Price Default Sort Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the admin account list default to channel price ascending while preserving each user's saved manual sort.

**Architecture:** Keep sorting server-side through the existing account list API. Change only the account page fallback sort state and matching `DataTable` defaults; the existing local-storage state remains authoritative when present.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils

## Global Constraints

- Default sort is exactly `sort_by=channel_price` and `sort_order=asc`.
- A valid persisted `account-table-sort` value continues to override the default.
- Do not change backend defaults or database ordering behavior.

---

### Task 1: Account list default sort

**Files:**
- Create: `frontend/src/views/admin/__tests__/AccountsView.defaultSort.spec.ts`
- Modify: `frontend/src/views/admin/AccountsView.vue:190-202`
- Modify: `frontend/src/views/admin/AccountsView.vue:628-632`

**Interfaces:**
- Consumes: `adminAPI.accounts.list(page, pageSize, filters, options)` and local-storage key `account-table-sort`.
- Produces: initial account request filters and `DataTable` default props using `channel_price asc`.

- [ ] **Step 1: Write the failing default-sort tests**

Create a focused `AccountsView.defaultSort.spec.ts` using the same API/store/i18n mocks and component stubs as the existing `AccountsView.usageWindowsHint.spec.ts`. The `DataTable` stub must declare `defaultSortKey` and `defaultSortOrder` props and render them as attributes.

Add these assertions:

```ts
it('defaults the first account request and table indicator to channel price ascending', async () => {
  const wrapper = mountView()
  await flushPromises()

  expect(listAccounts).toHaveBeenCalledWith(
    1,
    20,
    expect.objectContaining({
      sort_by: 'channel_price',
      sort_order: 'asc'
    }),
    expect.any(Object)
  )
  expect(wrapper.get('[data-test="data-table"]').attributes('data-sort-key')).toBe('channel_price')
  expect(wrapper.get('[data-test="data-table"]').attributes('data-sort-order')).toBe('asc')
})

it('keeps a valid persisted sort instead of replacing it with the default', async () => {
  localStorage.setItem('account-table-sort', JSON.stringify({ key: 'created_at', order: 'desc' }))

  mountView()
  await flushPromises()

  expect(listAccounts).toHaveBeenCalledWith(
    1,
    20,
    expect.objectContaining({
      sort_by: 'created_at',
      sort_order: 'desc'
    }),
    expect.any(Object)
  )
})
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AccountsView.defaultSort.spec.ts
```

Expected: the first test fails because the request and `DataTable` props still use `name asc`; the persisted-sort test passes.

- [ ] **Step 3: Implement the minimal default change**

In `AccountsView.vue`, change the table defaults:

```vue
default-sort-key="channel_price"
default-sort-order="asc"
```

Change the fallback in `loadInitialAccountSortState`:

```ts
const fallback: AccountSortState = { sort_by: 'channel_price', sort_order: 'asc' }
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AccountsView.defaultSort.spec.ts src/components/common/__tests__/DataTable.spec.ts
```

Expected: both test files pass with zero failures.

- [ ] **Step 5: Run static verification**

Run:

```bash
cd frontend && pnpm typecheck
cd frontend && pnpm eslint src/views/admin/AccountsView.vue src/views/admin/__tests__/AccountsView.defaultSort.spec.ts
```

Expected: both commands exit 0 with no errors.

- [ ] **Step 6: Review and commit**

Run `git diff --check` and inspect `git diff` to confirm only the planned defaults and test coverage changed, then commit:

```bash
git add frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/__tests__/AccountsView.defaultSort.spec.ts docs/superpowers/plans/2026-07-12-account-channel-price-default-sort.md
git commit -m "feat: default accounts to channel price sort"
```
