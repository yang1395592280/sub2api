import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import UserDailyStatsView from '../UserDailyStatsView.vue'
import zh from '@/i18n/locales/zh/admin/userDailyStats'
import type { DailyActivityStats, DailyUsage, UserDailyStatsResponse, UserDailyStatsRow } from '@/api/admin/userDailyStats'

const { list, activities, push, saveAs } = vi.hoisted(() => ({ list: vi.fn(), activities: vi.fn(), push: vi.fn(), saveAs: vi.fn() }))
vi.mock('@/api/admin/userDailyStats', () => ({ userDailyStatsAPI: { list, activities } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('file-saver', () => ({ saveAs }))

const usage: DailyUsage = { total_cost: 9.00000003, paid_cost: 6.00000002, welfare_cost: 3.00000001, checkin_cost: 1.00000001, liyu_cost: 1, gift_cost: 1, subscription_cost: 0, unclassified_cost: 0, requests: 3, tokens: 12345 }
const activity: DailyActivityStats = { event_count: 4, checkin_events: 2, checkin_count: 1, checkin_base: .25, checkin_bonus: -.25, checkin_net: 0, liyu_events: 2, liyu_draws: 1, liyu_reward: 1, liyu_lucky: -2, liyu_guess: 0, liyu_net: -1, liyu_ticket_face_value: 5, gift_credit: 2, welfare_adjustment: -.1 }
function fixture(): UserDailyStatsResponse {
  const row: UserDailyStatsRow = { user_id: 42, email: 'alice@example.test', username: 'Alice', status: 'active', deleted: false, balance: 15, welfare_balance: -1.25, checkin_balance: .25, liyu_balance: -2, gift_balance: .5, frozen_balance: .1, total_recharged: 100, spending_rank: 7, today_usage: { ...usage }, previous_usage: { ...usage, total_cost: 2, paid_cost: 1.5, welfare_cost: .5 }, activity: { ...activity } }
  return { date: '2026-11-13', previous_date: '2026-11-12', timezone: 'Asia/Shanghai', generated_at: '2026-11-13T04:00:00Z', items: [row], summary: { users: 51, active_users: 20, checkin_users: 12, liyu_users: 8, spending_users: 20, balance: 100, welfare_balance: 10, today_usage: { ...usage }, previous_usage: { ...usage }, activity: { ...activity } }, total: 51, page: 1, page_size: 20, pages: 3 }
}
const PaginationStub = { props: ['page', 'total', 'pageSize'], emits: ['update:page', 'update:pageSize'], template: '<div><button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button><button data-test="page-size" @click="$emit(\'update:pageSize\', 50)">size</button></div>' }
const wrappers: VueWrapper[] = []
function render() {
  const wrapper = mount(UserDailyStatsView, {
    global: {
      plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { admin: zh, common: { refresh: '刷新', loading: '加载中' }, empty: { noData: '无数据' } } } })],
      stubs: { AppLayout: { template: '<div><slot /></div>' }, BaseDialog: { props: ['show', 'title'], emits: ['close'], template: '<div v-if="show" data-test="dialog"><h2>{{ title }}</h2><button data-test="close" @click="$emit(\'close\')">close</button><slot /></div>' }, Pagination: PaginationStub }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  list.mockResolvedValue(fixture())
  activities.mockResolvedValue({ items: [{ kind: 'liyu_lucky', record_id: 12, amount: -.00000002, ticket_face_value: 0, outcome: 'zero', description: '测试奖励', created_at: '2026-11-13T01:02:03Z' }], total: 21, page: 1, page_size: 20, pages: 2 })
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()) })

