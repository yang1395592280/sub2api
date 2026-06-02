<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.bulkAddGroup.title')"
    width="normal"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="rounded-lg bg-primary-50 px-4 py-3 text-sm text-primary-900 dark:bg-primary-900/20 dark:text-primary-100">
        {{ t('admin.users.bulkAddGroup.selectedUsers', { count: userIds.length }) }}
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.users.bulkAddGroup.groupLabel') }}
        </label>
        <Select
          v-model="selectedGroupId"
          :options="groupOptions"
        />
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button class="btn btn-primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? t('common.submitting') : t('admin.users.bulkAddGroup.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{
  show: boolean
  userIds: number[]
  groups: AdminGroup[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const selectedGroupId = ref<number | ''>('')
const submitting = ref(false)

const groupOptions = computed(() => [
  { value: '', label: t('admin.users.bulkAddGroup.groupPlaceholder') },
  ...props.groups
    .filter((group) => group.status === 'active' && group.is_exclusive && group.subscription_type === 'standard')
    .map((group) => ({
      value: group.id,
      label: group.name
    }))
])

const selectedGroupName = computed(() => {
  if (!selectedGroupId.value) return ''
  return props.groups.find((group) => group.id === Number(selectedGroupId.value))?.name || ''
})

watch(
  () => props.show,
  (show) => {
    if (show) {
      selectedGroupId.value = ''
    }
  }
)

const handleSubmit = async () => {
  if (!selectedGroupId.value) {
    appStore.showError(t('admin.users.bulkAddGroup.groupRequired'))
    return
  }

  submitting.value = true
  try {
    const result = await adminAPI.users.batchAddGroupToUsers(
      props.userIds,
      Number(selectedGroupId.value)
    )
    appStore.showSuccess(
      t('admin.users.bulkAddGroup.success', {
        count: result.processed_users,
        group: selectedGroupName.value
      })
    )
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.bulkAddGroup.failed'))
  } finally {
    submitting.value = false
  }
}
</script>
