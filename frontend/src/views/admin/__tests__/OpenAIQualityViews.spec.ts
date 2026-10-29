import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { defineComponent, ref, type Component } from 'vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import zh from '@/i18n/locales/zh/openAIQuality'
import type { QualityPolicy, QualityRun } from '@/api/admin/openaiAccountQuality'
import SettingsView from '../OpenAIQualitySettingsView.vue'
import RecordsView from '../OpenAIQualityRecordsView.vue'
import MonitorView from '../OpenAIQualityMonitorView.vue'

vi.mock('vue-i18n', async importOriginal => {
  const { default: messages } = await import('@/i18n/locales/zh/openAIQuality')
  const lookup = (key: string): unknown => key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, messages)
  return { ...await importOriginal<object>(), useI18n: () => ({
    t: (key: string, values?: Record<string, unknown>) => {
      const message = lookup(key)
      return typeof message === 'string' ? message.replace(/\{(\w+)\}/g, (_, name: string) => String(values?.[name] ?? `{${name}}`)) : key
    },
    te: (key: string) => typeof lookup(key) === 'string',
  }) }
})

const api = vi.hoisted(() => ({
  settings: vi.fn(), targets: vi.fn(), policies: vi.fn(), groups: vi.fn(), preview: vi.fn(), settingsPreview: vi.fn(), create: vi.fn(), runs: vi.fn(), run: vi.fn(), overview: vi.fn(), monitor: vi.fn(), gallery: vi.fn(),
  route: { path: '/admin/openai-quality/records', query: {} as Record<string, string> }, replace: vi.fn(),
}))
vi.mock('@/api/admin/openaiAccountQuality', async importOriginal => ({
  ...await importOriginal<object>(),
  getQualitySettings: api.settings, getAllQualityTargets: api.targets, getAllQualityPolicies: api.policies,
  previewQuality: api.preview, createQualityPolicies: api.create, getQualityRuns: api.runs, getQualityRun: api.run, getQualityOverview: api.overview, getQualityMonitor: api.monitor, getQualityGallery: api.gallery,
  previewQualitySettings: api.settingsPreview,
}))
vi.mock('@/api/admin/groups', async importOriginal => ({ ...await importOriginal<object>(), getAll: api.groups, getAllIncludingInactive: api.groups }))
vi.mock('vue-router', async importOriginal => ({ ...await importOriginal<object>(), useRoute: () => api.route, useRouter: () => ({ replace: api.replace }) }))

const rules = { auto_check_enabled: false, control_enabled: false, interval_seconds: 1800, jitter_seconds: 60, failure_threshold: 3, recovery_threshold: 2, cooldown_seconds: 1800, recovery_interval_seconds: 600, evidence_window_seconds: 7200, timeout_seconds: 180, max_output_tokens: 4096, daily_call_limit: 100 }
function policy(id = 1, model = 'gpt-5.5'): QualityPolicy {
  return { id, account_id: 42, account_name: 'Shared upstream', source_group_id: 78, compatible_source_group_ids: [78], condition: { model_id: model, protocol: 'responses', transport: 'http', reasoning_effort: 'upstream_default', service_tier: 'upstream_default', benchmark_id: 'candy', benchmark_version: '1', scorer_version: 'strict-json-v1' }, effective_upstream_model: model, effective_reasoning_effort: 'upstream_default', effective_service_tier: 'upstream_default', scope_hash: 'scope', revision: 1, control_epoch: 1, eligibility: 'eligible', rules: { ...rules }, state: { state: 'normal', freshness: 'fresh', failure_streak: 0, recovery_streak: 0, cooldown_until: null, last_checked_at: '2026-10-28T07:00:00Z', last_verdict: 'passed' }, control: { effective: true, action: 'baseline', reason: 'enabled' } }
}
function run(): QualityRun {
  return { id: 100, policy_id: 1, account_id: 42, trigger_source: 'manual', status: 'completed', verdict: 'passed', reason_code: '', queued_at: '2026-10-28T07:00:00Z', started_at: '2026-10-28T07:00:00Z', finished_at: '2026-10-28T07:00:03Z', state_applied: true, state_skip_reason: '', input_snapshot: policy(), result: { verdict: 'passed', final_answer: 21, expected_answer: 21, reason_code: '', latency_ms: 3000, input_tokens: null, output_tokens: null, total_tokens: null, estimated_cost_usd: null, response_truncated: false } }
}
const wrappers: ReturnType<typeof mount>[] = []
function render(component: Component) {
  const wrapper = mount(component, {
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh }, missingWarn: false, fallbackWarn: false })], stubs: {
      AppLayout: { template: '<div><slot /></div>' }, RouterLink: RouterLinkStub,
      BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>' },
    } },
  })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  api.route.query = {}
  api.groups.mockResolvedValue([
    { id: 78, name: '不降智 0.2', platform: 'openai', status: 'active', group_role: 'standard', allow_auto_cheapest_scheduling: true },
    { id: 1, name: '降智 0.1', platform: 'openai', status: 'active', group_role: 'standard', allow_auto_cheapest_scheduling: false },
    { id: 2, name: '其他平台', platform: 'anthropic', status: 'active' },
  ])
  api.settings.mockResolvedValue({ settings: { revision: 1, ...rules }, health: {}, budget: {} })
  api.targets.mockResolvedValue({ items: [{ account_id: 42, account_name: 'Shared upstream', compatible_source_group_ids: [78, 1], can_probe: true, can_enforce: true, reasons: [] }] })
  api.policies.mockResolvedValue({ items: [policy(), policy(2, 'gpt-5.4')] })
  api.preview.mockResolvedValue({ valid: true, warnings: [], daily_calls_estimate: 96, global_budget: 1000 })
  api.settingsPreview.mockResolvedValue({ valid: true, warnings: [] })
  api.create.mockResolvedValue({ items: [policy()] })
  api.gallery.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 12, pages: 1 })
  api.runs.mockResolvedValue({ items: [run()], total: 41, page: 1, page_size: 20, pages: 3 })
  api.run.mockResolvedValue({ run: { ...run(), response_text: '<script>unsafe</script>{"answer":21}' }, response_available: true, response_expired: false })
  api.overview.mockResolvedValue({ health: {}, inventory: {}, policies: {} })
  api.monitor.mockResolvedValue({ complete: true, as_of: '2026-10-28T07:00:00Z', summary: {}, buckets: [] })
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); document.body.innerHTML = '' })

