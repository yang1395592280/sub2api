import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { useAccountGenerationTests } from '../useAccountGenerationTests'
import type { PelicanTest } from '@/api/admin/accounts'
const { latest, warning } = vi.hoisted(() => ({ latest: vi.fn(), warning: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { listLatestPelicanTests: latest } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showWarning: warning }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, args: unknown) => JSON.stringify({ key, args }) }) }))
const row = (id: number, batch: string, status: string, account = 1) => ({ id, batch_id: batch, account_id: account, status }) as PelicanTest
function create() {
  let state!: ReturnType<typeof useAccountGenerationTests>
  const wrapper = mount(defineComponent({ setup() { state = useAccountGenerationTests('intelligence', ref([{ id: 1, name: 'A' }]), ref([1])); return {} }, template: '<div/>' }))
  return { wrapper, get state() { return state } }
}
beforeEach(() => { vi.clearAllMocks(); localStorage.clear(); latest.mockResolvedValue([]) })
describe('multi-question account batches', () => {
  it('counts independent records and polls the submitted batch when a newer batch replaces the account summary', async () => {
    const { wrapper, state } = create(); await flushPromises()
    latest.mockImplementation((_ids, _kind, batch) => Promise.resolve(batch ? [row(1, 'first', 'correct'), row(2, 'first', 'incorrect')] : [row(3, 'later', 'queued')]))
    state.submitted([row(1, 'first', 'queued'), row(2, 'first', 'queued')], 3)
    expect(state.progress.value.total).toBe(2)
    expect(state.batch.value?.ids).toEqual([1])
    await flushPromises()
    expect(latest).toHaveBeenCalledWith([1], 'intelligence', 'first')
    expect(state.progress.value).toEqual({ total: 2, done: 2, success: 1, failed: 1 })
    expect(state.records.value[1][0].batch_id).toBe('later')
    expect(warning).toHaveBeenCalledWith(expect.stringContaining('"count":1'))
    expect(localStorage.getItem('account-intelligence-active-batch-7')).toBeNull()
    wrapper.unmount()
  })
  it('restores all record IDs and keeps summaries and result lists for the whole latest batch', async () => {
    localStorage.setItem('account-intelligence-active-batch-7', JSON.stringify({ batchId: 'batch', ids: [1], recordIds: [1, 2] }))
    latest.mockResolvedValue([row(2, 'batch', 'running'), row(1, 'batch', 'correct')])
    const { wrapper, state } = create(); await flushPromises()
    expect(state.progress.value).toEqual({ total: 2, done: 1, success: 1, failed: 0 })
    expect(state.summaries.value[1].total).toBe(2)
    state.openResult({ id: 1, name: 'A' })
    expect(state.resultRecords.value).toHaveLength(2)
    expect(state.resultAccountName.value).toBe('A')
    wrapper.unmount()
  })
})
