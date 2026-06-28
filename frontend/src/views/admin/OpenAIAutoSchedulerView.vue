<template>
  <AppLayout>
    <div class="space-y-4 p-4 sm:p-6">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">全局调度</span>
              <div class="flex items-center gap-2">
                <Toggle :modelValue="settings?.enabled || false" @update:modelValue="toggleGlobalEnabled" />
                <span class="scheduler-stat-value">{{ settings?.enabled ? '已启用' : '已关闭' }}</span>
              </div>
            </div>
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">探测间隔</span>
              <span class="scheduler-stat-value">{{ formatSeconds(settings?.probe_interval_seconds) }}</span>
            </div>
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">慢响应阈值</span>
              <span class="scheduler-stat-value">{{ formatMs(settings?.slow_threshold_ms) }}</span>
            </div>
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">熔断冷却</span>
              <span class="scheduler-stat-value">{{ formatSeconds(settings?.cooldown_seconds) }}</span>
            </div>
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">成本权重</span>
              <span class="scheduler-stat-value">{{ formatPercent(settings?.cost_weight) }}</span>
            </div>
          </div>
          <button class="btn btn-secondary shrink-0" :disabled="loading" @click="reload">
            <Icon name="refresh" size="sm" />
            <span>刷新</span>
          </button>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(260px,320px)_1fr]">
        <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">OpenAI 分组</h2>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ groups.length }} 个</span>
          </div>

          <label class="input-label" for="scheduler-group">当前分组</label>
          <select id="scheduler-group" v-model.number="selectedGroupId" class="input mb-4" @change="handleGroupChange">
            <option :value="0">全部 OpenAI 分组</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">
              {{ group.name }}
            </option>
          </select>

          <div v-if="selectedGroup" class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ selectedGroup.name }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ selectedGroup.enabled ? '参与自动调度' : '不参与自动调度' }}
                </div>
              </div>
              <Toggle :modelValue="selectedGroup.enabled" @update:modelValue="toggleSelectedGroup" />
            </div>
          </div>
          <div v-else class="rounded-md border border-dashed border-gray-200 p-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
            选择一个分组后可控制该分组是否参与 OpenAI 自动调度。
          </div>
        </section>

        <section class="space-y-4">
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="grid gap-3 md:grid-cols-4">
              <div>
                <label class="input-label" for="scheduler-filter-group">分组</label>
                <select id="scheduler-filter-group" v-model.number="filters.group_id" class="input" @change="handleScoreGroupFilterChange">
                  <option :value="0">全部</option>
                  <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
                </select>
              </div>
              <div>
                <label class="input-label" for="scheduler-filter-model">模型</label>
                <input id="scheduler-filter-model" v-model="filters.model" class="input" placeholder="gpt-5" @keyup.enter="applyFilters" />
              </div>
              <div>
                <label class="input-label" for="scheduler-filter-state">状态</label>
                <select id="scheduler-filter-state" v-model="filters.state" class="input" @change="applyFilters">
                  <option value="">全部状态</option>
                  <option value="running">running</option>
                  <option value="observing">observing</option>
                  <option value="open">open</option>
                  <option value="half_open">half_open</option>
                </select>
              </div>
              <div>
                <label class="input-label" for="scheduler-filter-search">搜索</label>
                <div class="flex gap-2">
                  <input id="scheduler-filter-search" v-model="filters.search" class="input" placeholder="账号 / 错误 / 原因" @input="handleSearchInput" />
                  <button class="btn btn-secondary shrink-0 px-3" :disabled="loading" @click="applyFilters">
                    <Icon name="search" size="sm" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div v-if="loading" class="space-y-3 p-4">
              <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-md bg-gray-100 dark:bg-dark-800"></div>
            </div>
            <div v-else-if="visibleScores.length === 0" class="p-8">
              <EmptyState title="暂无调度分数" description="当前筛选条件下没有 OpenAI 自动调度分数。" />
            </div>
            <div v-else class="divide-y divide-gray-200 dark:divide-dark-700">
              <article
                v-for="score in visibleScores"
                :key="scoreKey(score)"
                class="grid gap-4 p-4 lg:grid-cols-[minmax(220px,1fr)_180px_minmax(360px,1.4fr)] lg:items-center"
              >
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                      Account #{{ score.account_id }}
                    </span>
                    <span :class="stateBadgeClass(score.state)">{{ stateLabel(score.state) }}</span>
                  </div>
                  <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                    <span>Group #{{ score.group_id }}</span>
                    <span class="max-w-full truncate">{{ score.model }}</span>
                    <span>{{ formatDateTime(score.last_checked_at) }}</span>
                  </div>
                  <p v-if="score.reason" class="mt-2 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">
                    {{ score.reason }}
                  </p>
                </div>

                <div>
                  <div :class="scoreTextClass(score.final_score)" class="text-2xl font-semibold tabular-nums">
                    {{ formatScore(score.final_score) }}
                  </div>
                  <div class="mt-1 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                    <div class="h-full rounded-full" :class="scoreBarClass(score.final_score)" :style="{ width: scoreWidth(score.final_score) }"></div>
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    base {{ formatScore(score.base_score) }}
                  </div>
                </div>

                <div class="grid gap-3 md:grid-cols-[1fr_auto] md:items-center">
                  <div class="grid gap-2 sm:grid-cols-3">
                    <div class="scheduler-signal">
                      <span>慢响应</span>
                      <strong>{{ formatRate(score.slow_rate) }}</strong>
                      <small>{{ score.consecutive_slow_count }} 连续</small>
                    </div>
                    <div class="scheduler-signal">
                      <span>错误</span>
                      <strong>{{ formatRate(score.error_rate) }}</strong>
                      <small>{{ score.consecutive_error_count }} 连续</small>
                    </div>
                    <div class="scheduler-signal">
                      <span>样本</span>
                      <strong>{{ score.request_count }}</strong>
                      <small>TTFB {{ score.ttfb_sample_count }}</small>
                    </div>
                  </div>

                  <div class="flex flex-wrap justify-end gap-2">
                    <button
                      class="btn btn-secondary px-3 py-1.5 text-xs"
                      :disabled="actionKey === scoreKey(score)"
                      @click="handleProbe(score)"
                    >
                      <Icon name="beaker" size="xs" />
                      <span>探测</span>
                    </button>
                    <button
                      class="btn btn-secondary px-3 py-1.5 text-xs"
                      :disabled="actionKey === scoreKey(score)"
                      @click="handleReset(score)"
                    >
                      <Icon name="refresh" size="xs" />
                      <span>重置</span>
                    </button>
                  </div>

                  <div class="md:col-span-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                    <span>latency {{ formatMs(score.last_latency_ms) }}</span>
                    <span>ttfb {{ formatMs(score.last_ttfb_ms) }}</span>
                    <span v-if="score.last_status_code">HTTP {{ score.last_status_code }}</span>
                    <span v-if="score.cooldown_until">cooldown {{ formatDateTime(score.cooldown_until) }}</span>
                    <span v-if="score.last_error" class="max-w-full truncate text-red-600 dark:text-red-400">{{ score.last_error }}</span>
                  </div>
                </div>
              </article>
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
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import type {
  OpenAIAutoSchedulerGroup,
  OpenAIAutoSchedulerListParams,
  OpenAIAutoSchedulerScore,
  OpenAIAutoSchedulerSettings,
  OpenAIAutoSchedulerState,
} from '@/api/admin/openaiAutoScheduler'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const appStore = useAppStore()

