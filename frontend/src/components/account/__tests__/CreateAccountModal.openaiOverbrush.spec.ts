import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'

const { createAccountMock, showErrorMock, showSuccessMock } = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
    showWarning: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] })
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/composables/useQuotaNotifyState', () => ({
  useQuotaNotifyState: () => ({
    globalEnabled: ref(false),
    state: ref({
      daily: { enabled: false, threshold: 0, threshold_type: 'percent' },
      weekly: { enabled: false, threshold: 0, threshold_type: 'percent' },
      total: { enabled: false, threshold: 0, threshold_type: 'percent' }
    }),
    loadGlobalState: vi.fn(),
    writeToExtra: vi.fn()
  })
}))

function createOAuthMock() {
  return {
    authUrl: ref(''),
    sessionId: ref(''),
    loading: ref(false),
    error: ref(''),
    state: ref(''),
    resetState: vi.fn(),
    generateAuthUrl: vi.fn(),
    getCapabilities: vi.fn().mockResolvedValue({ ai_studio_oauth_enabled: false }),
    validateRefreshToken: vi.fn(),
    exchangeAuthCode: vi.fn(),
    buildCredentials: vi.fn(() => ({})),
    buildExtraInfo: vi.fn(() => ({})),
    parseSessionKeys: vi.fn(() => [])
  }
}

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: createOAuthMock
}))

vi.mock('@/composables/useOpenAIOAuth', () => ({
  useOpenAIOAuth: createOAuthMock
}))

vi.mock('@/composables/useGeminiOAuth', () => ({
  useGeminiOAuth: createOAuthMock
}))

vi.mock('@/composables/useAntigravityOAuth', () => ({
  useAntigravityOAuth: createOAuthMock
}))

vi.mock('@/composables/useGrokOAuth', () => ({
  useGrokOAuth: createOAuthMock
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

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function mountModal() {
  return mount(CreateAccountModal, {
    props: {
      show: true,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Select: SelectStub,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        QuotaLimitCard: true,
        OAuthAuthorizationFlow: true
      }
    }
  })
}

describe('CreateAccountModal OpenAI overbrush', () => {
  beforeEach(() => {
    createAccountMock.mockReset()
    createAccountMock.mockResolvedValue({})
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('does not show or persist overbrush for new OpenAI API key accounts', async () => {
    const wrapper = mountModal()
    const vm = wrapper.vm as any
    vm.form.name = 'OpenAI Key'
    vm.form.platform = 'openai'
    vm.accountCategory = 'apikey'
    vm.apiKeyValue = 'sk-test'
    await nextTick()

    expect(wrapper.find('[data-testid="create-openai-overbrush-toggle"]').exists()).toBe(false)

    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'openai',
      type: 'apikey'
    }))
    expect(createAccountMock.mock.calls[0][0].extra).not.toMatchObject({
      openai_overbrush_enabled: true
    })
  })

  it('hides overbrush when an upstream admin type is selected during create', async () => {
    const wrapper = mountModal()
    const vm = wrapper.vm as any
    vm.form.name = 'OpenAI Key'
    vm.form.platform = 'openai'
    vm.accountCategory = 'apikey'
    vm.apiKeyValue = 'sk-test'
    vm.upstreamAdminType = 'sub2api'
    await nextTick()

    expect(wrapper.find('[data-testid="create-openai-overbrush-toggle"]').exists()).toBe(false)

    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'openai',
      type: 'apikey'
    }))
    expect(createAccountMock.mock.calls[0][0].extra).not.toMatchObject({
      openai_overbrush_enabled: true
    })
  })
})
