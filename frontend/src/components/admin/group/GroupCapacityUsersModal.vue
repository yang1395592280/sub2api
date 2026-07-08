<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.capacityUsersTitle', { name: group?.name || '-' })"
    width="full"
    @close="emit('close')"
  >
    <div v-if="group" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
          <span class="text-gray-400">|</span>
          <span class="text-gray-600 dark:text-gray-400">
            {{ t('admin.groups.capacityUsersActiveOnly') }}
          </span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300">
            {{ pagination.total }}
          </span>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="loadUsers"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          <span>{{ t('common.refresh') }}</span>
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div
        v-else-if="items.length === 0"
        class="rounded-lg border border-dashed border-gray-300 bg-white px-6 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400"
      >
        {{ t('admin.groups.noActiveCapacityUsers') }}
      </div>

      <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
        <div class="max-h-[68vh] overflow-auto">
          <table class="w-full min-w-[1080px] text-sm">
            <thead class="sticky top-0 z-[1] bg-gray-50 dark:bg-dark-700">
              <tr class="border-b border-gray-200 dark:border-dark-600">
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.user') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userNotes') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.currentConcurrency') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.currentRPM') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.rpmSource') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.rpmLimits') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userStatus') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-600 dark:bg-dark-800">
              <tr v-for="item in items" :key="item.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-3 py-2">
                  <div class="flex flex-col">
                    <span class="font-medium text-gray-900 dark:text-white">{{ item.username || '-' }}</span>
                    <span class="break-all text-xs text-gray-500 dark:text-gray-400">#{{ item.user_id }} · {{ item.email }}</span>
                  </div>
                </td>
                <td class="max-w-[180px] truncate px-3 py-2 text-gray-500 dark:text-gray-400" :title="item.notes">
                  {{ item.notes || '-' }}
                </td>
                <td class="px-3 py-2">
                  <span :class="metricClass(item.current_concurrency, item.concurrency_limit)">
                    {{ item.current_concurrency }} / {{ item.concurrency_limit }}
                  </span>
                </td>
                <td class="px-3 py-2">
                  <span :class="metricClass(item.current_rpm, item.effective_rpm_limit)">
                    {{ item.current_rpm }} / {{ item.effective_rpm_limit || t('common.unlimited') }}
                  </span>
                </td>
                <td class="px-3 py-2">
                  <span class="rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">
                    {{ rpmSourceLabel(item.rpm_limit_source) }}
                  </span>
                </td>
                <td class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400">
                  <div>{{ t('admin.groups.groupRpmLimit') }}: {{ limitText(item.group_rpm_limit) }}</div>
                  <div>{{ t('admin.groups.userRpmLimit') }}: {{ limitText(item.user_rpm_limit) }}</div>
                  <div v-if="item.rpm_override !== null && item.rpm_override !== undefined">
                    {{ t('admin.groups.rpmOverride') }}: {{ limitText(item.rpm_override) }}
                  </div>
                </td>
                <td class="px-3 py-2">
                  <span
                    :class="[
                      'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                      item.status === 'active'
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                        : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                    ]"
                  >
                    {{ item.status }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupCapacityUserDetail } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  show: boolean
  group: AdminGroup | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const loading = ref(false)
const items = ref<GroupCapacityUserDetail[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1
})

async function loadUsers() {
  if (!props.group) return
  loading.value = true
  try {
    const data = await adminAPI.groups.getGroupCapacityUsers(
      props.group.id,
      pagination.page,
      pagination.page_size,
      true
    )
    items.value = data.items || []
    pagination.total = data.total || 0
    pagination.page = data.page || pagination.page
    pagination.page_size = data.page_size || pagination.page_size
    pagination.pages = data.pages || 1
  } catch (error) {
    items.value = []
    pagination.total = 0
    console.error('Error loading group capacity users:', error)
  } finally {
    loading.value = false
  }
}

function resetState() {
  items.value = []
  pagination.page = 1
  pagination.page_size = 20
  pagination.total = 0
  pagination.pages = 1
}

function handlePageChange(page: number) {
  pagination.page = page
  loadUsers()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page = 1
  pagination.page_size = pageSize
  loadUsers()
}

function metricClass(used: number, max: number) {
  const base = 'inline-flex rounded bg-gray-100 px-2 py-0.5 font-mono text-xs font-medium dark:bg-dark-600'
  if (max > 0 && used >= max) return `${base} text-red-700 dark:text-red-400`
  if (used > 0) return `${base} text-amber-700 dark:text-amber-400`
  return `${base} text-gray-600 dark:text-gray-300`
}

function limitText(value: number | null | undefined) {
  return value && value > 0 ? String(value) : t('common.unlimited')
}

function rpmSourceLabel(source: string) {
  const key = `admin.groups.rpmLimitSources.${source}`
  const label = t(key)
  return label === key ? source : label
}

watch(
  () => [props.show, props.group?.id],
  ([show]) => {
    if (show && props.group) {
      resetState()
      loadUsers()
    } else if (!show) {
      resetState()
    }
  },
  { immediate: true }
)
</script>
