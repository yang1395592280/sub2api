import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { batchAddGroupToUsers, showError, showSuccess } = vi.hoisted(() => ({
  batchAddGroupToUsers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      batchAddGroupToUsers,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const templates: Record<string, string> = {
          'admin.users.bulkAddGroup.success': '已为 {count} 个用户添加「{group}」分组权限',
        }
        const template = templates[key] || key
        if (!params) return template
        return template.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
      },
    }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'Select',
    props: ['modelValue', 'options'],
    emits: ['update:modelValue'],
    template: `
      <select
        :value="modelValue"
        @change="$emit('update:modelValue', Number($event.target.value) || '')"
      >
        <option v-for="option in options" :key="String(option.value)" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    `,
  },
}))

import UserBatchAddGroupModal from '../UserBatchAddGroupModal.vue'

describe('UserBatchAddGroupModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    batchAddGroupToUsers.mockResolvedValue({
      group_id: 9,
      processed_users: 2,
    })
  })

  it('shows the selected group name in the success toast', async () => {
    const wrapper = mount(UserBatchAddGroupModal, {
      props: {
        show: true,
        userIds: [7, 8],
        groups: [
          {
            id: 9,
            name: 'VIP-C',
            status: 'active',
            is_exclusive: true,
            subscription_type: 'standard',
          },
        ],
      },
    })

    await wrapper.get('select').setValue('9')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(batchAddGroupToUsers).toHaveBeenCalledWith([7, 8], 9)
    expect(showSuccess).toHaveBeenCalledWith(
      '已为 2 个用户添加「VIP-C」分组权限'
    )
  })
})
