import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { batchAddBalanceToUsers, showError, showSuccess } = vi.hoisted(() => ({
  batchAddBalanceToUsers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      batchAddBalanceToUsers,
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
          'admin.users.bulkAddBalance.success': '已为 {count} 个用户增加 {amount} 余额',
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

import UserBatchBalanceModal from '../UserBatchBalanceModal.vue'

describe('UserBatchBalanceModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    batchAddBalanceToUsers.mockResolvedValue({
      affected: 2,
    })
  })

  it('posts selected user ids and amount then shows success toast', async () => {
    const wrapper = mount(UserBatchBalanceModal, {
      props: {
        show: true,
        userIds: [7, 8],
      },
    })

    await wrapper.get('input[type="number"]').setValue('1.5')
    await wrapper.get('textarea').setValue('bonus')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(batchAddBalanceToUsers).toHaveBeenCalledWith([7, 8], 1.5, 'bonus')
    expect(showSuccess).toHaveBeenCalledWith('已为 2 个用户增加 1.5 余额')
  })
})
