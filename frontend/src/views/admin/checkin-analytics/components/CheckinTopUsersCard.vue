<template>
  <div class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.checkinAnalytics.charts.topUsers') }}
      </h3>
    </div>

    <div v-if="loading" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="users.length === 0" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.checkinAnalytics.empty') }}
    </div>
    <div v-else class="space-y-3">
      <button
        v-for="user in users"
        :key="user.user_id"
        type="button"
        class="flex w-full items-center justify-between rounded-xl border border-gray-200 px-4 py-3 text-left transition hover:border-primary-300 hover:bg-primary-50/40 dark:border-dark-700 dark:hover:border-primary-700 dark:hover:bg-primary-900/10"
        @click="emit('select-user', user)"
      >
        <div class="min-w-0">
          <div class="truncate font-medium text-gray-900 dark:text-white">
            {{ user.user_name || user.user_email }}
          </div>
          <div class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ user.user_email }}
          </div>
        </div>
        <div class="ml-4 text-right">
          <div class="text-sm font-semibold text-primary-600 dark:text-primary-400">
            {{ user.checkin_count }}
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ formatCurrency(user.reward_amount) }}
          </div>
        </div>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatCurrency } from '@/utils/format'
import type { AdminCheckinTopUser } from '@/api/admin/checkins'

defineOptions({ name: 'CheckinTopUsersCard' })

withDefaults(defineProps<{
  users: AdminCheckinTopUser[]
  loading?: boolean
}>(), {
  loading: false
})

const emit = defineEmits<{
  'select-user': [user: AdminCheckinTopUser]
}>()

const { t } = useI18n()
</script>
