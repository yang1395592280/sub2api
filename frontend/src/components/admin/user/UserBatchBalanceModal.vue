<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.bulkAddBalance.title')"
    width="normal"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:bg-emerald-900/20 dark:text-emerald-100">
        {{ t('admin.users.bulkAddBalance.selectedUsers', { count: userIds.length }) }}
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.users.bulkAddBalance.amountLabel') }}
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
          :placeholder="t('admin.users.bulkAddBalance.notesPlaceholder')"
        ></textarea>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button class="btn btn-primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? t('common.submitting') : t('admin.users.bulkAddBalance.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  userIds: number[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)
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
    const result = await adminAPI.users.batchAddBalanceToUsers(
      props.userIds,
      form.amount,
      form.notes
    )
    appStore.showSuccess(
      t('admin.users.bulkAddBalance.success', {
        count: result.affected,
        amount: form.amount
      })
    )
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.bulkAddBalance.failed'))
  } finally {
    submitting.value = false
  }
}
</script>
