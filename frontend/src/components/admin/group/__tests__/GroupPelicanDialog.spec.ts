import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import GroupPelicanDialog from '../GroupPelicanDialog.vue'

const { get, save, models, questions } = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn(), models: vi.fn(), questions: vi.fn() }))
vi.mock('@/api/admin/pelican', () => ({ getGroupPelicanConfig: get, saveGroupPelicanConfig: save, listTestQuestions: questions }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getModelAllowlistCandidates: models } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
const BaseDialog = defineComponent({ props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' })
const config = { group_id: 7, enabled: false, model_id: '', prompt: 'Draw a pelican', question_ids: [1, 2], interval_minutes: 30, last_run_at: null, next_run_at: null }
const create = () => mount(GroupPelicanDialog, { props: { show: true, group: { id: 7, name: '0.1 分组' } }, global: { stubs: { BaseDialog, RouterLink: true } } })
beforeEach(() => { vi.clearAllMocks(); questions.mockResolvedValue([{ id: 1, name: 'First', prompt: 'A', enabled: true }, { id: 2, name: 'Second', prompt: 'B', enabled: true }]); get.mockResolvedValue({ ...config }); models.mockResolvedValue(['gpt-5.5', 'gpt-image-1']); save.mockResolvedValue(config) })

describe('GroupPelicanDialog', () => {
  it('loads and saves the independent intelligence schedule with multiple questions and a configurable interval', async () => {
    get.mockResolvedValue({ ...config, prompt: '固定糖果题' })
    const wrapper = mount(GroupPelicanDialog, { props: { show: true, intelligence: true, group: { id: 7, name: '分组' } }, global: { stubs: { BaseDialog, RouterLink: true } } })
    await flushPromises()
    expect(get).toHaveBeenCalledWith(7, 'intelligence')
    expect(questions).toHaveBeenCalledWith('intelligence')
    expect(wrapper.findAll('[data-question-id]')).toHaveLength(2)
    await wrapper.get('#group-test-interval').setValue(45)
    await wrapper.get('[type="checkbox"]').setValue(true)
    await wrapper.get('#group-pelican-model').setValue('gpt-5.5')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, { enabled: true, model_id: 'gpt-5.5', prompt: '固定糖果题', interval_minutes: 45, question_ids: [1, 2] }, 'intelligence')
    wrapper.unmount()
  })
  it('starts disabled, accepts custom models and only saves its own config', async () => {
    const wrapper = create(); await flushPromises()
    expect(wrapper.text()).toContain('0.1 分组')
    expect(wrapper.get<HTMLInputElement>('[type="checkbox"]').element.checked).toBe(false)
    expect(wrapper.findAll('datalist option').map(option => option.attributes('value'))).toEqual(['gpt-5.5'])
    await wrapper.get('[type="checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#group-pelican-model').setValue('vendor/my-model')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, { enabled: true, model_id: 'vendor/my-model', prompt: config.prompt, interval_minutes: 30, question_ids: [1, 2] }, 'pelican')
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it('permits turning off without a model when suggestions are unavailable', async () => {
    models.mockRejectedValue(new Error('offline'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.text()).toContain('pelican.modelUnavailable')
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, { enabled: false, model_id: '', prompt: config.prompt, interval_minutes: 30, question_ids: [1, 2] }, 'pelican')
    wrapper.unmount()
  })

  it('blocks saving on config load failure and supports retry', async () => {
    get.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeDefined()
    await wrapper.get('button').trigger('click'); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(save).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('rejects intervals outside the range and empty enabled selections', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.get('[data-test="pelican-group-enabled"]').setValue(true)
    await wrapper.get('#group-pelican-model').setValue('text-model')
    await wrapper.get('#group-test-interval').setValue(4)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#group-test-interval').setValue(1440)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-question-id="1"]').setValue(false)
    await wrapper.get('[data-question-id="2"]').setValue(false)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('allows disabling a schedule when the question library is unavailable', async () => {
    questions.mockRejectedValue(new Error('offline'))
    get.mockResolvedValue({ ...config, enabled: true })
    const wrapper = create(); await flushPromises()
    await wrapper.get('[data-test="pelican-group-enabled"]').setValue(false)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, expect.objectContaining({ enabled: false }), 'pelican')
    wrapper.unmount()
  })

})
