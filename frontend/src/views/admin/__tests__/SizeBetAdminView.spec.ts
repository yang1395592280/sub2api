import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const {
  getSettings,
  updateSettings,
  listRounds,
  listBets,
  listLedger,
  refundRound,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listRounds: vi.fn(),
  listBets: vi.fn(),
  listLedger: vi.fn(),
  refundRound: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

const storageStub = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn()
}

vi.stubGlobal('localStorage', storageStub)
vi.stubGlobal('sessionStorage', storageStub)

vi.mock('@/api/admin/sizeBet', () => ({
  getSettings,
  updateSettings,
  listRounds,
  listBets,
  listLedger,
  refundRound,
  default: {
    getSettings,
    updateSettings,
    listRounds,
    listBets,
    listLedger,
    refundRound
  }
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN'
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) =>
        values ? `${key} ${Object.values(values).join(' ')}` : key
    })
  }
})

const defaultSettings = () => ({
  enabled: true,
  round_duration_seconds: 60,
  bet_close_offset_seconds: 50,
  allowed_stakes: [2, 5, 10, 20],
  probabilities: { small: 45, mid: 10, big: 45 },
  odds: { small: 2, mid: 10, big: 2 },
  rules_markdown: 'rule-body'
})

const defaultPagination = {
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
}

async function mountView() {
  const { default: SizeBetAdminView } = await import('../SizeBetAdminView.vue')

  return mount(SizeBetAdminView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: { template: '<div data-test="loading-spinner">loading</div>' },
        EmptyState: {
          props: ['title', 'description', 'actionText'],
          emits: ['action'],
          template: `
            <div data-test="empty-state">
              <div>{{ title }}</div>
              <div>{{ description }}</div>
              <slot name="action">
                <button
                  v-if="actionText"
                  data-test="empty-state-action"
                  type="button"
                  @click="$emit('action')"
                >
                  {{ actionText }}
                </button>
              </slot>
            </div>
          `
        },
        Toggle: {
          props: ['modelValue', 'disabled'],
          emits: ['update:modelValue'],
          template: `
            <input
              data-test="toggle"
              type="checkbox"
              :checked="modelValue"
              :disabled="disabled"
              @change="$emit('update:modelValue', $event.target.checked)"
            />
          `
        },
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue', 'change'],
          methods: {
            normalize(value: string) {
              if (value === '') return ''
              const numeric = Number(value)
              return Number.isNaN(numeric) ? value : numeric
            }
          },
          template: `
            <select
              :value="String(modelValue ?? '')"
              @change="
                $emit('update:modelValue', normalize($event.target.value));
                $emit('change', normalize($event.target.value))
              "
            >
              <option
                v-for="option in options"
                :key="String(option.value)"
                :value="String(option.value ?? '')"
              >
                {{ option.label }}
              </option>
            </select>
          `
        },
        DataTable: {
          props: ['columns', 'data', 'loading'],
          template: `
            <div data-test="datatable">
              <div v-if="loading" data-test="datatable-loading">loading</div>
              <div v-for="row in data" :key="row.id" data-test="datatable-row">
                <div
                  v-for="column in columns"
                  :key="column.key"
                  :data-test="'cell-' + column.key"
                >
                  <slot
                    :name="'cell-' + column.key"
                    :row="row"
                    :value="row[column.key]"
                  >
                    {{
                      column.formatter
                        ? column.formatter(row[column.key], row)
                        : row[column.key]
                    }}
                  </slot>
                </div>
              </div>
            </div>
          `
        },
        Pagination: {
          props: ['page', 'total', 'pageSize'],
          emits: ['update:page', 'update:pageSize'],
          template: '<div data-test="pagination">{{ page }}-{{ total }}-{{ pageSize }}</div>'
        },
        ConfirmDialog: {
          props: ['show', 'title', 'message'],
          emits: ['confirm', 'cancel'],
          template: `
            <div v-if="show" data-test="confirm-dialog">
              <div>{{ title }}</div>
              <div>{{ message }}</div>
              <button data-test="confirm-dialog-confirm" type="button" @click="$emit('confirm')">
                confirm
              </button>
              <button data-test="confirm-dialog-cancel" type="button" @click="$emit('cancel')">
                cancel
              </button>
            </div>
          `
        }
      }
    }
  })
}

