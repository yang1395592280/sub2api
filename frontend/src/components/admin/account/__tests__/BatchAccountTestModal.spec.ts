import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import BatchAccountTestModal from '../BatchAccountTestModal.vue'

const { getAvailableModels, queryOpenAIQuota, localStorageMock } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  queryOpenAIQuota: vi.fn(),
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
      getAvailableModels,
      queryOpenAIQuota
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

const createObjectURLMock = vi.fn(() => 'blob:test')
const revokeObjectURLMock = vi.fn()

Object.defineProperty(globalThis, 'URL', {
  value: {
    createObjectURL: createObjectURLMock,
    revokeObjectURL: revokeObjectURLMock
  },
  configurable: true
})

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

function createDeferredResponse(text: string) {
  let resolve!: (value: any) => void
  const promise = new Promise<any>((innerResolve) => {
    resolve = innerResolve
  })

  return {
    promise,
    resolve: () => resolve({
      ok: true,
      text: vi.fn().mockResolvedValue(text)
    })
  }
}

describe('BatchAccountTestModal', () => {
  const originalFetch = global.fetch
  const originalCreateElement = document.createElement.bind(document)
  const originalBlob = globalThis.Blob
  const clickMock = vi.fn()

  beforeEach(() => {
    getAvailableModels.mockReset()
    queryOpenAIQuota.mockReset()
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    queryOpenAIQuota.mockResolvedValue({
      plan_type: 'free',
      rate_limit: {
        secondary_window: {
          used_percent: 2,
          limit_window_seconds: 604800,
          reset_after_seconds: 100,
          reset_at: 0
        }
      },
      fetched_at: 0
    })

    global.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        text: vi.fn().mockResolvedValue('data: {"type":"test_start","model":"gpt-5.4","connect_duration_ms":1580}\ndata: {"type":"content","text":"ok-1"}\n')
      } as any)
      .mockResolvedValueOnce({
        ok: true,
        text: vi.fn().mockResolvedValue('data: {"type":"test_start","model":"gpt-5.4","connect_duration_ms":8070}\ndata: {"type":"error","error":"boom"}\n')
      } as any)

    clickMock.mockReset()
    createObjectURLMock.mockReset()
    revokeObjectURLMock.mockReset()
    createObjectURLMock.mockReturnValue('blob:test')
    ;(globalThis as any).Blob = class MockBlob {
      parts: unknown[]
      type: string

      constructor(parts: unknown[], options?: { type?: string }) {
        this.parts = parts
        this.type = options?.type || ''
      }
    }

    vi.spyOn(document, 'createElement').mockImplementation(((tagName: string) => {
      const element = originalCreateElement(tagName)
      if (tagName.toLowerCase() === 'a') {
        Object.defineProperty(element, 'click', {
          value: clickMock,
          configurable: true
        })
      }
      return element
    }) as typeof document.createElement)
  })

  afterEach(() => {
    global.fetch = originalFetch
    ;(globalThis as any).Blob = originalBlob
    vi.restoreAllMocks()
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
    expect(wrapper.text()).toContain('admin.accounts.testLinkLabel：/api/v1/admin/accounts/1/test')
    expect(wrapper.text()).toContain('ok-1')
    expect(wrapper.text()).toContain('连接耗时 1.58s')
    expect(wrapper.text()).toContain('=== B (#2) ===')
    expect(wrapper.text()).toContain('admin.accounts.testLinkLabel：/api/v1/admin/accounts/2/test')
    expect(wrapper.text()).toContain('ERROR: boom')
    expect(wrapper.text()).toContain('连接耗时 8.07s')
  })

  it('runs account tests with the selected concurrency limit', async () => {
    const deferredResponses = [
      createDeferredResponse('data: {"type":"content","text":"ok-1"}\n'),
      createDeferredResponse('data: {"type":"content","text":"ok-2"}\n'),
      createDeferredResponse('data: {"type":"content","text":"ok-3"}\n'),
      createDeferredResponse('data: {"type":"content","text":"ok-4"}\n'),
      createDeferredResponse('data: {"type":"content","text":"ok-5"}\n'),
      createDeferredResponse('data: {"type":"content","text":"ok-6"}\n')
    ]
    const queuedResponses = [...deferredResponses]
    global.fetch = vi.fn()
      .mockImplementation(() => queuedResponses.shift()?.promise)

    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          buildAccount(1, 'A'),
          buildAccount(2, 'B'),
          buildAccount(3, 'C'),
          buildAccount(4, 'D'),
          buildAccount(5, 'E'),
          buildAccount(6, 'F')
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
    await wrapper.get('[data-testid="batch-test-concurrency"]').setValue('5')
    await wrapper.findAll('button').at(-1)?.trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(5)

    deferredResponses[0].resolve()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(6)

    deferredResponses[1].resolve()
    deferredResponses[2].resolve()
    deferredResponses[3].resolve()
    deferredResponses[4].resolve()
    deferredResponses[5].resolve()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(6)
    expect(wrapper.text()).toContain('ok-6')
  })

  it('shows progress and downloads success/failed emails separately', async () => {
    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          {
            ...buildAccount(1, 'A'),
            extra: { email_address: 'a@example.com' }
          },
          {
            ...buildAccount(2, 'B'),
            extra: { email_address: 'b@example.com' }
          }
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

    expect(wrapper.text()).toContain('admin.accounts.bulkTestProgress:2,2')

    await wrapper.get('[data-testid="download-success-emails"]').trigger('click')
    await wrapper.get('[data-testid="download-failed-emails"]').trigger('click')

    expect(createObjectURLMock).toHaveBeenCalledTimes(2)
    const blobs = createObjectURLMock.mock.calls.map(call => call[0] as { parts: unknown[] })
    expect(blobs[0].parts[0]).toBe('a@example.com')
    expect(blobs[1].parts[0]).toBe('b@example.com')
    expect(clickMock).toHaveBeenCalledTimes(2)
  })

  it('summarizes successes, failures, and failure categories while testing accounts', async () => {
    global.fetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        text: vi.fn().mockResolvedValue('data: {"type":"content","text":"ok"}\n')
      } as any)
      .mockResolvedValueOnce({
        ok: false,
        status: 429,
        text: vi.fn().mockResolvedValue('data: {"type":"error","error":"rate limited"}\n')
      } as any)
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: vi.fn().mockResolvedValue('data: {"type":"error","error":"unauthorized"}\n')
      } as any)
      .mockResolvedValueOnce({
        ok: false,
        status: 429,
        text: vi.fn().mockResolvedValue('data: {"type":"error","error":"too many requests"}\n')
      } as any)

    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          buildAccount(1, 'A'),
          buildAccount(2, 'B'),
          buildAccount(3, 'C'),
          buildAccount(4, 'D')
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

    expect(wrapper.text()).toContain('admin.accounts.bulkTestProgress:4,4')
    expect(wrapper.text()).toContain('admin.accounts.bulkTestSummary:1,3')
    expect(wrapper.text()).toContain('admin.accounts.bulkTestFailureBreakdownTitle')
    expect(wrapper.text()).toContain('admin.accounts.bulkTestFailureCategory:HTTP 429,2')
    expect(wrapper.text()).toContain('admin.accounts.bulkTestFailureCategory:HTTP 401,1')
  })

  it('summarizes OpenAI plans and seven-day usage after connection tests', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: vi.fn().mockResolvedValue('data: {"type":"content","text":"ok"}\n')
    } as any)
    queryOpenAIQuota.mockImplementation((id: number) => {
      if (id === 1) {
        return Promise.resolve({
          plan_type: 'free',
          rate_limit: {
            secondary_window: {
              used_percent: 5,
              limit_window_seconds: 604800,
              reset_after_seconds: 100,
              reset_at: 0
            }
          },
          fetched_at: 0
        })
      }
      if (id === 2) {
        return Promise.resolve({
          plan_type: 'PLUS',
          rate_limit: {
            secondary_window: {
              used_percent: 5.1,
              limit_window_seconds: 604800,
              reset_after_seconds: 100,
              reset_at: 0
            }
          },
          fetched_at: 0
        })
      }
      if (id === 3) {
        return Promise.resolve({
          plan_type: 'team',
          rate_limit: {
            secondary_window: {
              used_percent: 20,
              limit_window_seconds: 604800,
              reset_after_seconds: 100,
              reset_at: 0
            }
          },
          fetched_at: 0
        })
      }
      return Promise.reject(new Error('quota unavailable'))
    })

    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          buildAccount(1, 'A'),
          buildAccount(2, 'B'),
          buildAccount(3, 'C'),
          buildAccount(4, 'D')
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

    expect(queryOpenAIQuota).toHaveBeenCalledTimes(4)
    expect(wrapper.get('[data-testid="openai-batch-stats"]').text()).toContain('admin.accounts.bulkOpenAIStatsProgress:4,4')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsFree：1')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsPlus：1')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsOther：1')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsUnknown：1')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsUsageLow：1')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsUsageHigh：2')
    expect(wrapper.text()).toContain('admin.accounts.bulkOpenAIStatsUsageUnknown：1')
  })

  it('falls back to account name when extra.email_address is missing', async () => {
    const wrapper = mount(BatchAccountTestModal, {
      props: {
        show: false,
        accounts: [
          {
            ...buildAccount(1, 'a@example.com')
          },
          {
            ...buildAccount(2, 'b@example.com')
          }
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

    await wrapper.get('[data-testid="download-success-emails"]').trigger('click')
    await wrapper.get('[data-testid="download-failed-emails"]').trigger('click')

    const blobs = createObjectURLMock.mock.calls.slice(-2).map(call => call[0] as { parts: unknown[] })
    expect(blobs[0].parts[0]).toBe('a@example.com')
    expect(blobs[1].parts[0]).toBe('b@example.com')
  })
})
