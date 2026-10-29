import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import AccountPelicanHistoryView from '../AccountPelicanHistoryView.vue'

const { list, account } = vi.hoisted(() => ({ list: vi.fn(), account: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { listPelicanHistory: list, getById: account } } }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
const route = reactive({ params: { id: '7' } })
vi.mock('vue-router', () => ({ useRoute: () => route }))
const row = { id: 100, account_id: 7, model_id: 'gpt-5.5', status: 'previewable', source_group_id: 78, source_group_name: '正常分组', created_at: '2026-10-29T08:00:00Z', input_tokens: 101, output_tokens: 2048, latency_ms: 3000 }
const create = () => mount(AccountPelicanHistoryView, { global: { stubs: {
  AppLayout: { template: '<div><slot/></div>' }, RouterLink: { template: '<a><slot/></a>' }, Icon: true,
  PelicanInlinePreview: { props: ['testId', 'responsive'], template: '<div class="artwork">Animation</div>' },
  PelicanResultDialog: { props: ['show', 'testId', 'accountName'], template: '<div class="detail" v-if="show">{{testId}} / {{accountName}}</div>' },
  Pagination: defineComponent({ props: ['total', 'page'], emits: ['update:page'], template: '<button class="next-page" @click="$emit(\'update:page\',2)">Next</button>' })
} } })
beforeEach(() => { vi.clearAllMocks(); route.params.id = '7'; list.mockResolvedValue({ items: [row], total: 25 }); account.mockResolvedValue({ name: '测试账号' }) })

describe('Account pelican archive', () => {
  it('shows per-test provenance, time, tokens, preview and detail then paginates', async () => {
    const wrapper = create(); await flushPromises()
    expect(list).toHaveBeenCalledWith(7, 1, 12)
    expect(wrapper.text()).toContain('测试账号'); expect(wrapper.text()).toContain('正常分组')
    expect(wrapper.text()).toContain('gpt-5.5'); expect(wrapper.text()).toContain('2,048')
    expect(wrapper.get('time').attributes('datetime')).toBe(row.created_at)
    await wrapper.get('article button').trigger('click')
    expect(wrapper.get('.detail').text()).toBe('100 / 测试账号')
    await wrapper.get('.next-page').trigger('click'); await flushPromises()
    expect(list).toHaveBeenLastCalledWith(7, 2, 12)
    wrapper.unmount()
  })
  it('offers a retry after errors and a useful empty state', async () => {
    list.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    list.mockResolvedValue({ items: [], total: 0 })
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('pelican.emptyHint'); expect(wrapper.find('article').exists()).toBe(false)
    wrapper.unmount()
  })
})