describe('SizeBetAdminView', () => {
  beforeEach(() => {
    storageStub.getItem.mockClear()
    storageStub.setItem.mockClear()
    storageStub.removeItem.mockClear()
    storageStub.clear.mockClear()

    getSettings.mockReset()
    updateSettings.mockReset()
    listRounds.mockReset()
    listBets.mockReset()
    listLedger.mockReset()
    refundRound.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    getSettings.mockResolvedValue(defaultSettings())
    updateSettings.mockResolvedValue({ message: 'updated' })
    listRounds.mockResolvedValue(defaultPagination)
    listBets.mockResolvedValue(defaultPagination)
    listLedger.mockResolvedValue(defaultPagination)
    refundRound.mockResolvedValue({
      round_id: 1,
      refunded_count: 2,
      refunded_at: '2026-04-23T00:00:00Z'
    })
  })

  it('shows retry state and keeps save unavailable until settings load succeeds', async () => {
    getSettings
      .mockRejectedValueOnce(new Error('load failed'))
      .mockResolvedValueOnce(defaultSettings())

    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="save-settings"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="settings-load-retry"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('load failed')

    await wrapper.get('[data-test="settings-load-retry"]').trigger('click')
    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="save-settings"]').exists()).toBe(true)
  })

  it('saves edited settings payload', async () => {
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.get('[data-test="settings-round-duration"]').setValue('90')
    await wrapper.get('[data-test="settings-bet-close-offset"]').setValue('45')
    await wrapper.get('[data-test="settings-allowed-stakes"]').setValue('3, 6, 9')
    await wrapper.get('[data-test="settings-prob-small"]').setValue('40')
    await wrapper.get('[data-test="settings-prob-mid"]').setValue('20')
    await wrapper.get('[data-test="settings-prob-big"]').setValue('40')
    await wrapper.get('[data-test="settings-odds-small"]').setValue('1.8')
    await wrapper.get('[data-test="settings-odds-mid"]').setValue('12')
    await wrapper.get('[data-test="settings-odds-big"]').setValue('2.2')
    await wrapper.get('[data-test="settings-rules-markdown"]').setValue('updated rules')
    await wrapper.get('[data-test="save-settings"]').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith({
      enabled: true,
      round_duration_seconds: 90,
      bet_close_offset_seconds: 45,
      allowed_stakes: [3, 6, 9],
      probabilities: { small: 40, mid: 20, big: 40 },
      odds: { small: 1.8, mid: 12, big: 2.2 },
      rules_markdown: 'updated rules'
    })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('switches tabs and loads tab data', async () => {
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-rounds"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="tab-ledger"]').trigger('click')
    await flushPromises()

    expect(listRounds).toHaveBeenCalledTimes(1)
    expect(listLedger).toHaveBeenCalledTimes(1)
  })

  it('applies bet filters to load request', async () => {
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-bets"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="filter-round-id"]').setValue('88')
    await wrapper.get('[data-test="filter-user-id"]').setValue('9')
    await wrapper.get('[data-test="filter-status"]').setValue('won')
    await wrapper.get('[data-test="apply-filters"]').trigger('click')
    await flushPromises()

    expect(listBets).toHaveBeenLastCalledWith(1, 20, {
      round_id: 88,
      user_id: 9,
      status: 'won'
    })
  })

  it('shows round audit data and refunds a round after confirmation', async () => {
    listRounds.mockResolvedValue({
      ...defaultPagination,
      items: [
        {
          id: 1,
          round_no: 1001,
          status: 'open',
          starts_at: '2026-04-23T00:00:00Z',
          bet_closes_at: '2026-04-23T00:00:50Z',
          settles_at: '2026-04-23T00:01:00Z',
          prob_small: 45,
          prob_mid: 10,
          prob_big: 45,
          odds_small: 2,
          odds_mid: 10,
          odds_big: 2,
          allowed_stakes: [2, 5, 10],
          result_number: null,
          result_direction: null,
          server_seed_hash: 'hash-1001',
          server_seed: 'seed-1001'
        }
      ]
    })

    const wrapper = await mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-rounds"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('hash-1001')
    expect(wrapper.text()).toContain('seed-1001')

    await wrapper.get('[data-test="refund-round-1"]').trigger('click')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
    await wrapper.get('[data-test="confirm-dialog-confirm"]').trigger('click')
    await flushPromises()

    expect(refundRound).toHaveBeenCalledWith(1)
    expect(listRounds).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalled()
  })
})
