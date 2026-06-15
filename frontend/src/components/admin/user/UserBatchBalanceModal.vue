<template>
  <BaseDialog
    :show="show"
    :title="t(localePrefix + '.title')"
    width="normal"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div
        class="rounded-lg px-4 py-3 text-sm"
        :class="mode === 'subtract'
          ? 'bg-amber-50 text-amber-900 dark:bg-amber-900/20 dark:text-amber-100'
          : 'bg-emerald-50 text-emerald-900 dark:bg-emerald-900/20 dark:text-emerald-100'"
      >
        {{ t(localePrefix + '.selectedUsers', { count: userIds.length }) }}
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t(localePrefix + '.amountLabel') }}
        </label>
        <div class="relative">
          <div class="absolute left-3 top-1/2 -translate-y-1/2 font-medium text-gray-500">$</div>
          <input
            v-model.number="form.amount"
            type="number"
            step="any"
            min="0"
            class="input pl-8"
          />
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.users.notes') }}
        </label>
        <textarea
          v-model="form.notes"
          rows="3"
          class="input"
          :placeholder="t(localePrefix + '.notesPlaceholder')"
        ></textarea>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button class="btn btn-primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? t('common.submitting') : t(localePrefix + '.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  userIds: number[]
  mode?: 'add' | 'subtract'
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)
const mode = computed(() => props.mode || 'add')
const localePrefix = computed(() => mode.value === 'subtract'
  ? 'admin.users.bulkSubtractBalance'
  : 'admin.users.bulkAddBalance')
const form = reactive({
  amount: 0,
  notes: ''
})

watch(
  () => props.show,
  (show) => {
    if (show) {
      form.amount = 0
      form.notes = ''
    }
  }
)

const handleSubmit = async () => {
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }

  submitting.value = true
  try {
    const submitBalance = mode.value === 'subtract'
      ? adminAPI.users.batchSubtractBalanceFromUsers
      : adminAPI.users.batchAddBalanceToUsers
    const result = await submitBalance(props.userIds, form.amount, form.notes)
    appStore.showSuccess(
      t(localePrefix.value + '.success', {
        count: result.affected,
        amount: form.amount
      })
    )
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t(localePrefix.value + '.failed'))
  } finally {
    submitting.value = false
  }
}
</script>
