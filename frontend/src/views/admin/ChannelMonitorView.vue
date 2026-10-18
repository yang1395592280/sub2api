<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
            <Icon name="chart" size="sm" />
          </span>
          {{ t('admin.channelMonitor.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{
            isV1Mode
              ? t('channelMonitorV2.admin.descriptionV1')
              : t('channelMonitorV2.admin.descriptionV2')
          }}
        </p>
        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div
            class="tabs inline-flex w-full max-w-xl flex-wrap sm:w-auto"
            role="tablist"
            :aria-label="t('channelMonitorV2.admin.tabAria')"
          >
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="adminMonitorTab === 'v2' ? 'tab-active' : ''"
              :aria-selected="adminMonitorTab === 'v2'"
              @click="adminMonitorTab = 'v2'"
            >
              {{ t('channelMonitorV2.admin.tabV2') }}
            </button>
            <button
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="adminMonitorTab === 'legacy' ? 'tab-active' : ''"
              :aria-selected="adminMonitorTab === 'legacy'"
              @click="adminMonitorTab = 'legacy'"
            >
              {{ isV1Mode ? t('channelMonitorV2.admin.tabV1Active') : t('channelMonitorV2.admin.tabV1History') }}
            </button>
          </div>
        </div>
      </header>

      <MonitorSettingsPanel v-if="adminMonitorTab === 'v2'" />

      <TablePageLayout v-else>
      <template #filters>
        <MonitorFiltersBar
          v-model:search="searchQuery"
          v-model:provider="providerFilter"
          v-model:enabled="enabledFilter"
          :loading="loading"
          @reload="reload"
          @create="openCreateDialog"
          @sort="openSortModal"
          @manage-templates="showTemplateManager = true"
          @search-input="handleSearch"
        />
      </template>

      <template #table>
        <DataTable :columns="columns" :data="monitors" :loading="loading">
          <template #cell-name="{ row, value }">
            <div class="flex items-center gap-1.5">
              <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
              <HelpTooltip v-if="row.api_key_decrypt_failed" :content="t('admin.channelMonitor.apiKeyDecryptFailed')">
                <Icon name="exclamationTriangle" size="sm" class="text-red-500" />
              </HelpTooltip>
            </div>
          </template>

          <template #cell-provider="{ row }">
            <span class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(row.provider)">
              {{ providerLabel(row.provider) }}
            </span>
            <!-- 三种检测模式并列展示，quota 系配额数据源与纯探活一眼可分 -->
            <span class="ml-1 inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium" :class="checkModeBadgeClass(row.check_mode)">
              {{ checkModeLabel(row.check_mode) }}
            </span>
          </template>

          <template #cell-primary_model="{ row }">
            <MonitorPrimaryModelCell :row="row" />
          </template>

          <template #cell-availability_7d="{ row }">
            <span class="text-sm text-gray-900 dark:text-gray-100">{{ formatAvailability(row) }}</span>
          </template>

          <template #cell-latency="{ row }">
            <span class="text-sm text-gray-900 dark:text-gray-100">{{ formatLatency(row.primary_latency_ms) }}</span>
          </template>

          <template #cell-enabled="{ row }">
            <Toggle :modelValue="row.enabled" @update:modelValue="toggleEnabled(row)" />
          </template>

          <template #cell-actions="{ row }">
            <MonitorActionsCell
              :row="row"
              :running="runningId === row.id"
              :duplicating="duplicatingIds.has(row.id)"
              @run="handleRunNow"
              @duplicate="handleDuplicate"
              @edit="openEditDialog"
              @delete="handleDelete"
            />
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.channelMonitor.noMonitorsYet')"
              :description="t('admin.channelMonitor.createFirstMonitor')"
              :action-text="t('admin.channelMonitor.createButton')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
      </TablePageLayout>
    </div>

    <BaseDialog
      :show="showSortModal"
      :title="t('admin.channelMonitor.sortOrder')"
      width="normal"
      @close="closeSortModal"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.channelMonitor.sortOrderHint') }}
        </p>
        <VueDraggable v-model="sortableMonitors" :animation="200" class="space-y-2">
          <div
            v-for="(monitor, index) in sortableMonitors"
            :key="monitor.id"
            class="flex cursor-grab items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 transition-shadow hover:shadow-md active:cursor-grabbing dark:border-dark-600 dark:bg-dark-700"
          >
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ index + 1 }}
            </div>
            <div class="text-gray-400" :title="t('admin.channelMonitor.dragToSort')">
              <Icon name="menu" size="md" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ monitor.name }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ providerLabel(monitor.provider) }}
                <span class="ml-2">{{ t('admin.channelMonitor.currentSortOrder') }}: {{ monitor.sort_order }}</span>
              </div>
            </div>
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
                :disabled="index === 0"
                :title="t('admin.channelMonitor.moveUp')"
                :data-testid="`move-monitor-sort-${monitor.id}-up`"
                @click.stop="moveSortableMonitor(index, index - 1)"
              >
                <Icon name="arrowUp" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
                :disabled="index === sortableMonitors.length - 1"
                :title="t('admin.channelMonitor.moveDown')"
                :data-testid="`move-monitor-sort-${monitor.id}-down`"
                @click.stop="moveSortableMonitor(index, index + 1)"
              >
                <Icon name="arrowDown" size="sm" />
              </button>
            </div>
            <div class="text-sm text-gray-400">#{{ monitor.id }}</div>
          </div>
        </VueDraggable>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3 pt-4">
          <button type="button" class="btn btn-secondary" @click="closeSortModal">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="sortSubmitting" data-testid="save-monitor-sort-order" @click="saveSortOrder">
            <LoadingSpinner v-if="sortSubmitting" size="sm" class="mr-2" />
            {{ t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <MonitorFormDialog
      :show="showDialog"
      :monitor="editing"
      @close="closeDialog"
      @saved="reload"
    />

    <MonitorTemplateManagerDialog
      :show="showTemplateManager"
      @close="showTemplateManager = false"
      @updated="reload"
    />

    <MonitorRunResultDialog
      :show="showRunResult"
      :results="runResults"
      @close="showRunResult = false"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type {
  ChannelMonitor,
  CheckResult,
  ListParams,
  Provider,
} from '@/api/admin/channelMonitor'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Toggle from '@/components/common/Toggle.vue'
import MonitorFiltersBar from '@/components/admin/monitor/MonitorFiltersBar.vue'
import MonitorFormDialog from '@/components/admin/monitor/MonitorFormDialog.vue'
import MonitorTemplateManagerDialog from '@/components/admin/monitor/MonitorTemplateManagerDialog.vue'
import MonitorRunResultDialog from '@/components/admin/monitor/MonitorRunResultDialog.vue'
import MonitorPrimaryModelCell from '@/components/admin/monitor/MonitorPrimaryModelCell.vue'
import MonitorActionsCell from '@/components/admin/monitor/MonitorActionsCell.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import MonitorSettingsPanel from '@/features/channel-monitor-v2/MonitorSettingsPanel.vue'
import { isChannelMonitorV1Mode } from '@/utils/featureFlags'
import { VueDraggable } from 'vue-draggable-plus'

const { t } = useI18n()
const appStore = useAppStore()
const isV1Mode = computed(() => isChannelMonitorV1Mode())
const adminMonitorTab = ref<'v2' | 'legacy'>(isChannelMonitorV1Mode() ? 'legacy' : 'v2')
const {
  providerLabel,
  providerBadgeClass,
  checkModeLabel,
  checkModeBadgeClass,
  formatLatency,
  formatAvailability,
} = useChannelMonitorFormat()

const monitors = ref<ChannelMonitor[]>([])
const loading = ref(false)
const runningId = ref<number | null>(null)
const searchQuery = ref('')
const providerFilter = ref<Provider | ''>('')
const enabledFilter = ref<'' | 'true' | 'false'>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })

