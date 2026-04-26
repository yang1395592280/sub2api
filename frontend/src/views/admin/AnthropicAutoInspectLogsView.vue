<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="grid flex-1 grid-cols-1 gap-3 md:grid-cols-4">
            <label class="form-control">
              <span class="label-text">关键词</span>
              <input v-model="filters.search" type="text" class="input" placeholder="账号名 / 返回内容" />
            </label>
            <label class="form-control">
              <span class="label-text">结果</span>
              <select v-model="filters.result" class="input">
                <option value="">全部</option>
                <option value="success">success</option>
                <option value="rate_limited">rate_limited</option>
                <option value="error">error</option>
                <option value="skipped">skipped</option>
              </select>
            </label>
            <label class="form-control">
              <span class="label-text">启用巡检</span>
              <input v-model="settings.enabled" type="checkbox" class="toggle toggle-primary mt-2" />
            </label>
            <label class="form-control">
              <span class="label-text">错误冷却(分钟)</span>
              <input v-model.number="settings.error_cooldown_minutes" type="number" min="1" class="input" />
            </label>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadAll">刷新</button>
            <button class="btn btn-secondary" :disabled="saving" @click="saveSettings">保存设置</button>
            <button class="btn btn-primary" :disabled="running" @click="triggerRunNow">立即巡检</button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <div class="card overflow-hidden xl:col-span-2">
          <div class="border-b border-gray-100 px-4 py-3 text-sm font-semibold text-gray-900 dark:border-dark-700 dark:text-white">
            巡检日志
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full text-sm">
              <thead class="bg-gray-50 text-left text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                <tr>
                  <th class="px-4 py-3">时间</th>
                  <th class="px-4 py-3">账号</th>
                  <th class="px-4 py-3">结果</th>
                  <th class="px-4 py-3">恢复时间</th>
                  <th class="px-4 py-3">返回内容</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in logs" :key="item.id" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="px-4 py-3 align-top">{{ formatDateTime(item.created_at) }}</td>
                  <td class="px-4 py-3 align-top">{{ item.account_name_snapshot }}</td>
                  <td class="px-4 py-3 align-top">
                    <span class="rounded px-2 py-1 text-xs font-medium" :class="resultClass(item.result)">
                      {{ item.result }}
                    </span>
                  </td>
                  <td class="px-4 py-3 align-top">
                    {{ item.temp_unschedulable_until ? formatDateTime(item.temp_unschedulable_until) : '-' }}
                  </td>
                  <td class="max-w-xl px-4 py-3 align-top text-gray-600 dark:text-gray-300">
                    <div class="line-clamp-3 whitespace-pre-wrap break-all">
                      {{ item.error_message || item.response_text || item.skip_reason || '-' }}
                    </div>
                  </td>
                </tr>
                <tr v-if="!loading && logs.length === 0">
                  <td colspan="5" class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">暂无日志</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="border-t border-gray-100 p-4 dark:border-dark-700">
            <Pagination
              v-if="logPagination.total > 0"
              :page="logPagination.page"
              :total="logPagination.total"
              :page-size="logPagination.page_size"
              @update:page="handleLogPageChange"
              @update:pageSize="handleLogPageSizeChange"
            />
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 text-sm font-semibold text-gray-900 dark:border-dark-700 dark:text-white">
            最近批次
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="batch in batches" :key="batch.id" class="px-4 py-3">
              <div class="flex items-center justify-between">
                <span class="font-medium text-gray-900 dark:text-white">Batch #{{ batch.id }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ batch.status }}</span>
              </div>
              <div class="mt-2 grid grid-cols-2 gap-2 text-xs text-gray-600 dark:text-gray-300">
                <span>total: {{ batch.total_accounts }}</span>
                <span>processed: {{ batch.processed_accounts }}</span>
                <span>success: {{ batch.success_count }}</span>
                <span>rate_limited: {{ batch.rate_limited_count }}</span>
                <span>error: {{ batch.error_count }}</span>
                <span>skipped: {{ batch.skipped_count }}</span>
              </div>
              <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ formatDateTime(batch.created_at) }}
              </div>
            </div>
            <div v-if="!batches.length" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
              暂无批次记录
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminAPI } from '@/api/admin'
import type {
  AnthropicAutoInspectBatch,
  AnthropicAutoInspectLog,
  AnthropicAutoInspectSettings
} from '@/api/admin/anthropicAutoInspect'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const running = ref(false)
const logs = ref<AnthropicAutoInspectLog[]>([])
const batches = ref<AnthropicAutoInspectBatch[]>([])
const settings = reactive<AnthropicAutoInspectSettings>({
  enabled: false,
  interval_minutes: 1,
  error_cooldown_minutes: 30
})
const filters = reactive({
  page: 1,
  page_size: 20,
  search: '',
  result: ''
})
const logPagination = reactive({
  total: 0,
  page: 1,
  page_size: 20
})

const loadAll = async () => {
  loading.value = true
  try {
    const [settingsData, logsData, batchesData] = await Promise.all([
      adminAPI.anthropicAutoInspect.getSettings(),
      adminAPI.anthropicAutoInspect.listLogs(filters),
      adminAPI.anthropicAutoInspect.listBatches({ page: 1, page_size: 10 })
    ])
    Object.assign(settings, settingsData)
    logs.value = logsData.items
    batches.value = batchesData.items
    Object.assign(logPagination, logsData.pagination)
  } catch (error: any) {
    appStore.showError(error?.message || '加载 Anthropic 自动巡检数据失败')
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  saving.value = true
  try {
    await adminAPI.anthropicAutoInspect.updateSettings({
      ...settings,
      interval_minutes: Math.max(1, settings.interval_minutes || 1),
      error_cooldown_minutes: Math.max(1, settings.error_cooldown_minutes || 30)
    })
    appStore.showSuccess('Anthropic 自动巡检设置已保存')
    await loadAll()
  } catch (error: any) {
    appStore.showError(error?.message || '保存设置失败')
  } finally {
    saving.value = false
  }
}

const triggerRunNow = async () => {
  running.value = true
  try {
    await adminAPI.anthropicAutoInspect.runNow()
    appStore.showSuccess('已触发一次 Anthropic 自动巡检')
    await loadAll()
  } catch (error: any) {
    appStore.showError(error?.message || '触发巡检失败')
  } finally {
    running.value = false
  }
}

const handleLogPageChange = async (page: number) => {
  filters.page = page
  await loadAll()
}

const handleLogPageSizeChange = async (pageSize: number) => {
  filters.page = 1
  filters.page_size = pageSize
  await loadAll()
}

const resultClass = (result: string) => {
  if (result === 'success') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (result === 'rate_limited') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (result === 'error') return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

onMounted(loadAll)
</script>
