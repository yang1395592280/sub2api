import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import WorkbenchView from '../WorkbenchView.vue'

const {
  listConversations,
  createConversation,
  listMessages,
  deleteConversation,
  send,
  listModels,
  listKeys,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listConversations: vi.fn(),
  createConversation: vi.fn(),
  listMessages: vi.fn(),
  deleteConversation: vi.fn(),
  send: vi.fn(),
  listModels: vi.fn(),
  listKeys: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/workbench', () => ({
  workbenchAPI: { listConversations, listModels, createConversation, listMessages, deleteConversation, send },
}))

vi.mock('@/api/keys', () => ({
  default: { list: listKeys },
  keysAPI: { list: listKeys },
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
  afterEach(() => {
    vi.useRealTimers()
  })

  beforeEach(() => {
    listConversations.mockReset()
    createConversation.mockReset()
    listMessages.mockReset()
    deleteConversation.mockReset()
    send.mockReset()
    listModels.mockReset()
    listKeys.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listConversations.mockResolvedValue({
      items: [{ id: 1, title: '你好', mode: 'chat', message_count: 0, updated_at: '2026-06-21T00:00:00Z' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listMessages.mockResolvedValue([
      { id: 2, role: 'assistant', content: '历史消息', status: 'error', error_message: 'upstream failed with sk-test-1234567890abcdef token' },
    ])
    listKeys.mockResolvedValue({
      items: [{ id: 7, name: 'main', key: 'sk-test', status: 'active', quota: 10, quota_used: 2 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listModels.mockResolvedValue([{ name: 'gpt-5.5' }])
  })

  it('loads conversations and messages on mount', async () => {
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    expect(listConversations).toHaveBeenCalled()
    expect(listMessages).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('你好')
    expect(wrapper.text()).toContain('workbench.retentionNotice')
    expect(wrapper.text()).toContain('[redacted]')
    expect(wrapper.text()).not.toContain('sk-test-1234567890abcdef')
  })

  it('links to the image API docs from the workbench settings panel', async () => {
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    const link = wrapper.get('[data-testid="workbench-image-api-docs-link"]')
    expect(link.text()).toContain('workbench.imageApiDocs')
    expect(link.attributes('href')).toBe('/image-api-docs')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })

  it('deletes the selected conversation and switches to the next conversation', async () => {
    listConversations.mockResolvedValue({
      items: [
        { id: 1, title: '第一条', mode: 'chat', message_count: 1, updated_at: '2026-06-21T00:00:00Z' },
        { id: 2, title: '第二条', mode: 'image', message_count: 2, updated_at: '2026-06-21T00:01:00Z' },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listMessages
      .mockResolvedValueOnce([{ id: 10, role: 'assistant', content: '第一条消息', status: 'success' }])
      .mockResolvedValueOnce([{ id: 20, role: 'assistant', content: '第二条消息', status: 'success' }])
    deleteConversation.mockResolvedValue({ message: 'ok' })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-delete-conversation-1"]').trigger('click')
    await flushPromises()

    expect(deleteConversation).toHaveBeenCalledWith(1)
    expect(wrapper.text()).not.toContain('第一条')
    expect(wrapper.text()).toContain('第二条')
    expect(listMessages).toHaveBeenLastCalledWith(2)
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

  it('loads models from the selected workbench API key', async () => {
    listKeys.mockResolvedValue({
      items: [
        { id: 7, name: 'main', key: 'sk-test', status: 'active', quota: 10, quota_used: 2 },
        { id: 8, name: 'image', key: 'sk-image', status: 'active', quota: 10, quota_used: 2 },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listModels.mockImplementation(async (apiKeyId: number) => {
      if (apiKeyId === 8) return [{ name: 'gpt-image-2' }]
      return [{ name: 'gpt-5.5' }]
    })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    expect(listModels).toHaveBeenCalledWith(7)
    expect((wrapper.get('[data-testid="workbench-model-select"]').element as HTMLSelectElement).value).toBe('gpt-5.5')

    await wrapper.get('[data-testid="workbench-api-key-select"]').setValue('8')
    await flushPromises()

    expect(listModels).toHaveBeenCalledWith(8)
    expect((wrapper.get('[data-testid="workbench-model-select"]').element as HTMLSelectElement).value).toBe('gpt-image-2')
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
    const sizeSelect = wrapper.get('[data-testid="workbench-image-size-select"]')
    expect(sizeSelect.findAll('option').map((option) => option.attributes('value'))).toEqual(['1024x1024', '1536x1024', '3840x2160'])
    await sizeSelect.setValue('3840x2160')
    await wrapper.get('[data-testid="workbench-input"]').setValue('画一张图')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(send).toHaveBeenCalledWith(1, expect.objectContaining({ mode: 'image', endpoint: 'images_generations', options: expect.objectContaining({ n: 1, size: '3840x2160' }) }))
    expect(wrapper.find('img[src="https://img.example/1.png"]').exists()).toBe(true)
  })

  it('uses the default image model when only chat models are listed', async () => {
    send.mockResolvedValue({
      user_message: { id: 10, role: 'user', content: '画一张图', status: 'success' },
      assistant_message: { id: 11, role: 'assistant', content: '已生成图片', status: 'success', image_outputs: [] },
      conversation: { id: 1, title: '画一张图', mode: 'image', message_count: 2 },
    })
    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    expect((wrapper.get('[data-testid="workbench-model-select"]').element as HTMLSelectElement).value).toBe('gpt-5.5')

    await wrapper.get('[data-testid="workbench-mode-image"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="workbench-model-select"]').element as HTMLSelectElement).value).toBe('gpt-image-2')

    await wrapper.get('[data-testid="workbench-input"]').setValue('画一张图')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(send).toHaveBeenCalledWith(1, expect.objectContaining({ mode: 'image', model: 'gpt-image-2' }))
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
        assistant_message: { id: 21, role: 'assistant', content: '已返回部分结果', status: 'error', error_message: 'upstream failed with sk-test-1234567890abcdef token' },
        conversation: { id: 1, title: '你好', mode: 'chat', message_count: 2, updated_at: '2026-06-21T00:00:02Z' },
      },
      error: {
        message: 'upstream failed with Bearer abc.def.ghi token'
      }
    })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('继续')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('已返回部分结果')
    expect(wrapper.text()).toContain('upstream failed')
    expect(wrapper.text()).toContain('[redacted]')
    expect(wrapper.text()).not.toContain('sk-test-1234567890abcdef')
    expect(showError).toHaveBeenCalled()
    expect(String(showError.mock.calls.at(-1)?.[0] ?? '')).toContain('Bearer [redacted]')
    expect(String(showError.mock.calls.at(-1)?.[0] ?? '')).not.toContain('sk-test-1234567890abcdef')
  })

  it('refreshes messages after a timed-out send instead of leaving a false failure', async () => {
    listMessages
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        { id: 30, role: 'user', content: '继续', status: 'success' },
        { id: 31, role: 'assistant', content: '后台完成了', status: 'success' },
      ])
    send.mockRejectedValue({ code: 'ECONNABORTED', message: 'timeout of 30000ms exceeded' })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('继续')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(listMessages).toHaveBeenCalledTimes(2)
    expect(listMessages).toHaveBeenLastCalledWith(1)
    expect(wrapper.text()).toContain('后台完成了')
  })

  it('keeps the pending message when timed-out refresh has not persisted the send yet', async () => {
    listMessages
      .mockResolvedValueOnce([{ id: 2, role: 'assistant', content: '历史消息', status: 'success' }])
      .mockResolvedValueOnce([{ id: 2, role: 'assistant', content: '历史消息', status: 'success' }])
    send.mockRejectedValue({ code: 'ECONNABORTED', message: 'timeout of 30000ms exceeded' })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-input"]').setValue('继续')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('历史消息')
    expect(wrapper.text()).toContain('继续')
    expect(showError).not.toHaveBeenCalledWith('workbench.sendFailed')
  })

  it('polls pending image messages until generated image appears', async () => {
    vi.useFakeTimers()
    listMessages
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        { id: 40, role: 'user', content: '画一张图', mode: 'image', status: 'success' },
        { id: 41, role: 'assistant', content: '已生成图片', mode: 'image', status: 'success', image_outputs: [{ url: 'https://img.example/done.png' }] },
      ])
    send.mockResolvedValue({
      user_message: { id: 40, role: 'user', content: '画一张图', mode: 'image', status: 'success' },
      assistant_message: { id: 41, role: 'assistant', content: '生图任务已提交，正在生成图片。', mode: 'image', status: 'pending', image_outputs: [] },
      conversation: { id: 1, title: '画一张图', mode: 'image', message_count: 2 },
    })

    const wrapper = mount(WorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="workbench-mode-image"]').trigger('click')
    await wrapper.get('[data-testid="workbench-input"]').setValue('画一张图')
    await wrapper.get('[data-testid="workbench-send"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('生图任务已提交，正在生成图片。')

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(listMessages).toHaveBeenCalledTimes(2)
    expect(wrapper.find('img[src="https://img.example/done.png"]').exists()).toBe(true)
  })
})
