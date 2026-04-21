<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.checkins.searchPlaceholder')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <div class="w-40">
            <label class="sr-only" for="checkin-date">{{ t('admin.checkins.date') }}</label>
            <input
              id="checkin-date"
              v-model="filters.date"
              type="date"
              class="input"
              @change="handleDateChange"
            />
          </div>
          <div class="flex flex-1 justify-end">
            <button
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('common.refresh')"
              @click="loadRecords"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
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
              {{ formatReward(value) }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-user_timezone="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ value || '-' }}
            </span>
          </template>

          <template #empty>
            <div class="flex flex-col items-center py-10 text-center">
              <Icon name="inbox" size="xl" class="mb-4 text-gray-400 dark:text-dark-500" />
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">
                {{ t('admin.checkins.noRecords') }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCheckinRecord } from '@/api/admin/checkins'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const records = ref<AdminCheckinRecord[]>([])
const searchQuery = ref('')
const filters = reactive({
  date: defaultDateValue()
})
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
  { key: 'user_email', label: t('admin.checkins.columns.user') },
  { key: 'checkin_date', label: t('admin.checkins.columns.checkinDate'), sortable: true },
  { key: 'reward_amount', label: t('admin.checkins.columns.rewardAmount'), sortable: true },
  { key: 'user_timezone', label: t('admin.checkins.columns.timezone') },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt'), sortable: true }
])

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null

function defaultDateValue() {
  const now = new Date()
  const day = String(now.getDate()).padStart(2, '0')
  const month = String(now.getMonth() + 1).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

function formatReward(value: number) {
  return `$${value.toFixed(8).replace(/\.?0+$/, '')}`
}

async function loadRecords() {
  if (abortController) {
    abortController.abort()
  }

  const currentController = new AbortController()
  abortController = currentController
  loading.value = true

  try {
    const response = await adminAPI.checkins.list(
      pagination.page,
      pagination.page_size,
      {
        search: searchQuery.value || undefined,
        date: filters.date || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal: currentController.signal }
    )

    if (currentController.signal.aborted || abortController !== currentController) {
      return
    }

    records.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      abortController !== currentController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    console.error('Error loading admin check-in records:', error)
    appStore.showError(t('admin.checkins.failedToLoad'))
  } finally {
    if (abortController === currentController) {
      loading.value = false
      abortController = null
    }
  }
}

function handleSearch() {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    void loadRecords()
  }, 300)
}

function handleDateChange() {
  pagination.page = 1
  void loadRecords()
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

onMounted(() => {
  void loadRecords()
})

onUnmounted(() => {
  abortController?.abort()
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
})
</script>
