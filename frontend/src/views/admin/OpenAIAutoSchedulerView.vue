<template>
  <AppLayout>
    <div data-testid="scheduler-page" class="flex min-h-[calc(100vh-8rem)] flex-col gap-4">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <div class="scheduler-stat">
              <span class="scheduler-stat-label">全局调度</span>
              <div class="flex items-center gap-2">
                <Toggle :modelValue="settings?.enabled || false" @update:modelValue="toggleGlobalEnabled" />
                <span class="scheduler-stat-value">{{ settings?.enabled ? '已启用' : '已关闭' }}</span>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                全局关闭后走系统原调度；自动调度开启时仍先经过系统原候选过滤。
              </p>
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
          <div class="flex shrink-0 flex-wrap gap-2">
            <button class="btn btn-secondary" :disabled="!settings || loading" @click="openSettingsEditor">
              <Icon name="cog" size="sm" />
              <span>编辑调度配置</span>
            </button>
            <button class="btn btn-secondary" :disabled="loading" @click="reload">
              <Icon name="refresh" size="sm" />
              <span>刷新</span>
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="editingSettings"
        class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
      >
        <form data-testid="scheduler-settings-form" class="space-y-4" @submit.prevent="saveSettings">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">调度配置</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">调整自动探测、熔断和成本评分参数。</p>
            </div>
            <div class="flex items-center gap-2">
              <Toggle :modelValue="settingsForm.enabled" @update:modelValue="settingsForm.enabled = $event" />
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ settingsForm.enabled ? '已启用' : '已关闭' }}</span>
            </div>
          </div>

          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <div>
              <label class="input-label" for="scheduler-settings-probe-interval">探测间隔（秒）</label>
              <input id="scheduler-settings-probe-interval" v-model.number="settingsForm.probe_interval_seconds" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-slow-threshold">慢响应阈值（ms）</label>
              <input id="scheduler-settings-slow-threshold" v-model.number="settingsForm.slow_threshold_ms" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-severe-threshold">重慢阈值（ms）</label>
              <input id="scheduler-settings-severe-threshold" v-model.number="settingsForm.severe_slow_threshold_ms" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-cooldown">熔断冷却（秒）</label>
              <input id="scheduler-settings-cooldown" v-model.number="settingsForm.cooldown_seconds" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-cost-weight">成本权重（%）</label>
              <input id="scheduler-settings-cost-weight" v-model.number="settingsForm.cost_weight_percent" type="number" min="0" max="100" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-slow-breaker">连续慢响应熔断</label>
              <input id="scheduler-settings-slow-breaker" v-model.number="settingsForm.consecutive_slow_breaker_threshold" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-error-breaker">连续错误熔断</label>
              <input id="scheduler-settings-error-breaker" v-model.number="settingsForm.consecutive_error_breaker_threshold" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-half-open">半开成功阈值</label>
              <input id="scheduler-settings-half-open" v-model.number="settingsForm.half_open_success_threshold" type="number" min="1" class="input" />
            </div>
            <div>
              <label class="input-label" for="scheduler-settings-recovery">恢复步长</label>
              <input id="scheduler-settings-recovery" v-model.number="settingsForm.recovery_step" type="number" min="1" class="input" />
            </div>
          </div>

          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" :disabled="settingsSaving" @click="editingSettings = false">取消</button>
            <button type="submit" class="btn btn-primary" :disabled="settingsSaving">
              <Icon name="check" size="sm" />
              <span>保存配置</span>
            </button>
          </div>
        </form>
      </div>

      <div
        data-testid="scheduler-main-grid"
        class="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(240px,300px)_minmax(0,1fr)] xl:grid-cols-[clamp(260px,18vw,360px)_minmax(0,1fr)]"
      >
        <section
          data-testid="scheduler-group-sidebar"
          class="min-h-0 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">OpenAI 分组调度</h2>
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ groups.length }} 个</span>
          </div>
          <p class="mb-3 text-xs text-gray-500 dark:text-dark-400">
            当前分组关闭时只展示分数，不参与自动调度。
          </p>

          <div class="space-y-2 lg:max-h-[calc(100vh-22rem)] lg:overflow-y-auto lg:pr-1">
            <button
              v-for="group in groups"
              :key="group.id"
              type="button"
              :data-testid="`scheduler-group-card-${group.id}`"
              class="w-full rounded-md border p-3 text-left transition"
              :class="group.id === selectedGroupId
                ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-500/10'
                : 'border-gray-200 bg-white hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900'"
              @click="selectGroup(group.id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ group.name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ group.enabled ? '参与自动调度' : '不参与自动调度' }} · 默认模型 {{ filters.model || 'gpt-5.4' }}
                  </div>
                </div>
                <Toggle
                  :modelValue="group.enabled"
                  @click.stop
                  @update:modelValue="group.id === selectedGroupId ? toggleSelectedGroup($event) : selectGroup(group.id)"
                />
              </div>
            </button>
          </div>
        </section>

        <section data-testid="scheduler-score-panel" class="flex min-h-0 flex-1 flex-col gap-4">
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
                <select id="scheduler-filter-model" v-model="filters.model" class="input" @change="applyFilters">
                  <option v-for="model in schedulerModelOptions" :key="model" :value="model">{{ model }}</option>
                </select>
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

          <div
            data-testid="scheduler-score-card"
            class="flex min-h-[420px] flex-1 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
          >
            <div v-if="loading" class="space-y-3 p-4">
              <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-md bg-gray-100 dark:bg-dark-800"></div>
            </div>
            <div v-else-if="visibleScores.length === 0" class="p-8">
              <EmptyState title="暂无调度分数" description="当前筛选条件下没有 OpenAI 自动调度分数。" />
            </div>
            <div v-else class="min-h-0 flex-1 overflow-auto">
              <table data-testid="scheduler-score-table" class="w-full min-w-[1280px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">上游渠道</th>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">状态</th>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">实际调度分</th>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">健康分拆解</th>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">探测样本</th>
                    <th class="px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400">最近风险</th>
                    <th class="px-4 py-3 text-right text-xs font-semibold text-gray-500 dark:text-dark-400">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr v-for="score in visibleScores" :key="scoreKey(score)">
                    <td class="max-w-[260px] px-4 py-3">
                      <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ scoreTitle(score) }}</div>
                      <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">
                        #{{ score.account_id }} · Group #{{ score.group_id }} · {{ score.model }}
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <span :class="stateBadgeClass(score.state)">{{ stateLabel(score.state) }}</span>
                      <div v-if="score.cooldown_until" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                        冷却至 {{ formatDateTime(score.cooldown_until) }}
                      </div>
                    </td>
                    <td class="px-4 py-3">
                      <div :class="scoreTextClass(score.final_score)" class="text-xl font-semibold tabular-nums">
                        {{ formatScore(score.final_score) }}
                      </div>
                      <div class="mt-1 h-2 w-28 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                        <div class="h-full rounded-full" :class="scoreBarClass(score.final_score)" :style="{ width: scoreWidth(score.final_score) }"></div>
                      </div>
                      <div class="mt-1 max-w-[220px] text-xs text-gray-500 dark:text-dark-400">{{ dispatchScoreHint(score) }}</div>
                    </td>
                    <td class="px-4 py-3 text-xs text-gray-600 dark:text-dark-300">
                      <div>基础分 {{ formatScore(score.base_score) }}（新渠道默认起点）</div>
                      <div>延迟修正 {{ formatComponentScore(score.latency_score) }}</div>
                      <div>错误惩罚 {{ formatComponentScore(score.error_score) }}</div>
                      <div>恢复加分 {{ formatComponentScore(score.recovery_score) }}</div>
                      <div>成本修正 {{ formatComponentScore(score.cost_score) }}</div>
                    </td>
                    <td class="px-4 py-3 text-xs text-gray-600 dark:text-dark-300">
                      <div>请求样本 {{ score.request_count }}</div>
                      <div>TTFB样本 {{ score.ttfb_sample_count }}</div>
                      <div>最近延迟 {{ formatMs(score.last_latency_ms) }}</div>
                      <div>最近TTFB {{ formatMs(score.last_ttfb_ms) }}</div>
                      <div v-if="score.last_status_code">HTTP {{ score.last_status_code }}</div>
                    </td>
                    <td class="max-w-[260px] px-4 py-3">
                      <div class="truncate text-xs" :class="score.last_error ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'">
                        {{ errorSummary(score.last_error) }}
                      </div>
                      <div v-if="score.reason" class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">{{ score.reason }}</div>
                    </td>
                    <td class="px-4 py-3">
                      <div class="flex justify-end gap-2">
                        <button class="btn btn-secondary px-3 py-1.5 text-xs" @click="openScoreDrawer(score)">
                          <Icon name="eye" size="xs" />
                          <span>查看详情</span>
                        </button>
                        <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="actionKey === scoreKey(score)" @click="handleProbe(score)">
                          <Icon name="beaker" size="xs" />
                          <span>探测</span>
                        </button>
                        <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="actionKey === scoreKey(score)" @click="handleReset(score)">
                          <Icon name="refresh" size="xs" />
                          <span>重置</span>
                        </button>
                      </div>
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
        </section>
      </div>

      <aside
        v-if="selectedScore"
        data-testid="scheduler-score-drawer"
        class="fixed inset-y-0 right-0 z-40 w-full max-w-xl overflow-y-auto border-l border-gray-200 bg-white p-5 shadow-xl dark:border-dark-700 dark:bg-dark-900"
      >
        <div class="mb-4 flex items-start justify-between gap-4">
          <div class="min-w-0">
            <span :class="stateBadgeClass(selectedScore.state)">{{ stateLabel(selectedScore.state) }}</span>
            <h2 class="mt-3 truncate text-lg font-semibold text-gray-900 dark:text-white">{{ scoreTitle(selectedScore) }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              #{{ selectedScore.account_id }} · Group #{{ selectedScore.group_id }} · {{ selectedScore.model }}
            </p>
          </div>
          <button class="btn btn-secondary px-3" @click="closeScoreDrawer">关闭</button>
        </div>

        <div class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">实际调度分</div>
          <div :class="scoreTextClass(selectedScore.final_score)" class="mt-1 text-2xl font-semibold tabular-nums">
            {{ formatScore(selectedScore.final_score) }}
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ dispatchScoreHint(selectedScore) }}</p>
        </div>

        <section class="mt-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">评分拆解</h3>
          <div class="mt-2 grid gap-2 text-sm text-gray-700 dark:text-dark-200">
            <div>基础分 {{ formatScore(selectedScore.base_score) }}（新渠道默认起点）</div>
            <div>延迟修正 {{ formatComponentScore(selectedScore.latency_score) }}</div>
            <div>错误惩罚 {{ formatComponentScore(selectedScore.error_score) }}</div>
            <div>恢复加分 {{ formatComponentScore(selectedScore.recovery_score) }}</div>
            <div>成本修正 {{ formatComponentScore(selectedScore.cost_score) }}</div>
          </div>
        </section>

        <section class="mt-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">探测样本</h3>
          <div class="mt-2 grid grid-cols-2 gap-2 text-sm text-gray-700 dark:text-dark-200">
            <div>请求样本 {{ selectedScore.request_count }}</div>
            <div>TTFB样本 {{ selectedScore.ttfb_sample_count }}</div>
            <div>慢响应率 {{ formatRate(selectedScore.slow_rate) }}</div>
            <div>错误率 {{ formatRate(selectedScore.error_rate) }}</div>
            <div>卡住率 {{ formatRate(selectedScore.stuck_rate) }}</div>
            <div>最近延迟 {{ formatMs(selectedScore.last_latency_ms) }}</div>
            <div>最近TTFB {{ formatMs(selectedScore.last_ttfb_ms) }}</div>
            <div v-if="selectedScore.cooldown_until">冷却至 {{ formatDateTime(selectedScore.cooldown_until) }}</div>
          </div>
        </section>

        <section class="mt-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">完整错误</h3>
          <pre class="mt-2 whitespace-pre-wrap break-words rounded-md bg-gray-50 p-3 text-xs text-red-700 dark:bg-dark-800 dark:text-red-300">{{ selectedScore.last_error || '无异常' }}</pre>
        </section>

        <section class="mt-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">最近事件</h3>
          <div v-if="drawerLoading" class="mt-2 text-sm text-gray-500 dark:text-dark-400">加载中...</div>
          <div v-else-if="scoreEvents.length === 0" class="mt-2 text-sm text-gray-500 dark:text-dark-400">暂无事件</div>
          <div v-else class="mt-2 space-y-2">
            <div
              v-for="event in scoreEvents"
              :key="`${event.created_at}:${event.event_type}:${event.score_after}`"
              class="rounded-md border border-gray-200 p-3 text-xs dark:border-dark-700"
            >
              <div class="font-semibold text-gray-900 dark:text-white">{{ event.event_type }} · {{ formatDateTime(event.created_at) }}</div>
              <div class="mt-1 text-gray-600 dark:text-dark-300">{{ event.message || '无消息' }}</div>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import type {
  OpenAIAutoSchedulerEvent,
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
const editingSettings = ref(false)
const settingsSaving = ref(false)
const selectedScore = ref<OpenAIAutoSchedulerScore | null>(null)
const scoreEvents = ref<OpenAIAutoSchedulerEvent[]>([])
const drawerLoading = ref(false)
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const schedulerModelOptions = ['gpt-5.4', 'gpt-5.5']
const filters = reactive<OpenAIAutoSchedulerListParams>({
  group_id: 0,
  model: schedulerModelOptions[0],
  state: '',
  search: '',
})
const settingsForm = reactive({
  enabled: false,
  probe_interval_seconds: 60,
  slow_threshold_ms: 10000,
  severe_slow_threshold_ms: 20000,
  consecutive_slow_breaker_threshold: 3,
  consecutive_error_breaker_threshold: 2,
  cooldown_seconds: 120,
  half_open_success_threshold: 3,
  cost_weight_percent: 20,
  recovery_step: 800,
})

let abortController: AbortController | null = null
let drawerAbortController: AbortController | null = null
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
      score.account_name || '',
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
  const firstGroupId = nextGroups[0]?.id || 0
  if (!selectedGroupId.value || !nextGroups.some((group) => group.id === selectedGroupId.value)) {
    selectedGroupId.value = firstGroupId
  }
  if (!filters.group_id || !nextGroups.some((group) => group.id === filters.group_id)) {
    filters.group_id = firstGroupId
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

function selectGroup(groupId: number) {
  if (selectedGroupId.value === groupId) return
  selectedGroupId.value = groupId
  filters.group_id = groupId
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

function openSettingsEditor() {
  if (!settings.value) return
  syncSettingsForm(settings.value)
  editingSettings.value = true
}

function syncSettingsForm(nextSettings: OpenAIAutoSchedulerSettings) {
  settingsForm.enabled = nextSettings.enabled
  settingsForm.probe_interval_seconds = nextSettings.probe_interval_seconds
  settingsForm.slow_threshold_ms = nextSettings.slow_threshold_ms
  settingsForm.severe_slow_threshold_ms = nextSettings.severe_slow_threshold_ms
  settingsForm.consecutive_slow_breaker_threshold = nextSettings.consecutive_slow_breaker_threshold
  settingsForm.consecutive_error_breaker_threshold = nextSettings.consecutive_error_breaker_threshold
  settingsForm.cooldown_seconds = nextSettings.cooldown_seconds
  settingsForm.half_open_success_threshold = nextSettings.half_open_success_threshold
  settingsForm.cost_weight_percent = Math.round(nextSettings.cost_weight * 100)
  settingsForm.recovery_step = nextSettings.recovery_step
}

async function saveSettings() {
  if (!settings.value) return
  settingsSaving.value = true
  try {
    const payload: OpenAIAutoSchedulerSettings = {
      enabled: settingsForm.enabled,
      probe_interval_seconds: Number(settingsForm.probe_interval_seconds),
      slow_threshold_ms: Number(settingsForm.slow_threshold_ms),
      severe_slow_threshold_ms: Number(settingsForm.severe_slow_threshold_ms),
      consecutive_slow_breaker_threshold: Number(settingsForm.consecutive_slow_breaker_threshold),
      consecutive_error_breaker_threshold: Number(settingsForm.consecutive_error_breaker_threshold),
      cooldown_seconds: Number(settingsForm.cooldown_seconds),
      half_open_success_threshold: Number(settingsForm.half_open_success_threshold),
      cost_weight: Number(settingsForm.cost_weight_percent) / 100,
      recovery_step: Number(settingsForm.recovery_step),
    }
    settings.value = await adminAPI.openaiAutoScheduler.updateSettings(payload)
    syncSettingsForm(settings.value)
    editingSettings.value = false
    appStore.showSuccess('调度配置已更新')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, '更新调度配置失败'))
  } finally {
    settingsSaving.value = false
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

async function openScoreDrawer(score: OpenAIAutoSchedulerScore) {
  selectedScore.value = score
  scoreEvents.value = []
  drawerAbortController?.abort()
  const ctrl = new AbortController()
  drawerAbortController = ctrl
  drawerLoading.value = true
  try {
    const result = await adminAPI.openaiAutoScheduler.listEvents(
      {
        account_id: score.account_id,
        group_id: score.group_id,
        model: score.model,
        page: 1,
        page_size: 20,
      } as OpenAIAutoSchedulerListParams & { account_id: number },
      { signal: ctrl.signal }
    )
    if (ctrl.signal.aborted || drawerAbortController !== ctrl) return
    scoreEvents.value = result.items || []
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name !== 'AbortError' && e?.code !== 'ERR_CANCELED') {
      appStore.showError(extractApiErrorMessage(err, '加载调度事件失败'))
    }
  } finally {
    if (drawerAbortController === ctrl) {
      drawerLoading.value = false
      drawerAbortController = null
    }
  }
}

function closeScoreDrawer() {
  drawerAbortController?.abort()
  drawerAbortController = null
  selectedScore.value = null
  scoreEvents.value = []
  drawerLoading.value = false
}

function scoreKey(score: OpenAIAutoSchedulerScore): string {
  return `${score.account_id}:${score.group_id}:${score.model}`
}

function scoreTitle(score: OpenAIAutoSchedulerScore): string {
  return score.account_name?.trim() || `Account #${score.account_id}`
}

function dispatchScoreHint(score: OpenAIAutoSchedulerScore): string {
  return `当前分数 ${formatScore(score.final_score)}（已含成本修正 ${formatSignedScore(score.cost_score)}）；同状态选择时再叠加组内价格修正`
}

function formatSignedScore(score: number): string {
  const formatted = formatComponentScore(score)
  return score > 0 ? `+${formatted}` : formatted
}

function errorSummary(error?: string | null): string {
  if (!error) return '无异常'
  const text = error.trim()
  if (!text) return '无异常'
  if (text.includes('context deadline exceeded')) return '超时：context deadline exceeded'
  if (/status\s*429|rate limit/i.test(text)) return '限流：请求被上游限制'
  if (/status\s*401|unauthorized/i.test(text)) return '认证失败：上游拒绝授权'
  if (/status\s*403|forbidden/i.test(text)) return '权限失败：上游拒绝访问'
  return text.length > 48 ? `${text.slice(0, 48)}...` : text
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

function formatComponentScore(score: number): string {
  const bounded = Math.max(-10000, Math.min(10000, score))
  return (bounded / 10000).toFixed(4)
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
  drawerAbortController?.abort()
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
