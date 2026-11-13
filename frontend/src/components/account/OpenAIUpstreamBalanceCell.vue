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
    <div v-if="showDailyUsage" class="mt-1 space-y-0.5 border-t border-gray-100 pt-1 text-[11px] dark:border-dark-700" data-testid="upstream-daily-usage">
      <div class="text-gray-600 dark:text-gray-300" :title="t('admin.accounts.upstreamBalance.keyCostHint')">
        {{ t('admin.accounts.upstreamBalance.todayKeyCost') }}: <strong>{{ dailyMoney(dailyUsage?.key_cost) }}</strong>
      </div>
      <div v-if="dailyUsage?.cash_cost != null && usageIsToday" class="text-emerald-600 dark:text-emerald-400">
        {{ t('admin.accounts.upstreamBalance.cashCost') }}: {{ dailyMoney(dailyUsage.cash_cost) }}
      </div>
      <div v-if="dailyUsage?.account_cost != null && usageIsToday" class="text-gray-400" :title="t('admin.accounts.upstreamBalance.accountCostHint')">
        {{ t('admin.accounts.upstreamBalance.todayAccountCost') }}: {{ dailyMoney(dailyUsage.account_cost) }}
        <span aria-hidden="true">ⓘ</span>
      </div>
      <div class="text-[10px] text-gray-400">{{ dailyUsage?.date || '—' }} · {{ t('admin.accounts.upstreamBalance.beijingTime') }}<span v-if="currentExtra.upstream_key_id"> · Key #{{ currentExtra.upstream_key_id }}</span></div>
      <div v-if="usageMessage" class="max-w-[230px] text-[10px] text-amber-600 dark:text-amber-400" data-testid="upstream-usage-message">{{ usageMessage }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { accountsAPI } from '@/api/admin/accounts'
import type { Account } from '@/types'
import { formatDateTime } from '@/utils/format'

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
  if (props.account.platform === 'openai' || props.account.platform === 'anthropic' || props.account.platform === 'gemini') return true
  return props.account.platform === 'kimi' || props.account.platform === 'deepseek' || props.account.platform === 'zhipu' || props.account.platform === 'grok'
})

const currentAccount = computed(() => localAccount.value ?? props.account)
const currentExtra = computed(() => currentAccount.value.extra ?? {})
const now = useNow({ interval: 60_000 })
const dailyUsage = computed(() => currentExtra.value.upstream_balance_daily_usage)
const showDailyUsage = computed(() => !!dailyUsage.value || ['sub2api', 'new-api'].includes(String(currentExtra.value.upstream_balance_provider)))
const usageIsToday = computed(() => dailyUsage.value?.date === new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(now.value))
const dailyMoney = (value: number | null | undefined) => usageIsToday.value && typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(4)}` : '—'
const usageMessage = computed(() => {
  if (dailyUsage.value && !usageIsToday.value) return t('admin.accounts.upstreamBalance.usageStale')
  if (dailyUsage.value?.status === 'ok') return ''
  const error = dailyUsage.value?.error
  if (error === 'credentials') return t('admin.accounts.upstreamBalance.usageCredentials')
  if (error === 'key_unmatched') return t('admin.accounts.upstreamBalance.usageKeyUnmatched')
  if (error === 'limit') return t('admin.accounts.upstreamBalance.usageLimit')
  return t('admin.accounts.upstreamBalance.usageUnavailable')
})

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
  if (props.account.platform === 'kimi' || props.account.platform === 'deepseek') {
    const balances = currentExtra.value[`${props.account.platform}_balances`]
    if (Array.isArray(balances)) {
      const labels = balances.flatMap((item) => {
        if (!item || typeof item !== 'object') return []
        const row = item as { currency?: unknown; balance?: unknown }
        return typeof row.balance === 'number' && Number.isFinite(row.balance)
          ? `${typeof row.currency === 'string' && row.currency.trim() ? row.currency : 'CNY'} ${row.balance.toFixed(2)}`
          : []
      })
      if (labels.length > 0) return labels.join(' · ')
    }
    const balance = currentExtra.value[`${props.account.platform}_balance`]
    if (typeof balance === 'number' && Number.isFinite(balance)) {
      const currency = currentExtra.value[`${props.account.platform}_balance_currency`]
      return `${typeof currency === 'string' && currency.trim() ? currency : 'CNY'} ${balance.toFixed(2)}`
    }
  }
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
