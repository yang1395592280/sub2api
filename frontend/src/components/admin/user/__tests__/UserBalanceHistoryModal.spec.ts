import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const mocks = vi.hoisted(() => ({
  getUserBalanceHistory: vi.fn()
}))

const messages: Record<string, string> = {
  'admin.users.balanceHistoryTitle': '用户活动时间线',
  'admin.users.createdAt': '创建时间',
  'admin.users.currentBalance': '当前余额',
  'admin.users.notes': '备注',
  'admin.users.totalRecharged': '总充值',
  'admin.users.allTypes': '全部类型',
  'admin.users.typeBalance': '余额',
  'admin.users.typeAdminBalance': '余额（管理员调整）',
  'admin.users.typeCheckin': '签到',
  'admin.users.typeGame': '竞猜',
  'admin.users.typeConcurrency': '并发',
  'admin.users.typeAdminConcurrency': '并发（管理员调整）',
  'admin.users.typeSubscription': '订阅',
  'admin.users.deposit': '充值',
  'admin.users.withdraw': '退款',
  'admin.users.expandDetails': '展开明细',
  'admin.users.collapseDetails': '收起明细',
  'admin.users.gameStakeAmount': '参与扣减',
  'admin.users.gamePayoutAmount': '开奖结算',
  'admin.users.gameRoundNo': '回合',
  'admin.users.gameResult': '开奖结果',
  'admin.users.balanceAfter': '余额结余',
  'common.unknown': '未知'
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserBalanceHistory: (...args: unknown[]) => mocks.getUserBalanceHistory(...args)
    }
  }
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value
}))

describe('UserBalanceHistoryModal', () => {
  beforeEach(() => {
    mocks.getUserBalanceHistory.mockReset()
  })

  it('renders merged balance, sign-in, game, and concurrency events', async () => {
    mocks.getUserBalanceHistory.mockResolvedValue({
      items: [
        {
          id: 'game-1',
          code: 'GAME-1',
          type: 'game_net',
          value: 10,
          created_at: '2026-04-23T10:00:00Z',
          details: { round_no: 1001, stake_amount: 10, payout_amount: 20 }
        },
        {
          id: 'checkin-2',
          code: 'CHECKIN-2',
          type: 'checkin_reward',
          value: 0.02,
          created_at: '2026-04-23T09:00:00Z'
        },
        {
          id: 'concurrency-3',
          code: 'CONC-3',
          type: 'admin_concurrency',
          value: 1,
          created_at: '2026-04-23T08:00:00Z'
        }
      ],
      total: 3,
      total_recharged: 50
    })

    const wrapper = mount(UserBalanceHistoryModal, {
      props: {
        show: false,
        user: {
          id: 7,
          email: 'user@example.com',
          username: 'tester',
          created_at: '2026-04-20T00:00:00Z',
          balance: 88.88,
          notes: ''
        }
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show"><slot /></div>'
          },
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue', 'change'],
            template: '<select />'
          },
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(wrapper.text()).toContain('竞猜')
    expect(wrapper.text()).toContain('签到')
    expect(wrapper.text()).toContain('并发')

    const expandButton = wrapper.find('[data-testid="timeline-expand-game-1"]')
    expect(expandButton.exists()).toBe(true)
    await expandButton.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('参与扣减')
    expect(wrapper.text()).toContain('开奖结算')
  })
})
