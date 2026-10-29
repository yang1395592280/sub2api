import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import GroupPelicanDialog from '../GroupPelicanDialog.vue'

const { get, save, models } = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn(), models: vi.fn() }))
vi.mock('@/api/admin/pelican', () => ({ getGroupPelicanConfig: get, saveGroupPelicanConfig: save }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getModelAllowlistCandidates: models } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
const BaseDialog = defineComponent({ props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' })
const config = { group_id: 7, enabled: false, model_id: '', prompt: 'Draw a pelican', interval_minutes: 30, last_run_at: null, next_run_at: null }
const create = () => mount(GroupPelicanDialog, { props: { show: true, group: { id: 7, name: '0.1 分组' } }, global: { stubs: { BaseDialog } } })
beforeEach(() => { vi.clearAllMocks(); get.mockResolvedValue({ ...config }); models.mockResolvedValue(['gpt-5.5', 'gpt-image-1']); save.mockResolvedValue(config) })

describe('GroupPelicanDialog', () => {
  it('starts disabled, accepts custom models and only saves its own config', async () => {
    const wrapper = create(); await flushPromises()
    expect(wrapper.text()).toContain('0.1 分组')
    expect(wrapper.get<HTMLInputElement>('[type="checkbox"]').element.checked).toBe(false)
    expect(wrapper.findAll('datalist option').map(option => option.attributes('value'))).toEqual(['gpt-5.5'])
    await wrapper.get('[type="checkbox"]').setValue(true)
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#group-pelican-model').setValue('vendor/my-model')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, { enabled: true, model_id: 'vendor/my-model', prompt: config.prompt })
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it('permits turning off without a model when suggestions are unavailable', async () => {
    models.mockRejectedValue(new Error('offline'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.text()).toContain('pelican.modelUnavailable')
    expect(wrapper.get('[data-test="save-group-pelican"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, { enabled: false, model_id: '', prompt: config.prompt })
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
})
