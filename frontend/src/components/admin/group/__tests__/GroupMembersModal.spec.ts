import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import GroupMembersModal from '../GroupMembersModal.vue'
import { adminAPI } from '@/api/admin'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const showError = vi.fn()
const showSuccess = vi.fn()
const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: { en, zh }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupMembers: vi.fn(),
      removeGroupMember: vi.fn()
    }
  }
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
    vi.mocked(adminAPI.groups.removeGroupMember).mockReset()
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
        plugins: [i18n],
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

    expect(wrapper.text()).toContain('admin.groups.publicGroupNoFixedMembers')
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
})
