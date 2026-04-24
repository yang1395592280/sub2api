import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SizeBetStatsView from '../SizeBetStatsView.vue'

const { getStatsOverview, listStatsUsers, showError } = vi.hoisted(() => ({
  getStatsOverview: vi.fn(),
  listStatsUsers: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api', () => ({
  sizeBetAPI: {
    getStatsOverview,
    listStatsUsers,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

const messages: Record<string, string> = {
  'common.apply': '应用',
  'common.retry': '重试',
  'sizeBet.loadError.title': '加载失败',
  'sizeBet.loadError.description': '加载失败，请重试',
  'sizeBet.statsPage.title': '竞猜统计',
  'sizeBet.statsPage.description': '查看每日竞猜统计',
  'sizeBet.statsPage.subtitle': '公开展示竞猜统计',
  'sizeBet.statsPage.date': '统计日期',
  'sizeBet.statsPage.rank': '排名',
  'sizeBet.statsPage.emptyTitle': '暂无统计数据',
  'sizeBet.statsPage.emptyDescription': '还没有统计',
  'admin.sizeBet.columns.user': '用户',
  'admin.sizeBet.stats.participantCount': '参与人数',
  'admin.sizeBet.stats.totalStake': '总参与额度',
  'admin.sizeBet.stats.totalUserNet': '用户总盈亏',
  'admin.sizeBet.stats.houseNet': '系统总盈亏',
  'admin.sizeBet.stats.wonCount': '命中次数',
  'admin.sizeBet.stats.lostCount': '未命中次数',
  'admin.sizeBet.stats.refundedCount': '退款次数',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const LoadingSpinnerStub = { template: '<div data-test="loading">loading</div>' }
const EmptyStateStub = {
  props: ['title', 'description', 'actionText'],
  emits: ['action'],
  template: '<div data-test="empty-state">{{ title }}{{ description }}<button v-if="actionText" @click="$emit(\'action\')">{{ actionText }}</button></div>',
}
const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  template: '<div data-test="pagination">{{ page }}-{{ total }}-{{ pageSize }}</div>',
}
const DataTableStub = {
  props: ['data', 'loading'],
  template: '<div data-test="table">{{ loading ? "loading" : JSON.stringify(data) }}</div>',
}

describe('SizeBetStatsView', () => {
  beforeEach(() => {
    getStatsOverview.mockReset()
    listStatsUsers.mockReset()
    showError.mockReset()
  })

  it('loads and renders stats data', async () => {
    getStatsOverview.mockResolvedValue({
      date: '2026-04-24',
      participant_count: 3,
      total_stake: 30,
      total_payout: 20,
      total_user_net: -10,
      house_net: 10,
    })
    listStatsUsers.mockResolvedValue({
      items: [
        {
          username: 'alice',
          total_stake: 20,
          won_count: 1,
          lost_count: 1,
          refunded_count: 0,
          net_result: -5,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mount(SizeBetStatsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          LoadingSpinner: LoadingSpinnerStub,
          EmptyState: EmptyStateStub,
          Pagination: PaginationStub,
          DataTable: DataTableStub,
        },
      },
    })

    await flushPromises()

    expect(getStatsOverview).toHaveBeenCalledTimes(1)
    expect(listStatsUsers).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('竞猜统计')
    expect(wrapper.text()).toContain('3')
    expect(wrapper.get('[data-test="table"]').text()).toContain('alice')
    expect(wrapper.get('[data-test="table"]').text()).toContain('"rank":1')
  })
})
