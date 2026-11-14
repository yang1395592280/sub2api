import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PelicanBatchTestDialog from '../PelicanBatchTestDialog.vue'


const { getAvailableModels, startPelicanTests, listTestQuestions, getTestPreference } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  listTestQuestions: vi.fn(), getTestPreference: vi.fn(),
  startPelicanTests: vi.fn()
}))
vi.mock('@/api/admin/pelican', () => ({ listTestQuestions, getTestPreference }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getAvailableModels, startPelicanTests } } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})
const SelectStub = defineComponent({
  props: { modelValue: String },
  template: '<span data-test="selected-model">{{ modelValue }}</span>'
})

describe('PelicanBatchTestDialog', () => {
  beforeEach(() => {
    listTestQuestions.mockResolvedValue([{ id: 1, name: 'First', prompt: 'Question 1', enabled: true, expected_answer: '42' }, { id: 2, name: 'Second', prompt: 'Question 2', enabled: true, expected_answer: '7' }])
    getTestPreference.mockResolvedValue({ question_ids: [] })
    getAvailableModels.mockReset()
    startPelicanTests.mockReset().mockResolvedValue([])
  })

  it('sends the selected intelligence questions through the intelligence endpoint', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, intelligence: true, accountIds: [11], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    expect(listTestQuestions).toHaveBeenCalledWith('intelligence')
    await wrapper.get('[data-question-id="2"]').setValue(true)
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click'); await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11], 'gpt-6-astra', [1, 2], 'intelligence')
    wrapper.unmount()
  })

  it('waits for confirmation and submits the default selection with the Astra model ID', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    startPelicanTests.mockResolvedValue([])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11, 12], accountNames: { 11: 'A', 12: 'B' } },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    expect(startPelicanTests).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="selected-model"]').text()).toBe('gpt-6-astra')
    expect(wrapper.get<HTMLInputElement>('[data-question-id="1"]').element.checked).toBe(true)
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11, 12], 'gpt-6-astra', [1], 'pelican')
  })

  it('loads models from another selected account when the first is unavailable', async () => {
    getAvailableModels.mockReset()
    getAvailableModels.mockRejectedValueOnce(new Error('account deleted'))
      .mockResolvedValueOnce([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [21, 22], accountNames: { 21: 'Deleted', 22: 'Live' } },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    expect(getAvailableModels).toHaveBeenNthCalledWith(1, 21)
    expect(getAvailableModels).toHaveBeenNthCalledWith(2, 22)
    expect(wrapper.get('[data-test="selected-model"]').text()).toBe('gpt-6-astra')
  })

  it('uses a trimmed custom model ID in preference to the selected model', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    await wrapper.get('#pelican-custom-model').setValue('  vendor/custom-model  ')
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11], 'vendor/custom-model', [1], 'pelican')
  })

  it('allows a custom model when every model-list request fails', async () => {
    getAvailableModels.mockRejectedValue(new Error('models unavailable'))
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11, 12], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    const start = wrapper.get('[data-test="start-pelican-test"]')
    expect(start.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.accounts.pelican.modelListUnavailable')
    await wrapper.get('#pelican-custom-model').setValue('vendor/custom-model')
    expect(start.attributes('disabled')).toBeUndefined()
    await start.trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11, 12], 'vendor/custom-model', expect.any(Array), 'pelican')
  })

  it('keeps manual input usable while loading and after an empty model list', async () => {
    let resolveModels!: (models: unknown[]) => void
    getAvailableModels.mockImplementation(() => new Promise(resolve => { resolveModels = resolve }))
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    await wrapper.get('#pelican-custom-model').setValue('manual-model')
    const start = wrapper.get('[data-test="start-pelican-test"]')
    expect(start.attributes('disabled')).toBeUndefined()
    resolveModels([])
    await flushPromises()
    expect((wrapper.get('#pelican-custom-model').element as HTMLInputElement).value).toBe('manual-model')
    await start.trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11], 'manual-model', expect.any(Array), 'pelican')
  })

  it('falls back to the dropdown after clearing manual input and resets when reopened', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    await wrapper.get('#pelican-custom-model').setValue('manual-model')
    await wrapper.get('#pelican-custom-model').setValue('   ')
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11], 'gpt-6-astra', expect.any(Array), 'pelican')
    await wrapper.get('#pelican-custom-model').setValue('another-model')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect((wrapper.get('#pelican-custom-model').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-test="selected-model"]').text()).toBe('gpt-6-astra')
  })

  it('blocks blank and overlong manual model IDs when no dropdown model is available', async () => {
    getAvailableModels.mockResolvedValue([])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11], accountNames: {} },
      global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    const start = wrapper.get('[data-test="start-pelican-test"]')
    await wrapper.get('#pelican-custom-model').setValue('  ')
    expect(start.attributes('disabled')).toBeDefined()
    await wrapper.get('#pelican-custom-model').setValue('x'.repeat(256))
    expect(start.attributes('disabled')).toBeDefined()
    await wrapper.get('#pelican-custom-model').setValue('x'.repeat(255))
    expect(start.attributes('disabled')).toBeUndefined()
    expect(startPelicanTests).not.toHaveBeenCalled()
  })
  it('restores multiple remembered questions and leaves zero-created submissions open', async () => {
    getTestPreference.mockResolvedValue({ question_ids: [1, 2] })
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra' }])
    const wrapper = mount(PelicanBatchTestDialog, { props: { show: true, accountIds: [11], accountNames: {} }, global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } } })
    await flushPromises()
    expect(wrapper.findAll<HTMLInputElement>('[data-question-id]').every(input => input.element.checked)).toBe(true)
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click'); await flushPromises()
    expect(wrapper.emitted('submitted')).toBeUndefined()
    expect(wrapper.text()).toContain('pelican.library.emptyCreated')
    await wrapper.get('[data-question-id="2"]').setValue(false)
    await wrapper.setProps({ show: false }); await wrapper.setProps({ show: true }); await flushPromises()
    expect(wrapper.get<HTMLInputElement>('[data-question-id="2"]').element.checked).toBe(true)
    wrapper.unmount()
  })
  it('blocks an unavailable remembered question and capacity above 1000 records', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra' }])
    getTestPreference.mockResolvedValue({ question_ids: [999] })
    const wrapper = mount(PelicanBatchTestDialog, { props: { show: true, accountIds: [11], accountNames: {} }, global: { stubs: { RouterLink: true, BaseDialog: BaseDialogStub, Select: SelectStub } } })
    await flushPromises()
    expect(wrapper.get('[data-test="start-pelican-test"]').attributes('disabled')).toBeDefined()
    listTestQuestions.mockResolvedValue(Array.from({ length: 11 }, (_, i) => ({ id: i + 1, enabled: true, name: `Q${i}` })))
    getTestPreference.mockResolvedValue({ question_ids: Array.from({ length: 11 }, (_, i) => i + 1) })
    await wrapper.setProps({ show: false }); await wrapper.setProps({ show: true, accountIds: Array.from({ length: 100 }, (_, i) => i + 1) }); await flushPromises()
    expect(wrapper.get('[data-test="start-pelican-test"]').attributes('disabled')).toBeDefined()
    expect(startPelicanTests).not.toHaveBeenCalled()
    wrapper.unmount()
  })

})