describe('OpenAI quality settings', () => {
  it('upgrades explanations without changing the selected suite and links directly to global settings', async () => {
    const dual = policy(2)
    dual.condition.benchmark_id = 'dual'; dual.condition.scorer_version = 'dual-visual-v1'
    api.policies.mockResolvedValue({ items: [policy(), dual] })
    api.route.query = { section: 'global' }
    const wrapper = render(SettingsView)
    await flushPromises()
    expect(wrapper.find('form').isVisible()).toBe(true)
    await wrapper.findAll('button').find(b => b.text().startsWith('已配置策略'))!.trigger('click')
    const upgrades = wrapper.findAll('button').filter(b => b.text() === '升级含解题说明')
    await upgrades[0].trigger('click'); await flushPromises()
    expect(api.preview).toHaveBeenLastCalledWith([expect.objectContaining({ condition: expect.objectContaining({ benchmark_id: 'candy', benchmark_version: '2', scorer_version: 'answer-explanation-v2' }), replaces_policy_id: 1 })])
    await upgrades[1].trigger('click'); await flushPromises()
    expect(api.preview).toHaveBeenLastCalledWith([expect.objectContaining({ condition: expect.objectContaining({ benchmark_id: 'dual', benchmark_version: '2', scorer_version: 'dual-visual-v2' }), replaces_policy_id: 2 })])
  })
  it('upgrades a legacy policy through preview and preserves its identity and switches', async () => {
    const wrapper = render(SettingsView)
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().startsWith('已配置策略'))!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === '升级双检测')!.trigger('click')
    await flushPromises()
    const expected = expect.objectContaining({ account_id: 42, source_group_id: 78, replaces_policy_id: 1, replaces_expected_revision: 1, condition: expect.objectContaining({ benchmark_id: 'dual', scorer_version: 'dual-visual-v2', benchmark_version: '2', model_id: 'gpt-5.5' }), rules: expect.objectContaining({ control_enabled: false, auto_check_enabled: false, timeout_seconds: 420, max_output_tokens: 8192 }) })
    expect(api.preview).toHaveBeenLastCalledWith([expected])
    await wrapper.findAll('button').find(b => b.text() === '确认升级')!.trigger('click')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith([expected])
  })
  it('configures a reviewer from its selected group and invalidates preview after a model edit', async () => {
    const wrapper = render(SettingsView)
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '全局设置')!.trigger('click')
    const enable = wrapper.findAll('label').find(l => l.text() === '启用视觉评审')!.find('input')
    await enable.setValue(true)
    wrapper.findAllComponents(Select).find(s => s.props('ariaLabel') === '评审来源分组')!.vm.$emit('update:modelValue', 1)
    await flushPromises()
    const account = wrapper.findAllComponents(Select).find(s => s.props('ariaLabel') === '评审账号')!
    expect(account.props('options')).toEqual([{ value: 42, label: 'Shared upstream' }])
    account.vm.$emit('update:modelValue', 42)
    await wrapper.find('input[aria-label="视觉评审模型"]').setValue('review-model')
    await wrapper.find('form').findAll('button').find(b => b.text() === '预览')!.trigger('click')
    await flushPromises()
    expect(api.settingsPreview).toHaveBeenCalledWith(expect.objectContaining({ reviewer: expect.objectContaining({ account_id: 42, source_group_id: 1, condition: expect.objectContaining({ model_id: 'review-model', benchmark_id: 'candy' }) }) }))
    await wrapper.find('input[aria-label="视觉评审模型"]').setValue('different-reviewer')
    expect(wrapper.findAll('button').find(b => b.text() === 'common.save')!.attributes('disabled')).toBeDefined()
  })
  it('allows ordinary groups and builds two independent model policies without routing control', async () => {
    const wrapper = render(SettingsView)
    await flushPromises()
    const groupSelect = wrapper.findAllComponents(Select)[0]
    expect(groupSelect.props('options')).toEqual([{ value: 78, label: '不降智 0.2' }, { value: 1, label: '降智 0.1' }])
    groupSelect.vm.$emit('update:modelValue', 1)
    await flushPromises()
    const account = wrapper.find('input[type="checkbox"][value="42"]')
    await account.setValue(true)
    await wrapper.find('input[placeholder="gpt-5.5"]').setValue('gpt-5.5')
    await wrapper.findAll('button').find(b => b.text() === '添加第二个模型')!.trigger('click')
    await wrapper.find('input[placeholder="gpt-5.4"]').setValue('gpt-5.4')
    const control = wrapper.findAll('label').find(label => label.text().includes('允许此策略控制调度'))!.find('input')
    expect(control.attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(b => b.text() === '预览')!.trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith([
      expect.objectContaining({ account_id: 42, source_group_id: 1, condition: expect.objectContaining({ model_id: 'gpt-5.5' }), rules: expect.objectContaining({ control_enabled: false }) }),
      expect.objectContaining({ account_id: 42, source_group_id: 1, condition: expect.objectContaining({ model_id: 'gpt-5.4' }), rules: expect.objectContaining({ control_enabled: false }) }),
    ])
    groupSelect.vm.$emit('update:modelValue', 78)
    await flushPromises()
    expect((wrapper.find('input[value="42"]').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.findAll('button').find(b => b.text() === '创建策略')!.attributes('disabled')).toBeDefined()
  })
})