const settings = ref<OpenAIAutoSchedulerSettings | null>(null)
const groups = ref<OpenAIAutoSchedulerGroup[]>([])
const scores = ref<OpenAIAutoSchedulerScore[]>([])
const loading = ref(false)
const selectedGroupId = ref(0)
const actionKey = ref<string | null>(null)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const filters = reactive<OpenAIAutoSchedulerListParams>({
  group_id: 0,
  model: 'gpt-5',
  state: '',
  search: '',
})

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
const localFilterPageSize = 200

const selectedGroup = computed(() => groups.value.find((group) => group.id === selectedGroupId.value) || null)
const hasLocalOnlyFilters = computed(() => Boolean(filters.state || String(filters.search || '').trim()))

const locallyFilteredScores = computed(() => filterScoresLocally(scores.value))

const visibleScores = computed(() => {
  if (!hasLocalOnlyFilters.value) return scores.value
  const start = (pagination.page - 1) * pagination.page_size
  return locallyFilteredScores.value.slice(start, start + pagination.page_size)
})

function filterScoresLocally(items: OpenAIAutoSchedulerScore[]): OpenAIAutoSchedulerScore[] {
  const state = filters.state
  const search = String(filters.search || '').trim().toLowerCase()
  return items.filter((score) => {
    if (state && score.state !== state) return false
    if (!search) return true
    return [
      String(score.account_id),
      String(score.group_id),
      score.model,
      score.reason,
      score.last_error || '',
      score.state,
    ].some((value) => value.toLowerCase().includes(search))
  })
}

