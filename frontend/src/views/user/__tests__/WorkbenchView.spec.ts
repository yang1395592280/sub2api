import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import WorkbenchView from '../WorkbenchView.vue'

const {
  listConversations,
  createConversation,
  listMessages,
  deleteConversation,
  send,
  listKeys,
  getAvailable,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listConversations: vi.fn(),
  createConversation: vi.fn(),
  listMessages: vi.fn(),
  deleteConversation: vi.fn(),
  send: vi.fn(),
  listKeys: vi.fn(),
  getAvailable: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/workbench', () => ({
  workbenchAPI: { listConversations, createConversation, listMessages, deleteConversation, send },
}))

vi.mock('@/api/keys', () => ({
  default: { list: listKeys },
  keysAPI: { list: listKeys },
}))

vi.mock('@/api/channels', () => ({
  default: { getAvailable },
  userChannelsAPI: { getAvailable },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const AppLayoutStub = { template: '<div><slot /></div>' }

describe('WorkbenchView', () => {
  beforeEach(() => {
    listConversations.mockReset()
    createConversation.mockReset()
    listMessages.mockReset()
    deleteConversation.mockReset()
    send.mockReset()
    listKeys.mockReset()
    getAvailable.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listConversations.mockResolvedValue({
      items: [{ id: 1, title: '你好', mode: 'chat', message_count: 0, updated_at: '2026-06-21T00:00:00Z' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listMessages.mockResolvedValue([])
    listKeys.mockResolvedValue({
      items: [{ id: 7, name: 'main', key: 'sk-test', status: 'active', quota: 10, quota_used: 2 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAvailable.mockResolvedValue([
      {
        name: 'OpenAI',
        platforms: [{ platform: 'openai', groups: [], supported_models: [{ name: 'gpt-5.5', platform: 'openai', pricing: null }] }]
      }
    ])
  })

  it('loads conversations and messages on mount', async () => {
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    expect(listConversations).toHaveBeenCalled()
    expect(listMessages).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('你好')
  })

  it('sends chat message through workbench API', async () => {
    send.mockResolvedValue({
      user_message: { id: 10, role: 'user', content: '你好', status: 'success' },
      assistant_message: { id: 11, role: 'assistant', content: '你好，我可以帮你。', status: 'success' },
      conversation: { id: 1, title: '你好', mode: 'chat', message_count: 2 },
    })
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('你好')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(send).toHaveBeenCalledWith(1, expect.objectContaining({ mode: 'chat', api_key_id: 7, model: 'gpt-5.5', input: '你好' }))
    expect(wrapper.text()).toContain('你好，我可以帮你。')
  })

  it('switches to image mode and sends image options', async () => {
    send.mockResolvedValue({
      user_message: { id: 10, role: 'user', content: '画一张图', status: 'success' },
      assistant_message: { id: 11, role: 'assistant', content: '已生成图片', status: 'success', image_outputs: [{ url: 'https://img.example/1.png' }] },
      conversation: { id: 1, title: '画一张图', mode: 'image', message_count: 2 },
    })
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-mode-image"]').trigger('click')
    await wrapper.get('[data-testid="workbench-input"]').setValue('画一张图')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(send).toHaveBeenCalledWith(1, expect.objectContaining({ mode: 'image', endpoint: 'images_generations', options: expect.objectContaining({ n: 1 }) }))
    expect(wrapper.find('img[src="https://img.example/1.png"]').exists()).toBe(true)
  })

  it('creates a conversation before first send when the list is empty', async () => {
    listConversations.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    createConversation.mockResolvedValue({
      id: 9,
      title: 'workbench.newConversation',
      mode: 'chat',
      message_count: 0,
      updated_at: '2026-06-21T00:00:00Z'
    })
    send.mockResolvedValue({
      user_message: { id: 12, role: 'user', content: '首条消息', status: 'success' },
      assistant_message: { id: 13, role: 'assistant', content: '收到首条消息', status: 'success' },
      conversation: { id: 9, title: '首条消息', mode: 'chat', message_count: 2, updated_at: '2026-06-21T00:00:01Z' },
    })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('首条消息')
    expect(wrapper.get('[data-testid="workbench-send"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(createConversation).toHaveBeenCalledTimes(1)
    expect(send).toHaveBeenCalledWith(9, expect.objectContaining({ input: '首条消息' }))
    expect(wrapper.text()).toContain('收到首条消息')
  })

  it('renders partial result envelope and redacts raw secret from shown errors', async () => {
    send.mockResolvedValue({
      result: {
        user_message: { id: 20, role: 'user', content: '继续', status: 'success' },
        assistant_message: { id: 21, role: 'assistant', content: '已返回部分结果', status: 'error' },
        conversation: { id: 1, title: '你好', mode: 'chat', message_count: 2, updated_at: '2026-06-21T00:00:02Z' },
      },
      error: {
        message: 'upstream failed with sk-test-1234567890abcdef token'
      }
    })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('继续')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('已返回部分结果')
    expect(wrapper.text()).toContain('upstream failed')
    expect(wrapper.text()).not.toContain('sk-test-1234567890abcdef')
    expect(showError).toHaveBeenCalled()
    expect(String(showError.mock.calls.at(-1)?.[0] ?? '')).not.toContain('sk-test-1234567890abcdef')
  })
})
