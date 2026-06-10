<template>
  <div
    v-if="selectedIds.length > 0"
    class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20"
  >
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.users.bulkActions.selected', { count: selectedIds.length }) }}
      </span>
      <button
        class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
        @click="$emit('select-page')"
      >
        {{ t('admin.users.bulkActions.selectCurrentPage') }}
      </button>
      <span class="text-gray-300 dark:text-primary-800">•</span>
      <button
        class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
        @click="$emit('clear')"
      >
        {{ t('admin.users.bulkActions.clear') }}
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <button class="btn btn-primary btn-sm" @click="$emit('add-balance')">
        {{ t('admin.users.bulkActions.addBalance') }}
      </button>
      <button class="btn btn-primary btn-sm" @click="$emit('add-group')">
        {{ t('admin.users.bulkActions.addGroup') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  selectedIds: number[]
}>()

defineEmits<{
  (e: 'clear'): void
  (e: 'select-page'): void
  (e: 'add-balance'): void
  (e: 'add-group'): void
}>()

const { t } = useI18n()
</script>
