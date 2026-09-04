<template>
  <div v-if="visible" class="space-y-0.5">
    <div class="leading-tight">
      <span
        data-testid="openai-upstream-balance-amount"
        class="text-sm font-semibold"
        :class="statusClass"
      >
        {{ balanceLabel }}
      </span>
    </div>
    <div class="flex flex-wrap items-center gap-1.5 text-[10px] leading-tight">
      <span v-if="providerLabel" class="text-gray-400">{{ providerLabel }}</span>
      <button
        type="button"
        class="inline-flex items-center rounded px-1.5 py-0.5 text-blue-600 hover:bg-blue-50 disabled:opacity-50 dark:text-blue-400 dark:hover:bg-blue-900/30"
        :disabled="loading"
        @click="refresh"
      >
        {{ loading ? t('common.loading') : t('admin.accounts.upstreamBalance.refresh') }}
      </button>
    </div>
    <div v-if="errorText" class="max-w-[200px] truncate text-[10px] text-red-500" :title="errorText">
      {{ errorText }}
    </div>
    <div v-else-if="updatedAtText" class="text-[10px] text-gray-400">{{ updatedAtText }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { accountsAPI } from '@/api/admin/accounts'
import type { Account } from '@/types'
import { formatDateTime } from '@/utils/format'
import { hasConfiguredUpstreamAdminSettings } from './credentialsBuilder'

const props = defineProps<{
  account: Account
}>()

const emit = defineEmits<{
  refreshed: [account: Account]
}>()

const { t } = useI18n()
const loading = ref(false)
const localAccount = ref<Account | null>(null)
const localError = ref<string | null>(null)

const visible = computed(() => {
  if (props.account.type !== 'apikey') return false
  if (props.account.platform === 'openai' || props.account.platform === 'anthropic') return true
  if (props.account.platform !== 'kimi' && props.account.platform !== 'deepseek') return false
  const extra = props.account.extra ?? {}
  return hasConfiguredUpstreamAdminSettings(props.account.credentials, props.account.credentials_status) ||
    (typeof extra.upstream_balance_status === 'string' && extra.upstream_balance_status.trim() !== '') ||
    (typeof extra.upstream_balance_remaining === 'number' && Number.isFinite(extra.upstream_balance_remaining))
})

const currentAccount = computed(() => localAccount.value ?? props.account)
const currentExtra = computed(() => currentAccount.value.extra ?? {})

const status = computed(() => String(currentExtra.value.upstream_balance_status ?? '').toLowerCase())
const providerLabel = computed(() => {
  const provider = currentExtra.value.upstream_balance_provider
  return typeof provider === 'string' && provider.trim() ? provider.trim() : ''
})

const formatBalance = (value: number, unit: string) => {
  const normalizedUnit = unit.trim().toUpperCase()
  if (normalizedUnit === 'USD') {
    return `$${new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(value)}`
  }
  const formattedNumber = new Intl.NumberFormat(undefined, { maximumFractionDigits: 4 }).format(value)
  return `${formattedNumber} ${unit.trim() || 'quota'}`
}

const balanceLabel = computed(() => {
  const value = currentExtra.value.upstream_balance_remaining
  const unit = String(currentExtra.value.upstream_balance_unit ?? '')

  if (typeof value === 'number' && Number.isFinite(value)) {
    return formatBalance(value, unit)
  }

  if (status.value === 'error') return t('admin.accounts.upstreamBalance.failed')
  if (status.value === 'unsupported') return t('admin.accounts.upstreamBalance.unsupported')
  return t('admin.accounts.upstreamBalance.unknown')
})

const errorText = computed(() => {
  if (localError.value) return localError.value

  const upstreamError = currentExtra.value.upstream_balance_error
  if (typeof upstreamError === 'string' && upstreamError.trim()) {
    return upstreamError.trim()
  }

  return status.value === 'error' ? t('admin.accounts.upstreamBalance.failed') : ''
})

const updatedAtText = computed(() => {
  const updatedAt = currentExtra.value.upstream_balance_updated_at
  if (typeof updatedAt !== 'string' || !updatedAt.trim()) return ''

  const formatted = formatDateTime(updatedAt)
  if (!formatted) return ''
  return t('admin.accounts.upstreamBalance.updatedAt', { time: formatted })
})

const statusClass = computed(() => {
  if (status.value === 'error') return 'text-red-500'
  if (status.value === 'unsupported') return 'text-amber-500'
  return 'text-gray-700 dark:text-gray-200'
})

const extractErrorMessage = (error: unknown): string => {
  const err = error as {
    message?: string
    reason?: string
    response?: { data?: { message?: string; error?: string } }
  }

  return (
    err?.message ||
    err?.reason ||
    err?.response?.data?.message ||
    err?.response?.data?.error ||
    t('common.error')
  )
}

const refresh = async () => {
  if (loading.value) return

  loading.value = true
  localError.value = null
  try {
    const account = await accountsAPI.refreshUpstreamBalance(props.account.id)
    localAccount.value = account
    emit('refreshed', account)
  } catch (error) {
    localError.value = extractErrorMessage(error)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.account,
  () => {
    localAccount.value = null
    localError.value = null
  },
  { deep: true }
)
</script>
