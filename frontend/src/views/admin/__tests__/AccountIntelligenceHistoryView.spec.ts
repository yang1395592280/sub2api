import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import AccountIntelligenceHistoryView from '../AccountIntelligenceHistoryView.vue'

const { list, account } = vi.hoisted(() => ({ list: vi.fn(), account: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { listPelicanHistory: list, getById: account } } }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: '7' } }) }))
const row = { account_id: 7, model_id: 'gpt-5.5', created_at: '2026-11-13T08:00:00Z', latency_ms: 500 }
const create = () => mount(AccountIntelligenceHistoryView, { global: { stubs: {
  AppLayout: { template: '<div><slot/></div>' }, RouterLink: { template: '<a><slot/></a>' }, Icon: true,
  PelicanResultDialog: { props: ['show', 'testId', 'intelligence'], template: '<div v-if="show" class="detail">{{testId}} / {{intelligence}}</div>' },
  Pagination: defineComponent({ emits: ['update:page'], template: '<button class="next" @click="$emit(\'update:page\',2)">Next</button>' })
} } })
beforeEach(() => {
  vi.clearAllMocks(); account.mockResolvedValue({ name: '账号 A' })
  list.mockResolvedValue({ total: 30, items: [{ ...row, id: 1, status: 'correct', answer: '20' }, { ...row, id: 2, status: 'incorrect', answer: '29' }, { ...row, id: 3, status: 'failed', answer: '' }] })
})
describe('Intelligence history timeline', () => {
  it('shows green/red answers and failed requests, opens evidence and paginates only intelligence records', async () => {
    const wrapper = create(); await flushPromises()
    expect(list).toHaveBeenCalledWith(7, 1, 20, 'intelligence')
    expect(wrapper.get('[data-test="intelligence-result-1"]').classes()).toContain('text-emerald-700')
    expect(wrapper.get('[data-test="intelligence-result-2"]').text()).toBe('29')
    expect(wrapper.get('[data-test="intelligence-result-2"]').classes()).toContain('text-red-700')
    expect(wrapper.get('[data-test="intelligence-result-3"]').text()).toBe('×')
    await wrapper.get('[data-test="intelligence-result-2"]').trigger('click')
    expect(wrapper.get('.detail').text()).toBe('2 / true')
    await wrapper.get('.next').trigger('click'); await flushPromises()
    expect(list).toHaveBeenLastCalledWith(7, 2, 20, 'intelligence')
    wrapper.unmount()
  })
  it('shows an actionable error and then an empty state after retry', async () => {
    list.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    list.mockResolvedValue({ items: [], total: 0 })
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('pelican.intelligence.empty')
    wrapper.unmount()
  })
})
