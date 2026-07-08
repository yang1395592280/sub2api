import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupCapacityUsersModal from '../GroupCapacityUsersModal.vue'

const { getGroupCapacityUsers } = vi.hoisted(() => ({
  getGroupCapacityUsers: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupCapacityUsers
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'admin.groups.capacityUsersTitle') return `容量明细 ${params?.name}`
      const messages: Record<string, string> = {
        'common.refresh': '刷新',
        'common.loading': '加载中',
        'admin.groups.noActiveCapacityUsers': '暂无活跃用户'
      }
      return messages[key] ?? key
    }
  })
}))

const group: AdminGroup = {
  id: 10,
  name: 'plus',
  description: null,
  platform: 'openai',
  rate_multiplier: 0.1,
  rpm_limit: 10,
  is_exclusive: true,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  upstream_balance_refresh_enabled: false,
  upstream_balance_refresh_interval_seconds: 600,
  upstream_price_max_multiplier: 0,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  require_oauth_only: false,
  require_privacy_set: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  model_routing: null,
  model_routing_enabled: false,
  account_count: 0,
  active_account_count: 0,
  rate_limited_account_count: 0,
  sort_order: 1,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z'
}

describe('GroupCapacityUsersModal', () => {
  beforeEach(() => {
    getGroupCapacityUsers.mockReset()
    getGroupCapacityUsers.mockResolvedValue({
      items: [
        {
          user_id: 1,
          username: 'alpha',
          email: 'a@example.com',
          notes: 'vip',
          status: 'active',
          current_concurrency: 2,
          concurrency_limit: 3,
          current_rpm: 4,
          effective_rpm_limit: 6,
          rpm_limit_source: 'override',
          rpm_override: 6,
          group_rpm_limit: 10,
          user_rpm_limit: 20
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
  })

  it('loads and renders active capacity users', async () => {
    const wrapper = mount(GroupCapacityUsersModal, {
      props: { show: true, group },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Pagination: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(getGroupCapacityUsers).toHaveBeenCalledWith(10, 1, 20, true)
    expect(wrapper.text()).toContain('alpha')
    expect(wrapper.text()).toContain('a@example.com')
    expect(wrapper.text()).toContain('2 / 3')
    expect(wrapper.text()).toContain('4 / 6')
  })
})
