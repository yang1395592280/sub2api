import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GroupMembersModal from '../GroupMembersModal.vue'
import { adminAPI } from '@/api/admin'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const showError = vi.fn()
const showSuccess = vi.fn()

function getMessage(messages: Record<string, any>, key: string): string {
  return key.split('.').reduce<any>((value, segment) => value?.[segment], messages) ?? key
}

function translate(key: string, params?: Record<string, string | number>): string {
  const template = getMessage(zh, key)
  if (typeof template !== 'string' || !params) return template
  return template.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`))
}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupMembers: vi.fn(),
      getGroupMemberUsageComparison: vi.fn(),
      removeGroupMember: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: translate
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

describe('GroupMembersModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    vi.mocked(adminAPI.groups.getGroupMembers).mockReset()
    vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockReset()
    vi.mocked(adminAPI.groups.removeGroupMember).mockReset()
    vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockResolvedValue({
      group_id: 12,
      today: '2026-06-26',
      yesterday: '2026-06-25',
      stats: {}
    })
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  function mountModal(group?: any, show = true) {
    return mount(GroupMembersModal, {
      props: {
        show,
        group: group ?? {
          id: 12,
          name: 'VIP 分组',
          is_exclusive: true,
          subscription_type: 'standard'
        }
      },
      global: {
        stubs: {
          Teleport: true,
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show"><div>{{ title }}</div><slot /></div>'
          }
        }
      }
    })
  }

  it('loads and renders fixed members', async () => {
    vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
      group_id: 12,
      has_fixed_members: true,
      total: 2,
      items: [
        { id: 1, username: 'alice', email: 'alice@test.com', notes: 'vip', status: 'active' },
        { id: 2, username: 'bob', email: 'bob@test.com', notes: '', status: 'disabled' }
      ]
    })

    const wrapper = mountModal()
    await flushPromises()

    expect(adminAPI.groups.getGroupMembers).toHaveBeenCalledWith(12)
    expect(wrapper.text()).toContain('VIP 分组')
    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('bob@test.com')
    expect(wrapper.text()).toContain('vip')
  })

  it('shows public group hint when there is no fixed member list', async () => {
    vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
      group_id: 13,
      has_fixed_members: false,
      total: 0,
      items: []
    })

    const wrapper = mountModal({ id: 13, name: '公开分组' })
    await flushPromises()

    expect(wrapper.text()).toContain('这是公开标准分组，所有用户都可访问，因此没有固定的用户名单。')
  })

  it('removes a member for exclusive standard groups', async () => {
    vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
      group_id: 12,
      has_fixed_members: true,
      total: 1,
      items: [{ id: 1, username: 'alice', email: 'alice@test.com', notes: '', status: 'active' }]
    })
    vi.mocked(adminAPI.groups.removeGroupMember).mockResolvedValue({ message: 'ok' })

    const wrapper = mountModal()
    await flushPromises()

    const button = wrapper.get('[data-testid="remove-member-1"]')
    await button.trigger('click')
    await flushPromises()

    expect(global.confirm).toHaveBeenCalled()
    expect(adminAPI.groups.removeGroupMember).toHaveBeenCalledWith(12, 1)
    expect(showSuccess).toHaveBeenCalled()
  })

  it('loads and renders yesterday and today usage for exclusive members', async () => {
    vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
      group_id: 12,
      has_fixed_members: true,
      total: 1,
      items: [{ id: 1, username: 'alice', email: 'alice@test.com', notes: '', status: 'active' }]
    })
    vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockResolvedValue({
      group_id: 12,
      today: '2026-06-26',
      yesterday: '2026-06-25',
      stats: {
        '1': {
          today: { requests: 2600, tokens: 232900000, cost: 281.55, standard_cost: 250.1, user_cost: 41.79 },
          yesterday: { requests: 1900, tokens: 180200000, cost: 210.3, standard_cost: 198.4, user_cost: 35.12 }
        }
      }
    })

    const wrapper = mountModal()
    await flushPromises()

    expect(adminAPI.groups.getGroupMemberUsageComparison).toHaveBeenCalledWith(
      12,
      [1],
      expect.any(String)
    )
    expect(wrapper.text()).toContain('今天')
    expect(wrapper.text()).toContain('昨天')
    expect(wrapper.text()).toContain('2.6K req')
    expect(wrapper.text()).toContain('232.90M token')
    expect(wrapper.text()).toContain('A $281.55')
    expect(wrapper.text()).toContain('U $41.79')
  })

  it('keeps members visible when usage comparison fails', async () => {
    vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
      group_id: 12,
      has_fixed_members: true,
      total: 1,
      items: [{ id: 1, username: 'alice', email: 'alice@test.com', notes: '', status: 'active' }]
    })
    vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockRejectedValue(new Error('boom'))

    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.text()).toContain('alice')
    expect(wrapper.text()).toContain('用量加载失败')
  })
})
