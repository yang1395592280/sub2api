import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh/openAIQuality'
import Panel from '../QualityResultsPanel.vue'
import Detail from '../QualityRunDetailDialog.vue'
import CandyDetail from '../QualityCandyDetail.vue'
import { qualityVisualOutcome } from '../qualityPresentation'
import type { QualityRun } from '@/api/admin/openaiAccountQuality'
const api = vi.hoisted(() => ({ runs: vi.fn(), gallery: vi.fn() }))
vi.mock('@/api/admin/openaiAccountQuality', () => ({ getQualityRuns: api.runs, getQualityGallery: api.gallery }))
vi.mock('vue-i18n', async importOriginal => {
  const { default: messages } = await import('@/i18n/locales/zh/openAIQuality')
  const lookup = (key: string): unknown => key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, messages)
  return { ...await importOriginal<object>(), useI18n: () => ({ t: (key: string) => lookup(key) ?? key, te: (key: string) => typeof lookup(key) === 'string' }) }
})
const metrics = { latency_ms: 2000, input_tokens: 200, output_tokens: 100, total_tokens: 300, estimated_cost_usd: null, response_truncated: false }
const sample = (id: number, verdict: string): QualityRun => ({ id, account_id: 42, policy_id: 1, status: 'completed', verdict, reason_code: '', queued_at: '2026-10-28T08:00:00Z', started_at: null, finished_at: '2026-10-28T08:01:00Z', trigger_source: 'manual', state_applied: true, state_skip_reason: '', result: { ...metrics, verdict, reason_code: '', final_answer: 21, expected_answer: 21, candy: { ...metrics, verdict: 'passed', reason_code: 'correct_answer', final_answer: 21 }, pelican: { ...metrics, verdict, reason_code: 'visual_failed', previewable: true }, review: { ...metrics, verdict, reason_code: 'visual_failed', checks: { pelican: true, bicycle: true, riding: false, motion: null } } } })
const wrappers: ReturnType<typeof mount>[] = []
const options = () => ({ global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh }, missingWarn: false, fallbackWarn: false })], stubs: { RouterLink: RouterLinkStub, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } } })
beforeEach(() => {
  vi.clearAllMocks()
  api.runs.mockResolvedValue({ items: [sample(1, 'passed'), sample(2, 'incorrect'), sample(3, 'unknown'), sample(4, 'error')], total: 4 })
  api.gallery.mockResolvedValue({ items: [sample(2, 'incorrect')], total: 1 })
})
afterEach(() => wrappers.splice(0).forEach(w => w.unmount()))
describe('dual quality results', () => {
  it('distinguishes missing setup, processing errors, uncertain review and explicit failure', () => {
    const check = sample(1, 'unknown').result.pelican!
    expect(qualityVisualOutcome({ ...check, reason_code: 'review_not_configured' })).toBe('not_reviewed')
    expect(qualityVisualOutcome({ ...check, reason_code: 'renderer_not_configured' })).toBe('not_reviewed')
    expect(qualityVisualOutcome({ ...check, reason_code: 'render_failed' })).toBe('visual_check_error')
    expect(qualityVisualOutcome({ ...check, verdict: 'error', reason_code: 'upstream_http_429' })).toBe('visual_check_error')
    expect(qualityVisualOutcome({ ...check, reason_code: 'review_uncertain' })).toBe('visual_uncertain')
    expect(qualityVisualOutcome({ ...check, verdict: 'incorrect', reason_code: 'visual_failed' })).toBe('incorrect')
    expect(qualityVisualOutcome(undefined, 'running')).toBe('running')
  })
  it('shows missing review configuration and standards without inventing reviewed checks', async () => {
    const run = sample(1, 'unknown')
    run.input_snapshot = { condition: { benchmark_id: 'dual', model_id: 'test-model' } } as QualityRun['input_snapshot']
    run.result.pelican!.reason_code = 'review_not_configured'
    run.result.review = undefined
    const wrapper = mount(Detail, { ...options(), props: { detail: { run, response_available: true, response_expired: false } } }); wrappers.push(wrapper)
    await wrapper.findAll('button').find(b => b.text().includes('鹈鹕动画'))!.trigger('click')
    expect(wrapper.text()).toContain('未配置评审账号与模型')
    expect(wrapper.text()).toContain('至少一只可见脚接触曲柄末端的踏板')
    expect(wrapper.text()).toContain('旧记录不会自动补评')
    expect(wrapper.findComponent(RouterLinkStub).props('to')).toBe('/admin/openai-quality/settings?section=global')
    expect(wrapper.text()).not.toContain('画面证据不足，无法判定')
  })
  it('renders real explanation headings and tables but removes executable content and remote resources', async () => {
    const detail = { run: sample(1, 'passed'), response_available: true, response_expired: false, candy: { question: '完整题目：圆形7、9、8；五角星7、6、4。', prompt: 'actual prompt', explanation_requested: true, explanation: '## 数量选择\n\n| 形状 | 数量 |\n| --- | --- |\n| 圆形 | 1 |\n\n<script>alert(1)</script><img src="https://external.test/track" onerror="alert(1)"><a href="javascript:alert(1)">unsafe link</a><iframe src="https://external.test"></iframe>' } }
    const wrapper = mount(CandyDetail, { ...options(), props: { detail, answer: 21 } }); wrappers.push(wrapper)
    expect(wrapper.find('h2').text()).toBe('数量选择')
    expect(wrapper.find('table').text()).toContain('圆形')
    expect(wrapper.findAll('script,img,a,iframe,[src],[href],[style]')).toHaveLength(0)
    expect(wrapper.text()).toContain('完整题目：圆形7、9、8')
    await wrapper.setProps({ detail: { ...detail, response_expired: true } })
    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.text()).toContain('按保留策略清理')
    await wrapper.setProps({ detail: { ...detail, response_available: false } })
    expect(wrapper.find('table').exists()).toBe(false)
    await wrapper.setProps({ detail: { ...detail, candy: { ...detail.candy, explanation_requested: false, explanation: '' } } })
    expect(wrapper.text()).toContain('未采集解题说明')
  })
  it('uses conclusive verdicts for the rate and opens the exact timeline run', async () => {
    const wrapper = mount(Panel, { ...options(), props: { groups: [] } }); wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.text()).toContain('50.0%')
    const timeline = wrapper.find('[aria-label="最近 60 次检测"]')
    expect(timeline.findAll('button')).toHaveLength(4)
    await timeline.findAll('button')[0].trigger('click')
    expect(wrapper.emitted('open')?.[0]).toEqual([4])
    expect(wrapper.text()).toContain('疑似降智')
    expect(api.gallery).toHaveBeenCalledWith(expect.objectContaining({ page_size: 12, page: 1 }))
  })
  it('shows a retryable failure and does not claim an empty gallery succeeded', async () => {
    api.gallery.mockRejectedValue(new Error('offline'))
    const wrapper = mount(Panel, { ...options(), props: { groups: [] } }); wrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('offline')
  })
  it('shows the four visual criteria separately from the candy answer', async () => {
    const run = sample(2, 'incorrect')
    run.input_snapshot = { condition: { benchmark_id: 'dual', model_id: 'test-model' } } as QualityRun['input_snapshot']
    run.evidence = { candy: '{"answer":21}', pelican: '<svg>source</svg>', svg: '', review: '<script>ignored</script>' }
    const wrapper = mount(Detail, { ...options(), props: { detail: { run, response_available: true, response_expired: false } } }); wrappers.push(wrapper)
    await wrapper.findAll('button').find(b => b.text().includes('视觉评审'))!.trigger('click')
    expect(wrapper.text()).toContain('鹈鹕形态')
    expect(wrapper.text()).toContain('骑乘姿态')
    expect(wrapper.find('pre').text()).toContain('<script>ignored</script>')
    expect(wrapper.find('script').exists()).toBe(false)
    await wrapper.findAll('button').find(b => b.text().includes('糖果逻辑题'))!.trigger('click')
    expect(wrapper.find('pre').text()).toBe('{"answer":21}')
  })
  it('resets the gallery page and clears stale data when a new model query fails', async () => {
    api.gallery.mockResolvedValue({ items: [sample(2, 'incorrect')], total: 30 })
    const wrapper = mount(Panel, { ...options(), props: { groups: [] } }); wrappers.push(wrapper)
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '下一页')!.trigger('click')
    await flushPromises()
    expect(api.gallery).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
    await wrapper.find('input').setValue('different-model')
    api.gallery.mockRejectedValue(new Error('offline'))
    await wrapper.find('input').trigger('keyup.enter')
    await flushPromises()
    expect(api.gallery).toHaveBeenLastCalledWith(expect.objectContaining({ model_id: 'different-model', page: 1 }))
    expect(wrapper.find('[role="alert"]').text()).toContain('offline')
    expect(wrapper.findAll('aside button')).toHaveLength(0)
  })
})
