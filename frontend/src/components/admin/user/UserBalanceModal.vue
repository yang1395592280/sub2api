<template>
  <BaseDialog :show="show" :title="operation === 'add' ? t('admin.users.deposit') : t('admin.users.withdraw')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="balance-form" @submit.prevent="handleBalanceSubmit" class="space-y-5">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100"><span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span></div>
        <div class="flex-1"><p class="font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('wallet.paid') }}: ${{ formatBalance(user.balance) }}</p></div>
      </div>
      <div>
        <label class="input-label" for="paid-balance-amount">{{ operation === 'add' ? t('admin.users.depositAmount') : t('admin.users.withdrawAmount') }}</label>
        <div class="relative flex gap-2">
          <div class="relative flex-1"><div class="absolute left-3 top-1/2 -translate-y-1/2 font-medium text-gray-500">$</div><input id="paid-balance-amount" v-model.number="form.amount" type="number" step="any" min="0" class="input pl-8" /></div>
          <button v-if="operation === 'subtract'" type="button" @click="fillAllBalance" class="btn btn-secondary whitespace-nowrap">{{ t('admin.users.withdrawAll') }}</button>
        </div>
      </div>
      <div v-if="operation === 'subtract'">
        <label class="input-label" for="welfare-refund-source">{{ t('wallet.refundSource') }}</label>
        <select id="welfare-refund-source" v-model="form.welfareSource" class="input">
          <option value="checkin">{{ t('wallet.checkin') }} (${{ formatBalance(user.checkin_balance) }})</option>
          <option value="liyu">{{ t('wallet.liyu') }} (${{ formatBalance(user.liyu_balance) }})</option>
          <option value="gift">{{ t('wallet.gift') }} (${{ formatBalance(user.gift_balance) }})</option>
        </select>
      </div>
      <div>
        <label class="input-label" for="welfare-gift-amount">{{ operation === 'add' ? t('wallet.giftAmount') : t('wallet.refundAmount') }}</label>
        <div class="flex gap-2">
          <input id="welfare-gift-amount" v-model.number="form.giftAmount" type="number" min="0" step="any" class="input min-w-0 flex-1" />
          <button v-if="operation === 'subtract'" type="button" @click="form.giftAmount = Math.max(0, selectedWelfareBalance)" class="btn btn-secondary whitespace-nowrap">{{ t('admin.users.withdrawAll') }}</button>
        </div>
        <p v-if="operation === 'subtract'" class="mt-2 text-xs text-gray-500">{{ t('wallet.refundHint') }}</p>
      </div>
      <p class="text-sm text-gray-500">{{ selectedWelfareLabel }}: ${{ formatBalance(selectedWelfareBalance) }} → ${{ formatBalance(selectedWelfareBalance + (operation === 'add' ? form.giftAmount : -form.giftAmount)) }}</p>
      <div><label class="input-label">{{ t('admin.users.notes') }}</label><textarea v-model="form.notes" rows="3" class="input"></textarea></div>
      <div v-if="form.amount > 0" class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950"><div class="flex items-center justify-between text-sm"><span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newBalance') }}:</span><span class="font-bold text-gray-900 dark:text-gray-100">${{ formatBalance(calculateNewBalance()) }}</span></div></div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="balance-form" :disabled="submitting || !(form.amount > 0 || form.giftAmount > 0)" class="btn" :class="operation === 'add' ? 'bg-emerald-600 text-white' : 'btn-danger'">{{ submitting ? t('common.saving') : t('common.confirm') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatWalletAmount as formatBalance } from '@/utils/walletDisplay'
import type { WelfareSource } from '@/api/admin/users'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null, operation: 'add' | 'subtract' }>()
const emit = defineEmits(['close', 'success']); const { t } = useI18n(); const appStore = useAppStore()

const submitting = ref(false)
const form = reactive({ amount: 0, giftAmount: 0, welfareSource: 'gift' as WelfareSource, notes: '' })
const requestKey = ref(crypto.randomUUID())
watch(() => props.show, (v) => { if(v) { form.amount = 0; form.giftAmount = 0; form.welfareSource = 'gift'; form.notes = ''; requestKey.value = crypto.randomUUID() } })
// Retrying the same request reuses its key; editing a request creates a new operation.
watch(() => [form.amount, form.giftAmount, form.welfareSource, form.notes, props.user?.id, props.operation], () => { requestKey.value = crypto.randomUUID() })
const selectedWelfareSource = computed(() => props.operation === 'add' ? 'gift' : form.welfareSource)
const selectedWelfareBalance = computed(() => Number(props.user?.[`${selectedWelfareSource.value}_balance`] ?? 0))
const selectedWelfareLabel = computed(() => t(`wallet.${selectedWelfareSource.value}`))

// 填入全部余额
const fillAllBalance = () => {
  if (props.user) {
    form.amount = Math.max(0, props.user.balance)
  }
}

const calculateNewBalance = () => {
  if (!props.user) return 0
  const result = props.operation === 'add' ? props.user.balance + form.amount : props.user.balance - form.amount
  // 避免浮点数精度问题导致的 -0.00 显示
  return Math.abs(result) < 1e-10 ? 0 : result
}
const handleBalanceSubmit = async () => {
  if (!props.user || submitting.value) return
  if (!Number.isFinite(form.amount) || form.amount < 0 || !Number.isFinite(form.giftAmount) || form.giftAmount < 0 || !(form.amount > 0 || form.giftAmount > 0)) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }
  // 退款时验证金额不超过实际余额
  if (props.operation === 'subtract' && form.amount > 0 && form.amount > props.user.balance) {
    appStore.showError(t('admin.users.insufficientBalance'))
    return
  }
  if (props.operation === 'subtract' && form.giftAmount > 0 && form.giftAmount > selectedWelfareBalance.value) {
    appStore.showError(t('wallet.insufficientRefundBalance'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.users.updateBalance(props.user.id, form.amount, props.operation, form.notes, form.giftAmount, props.operation === 'subtract' ? form.welfareSource : undefined, requestKey.value)
    appStore.showSuccess(t('common.success')); emit('success'); emit('close')
  } catch (e: any) {
    console.error('Failed to update balance:', e)
    appStore.showError(extractApiErrorMessage(e, t('common.error')))
  } finally { submitting.value = false }
}
</script>
