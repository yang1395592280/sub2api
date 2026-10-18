import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import GroupMonitorPanel from '../GroupMonitorPanel.vue'
const { list, save, run, history, getAll, candidates } = vi.hoisted(() => ({ list: vi.fn(), save: vi.fn(), run: vi.fn(), history: vi.fn(), getAll: vi.fn(), candidates: vi.fn() }))
vi.mock('@/api/groupMonitor', () => ({ groupMonitorAPI: { list, save, run, history, getRun: vi.fn(async (_admin: boolean, _id: number, runId: number) => (await history()).find((r: { id: number }) => r.id === runId)), review: vi.fn(), remove: vi.fn() } }))
vi.mock('@/api/admin/groups', () => ({ getAll, getModelAllowlistCandidates: candidates }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const BaseDialog = defineComponent({ props: { show: Boolean }, template: '<section v-if="show"><slot /></section>' })
const config = { models: ['model-a'], enabled: true, probe_interval_seconds: 300, pelican_enabled: true, pelican_interval_seconds: 3600, pelican_prompt: 'Draw', daily_request_limit: 500, keep_history: 100 }
const plan = { id: 1, group_id: 9, group_name: 'Test group', platform: 'openai', config, latest_runs: [] }
let wrapper: ReturnType<typeof mount>
const button = (text: string) => wrapper.findAll('button').find(b => b.text() === text)!
beforeEach(() => { vi.clearAllMocks(); list.mockResolvedValue([plan]); getAll.mockResolvedValue([{ id: 9, name: 'Test group', platform: 'openai' }]); save.mockResolvedValue({}); history.mockResolvedValue([]) })
afterEach(() => wrapper?.unmount())
describe('group monitor configuration and access', () => {
  it('creates a paused batch plan and never triggers a paid probe on save', async () => {
    wrapper = mount(GroupMonitorPanel, { props: { admin: true }, global: { stubs: { BaseDialog } } }); await flushPromises()
    await button('groupMonitor.configure').trigger('click'); await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.findAll('textarea')[0]!.setValue('model-a\nmodel-b')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith([9], expect.objectContaining({ models: ['model-a', 'model-b'], enabled: false, pelican_enabled: false }))
    expect(run).not.toHaveBeenCalled()
  })
  it('shows only read actions for users and keeps generated HTML in an opaque sandbox', async () => {
    list.mockResolvedValue([{ ...plan, config: undefined, models: ['model-a'], pelican_visible: true }])
    history.mockResolvedValue([{ id: 8, kind: 'pelican', model: 'model-a', status: 'success', created_at: '2026-10-18T01:00:00Z', html: '<html><script>fetch("https://invalid")</script></html>', review: 'pass' }])
    wrapper = mount(GroupMonitorPanel, { global: { stubs: { BaseDialog } } }); await flushPromises()
    expect(wrapper.text()).not.toContain('groupMonitor.configure'); expect(wrapper.text()).not.toContain('groupMonitor.probe')
    await button('groupMonitor.pelican').trigger('click'); await flushPromises()
    const frame = wrapper.get('iframe'); expect(frame.attributes('sandbox')).toBe('allow-scripts'); expect(frame.attributes('srcdoc')).toContain("connect-src 'none'")
    expect(wrapper.text()).not.toContain('groupMonitor.account'); expect(wrapper.find('#pelican-review').exists()).toBe(false)
    expect(history).toHaveBeenCalledWith(false, 1, 'pelican', '')
  })
  it('shows the actual sampled account and original upstream IDs to admins', async () => {
    history.mockResolvedValue([{ id: 8, kind: 'pelican', model: 'model-a', status: 'failed', created_at: '2026-10-18T01:00:00Z', detail: { account_id: 42, account_name: 'sampled', upstream_message_id: 'msg-real', upstream_request_id: 'req-real', trace_id: 'gm-trace', review: 'unreviewed', latency_ms: 10 } }])
    wrapper = mount(GroupMonitorPanel, { props: { admin: true }, global: { stubs: { BaseDialog } } }); await flushPromises()
    await button('groupMonitor.pelican').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('sampled'); expect(wrapper.text()).toContain('#42'); expect(wrapper.text()).toContain('msg-real'); expect(wrapper.text()).toContain('req-real'); expect(wrapper.text()).toContain('gm-trace')
  })
})