describe('OpenAI quality records', () => {
  it('filters on the server, resets pagination and opens escaped response details', async () => {
    const wrapper = render(RecordsView)
    await flushPromises()
    expect(wrapper.text()).toContain('不降智 0.2')
    expect(wrapper.text()).toContain('共 41 条')
    await wrapper.find('input[placeholder="搜索账号名称"]').setValue('Shared')
    await wrapper.find('input[placeholder="输入完整模型名称"]').setValue('gpt-5.5')
    const selects = wrapper.findAllComponents(Select)
    selects.find(s => s.props('ariaLabel') === '来源分组')!.vm.$emit('update:modelValue', '78')
    selects.find(s => s.props('ariaLabel') === '检测结果')!.vm.$emit('update:modelValue', 'incorrect')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(api.runs).toHaveBeenLastCalledWith(expect.objectContaining({ source_group_id: 78, model_id: 'gpt-5.5', search: 'Shared', verdict: 'incorrect', page: 1, page_size: 20 }))
    const firstRange = api.runs.mock.lastCall![0]
    wrapper.findComponent(Pagination).vm.$emit('update:page', 2)
    await flushPromises()
    expect(api.runs).toHaveBeenLastCalledWith({ ...firstRange, page: 2 })
    await wrapper.findAll('button').find(b => b.text() === '查看详情')!.trigger('click')
    await flushPromises()
    expect(api.run).toHaveBeenCalledWith(100)
    expect(wrapper.find('pre').text()).toContain('<script>unsafe</script>')
    expect(wrapper.find('pre script').exists()).toBe(false)
  })
  it('opens policy history from the monitor without losing older scopes', async () => {
    api.route.query = { policy_id: '17' }
    render(RecordsView)
    await flushPromises()
    expect(api.runs).toHaveBeenCalledWith(expect.objectContaining({ policy_id: 17 }))
    expect(api.runs.mock.lastCall![0]).not.toHaveProperty('scope_hash')
  })
})

describe('OpenAI quality monitor', () => {
  it('closes cached dialogs when leaving the page and keeps them closed on return', async () => {
    const active = ref(true)
    const host = defineComponent({ components: { MonitorView }, setup: () => ({ active }), template: '<KeepAlive><MonitorView v-if="active" /></KeepAlive>' })
    const wrapper = render(host)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '时间线')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('最近 24 小时')
    active.value = false
    await flushPromises()
    active.value = true
    await flushPromises()
    expect(wrapper.text()).not.toContain('最近 24 小时')
    expect(wrapper.text()).not.toContain('全部检测记录')
  })
  it('groups models under one named source and keeps model selection independent', async () => {
    const wrapper = render(MonitorView)
    await flushPromises()
    expect(wrapper.findAll('td[rowspan="2"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('不降智 0.2')
    expect(wrapper.text()).not.toContain('分组 #78')
    const checkboxes = wrapper.findAll('tbody input[type="checkbox"]')
    await checkboxes[0].setValue(true)
    expect((checkboxes[1].element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.text()).toContain('已选择 1 条策略')
    const history = wrapper.findAllComponents(RouterLinkStub).find(link => typeof link.props('to') === 'object')!
    expect(history.props('to')).toEqual({ path: '/admin/openai-quality/records', query: { account_id: 42, source_group_id: 78 } })
  })
})