async function loadSettingsAndGroups() {
  const [nextSettings, nextGroups] = await Promise.all([
    adminAPI.openaiAutoScheduler.getSettings(),
    adminAPI.openaiAutoScheduler.listGroups(),
  ])
  settings.value = nextSettings
  groups.value = nextGroups
  if (selectedGroupId.value && !nextGroups.some((group) => group.id === selectedGroupId.value)) {
    selectedGroupId.value = 0
  }
}

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    await loadSettingsAndGroups()
    const params = listParams()
    const result = await adminAPI.openaiAutoScheduler.listScores(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    scores.value = result.items || []
    pagination.total = hasLocalOnlyFilters.value ? locallyFilteredScores.value.length : result.total || 0
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, '加载 OpenAI 自动调度数据失败'))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function listParams(): OpenAIAutoSchedulerListParams {
  const params: OpenAIAutoSchedulerListParams = {
    page: hasLocalOnlyFilters.value ? 1 : pagination.page,
    page_size: hasLocalOnlyFilters.value ? localFilterPageSize : pagination.page_size,
  }
  if (filters.group_id) params.group_id = filters.group_id
  if (filters.model?.trim()) params.model = filters.model.trim()
  return params
}

function applyFilters() {
  pagination.page = 1
  reload()
}

function handleSearchInput() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(applyFilters, 300)
}

function handleGroupChange() {
  filters.group_id = selectedGroupId.value
  applyFilters()
}

function handlePageChange(page: number) {
  pagination.page = page
  reload()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  reload()
}

function handleScoreGroupFilterChange() {
  if (selectedGroupId.value !== filters.group_id) selectedGroupId.value = Number(filters.group_id || 0)
  applyFilters()
}

async function toggleGlobalEnabled(enabled: boolean) {
  if (!settings.value) return
  const previous = settings.value
  settings.value = { ...previous, enabled }
  try {
    settings.value = await adminAPI.openaiAutoScheduler.updateSettings(settings.value)
    appStore.showSuccess(enabled ? 'OpenAI 自动调度已启用' : 'OpenAI 自动调度已关闭')
  } catch (err: unknown) {
    settings.value = previous
    appStore.showError(extractApiErrorMessage(err, '更新全局调度设置失败'))
  }
}

async function toggleSelectedGroup(enabled: boolean) {
  if (!selectedGroup.value) return
  const group = selectedGroup.value
  const previous = group.enabled
  group.enabled = enabled
  try {
    const updated = await adminAPI.openaiAutoScheduler.updateGroup(group.id, { enabled })
    const index = groups.value.findIndex((item) => item.id === group.id)
    if (index >= 0) groups.value[index] = updated
    appStore.showSuccess(enabled ? '分组已加入自动调度' : '分组已退出自动调度')
  } catch (err: unknown) {
    group.enabled = previous
    appStore.showError(extractApiErrorMessage(err, '更新分组调度状态失败'))
  }
}

