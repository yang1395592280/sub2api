import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PelicanBatchTestDialog from '../PelicanBatchTestDialog.vue'

const { getAvailableModels, startPelicanTests } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  startPelicanTests: vi.fn()
}))
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
  it('waits for confirmation and submits the exact default prompt with the Astra model ID', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    startPelicanTests.mockResolvedValue([])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [11, 12], accountNames: { 11: 'A', 12: 'B' } },
      global: { stubs: { BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    expect(startPelicanTests).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="selected-model"]').text()).toBe('gpt-6-astra')
    const prompt = '创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试'
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe(prompt)
    await wrapper.get('[data-test="start-pelican-test"]').trigger('click')
    await flushPromises()
    expect(startPelicanTests).toHaveBeenCalledWith([11, 12], 'gpt-6-astra', prompt)
  })

  it('loads models from another selected account when the first is unavailable', async () => {
    getAvailableModels.mockReset()
    getAvailableModels.mockRejectedValueOnce(new Error('account deleted'))
      .mockResolvedValueOnce([{ id: 'gpt-6-astra', display_name: 'GPT-6 Astra' }])
    const wrapper = mount(PelicanBatchTestDialog, {
      props: { show: true, accountIds: [21, 22], accountNames: { 21: 'Deleted', 22: 'Live' } },
      global: { stubs: { BaseDialog: BaseDialogStub, Select: SelectStub } }
    })
    await flushPromises()
    expect(getAvailableModels).toHaveBeenNthCalledWith(1, 21)
    expect(getAvailableModels).toHaveBeenNthCalledWith(2, 22)
    expect(wrapper.get('[data-test="selected-model"]').text()).toBe('gpt-6-astra')
  })
})
