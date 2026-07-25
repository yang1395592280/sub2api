<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkActions.testConnection')"
    width="extra-wide"
    @close="handleClose"
  >
    <div class="grid gap-4 lg:grid-cols-[20rem_minmax(0,1fr)]">
      <div class="space-y-4">
        <div class="rounded-xl border border-gray-200 bg-gradient-to-r from-gray-50 to-gray-100 p-3 dark:border-dark-500 dark:from-dark-700 dark:to-dark-600">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="font-semibold text-gray-900 dark:text-gray-100">
                {{ t('admin.accounts.bulkActions.selected', { count: accounts.length }) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.bulkTestHint') }}
              </div>
            </div>
            <span class="rounded-full bg-primary-100 px-2.5 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
              {{ accounts.length }}
            </span>
          </div>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.selectTestModel') }}
          </label>
          <Select
            v-model="selectedModelId"
            :options="availableModels"
            :disabled="loadingModels || status === 'connecting'"
            value-key="id"
            label-key="display_name"
            :placeholder="loadingModels ? t('common.loading') + '...' : t('admin.accounts.selectTestModel')"
          />
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.bulkTestConcurrency') }}
          </label>
          <select
            v-model.number="selectedConcurrency"
            data-testid="batch-test-concurrency"
            :disabled="status === 'connecting'"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 disabled:cursor-not-allowed disabled:bg-gray-100 dark:border-gray-600 dark:bg-dark-700 dark:text-white dark:disabled:bg-dark-600"
          >
            <option v-for="option in concurrencyOptions" :key="option" :value="option">
              {{ option }}
            </option>
          </select>
        </div>

        <div
          v-if="status !== 'idle'"
          class="space-y-2 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-700 dark:border-blue-900/40 dark:bg-blue-900/20 dark:text-blue-300"
        >
          <div>
            {{ t('admin.accounts.bulkTestProgress', { current: progressCurrent, total: accounts.length }) }}
          </div>
          <div v-if="progressCurrent > 0" class="font-medium">
            {{ t('admin.accounts.bulkTestSummary', { success: successCount, failed: failedCount }) }}
          </div>
          <div v-if="failureBreakdown.length > 0" class="space-y-1">
            <div class="font-medium">
              {{ t('admin.accounts.bulkTestFailureBreakdownTitle') }}
            </div>
            <div
              v-for="item in failureBreakdown"
              :key="item.category"
              class="text-xs"
            >
              {{ t('admin.accounts.bulkTestFailureCategory', { category: item.category, count: item.count }) }}
            </div>
          </div>
        </div>

        <div
          v-if="status !== 'idle' && openAIAccounts.length > 0"
          data-testid="openai-batch-stats"
          class="space-y-3 rounded-lg border border-emerald-100 bg-emerald-50 px-3 py-3 text-sm text-emerald-800 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-200"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="font-semibold">
              {{ t('admin.accounts.bulkOpenAIStatsTitle') }}
            </div>
            <div class="text-xs tabular-nums text-emerald-600 dark:text-emerald-300">
              {{
                t('admin.accounts.bulkOpenAIStatsProgress', {
                  current: openAIStats.completed,
                  total: openAIAccounts.length
                })
              }}
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
              {{ t('admin.accounts.bulkOpenAIStatsPlan') }}
            </div>
            <div class="grid grid-cols-2 gap-1.5 text-xs">
              <button
                type="button"
                data-testid="openai-stat-plan-free"
                :aria-pressed="activeOpenAIFilter === 'plan-free'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'plan-free' }"
                @click="toggleOpenAIFilter('plan-free')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsFree') }}：{{ openAIStats.free }}
              </button>
              <button
                type="button"
                data-testid="openai-stat-plan-plus"
                :aria-pressed="activeOpenAIFilter === 'plan-plus'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'plan-plus' }"
                @click="toggleOpenAIFilter('plan-plus')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsPlus') }}：{{ openAIStats.plus }}
              </button>
              <button
                type="button"
                data-testid="openai-stat-plan-other"
                :aria-pressed="activeOpenAIFilter === 'plan-other'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'plan-other' }"
                @click="toggleOpenAIFilter('plan-other')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsOther') }}：{{ openAIStats.other }}
              </button>
              <button
                type="button"
                data-testid="openai-stat-plan-unknown"
                :aria-pressed="activeOpenAIFilter === 'plan-unknown'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'plan-unknown' }"
                @click="toggleOpenAIFilter('plan-unknown')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsUnknown') }}：{{ openAIStats.planUnknown }}
              </button>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
              {{ t('admin.accounts.bulkOpenAIStatsSevenDayUsage') }}
            </div>
            <div class="grid grid-cols-2 gap-1.5 text-xs">
              <button
                type="button"
                data-testid="openai-stat-usage-low"
                :aria-pressed="activeOpenAIFilter === 'usage-low'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'usage-low' }"
                @click="toggleOpenAIFilter('usage-low')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsUsageLow') }}：{{ openAIStats.usageLow }}
              </button>
              <button
                type="button"
                data-testid="openai-stat-usage-high"
                :aria-pressed="activeOpenAIFilter === 'usage-high'"
                class="rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'usage-high' }"
                @click="toggleOpenAIFilter('usage-high')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsUsageHigh') }}：{{ openAIStats.usageHigh }}
              </button>
              <button
                type="button"
                data-testid="openai-stat-usage-unknown"
                :aria-pressed="activeOpenAIFilter === 'usage-unknown'"
                class="col-span-2 rounded bg-white/70 px-2 py-1 text-left transition-colors hover:bg-white dark:bg-dark-800/60 dark:hover:bg-dark-700"
                :class="{ 'ring-2 ring-emerald-500 dark:ring-emerald-400': activeOpenAIFilter === 'usage-unknown' }"
                @click="toggleOpenAIFilter('usage-unknown')"
              >
                {{ t('admin.accounts.bulkOpenAIStatsUsageUnknown') }}：{{ openAIStats.usageUnknown }}
              </button>
            </div>
          </div>

          <div v-if="activeOpenAIFilter" class="flex items-center justify-between gap-2 border-t border-emerald-200 pt-2 text-xs dark:border-emerald-800">
            <span>
              {{
                t('admin.accounts.bulkOpenAIStatsFilterActive', {
                  label: activeOpenAIFilterLabel,
                  count: filteredTestEntries.length
                })
              }}
            </span>
            <button
              type="button"
              data-testid="openai-stat-clear-filter"
              class="font-medium underline underline-offset-2 hover:no-underline"
              @click="clearOpenAIFilter"
            >
              {{ t('admin.accounts.bulkOpenAIStatsClearFilter') }}
            </button>
          </div>
        </div>
      </div>

      <div class="group relative">
        <div
          ref="terminalRef"
          class="max-h-[520px] min-h-[320px] overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
        >
          <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
            <Icon name="play" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.readyToTest') }}</span>
          </div>
          <div v-else-if="status === 'connecting'" class="flex items-center gap-2 text-yellow-400">
            <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <span>{{ t('admin.accounts.connectingToApi') }}</span>
          </div>

          <div
            v-for="entry in filteredTestEntries"
            :key="entry.accountId"
            class="mb-3 rounded-xl border px-3 py-2"
            :class="entry.cardClass"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0 space-y-1">
                <div class="break-all text-cyan-400">
                  === {{ entry.accountName }} (#{{ entry.accountId }}) ===
                </div>
                <div class="break-all text-gray-500">
                  {{ t('admin.accounts.testLinkLabel') }}：{{ entry.endpoint }}
                </div>
              </div>
              <span class="rounded-full px-2.5 py-1 text-xs font-semibold" :class="entry.statusClass">
                {{ entry.statusLabel }}
              </span>
            </div>

            <div class="mt-3 space-y-1">
              <div v-for="(line, index) in entry.lines" :key="index" :class="line.class">
                {{ line.text }}
              </div>
            </div>
          </div>
          <div
            v-if="status !== 'idle' && activeOpenAIFilter && filteredTestEntries.length === 0"
            class="flex min-h-24 items-center justify-center rounded-xl border border-dashed border-gray-700 px-3 text-center text-sm text-gray-500"
          >
            {{ t('admin.accounts.bulkOpenAIStatsNoMatches') }}
          </div>

          <div
            v-if="status === 'success'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
          >
            <Icon name="check" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.testCompleted') }}</span>
          </div>
          <div
            v-else-if="status === 'error'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-red-400"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
            <span>{{ errorMessage }}</span>
          </div>
        </div>

        <button
          v-if="testEntries.length > 0"
          @click="copyOutput"
          class="absolute right-2 top-2 rounded-lg bg-gray-800/80 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white group-hover:opacity-100"
          :title="t('admin.accounts.copyOutput')"
        >
          <Icon name="link" size="sm" :stroke-width="2" />
        </button>
      </div>
    </div>

    <template #footer>
      <div class="flex flex-wrap justify-between gap-3">
        <div class="flex flex-wrap gap-2">
          <button
            v-if="successEmails.length > 0"
            @click="downloadEmails(successEmails, 'success-emails.txt')"
            data-testid="download-success-emails"
            class="rounded-lg bg-emerald-100 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-200 dark:bg-emerald-900/30 dark:text-emerald-300 dark:hover:bg-emerald-900/50"
          >
            {{ t('admin.accounts.bulkDownloadSuccessEmails') }}
          </button>
          <button
            v-if="failedEmails.length > 0"
            @click="downloadEmails(failedEmails, 'failed-emails.txt')"
            data-testid="download-failed-emails"
            class="rounded-lg bg-rose-100 px-4 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-200 dark:bg-rose-900/30 dark:text-rose-300 dark:hover:bg-rose-900/50"
          >
            {{ t('admin.accounts.bulkDownloadFailedEmails') }}
          </button>
        </div>
        <div class="flex justify-end gap-3">
          <button
            @click="handleClose"
            class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          >
            {{ t('common.close') }}
          </button>
          <button
            @click="startBatchTest"
            :disabled="status === 'connecting' || !selectedModelId || accounts.length === 0"
            :class="[
              'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all',
              status === 'connecting' || !selectedModelId || accounts.length === 0
                ? 'cursor-not-allowed bg-primary-400 text-white'
                : 'bg-primary-500 text-white hover:bg-primary-600'
            ]"
          >
            <Icon
              v-if="status === 'connecting'"
              name="refresh"
              size="sm"
              class="animate-spin"
              :stroke-width="2"
            />
            <Icon v-else name="play" size="sm" :stroke-width="2" />
            <span>
              {{
                status === 'connecting'
                  ? t('admin.accounts.testing')
                  : t('admin.accounts.startTest')
              }}
            </span>
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { OpenAIQuotaUsage, OpenAIRateLimitWindow } from '@/api/admin/accounts'
import type { Account, ClaudeModel } from '@/types'

interface OutputLine {
  text: string
  class: string
}

type TestStatus = 'running' | 'success' | 'error'

interface TestEntry {
  accountId: number
  accountName: string
  endpoint: string
  lines: OutputLine[]
  cardClass: string
  statusClass: string
  statusLabel: string
}

type OpenAIFilter =
  | 'plan-free'
  | 'plan-plus'
  | 'plan-other'
  | 'plan-unknown'
  | 'usage-low'
  | 'usage-high'
  | 'usage-unknown'

interface OpenAIBatchStats {
  completed: number
  free: number
  plus: number
  other: number
  planUnknown: number
  usageLow: number
  usageHigh: number
  usageUnknown: number
}

const createEmptyOpenAIStats = (): OpenAIBatchStats => ({
  completed: 0,
  free: 0,
  plus: 0,
  other: 0,
  planUnknown: 0,
  usageLow: 0,
  usageHigh: 0,
  usageUnknown: 0
})

const SEVEN_DAYS_SECONDS = 7 * 24 * 60 * 60
const SIX_DAYS_SECONDS = 6 * 24 * 60 * 60

const props = defineProps<{
  show: boolean
  accounts: Account[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const testEntries = ref<TestEntry[]>([])
const errorMessage = ref('')
const availableModels = ref<ClaudeModel[]>([])
const selectedModelId = ref('')
const loadingModels = ref(false)
const progressCurrent = ref(0)
const successCount = ref(0)
const failedCount = ref(0)
const failureCategories = ref<Record<string, number>>({})
const successEmails = ref<string[]>([])
const failedEmails = ref<string[]>([])
const selectedConcurrency = ref(20)
const concurrencyOptions = [5, 10, 20, 50]
const openAIStats = ref<OpenAIBatchStats>(createEmptyOpenAIStats())
const activeOpenAIFilter = ref<OpenAIFilter | null>(null)
const openAIClassifications = ref<Record<number, { plan: OpenAIFilter; usage: OpenAIFilter }>>({})

const openAIAccounts = computed(() => props.accounts.filter(
  (account) => account.platform === 'openai' && account.type === 'oauth'
))

const formatConnectDuration = (durationMs: number) => {
  const seconds = Math.max(0, durationMs) / 1000
  return seconds.toFixed(2)
}

const failureBreakdown = computed(() => Object.entries(failureCategories.value)
  .map(([category, count]) => ({ category, count }))
  .sort((left, right) => {
    if (right.count !== left.count) return right.count - left.count
    return left.category.localeCompare(right.category)
  }))

const filteredTestEntries = computed(() => {
  if (!activeOpenAIFilter.value) return testEntries.value
  return testEntries.value.filter((entry) => {
    const classification = openAIClassifications.value[entry.accountId]
    if (!classification) return false
    return classification.plan === activeOpenAIFilter.value || classification.usage === activeOpenAIFilter.value
  })
})

const activeOpenAIFilterLabel = computed(() => {
  switch (activeOpenAIFilter.value) {
    case 'plan-free': return t('admin.accounts.bulkOpenAIStatsFree')
    case 'plan-plus': return t('admin.accounts.bulkOpenAIStatsPlus')
    case 'plan-other': return t('admin.accounts.bulkOpenAIStatsOther')
    case 'plan-unknown': return t('admin.accounts.bulkOpenAIStatsUnknown')
    case 'usage-low': return t('admin.accounts.bulkOpenAIStatsUsageLow')
    case 'usage-high': return t('admin.accounts.bulkOpenAIStatsUsageHigh')
    case 'usage-unknown': return t('admin.accounts.bulkOpenAIStatsUsageUnknown')
    default: return ''
  }
})

const getTestEndpoint = (account: Account) => `/api/v1/admin/accounts/${account.id}/test`

const getEntryPresentation = (status: TestStatus) => {
  if (status === 'success') {
    return {
      cardClass: 'border-emerald-500/30 bg-emerald-500/8',
      statusClass: 'bg-emerald-500/15 text-emerald-300',
      statusLabel: t('admin.accounts.testResultSuccess')
    }
  }
  if (status === 'error') {
    return {
      cardClass: 'border-rose-500/30 bg-rose-500/8',
      statusClass: 'bg-rose-500/15 text-rose-300',
      statusLabel: t('admin.accounts.testResultFailed')
    }
  }
  return {
    cardClass: 'border-blue-500/30 bg-blue-500/8',
    statusClass: 'bg-blue-500/15 text-blue-300',
    statusLabel: t('admin.accounts.testResultRunning')
  }
}

const createEntry = (account: Account): TestEntry => {
  const entry: TestEntry = {
    accountId: account.id,
    accountName: account.name,
    endpoint: getTestEndpoint(account),
    lines: [],
    ...getEntryPresentation('running')
  }
  testEntries.value.push(entry)
  return entry
}

const setEntryStatus = (entry: TestEntry, nextStatus: TestStatus) => {
  Object.assign(entry, getEntryPresentation(nextStatus))
}

const addEntryLine = (entry: TestEntry, text: string, className: string = 'text-gray-300') => {
  entry.lines.push({ text, class: className })
  scrollToBottom()
}

const scrollToBottom = async () => {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

const resetState = () => {
  status.value = 'idle'
  testEntries.value = []
  errorMessage.value = ''
  progressCurrent.value = 0
  successCount.value = 0
  failedCount.value = 0
  failureCategories.value = {}
  successEmails.value = []
  failedEmails.value = []
  openAIStats.value = createEmptyOpenAIStats()
  activeOpenAIFilter.value = null
  openAIClassifications.value = {}
}

const toggleOpenAIFilter = (filter: OpenAIFilter) => {
  activeOpenAIFilter.value = activeOpenAIFilter.value === filter ? null : filter
  scrollTerminalToTop()
}

const clearOpenAIFilter = () => {
  activeOpenAIFilter.value = null
  scrollTerminalToTop()
}

const scrollTerminalToTop = async () => {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = 0
  }
}

const handleClose = () => {
  emit('close')
}

const extractEmail = (account: Account): string => {
  const value = account.extra && typeof account.extra.email_address === 'string'
    ? account.extra.email_address.trim()
    : ''
  if (value) return value
  return typeof account.name === 'string' && account.name.includes('@')
    ? account.name.trim()
    : ''
}

const getFailureCategory = (statusCode: number, message: string) => {
  if (statusCode >= 400) {
    return `HTTP ${statusCode}`
  }

  const match = message.match(/\b(?:HTTP\s*)?([45]\d{2})\b/i)
  if (match) {
    return `HTTP ${match[1]}`
  }

  const trimmed = message.trim()
  return trimmed || t('common.unknown')
}

const recordFailureCategory = (category: string) => {
  failureCategories.value = {
    ...failureCategories.value,
    [category]: (failureCategories.value[category] || 0) + 1
  }
}

const parseSSEOutput = (body: string) => {
  const lines = body.split('\n')
  let responseText = ''
  let error = ''
  let model = ''
  let connectDurationMs: number | null = null

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed.startsWith('data: ')) continue
    const payload = trimmed.slice(6).trim()
    if (!payload) continue
    try {
      const event = JSON.parse(payload) as {
        type: string
        text?: string
        error?: string
        model?: string
        connect_duration_ms?: number
      }
      if (event.type === 'test_start' && event.model) {
        model = event.model
        if (typeof event.connect_duration_ms === 'number') {
          connectDurationMs = event.connect_duration_ms
        }
      } else if (event.type === 'connect_timing' && typeof event.connect_duration_ms === 'number') {
        connectDurationMs = event.connect_duration_ms
      } else if (event.type === 'content' && event.text) {
        responseText += event.text
      } else if (event.type === 'error' && event.error) {
        error = event.error
      }
    } catch {
      // ignore malformed line
    }
  }

  return { responseText, error, model, connectDurationMs }
}

const loadAvailableModels = async () => {
  if (props.accounts.length === 0) return

  loadingModels.value = true
  selectedModelId.value = ''
  try {
    const models = await adminAPI.accounts.getAvailableModels(props.accounts[0].id)
    availableModels.value = models
    if (models.length > 0) {
      selectedModelId.value = models[0].id
    }
  } catch (error) {
    console.error('Failed to load available models for batch test:', error)
    availableModels.value = []
  } finally {
    loadingModels.value = false
  }
}

const testSingleAccount = async (account: Account) => {
  const entry = createEntry(account)
  addEntryLine(entry, t('admin.accounts.testAccountTypeLabel', { type: account.type }), 'text-gray-400')

  try {
    const response = await fetch(entry.endpoint, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        model_id: selectedModelId.value,
        prompt: '',
        mode: 'default'
      })
    })

    const body = await response.text()
    const result = parseSSEOutput(body)

    if (!response.ok || result.error) {
      const email = extractEmail(account)
      if (email) failedEmails.value.push(email)
      const failureMessage = result.error || `HTTP ${response.status}`
      recordFailureCategory(getFailureCategory(response.status, failureMessage))
      if (typeof result.connectDurationMs === 'number') {
        addEntryLine(entry, `连接耗时 ${formatConnectDuration(result.connectDurationMs)}s`, 'rounded-md bg-amber-300/20 px-2 py-1 font-semibold text-amber-300 ring-1 ring-amber-300/40')
      }
      addEntryLine(entry, `ERROR: ${failureMessage}`, 'text-red-400')
      setEntryStatus(entry, 'error')
      return false
    }

    const email = extractEmail(account)
    if (email) successEmails.value.push(email)
    if (result.model) {
      addEntryLine(entry, t('admin.accounts.usingModel', { model: result.model }), 'text-green-400')
    }
    if (typeof result.connectDurationMs === 'number') {
      addEntryLine(entry, `连接耗时 ${formatConnectDuration(result.connectDurationMs)}s`, 'rounded-md bg-amber-300/20 px-2 py-1 font-semibold text-amber-300 ring-1 ring-amber-300/40')
    }
    addEntryLine(entry, result.responseText || t('admin.accounts.testCompleted'), 'text-green-300')
    setEntryStatus(entry, 'success')
    return true
  } catch (error: unknown) {
    const email = extractEmail(account)
    if (email) failedEmails.value.push(email)
    const message = error instanceof Error ? error.message : 'Unknown error'
    recordFailureCategory(getFailureCategory(0, message))
    addEntryLine(entry, `ERROR: ${message}`, 'text-red-400')
    setEntryStatus(entry, 'error')
    return false
  } finally {
    progressCurrent.value += 1
  }
}

