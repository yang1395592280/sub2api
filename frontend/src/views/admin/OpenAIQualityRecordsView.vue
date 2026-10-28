<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-5">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('openAIQuality.recordsTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('openAIQuality.recordsDescription') }}</p></div>
        <div class="flex gap-2"><router-link class="btn btn-secondary" to="/admin/openai-quality">{{ t('nav.openAIQuality') }}</router-link><button class="btn btn-secondary" :disabled="busy" @click="applyFilters"><Icon name="refresh" size="sm" />{{ t('common.refresh') }}</button></div>
      </header>

      <form class="card space-y-4 p-5" @submit.prevent="applyFilters">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.timeRange') }}</span><Select v-model="range" :options="rangeOptions" :aria-label="t('openAIQuality.timeRange')" /></label>
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.sourceGroup') }}</span><Select v-model="sourceGroupId" :options="groupOptions" searchable :aria-label="t('openAIQuality.sourceGroup')" /></label>
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.account') }}</span><input v-model.trim="search" class="input" :placeholder="t('openAIQuality.searchAccountName')" /></label>
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.result') }}</span><Select v-model="outcome" :options="outcomeOptions" :aria-label="t('openAIQuality.result')" /></label>
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.model') }}</span><input v-model.trim="modelId" class="input" :placeholder="t('openAIQuality.exactModel')" /></label>
          <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.trigger') }}</span><Select v-model="trigger" :options="triggerOptions" :aria-label="t('openAIQuality.trigger')" /></label>
          <template v-if="range === 'custom'">
            <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.fromTime') }}</span><input v-model="fromTime" type="datetime-local" class="input" required /></label>
            <label class="space-y-1.5"><span class="input-label">{{ t('openAIQuality.toTime') }}</span><input v-model="toTime" type="datetime-local" class="input" required /></label>
          </template>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <span class="text-xs text-gray-500">{{ t('openAIQuality.historyRetentionHelp') }}</span>
          <div class="flex gap-2"><button type="button" class="btn btn-secondary" @click="resetFilters">{{ t('openAIQuality.resetFilters') }}</button><button class="btn btn-primary" :disabled="busy"><Icon name="search" size="sm" />{{ t('openAIQuality.queryRecords') }}</button></div>
        </div>
      </form>

      <div v-if="policyId || accountId" class="flex flex-wrap items-center justify-between gap-2 rounded-xl bg-primary-50 px-4 py-3 text-sm text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"><span>{{ t(policyId ? 'openAIQuality.policyHistoryScope' : 'openAIQuality.accountHistoryScope') }}</span><button class="font-medium underline" @click="clearContext">{{ t('openAIQuality.showAllRecords') }}</button></div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>

      <section class="card overflow-hidden" :aria-busy="busy">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700"><h2 class="font-semibold">{{ t('openAIQuality.executionHistory') }} <span class="ml-2 text-sm font-normal text-gray-400">{{ t('openAIQuality.recordCount', { count: total }) }}</span></h2><span class="text-xs text-gray-400">{{ t('openAIQuality.newestFirst') }}</span></div>
        <div v-if="busy && !runs.length" class="p-12 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else-if="!runs.length" class="flex flex-col items-center gap-3 p-12 text-center"><Icon name="clock" size="xl" class="text-gray-300 dark:text-dark-500" /><p class="font-medium text-gray-700 dark:text-dark-200">{{ t('openAIQuality.noRecords') }}</p><p class="text-sm text-gray-500">{{ t('openAIQuality.noRecordsHelp') }}</p></div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1000px] text-left text-sm">
            <thead class="bg-gray-50/80 text-xs text-gray-500 dark:bg-dark-900/50"><tr><th class="whitespace-nowrap px-5 py-3">{{ t('openAIQuality.time') }}</th><th class="px-4 py-3">{{ t('openAIQuality.account') }} / {{ t('openAIQuality.sourceGroup') }}</th><th class="px-4 py-3">{{ t('openAIQuality.model') }}</th><th class="px-4 py-3">{{ t('openAIQuality.result') }}</th><th class="px-4 py-3">{{ t('openAIQuality.latency') }}</th><th class="px-4 py-3">{{ t('openAIQuality.trigger') }}</th><th class="px-5 py-3 text-right">{{ t('openAIQuality.actions') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="run in runs" :key="run.id" class="transition-colors hover:bg-gray-50/60 dark:hover:bg-dark-700/30">
                <td class="whitespace-nowrap px-5 py-4"><time class="tabular-nums">{{ formatDate(run.started_at || run.queued_at) }}</time><p class="mt-1 text-xs text-gray-400">#{{ run.id }}</p></td>
                <td class="min-w-52 max-w-64 px-4 py-4"><p class="truncate font-medium" :title="run.input_snapshot?.account_name">{{ run.input_snapshot?.account_name || `#${run.account_id}` }}</p><p class="mt-1 truncate text-xs text-gray-500">{{ groupName(run) }}</p></td>
                <td class="px-4 py-4"><p class="whitespace-nowrap font-medium">{{ run.input_snapshot?.condition.model_id || '—' }}</p><p v-if="run.input_snapshot?.effective_upstream_model !== run.input_snapshot?.condition.model_id" class="mt-1 text-xs text-gray-500">→ {{ run.input_snapshot?.effective_upstream_model }}</p></td>
                <td class="px-4 py-4"><span class="whitespace-nowrap rounded-lg px-2.5 py-1 text-xs font-medium" :class="qualityOutcomeClass(qualityRunOutcome(run))">{{ label(qualityRunOutcome(run)) }}</span><p v-if="run.verdict === 'incorrect'" class="mt-2 text-xs text-gray-500">{{ run.result.final_answer ?? '—' }} / {{ run.result.expected_answer }}</p></td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums text-gray-500">{{ run.finished_at ? `${(run.result.latency_ms / 1000).toFixed(2)} s` : '—' }}</td>
                <td class="whitespace-nowrap px-4 py-4 text-xs text-gray-500">{{ label(run.trigger_source) }}</td>
                <td class="px-5 py-4"><div class="flex justify-end gap-2"><button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="detailBusy" @click="openRun(run.id)">{{ t('openAIQuality.viewDetails') }}</button><button v-if="['queued', 'running'].includes(run.status)" class="btn btn-ghost px-2 py-1.5 text-xs text-red-600" :disabled="cancelling === run.id" @click="cancelRun(run)">{{ t('openAIQuality.cancelRun') }}</button></div></td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :total="total" :page="page" :page-size="pageSize" @update:page="changePage" @update:page-size="changePageSize" />
      </section>
      <QualityRunDetailDialog :detail="detail" :group-name="detail ? groupName(detail.run) : ''" @close="detail = null" />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityRunDetailDialog from '@/components/admin/account-quality/QualityRunDetailDialog.vue'
import { qualityOutcomeClass, qualityRunOutcome } from '@/components/admin/account-quality/qualityPresentation'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { cancelQualityRun, getQualityRun, getQualityRuns, type QualityRun, type QualityRunDetail } from '@/api/admin/openaiAccountQuality'
import type { AdminGroup } from '@/types'

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const groups = ref<AdminGroup[]>([])
const runs = ref<QualityRun[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const range = ref('7')
const sourceGroupId = ref('')
const search = ref('')
const modelId = ref('')
const outcome = ref('')
const trigger = ref('')
const policyId = ref('')
const accountId = ref('')
const localDate = (date: Date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
const fromTime = ref(localDate(new Date(Date.now() - 7 * 86400000)))
const toTime = ref(localDate(new Date()))
const busy = ref(false)
const error = ref('')
const detail = ref<QualityRunDetail | null>(null)
const detailBusy = ref(false)
const cancelling = ref<number | null>(null)
let requestId = 0
let detailRequest = 0
let active = true
let activatedOnce = false
let appliedFilters: Record<string, unknown> = {}

function label(value: string) { return te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
const rangeOptions = computed(() => ['1', '7', '30', 'custom'].map(value => ({ value, label: t(`openAIQuality.ranges.${value}`) })))
const groupOptions = computed(() => [{ value: '', label: t('openAIQuality.allSourceGroups') }, ...groups.value.filter(g => g.platform === 'openai' && g.group_role !== 'self_hosted_pool').map(g => ({ value: String(g.id), label: g.name }))])
const outcomeOptions = computed(() => [{ value: '', label: t('openAIQuality.allResults') }, ...['passed', 'incorrect', 'unknown', 'error', 'queued', 'running', 'skipped', 'cancelled', 'abandoned'].map(value => ({ value, label: label(value) }))])
const triggerOptions = computed(() => [{ value: '', label: t('openAIQuality.allTriggers') }, ...['manual', 'scheduled', 'recovery'].map(value => ({ value, label: label(value) }))])
function groupName(run: QualityRun) { return run.input_snapshot?.source_group_name || groups.value.find(g => g.id === run.input_snapshot?.source_group_id)?.name || t('openAIQuality.unavailableGroup') }
function formatDate(value: string) { return new Date(value).toLocaleString() }
function errorMessage(value: unknown) { return extractApiErrorMessage(value, t('common.error')) }

async function loadRuns() {
  if (!active) return
  const request = ++requestId
  busy.value = true
  error.value = ''
  try {
    const result = await getQualityRuns({ ...appliedFilters, page: page.value, page_size: pageSize.value })
    if (request !== requestId) return
    if (page.value > result.pages && result.pages > 0) { page.value = result.pages; await loadRuns(); return }
    runs.value = result.items
    total.value = result.total
  } catch (e) { if (request === requestId) error.value = errorMessage(e) }
  finally { if (request === requestId) busy.value = false }
}
function applyFilters() {
  const to = range.value === 'custom' ? new Date(toTime.value) : new Date()
  const from = range.value === 'custom' ? new Date(fromTime.value) : new Date(to.getTime() - Number(range.value) * 86400000)
  if (!Number.isFinite(from.getTime()) || !Number.isFinite(to.getTime()) || from >= to || to.getTime() - from.getTime() > 180 * 86400000) { error.value = t('openAIQuality.invalidDateRange'); return }
  appliedFilters = { from: from.toISOString(), to: to.toISOString(), ...(sourceGroupId.value ? { source_group_id: Number(sourceGroupId.value) } : {}), ...(policyId.value ? { policy_id: Number(policyId.value) } : {}), ...(accountId.value ? { account_id: Number(accountId.value) } : {}), ...(search.value ? { search: search.value } : {}), ...(modelId.value ? { model_id: modelId.value } : {}), ...(trigger.value ? { trigger_source: trigger.value } : {}), ...(outcome.value ? { [['passed', 'incorrect', 'unknown', 'error'].includes(outcome.value) ? 'verdict' : 'status']: outcome.value } : {}) }
  page.value = 1
  void loadRuns()
}
function changePage(value: number) { page.value = value; void loadRuns() }
function changePageSize(value: number) { pageSize.value = value; page.value = 1; void loadRuns() }
function resetFilters() { range.value = '7'; sourceGroupId.value = ''; search.value = ''; modelId.value = ''; outcome.value = ''; trigger.value = ''; policyId.value = ''; accountId.value = ''; if (Object.keys(route.query).length) void router.replace({ path: route.path }); else applyFilters() }
function clearContext() { const { policy_id: _policy, account_id: _account, ...query } = route.query; policyId.value = ''; accountId.value = ''; void router.replace({ path: route.path, query }) }
function readContext() { policyId.value = String(route.query.policy_id || ''); accountId.value = String(route.query.account_id || ''); sourceGroupId.value = String(route.query.source_group_id || ''); modelId.value = String(route.query.model_id || ''); applyFilters() }
async function openRun(id: number) {
  const request = ++detailRequest
  detailBusy.value = true
  try { const result = await getQualityRun(id); if (request === detailRequest) detail.value = result }
  catch (e) { if (request === detailRequest) error.value = errorMessage(e) }
  finally { if (request === detailRequest) detailBusy.value = false }
}
async function cancelRun(run: QualityRun) {
  const reason = window.prompt(t('openAIQuality.cancelRunReason'))
  if (!reason?.trim()) return
  cancelling.value = run.id
  try { await cancelQualityRun(run.id, reason.trim()); await loadRuns() }
  catch (e) { error.value = errorMessage(e) }
  finally { cancelling.value = null }
}
onMounted(() => {
  readContext()
  void getAllIncludingInactive().then(result => { groups.value = result }).catch(e => { error.value = errorMessage(e) })
})
watch(() => route.query, () => { if (active && route.path === '/admin/openai-quality/records') readContext() })
onActivated(() => { active = true; if (activatedOnce) readContext(); activatedOnce = true })
onDeactivated(() => { active = false; detail.value = null; ++requestId; ++detailRequest; busy.value = false; detailBusy.value = false })
onUnmounted(() => { ++requestId; ++detailRequest })
</script>