describe('Daily user statistics', () => {
  it('renders precise paid/welfare debits, reward gains/losses, balances, and global ranks', async () => {
    const w = render()
    await flushPromises()
    const table = w.get('[data-test="user-table"]')
    expect(table.text()).toContain('6.00000002')
    expect(table.text()).toContain('3.00000001')
    expect(table.text()).toContain('-1.25000000')
    expect(table.text()).toContain('-0.25000000')
    expect(table.text()).toContain('#7')
    expect(table.text()).toContain('alice@example.test')
    expect(w.text()).toContain('匹配 51 人')
    expect(w.text()).toContain('2026-11-12')
    await w.get('[data-test="usage-42"]').trigger('click')
    expect(push).toHaveBeenCalledWith({ path: '/admin/usage', query: { user_id: '42', start_date: '2026-11-13', end_date: '2026-11-13' } })
  })

  it('resets pagination for date, participation, search and sorting changes', async () => {
    const w = render()
    await flushPromises()
    await w.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(list.mock.lastCall![0].page).toBe(2)
    await w.get('[data-test="activity"]').setValue('liyu')
    await flushPromises()
    expect(list.mock.lastCall![0]).toMatchObject({ activity: 'liyu', page: 1 })
    await w.get('[data-test="date"]').setValue('2026-11-10')
    await flushPromises()
    expect(list.mock.lastCall![0].date).toBe('2026-11-10')
    await w.get('[data-test="search"]').setValue(' alice ')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(list.mock.lastCall![0].search).toBe('alice')
    await w.get('[data-test="sort"]').setValue('welfare_balance')
    await w.get('[data-test="sort-order"]').trigger('click')
    await flushPromises()
    expect(list.mock.lastCall![0]).toMatchObject({ sort_by: 'welfare_balance', sort_order: 'asc', page: 1 })
    await w.get('[data-test="page-size"]').trigger('click')
    await flushPromises()
    expect(list.mock.lastCall![0]).toMatchObject({ page_size: 50, page: 1 })
  })

  it('uses Beijing calendar dates even around UTC midnight', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-12T16:00:01Z'))
    try {
      const w = render()
      await flushPromises()
      expect(list.mock.calls[0][0].date).toBe('2026-11-13')
      await w.get('[data-test="yesterday"]').trigger('click')
      await flushPromises()
      expect(list.mock.lastCall![0].date).toBe('2026-11-12')
    } finally { vi.useRealTimers() }
  })

  it('paginates activities for the selected user and day, and closes stale dialogs on filtering', async () => {
    const w = render()
    await flushPromises()
    await w.get('[data-test="activities-42"]').trigger('click')
    await flushPromises()
    expect(activities.mock.lastCall!.slice(0, 2)).toEqual([42, { date: '2026-11-13', page: 1, page_size: 20 }])
    expect(w.get('[data-test="dialog"]').text()).toContain('-0.00000002')
    expect(w.get('[data-test="dialog"]').text()).toContain('2026/11/13 09:02:03')
    expect(w.get('[data-test="dialog"]').text()).toContain('礼遇幸运翻倍')
    await w.get('[data-test="dialog"] [data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(activities.mock.lastCall![1].page).toBe(2)
    await w.get('[data-test="yesterday"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-test="dialog"]').exists()).toBe(false)
  })

  it('keeps the latest response when requests finish out of order', async () => {
    let finishFirst!: (value: UserDailyStatsResponse) => void
    list.mockImplementationOnce(() => new Promise(resolve => { finishFirst = resolve }))
    const w = render()
    await w.get('[data-test="activity"]').setValue('checkin')
    await flushPromises()
    const old = fixture(); old.items[0].email = 'stale@example.test'
    finishFirst(old)
    await flushPromises()
    expect(w.text()).toContain('alice@example.test')
    expect(w.text()).not.toContain('stale@example.test')
    expect(list.mock.calls[0][1].aborted).toBe(true)
  })

  it('shows retryable list and activity errors without leaving stale totals', async () => {
    list.mockRejectedValueOnce(new Error('offline'))
    const w = render()
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('统计加载失败')
    expect(w.find('[data-test="user-table"]').exists()).toBe(false)
    await w.get('[role="alert"] button').trigger('click')
    await flushPromises()
    activities.mockRejectedValueOnce(new Error('offline'))
    await w.get('[data-test="activities-42"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-test="dialog"]').text()).toContain('活动流水加载失败')
    await w.get('[data-test="retry-details"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-test="dialog"]').text()).toContain('测试奖励')
  })

  it('exports only displayed rows with precise amounts and neutralized spreadsheet formulas', async () => {
    const result = fixture(); result.items[0].username = '=1+1'; result.items[0].email = '"quoted",user@example.test'
    list.mockResolvedValue(result)
    const w = render()
    await flushPromises()
    await w.get('[data-test="export"]').trigger('click')
    expect(saveAs).toHaveBeenCalledTimes(1)
    const [blob, filename] = saveAs.mock.calls[0]
    expect(filename).toBe('user_daily_stats_2026-11-13_page_1.csv')
    const csv = await new Promise<string>(resolve => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.readAsText(blob) })
    expect(csv.split('\r\n')).toHaveLength(2)
    expect(csv).toContain('"\'=1+1"')
    expect(csv).toContain('""quoted"",user@example.test')
    expect(csv).toContain('"6.00000002"')
    expect(csv).toContain('"-1.25000000"')
    expect(list).toHaveBeenCalledTimes(1)
  })
})