const runAccountsWithConcurrency = async (
  accounts: Account[],
  task: (account: Account) => Promise<void>
) => {
  if (accounts.length === 0) return

  const concurrency = Math.max(1, Math.min(selectedConcurrency.value, accounts.length))
  let nextIndex = 0

  const worker = async () => {
    while (nextIndex < accounts.length) {
      const account = accounts[nextIndex]
      nextIndex += 1
      await task(account)
    }
  }

  await Promise.all(Array.from({ length: concurrency }, () => worker()))
}

const runWithConcurrency = async () => {
  await runAccountsWithConcurrency(props.accounts, async (account) => {
    const success = await testSingleAccount(account)
    if (success) {
      successCount.value += 1
    } else {
      failedCount.value += 1
    }
  })
}

const getSevenDayUsedPercent = (quota: OpenAIQuotaUsage): number | null => {
  const windows = [
    quota.rate_limit?.primary_window,
    quota.rate_limit?.secondary_window
  ].filter((window): window is OpenAIRateLimitWindow => Boolean(window))

  const sevenDayWindow = windows
    .filter((window) => window.limit_window_seconds >= SIX_DAYS_SECONDS)
    .sort((left, right) => (
      Math.abs(left.limit_window_seconds - SEVEN_DAYS_SECONDS) -
      Math.abs(right.limit_window_seconds - SEVEN_DAYS_SECONDS)
    ))[0]

  if (!sevenDayWindow || !Number.isFinite(sevenDayWindow.used_percent)) return null
  return sevenDayWindow.used_percent
}