const showDialog = ref(false)
const showTemplateManager = ref(false)
const editing = ref<ChannelMonitor | null>(null)
const showDeleteDialog = ref(false)
const deleting = ref<ChannelMonitor | null>(null)
const showRunResult = ref(false)
const runResults = ref<CheckResult[]>([])
const duplicatingIds = reactive(new Set<number>())
const showSortModal = ref(false)
const sortableMonitors = ref<ChannelMonitor[]>([])
const sortSubmitting = ref(false)

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channelMonitor.columns.name'), sortable: false },
  { key: 'provider', label: t('admin.channelMonitor.columns.provider'), sortable: false },
  { key: 'primary_model', label: t('admin.channelMonitor.columns.primaryModel'), sortable: false },
  { key: 'availability_7d', label: t('admin.channelMonitor.columns.availability7d'), sortable: false },
  { key: 'latency', label: t('admin.channelMonitor.columns.latency'), sortable: false },
  { key: 'enabled', label: t('admin.channelMonitor.columns.enabled'), sortable: false },
  { key: 'actions', label: t('admin.channelMonitor.columns.actions'), sortable: false },
])

const deleteConfirmMessage = computed(() => {
  const name = deleting.value?.name || ''
  return t('admin.channelMonitor.deleteConfirm', { name })
})

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const params: ListParams = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (providerFilter.value) params.provider = providerFilter.value
    if (enabledFilter.value === 'true') params.enabled = true
    if (enabledFilter.value === 'false') params.enabled = false
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()

    const res = await adminAPI.channelMonitor.list(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = res.items || []
    pagination.total = res.total
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 300)
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  reload()
}

