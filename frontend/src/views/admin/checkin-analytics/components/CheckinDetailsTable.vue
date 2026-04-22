<template>
  <div class="rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.checkinAnalytics.table.title') }}
        </h3>
        <p v-if="linkedDate || linkedUser" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.checkinAnalytics.filters.currentView') }}:
          <span v-if="linkedDate">{{ linkedDate }}</span>
          <span v-if="linkedUser">{{ linkedUser.user_email || linkedUser.user_name }}</span>
        </p>
      </div>
      <button
        v-if="linkedDate || linkedUser"
        type="button"
        class="btn btn-secondary"
        @click="emit('clear-linkage')"
      >
        {{ t('admin.checkinAnalytics.filters.clearLinkage') }}
      </button>
    </div>

    <DataTable
      :columns="columns"
      :data="records"
      :loading="loading"
      :server-side-sort="true"
      default-sort-key="created_at"
      default-sort-order="desc"
      @sort="handleSort"
    >
      <template #cell-user_email="{ row, value }">
        <div class="flex flex-col">
          <span class="font-medium text-gray-900 dark:text-white">
            {{ row.user_name || value }}
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ value }}
          </span>
        </div>
      </template>

      <template #cell-reward_amount="{ value }">
        <span class="font-medium text-emerald-600 dark:text-emerald-400">
          {{ formatCurrency(value) }}
        </span>
      </template>

      <template #cell-created_at="{ value }">
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ formatDateTime(value) }}
        </span>
      </template>
    </DataTable>

    <Pagination
      v-if="pagination.total > 0"
      :page="pagination.page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      @update:page="handlePageChange"
      @update:pageSize="handlePageSizeChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCheckinRecord, AdminCheckinTopUser } from '@/api/admin/checkins'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'
import { formatCurrency, formatDateTime } from '@/utils/format'

defineOptions({ name: 'CheckinDetailsTable' })

const props = defineProps<{
  filters: {
    search?: string
    date?: string
    timezone?: string
  }
  linkedDate?: string
  linkedUser?: AdminCheckinTopUser | null
}>()

const emit = defineEmits<{
  'clear-linkage': []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const records = ref<AdminCheckinRecord[]>([])
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const columns = computed<Column[]>(() => [
  { key: 'user_email', label: t('admin.checkinAnalytics.table.user') },
  { key: 'checkin_date', label: t('admin.checkinAnalytics.table.checkinDate'), sortable: true },
  { key: 'reward_amount', label: t('admin.checkinAnalytics.table.rewardAmount'), sortable: true },
  { key: 'user_timezone', label: t('admin.checkinAnalytics.table.timezone') },
  { key: 'created_at', label: t('admin.checkinAnalytics.table.createdAt'), sortable: true }
])

async function loadRecords() {
  loading.value = true
  try {
    const response = await adminAPI.checkins.list(
      pagination.page,
      pagination.page_size,
      {
        search: props.filters.search,
        date: props.filters.date,
        timezone: props.filters.timezone,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      {}
    )
    records.value = response.items
    pagination.total = response.total
  } catch (error) {
    console.error('Error loading checkin analytics details:', error)
    appStore.showError(t('admin.checkins.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadRecords()
}

watch(
  () => ({ ...props.filters }),
  () => {
    pagination.page = 1
    void loadRecords()
  },
  { deep: true, immediate: true }
)
</script>