async function handleReset(score: OpenAIAutoSchedulerScore) {
  const key = scoreKey(score)
  actionKey.value = key
  try {
    await adminAPI.openaiAutoScheduler.resetScore(score.account_id, {
      group_id: score.group_id,
      model: score.model,
    })
    appStore.showSuccess('调度分数已重置')
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, '重置调度分数失败'))
  } finally {
    if (actionKey.value === key) actionKey.value = null
  }
}

async function handleProbe(score: OpenAIAutoSchedulerScore) {
  const key = scoreKey(score)
  actionKey.value = key
  try {
    const result = await adminAPI.openaiAutoScheduler.probeScore(score.account_id, {
      group_id: score.group_id,
      model: score.model,
    })
    appStore.showSuccess(result.success ? '探测成功' : `探测完成：${result.message || '未通过'}`)
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, '执行手动探测失败'))
  } finally {
    if (actionKey.value === key) actionKey.value = null
  }
}

function scoreKey(score: OpenAIAutoSchedulerScore): string {
  return `${score.account_id}:${score.group_id}:${score.model}`
}

function stateLabel(state: OpenAIAutoSchedulerState): string {
  const labels: Record<OpenAIAutoSchedulerState, string> = {
    running: 'running',
    observing: 'observing',
    open: 'open',
    half_open: 'half-open',
  }
  return labels[state] || state
}

function stateBadgeClass(state: OpenAIAutoSchedulerState): string {
  const base = 'inline-flex rounded-md px-2 py-0.5 text-xs font-medium'
  const classes: Record<OpenAIAutoSchedulerState, string> = {
    running: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
    observing: 'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
    open: 'bg-red-50 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    half_open: 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300',
  }
  return `${base} ${classes[state]}`
}

function scoreTextClass(score: number): string {
  if (score >= 8500) return 'text-emerald-600 dark:text-emerald-400'
  if (score >= 6500) return 'text-sky-600 dark:text-sky-400'
  if (score >= 4500) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

function scoreBarClass(score: number): string {
  if (score >= 8500) return 'bg-emerald-500'
  if (score >= 6500) return 'bg-sky-500'
  if (score >= 4500) return 'bg-amber-500'
  return 'bg-red-500'
}

function scoreWidth(score: number): string {
  return `${Math.max(0, Math.min(100, score / 100))}%`
}

function formatScore(score: number): string {
  return (Math.max(0, Math.min(10000, score)) / 10000).toFixed(4)
}

function formatMs(value?: number | null): string {
  if (value == null) return '-'
  return `${value}ms`
}

function formatSeconds(value?: number | null): string {
  if (value == null) return '-'
  return `${value}s`
}

function formatPercent(value?: number | null): string {
  if (value == null) return '-'
  return `${Math.round(value * 100)}%`
}

function formatRate(value: number): string {
  return `${Math.round(value * 100)}%`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '未采样'
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

watch(selectedGroupId, (groupId) => {
  if (filters.group_id !== groupId) filters.group_id = groupId
})

onMounted(reload)
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.scheduler-stat {
  min-width: 0;
}

.scheduler-stat-label {
  display: block;
  margin-bottom: 0.25rem;
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

.scheduler-stat-value {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(17 24 39);
}

:global(.dark) .scheduler-stat-label {
  color: rgb(156 163 175);
}

:global(.dark) .scheduler-stat-value {
  color: white;
}

.scheduler-signal {
  min-width: 0;
  border-radius: 0.375rem;
  background: rgb(249 250 251);
  padding: 0.5rem 0.625rem;
}

:global(.dark) .scheduler-signal {
  background: rgb(31 41 55 / 0.6);
}

.scheduler-signal span,
.scheduler-signal small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

.scheduler-signal strong {
  display: block;
  margin-top: 0.125rem;
  font-size: 0.875rem;
  color: rgb(17 24 39);
}

:global(.dark) .scheduler-signal span,
:global(.dark) .scheduler-signal small {
  color: rgb(156 163 175);
}

:global(.dark) .scheduler-signal strong {
  color: white;
}
</style>
