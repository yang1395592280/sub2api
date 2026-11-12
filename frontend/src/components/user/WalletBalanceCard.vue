<template>
  <section v-if="user" class="card space-y-4 p-5" data-testid="wallet-balances">
    <h2 class="font-semibold text-gray-900 dark:text-gray-100">{{ t('wallet.title') }}</h2>
    <div class="grid grid-cols-2 gap-4">
      <div><p class="text-sm text-gray-500">{{ t('wallet.paid') }}</p><p class="text-2xl font-semibold text-emerald-600">${{ money(user.balance) }}</p></div>
      <div><p class="text-sm text-gray-500">{{ t('wallet.welfare') }}</p><p class="text-2xl font-semibold" :class="(user.welfare_balance ?? 0) < 0 ? 'text-red-500' : 'text-amber-600'">${{ money(user.welfare_balance) }}</p></div>
    </div>
    <dl class="grid gap-2 text-sm sm:grid-cols-3">
      <div><dt class="text-gray-500">{{ t('wallet.checkin') }}</dt><dd>${{ money(user.checkin_balance) }}</dd></div>
      <div><dt class="text-gray-500">{{ t('wallet.liyu') }}</dt><dd>${{ money(user.liyu_balance) }}</dd></div>
      <div><dt class="text-gray-500">{{ t('wallet.gift') }}</dt><dd>${{ money(user.gift_balance) }}</dd></div>
    </dl>
    <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
      <p>{{ t('wallet.available') }}: <strong>${{ money(user.available_balance ?? user.balance) }}</strong></p>
      <p v-if="user.frozen_balance" class="mt-1">{{ t('wallet.frozen') }}: ${{ money(user.frozen_balance) }}</p>
      <p class="mt-2 text-gray-500">{{ t('wallet.rule', { paid: user.wallet_usage_policy?.paid_units ?? 2, welfare: user.wallet_usage_policy?.welfare_units ?? 1 }) }}</p>
      <p class="mt-1 text-xs text-gray-500">{{ t('wallet.debt') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { formatWalletAmount as money } from '@/utils/walletDisplay'
import { useI18n } from 'vue-i18n'
import type { User } from '@/types'
defineProps<{ user: User | null }>()
const { t } = useI18n()
</script>
