import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import BatchAccountTestModal from '../BatchAccountTestModal.vue'

const { getAvailableModels, localStorageMock } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  localStorageMock: {
    getItem: vi.fn(() => 'test-token'),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn()
  }
}))

Object.defineProperty(globalThis, 'localStorage', {
  value: localStorageMock,
  configurable: true
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        return `${key}:${Object.values(params).join(',')}`
      }
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: { type: [String, Number], default: '' },
    options: { type: Array, default: () => [] },
    valueKey: { type: String, default: 'value' },
    labelKey: { type: String, default: 'label' }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-for="option in options"
        :key="option[valueKey]"
        :value="option[valueKey]"
      >
        {{ option[labelKey] }}
      </option>
    </select>
  `
})

function buildAccount(id: number, name: string) {
  return {
    id,
    name,
    platform: 'openai',
    type: 'oauth',
    status: 'active'
  } as any
}

describe('BatchAccountTestModal', () => {
  const originalFetch = global.fetch

  beforeEach(() => {
    getAvailableModels.mockReset()
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])

    global.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        text: vi.fn().mockResolvedValue('data: {"type":"test_start","model":"gpt-5.4"}\ndata: {"type":"content","text":"ok-1"}\n')
      } as any)
      .mockResolvedValueOnce({
        ok: true,
        text: vi.fn().mockResolvedValue('data: {"type":"error","error":"boom"}\n')
      } as any)
  })

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('tests selected accounts one by one and prints each result', async () => {
    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          buildAccount(1, 'A'),
          buildAccount(2, 'B')
        ]
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.findAll('button').at(-1)?.trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(2)
    expect((global.fetch as any).mock.calls[0][0]).toBe('/api/v1/admin/accounts/1/test')
    expect((global.fetch as any).mock.calls[1][0]).toBe('/api/v1/admin/accounts/2/test')
    expect(wrapper.text()).toContain('=== A (#1) ===')
    expect(wrapper.text()).toContain('ok-1')
    expect(wrapper.text()).toContain('=== B (#2) ===')
    expect(wrapper.text()).toContain('ERROR: boom')
  })
})
