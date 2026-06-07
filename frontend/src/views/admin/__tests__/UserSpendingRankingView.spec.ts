import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserSpendingRankingView from '../UserSpendingRankingView.vue'

const {
  getUserSpendingRanking,
  saveAsMock,
  bookNewMock,
  bookAppendSheetMock,
  aoaToSheetMock,
  sheetAddAoaMock
} = vi.hoisted(() => ({
  getUserSpendingRanking: vi.fn(),
  saveAsMock: vi.fn(),
  bookNewMock: vi.fn(() => ({})),
  bookAppendSheetMock: vi.fn(),
  aoaToSheetMock: vi.fn(() => ({})),
  sheetAddAoaMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('file-saver', () => ({
  saveAs: saveAsMock
}))

vi.mock('xlsx', () => ({
  utils: {
    book_new: bookNewMock,
    book_append_sheet: bookAppendSheetMock,
    aoa_to_sheet: aoaToSheetMock,
    sheet_add_aoa: sheetAddAoaMock
  },
  write: vi.fn(() => new ArrayBuffer(8))
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {}
  }),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const PaginationStub = {
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="next-page" @click="$emit('update:page', 2)">page</button>
      <button data-test="page-size" @click="$emit('update:pageSize', 50)">size</button>
    </div>
  `
}

describe('admin UserSpendingRankingView', () => {
  beforeEach(() => {
    getUserSpendingRanking.mockReset()
    saveAsMock.mockReset()
    bookNewMock.mockClear()
    bookAppendSheetMock.mockClear()
    aoaToSheetMock.mockClear()
    sheetAddAoaMock.mockClear()
    getUserSpendingRanking.mockResolvedValue({
      ranking: [
        { user_id: 1, email: 'alpha@example.com', actual_cost: 12.5, requests: 3, tokens: 300 }
      ],
      total_actual_cost: 12.5,
      total_requests: 3,
      total_tokens: 300,
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      start_date: '2026-06-06',
      end_date: '2026-06-07'
    })
  })

  it('loads ranking on mount and refetches when page or page size changes', async () => {
    const wrapper = mount(UserSpendingRankingView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Pagination: PaginationStub,
          DateRangePicker: true,
          LoadingSpinner: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      page_size: 20
    }))
    expect(wrapper.text()).toContain('alpha@example.com')

    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 2,
      page_size: 20
    }))

    await wrapper.get('[data-test="page-size"]').trigger('click')
    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      page_size: 50
    }))
  })

  it('sends current sort params and exports all filtered users with headers', async () => {
    getUserSpendingRanking
      .mockResolvedValueOnce({
        ranking: [
          { user_id: 1, email: 'alpha@example.com', actual_cost: 12.5, requests: 3, tokens: 300 }
        ],
        total_actual_cost: 30,
        total_requests: 8,
        total_tokens: 900,
        total: 2,
        page: 1,
        page_size: 20,
        pages: 1,
        start_date: '2026-06-06',
        end_date: '2026-06-07'
      })
      .mockResolvedValueOnce({
        ranking: [
          { user_id: 1, email: 'alpha@example.com', actual_cost: 12.5, requests: 3, tokens: 300 }
        ],
        total_actual_cost: 30,
        total_requests: 8,
        total_tokens: 900,
        total: 2,
        page: 1,
        page_size: 20,
        pages: 1,
        start_date: '2026-06-06',
        end_date: '2026-06-07'
      })
      .mockResolvedValueOnce({
        ranking: [
          { user_id: 1, email: 'alpha@example.com', actual_cost: 12.5, requests: 3, tokens: 300 },
          { user_id: 2, email: 'beta@example.com', actual_cost: 11.5, requests: 5, tokens: 600 }
        ],
        total_actual_cost: 24,
        total_requests: 8,
        total_tokens: 900,
        total: 2,
        page: 1,
        page_size: 100,
        pages: 1,
        start_date: '2026-06-06',
        end_date: '2026-06-07'
      })

    const wrapper = mount(UserSpendingRankingView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Pagination: PaginationStub,
          DateRangePicker: true,
          LoadingSpinner: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    await wrapper.get('[data-test="sort-requests"]').trigger('click')
    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      sort_by: 'requests',
      sort_order: 'desc'
    }))

    await wrapper.get('[data-test="export"]').trigger('click')
    await flushPromises()

    expect(getUserSpendingRanking).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1,
      page_size: 100,
      sort_by: 'requests',
      sort_order: 'desc'
    }))
    expect(aoaToSheetMock).toHaveBeenCalled()
    expect(bookAppendSheetMock).toHaveBeenCalled()
    expect(saveAsMock).toHaveBeenCalled()
  })
})
