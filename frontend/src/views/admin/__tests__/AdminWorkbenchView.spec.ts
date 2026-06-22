import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminWorkbenchView from '../AdminWorkbenchView.vue'

const { getStats, listConversations, getConversation, batchDeleteConversations, cleanupExpiredConversations } = vi.hoisted(() => ({
  getStats: vi.fn(),
  listConversations: vi.fn(),
  getConversation: vi.fn(),
  batchDeleteConversations: vi.fn(),
  cleanupExpiredConversations: vi.fn(),
}))

const appStoreMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    workbench: {
      getStats,
      listConversations,
      getConversation,
      batchDeleteConversations,
      cleanupExpiredConversations,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreMocks,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.workbench.deleteSelectedSuccess') return `deleted ${params?.count}`
        if (key === 'admin.workbench.cleanupSuccess') return `cleaned ${params?.count}`
        return key
      },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }

describe('AdminWorkbenchView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getStats.mockResolvedValue({
      total_conversations: 2,
      total_messages: 4,
      image_messages: 1,
      expired_conversations: 1,
      image_bytes: 8,
      retention_days: 7,
    })
    listConversations.mockResolvedValue({
      items: [
        {
          id: 1,
          user_id: 7,
          user_email: 'a@example.com',
          username: 'alice',
          title: 'chat',
          mode: 'chat',
          endpoint: 'chat_completions',
          model: 'gpt-5.5',
          message_count: 2,
          image_count: 0,
          image_bytes: 0,
          updated_at: '2026-06-21T00:00:00Z',
        },
        {
          id: 2,
          user_id: 8,
          user_email: 'b@example.com',
          username: 'bob',
          title: 'image',
          mode: 'image',
          endpoint: 'images_generations',
          model: 'gpt-image-2',
          message_count: 2,
          image_count: 1,
          image_bytes: 8,
          updated_at: '2026-06-21T00:01:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getConversation.mockResolvedValue({
      conversation: { id: 2, title: 'image', mode: 'image', user_email: 'b@example.com' },
      messages: [{ id: 20, role: 'assistant', content: 'done', status: 'success', image_outputs: [] }],
    })
    batchDeleteConversations.mockResolvedValue({ deleted: 1 })
    cleanupExpiredConversations.mockResolvedValue({ deleted: 1 })
  })

  it('loads stats and opens a conversation detail', async () => {
    const wrapper = mount(AdminWorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    expect(getStats).toHaveBeenCalledWith(7)
    expect(listConversations).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    expect(wrapper.text()).toContain('a@example.com')

    await wrapper.get('[data-testid="admin-workbench-open-2"]').trigger('click')
    await flushPromises()

    expect(getConversation).toHaveBeenCalledWith(2)
    expect(wrapper.text()).toContain('done')
  })

  it('deletes selected conversations and cleans expired records', async () => {
    const wrapper = mount(AdminWorkbenchView, { global: { stubs: { AppLayout: AppLayoutStub } } })
    await flushPromises()

    await wrapper.get('[data-testid="admin-workbench-select-1"]').setValue(true)
    await wrapper.get('[data-testid="admin-workbench-delete-selected"]').trigger('click')
    await flushPromises()

    expect(batchDeleteConversations).toHaveBeenCalledWith([1])
    expect(appStoreMocks.showSuccess).toHaveBeenCalledWith('deleted 1')

    await wrapper.get('[data-testid="admin-workbench-cleanup-expired"]').trigger('click')
    await flushPromises()

    expect(cleanupExpiredConversations).toHaveBeenCalledWith(7)
    expect(appStoreMocks.showSuccess).toHaveBeenCalledWith('cleaned 1')
  })
})