async function openSortModal() {
  try {
    const pageSize = 100
    const first = await adminAPI.channelMonitor.list({ page: 1, page_size: pageSize })
    const all = [...(first.items || [])]
    for (let page = 2; page <= (first.pages || 1); page += 1) {
      const next = await adminAPI.channelMonitor.list({ page, page_size: pageSize })
      all.push(...(next.items || []))
    }
    sortableMonitors.value = all.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
    showSortModal.value = true
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.loadError')))
  }
}

function closeSortModal() {
  showSortModal.value = false
  sortableMonitors.value = []
}

function moveSortableMonitor(fromIndex: number, toIndex: number) {
  if (fromIndex < 0 || toIndex < 0 || fromIndex >= sortableMonitors.value.length || toIndex >= sortableMonitors.value.length) return
  const next = [...sortableMonitors.value]
  const [moved] = next.splice(fromIndex, 1)
  next.splice(toIndex, 0, moved)
  sortableMonitors.value = next
}

async function saveSortOrder() {
  sortSubmitting.value = true
  try {
    await adminAPI.channelMonitor.updateSortOrder(
      sortableMonitors.value.map((monitor, index) => ({ id: monitor.id, sort_order: index * 10 }))
    )
    appStore.showSuccess(t('admin.channelMonitor.sortOrderUpdated'))
    closeSortModal()
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.sortOrderFailed')))
  } finally {
    sortSubmitting.value = false
  }
}

function openCreateDialog() {
  editing.value = null
  showDialog.value = true
}

function openEditDialog(row: ChannelMonitor) {
  editing.value = row
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
  editing.value = null
}

async function toggleEnabled(row: ChannelMonitor) {
  const next = !row.enabled
  try {
    await adminAPI.channelMonitor.update(row.id, { enabled: next })
    row.enabled = next
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

async function handleRunNow(row: ChannelMonitor) {
  if (!isV1Mode.value) {
    appStore.showError(t('admin.channelMonitor.runFailed'))
    return
  }
  if (runningId.value != null) return
  runningId.value = row.id
  try {
    const res = await adminAPI.channelMonitor.runNow(row.id)
    runResults.value = res.results || []
    showRunResult.value = true
    appStore.showSuccess(t('admin.channelMonitor.runSuccess'))
    // Refresh row to get latest status from backend
    void reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.runFailed')))
  } finally {
    runningId.value = null
  }
}

async function handleDuplicate(row: ChannelMonitor) {
  if (row.api_key_decrypt_failed) {
    appStore.showError(t('admin.channelMonitor.duplicateKeyUnavailable'))
    return
  }
  if (duplicatingIds.has(row.id)) return

  duplicatingIds.add(row.id)
  try {
    const duplicate = await adminAPI.channelMonitor.duplicate(row.id)
    appStore.showSuccess(t('admin.channelMonitor.duplicateSuccess', { name: duplicate.name }))
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.duplicateFailed')))
  } finally {
    duplicatingIds.delete(row.id)
  }
}

function handleDelete(row: ChannelMonitor) {
  deleting.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.channelMonitor.del(deleting.value.id)
    appStore.showSuccess(t('admin.channelMonitor.deleteSuccess'))
    showDeleteDialog.value = false
    deleting.value = null
    reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

watch(adminMonitorTab, (tab) => {
  if (tab === 'legacy' && monitors.value.length === 0) void reload()
})
onMounted(() => {
  if (adminMonitorTab.value === 'legacy') void reload()
})
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