const recordOpenAIQuota = (accountId: number, quota: OpenAIQuotaUsage) => {
  const planType = quota.plan_type?.trim().toLowerCase() || ''
  let planFilter: OpenAIFilter
  if (planType.includes('free')) {
    openAIStats.value.free += 1
    planFilter = 'plan-free'
  } else if (planType.includes('plus')) {
    openAIStats.value.plus += 1
    planFilter = 'plan-plus'
  } else if (planType) {
    openAIStats.value.other += 1
    planFilter = 'plan-other'
  } else {
    openAIStats.value.planUnknown += 1
    planFilter = 'plan-unknown'
  }

  const usedPercent = getSevenDayUsedPercent(quota)
  let usageFilter: OpenAIFilter
  if (usedPercent === null) {
    openAIStats.value.usageUnknown += 1
    usageFilter = 'usage-unknown'
  } else if (usedPercent <= 5) {
    openAIStats.value.usageLow += 1
    usageFilter = 'usage-low'
  } else {
    openAIStats.value.usageHigh += 1
    usageFilter = 'usage-high'
  }
  openAIClassifications.value[accountId] = { plan: planFilter, usage: usageFilter }
}

const queryOpenAIStats = async () => {
  await runAccountsWithConcurrency(openAIAccounts.value, async (account) => {
    try {
      const quota = await adminAPI.accounts.queryOpenAIQuota(account.id)
      recordOpenAIQuota(account.id, quota)
    } catch {
      openAIStats.value.planUnknown += 1
      openAIStats.value.usageUnknown += 1
      openAIClassifications.value[account.id] = {
        plan: 'plan-unknown',
        usage: 'usage-unknown'
      }
    } finally {
      openAIStats.value.completed += 1
    }
  })
}

const startBatchTest = async () => {
  if (!selectedModelId.value || props.accounts.length === 0) return

  resetState()
  status.value = 'connecting'

  await runWithConcurrency()
  await queryOpenAIStats()

  status.value = failedCount.value > 0 ? 'error' : 'success'
  errorMessage.value = failedCount.value > 0 ? t('admin.accounts.bulkTestHasFailures') : ''
}

const downloadEmails = (emails: string[], filename: string) => {
  if (emails.length === 0) return
  const blob = new Blob([emails.join(',')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

const copyOutput = () => {
  const text = filteredTestEntries.value
    .map((entry) => {
      const body = entry.lines.map((line) => line.text).filter(Boolean)
      return [`=== ${entry.accountName} (#${entry.accountId}) ===`, `${t('admin.accounts.testLinkLabel')}：${entry.endpoint}`, ...body].join('\n')
    })
    .join('\n\n')
  copyToClipboard(text, t('admin.accounts.outputCopied'))
}

watch(
  () => props.show,
  async (show) => {
    if (show) {
      resetState()
      await loadAvailableModels()
    }
  }
)
</script>
