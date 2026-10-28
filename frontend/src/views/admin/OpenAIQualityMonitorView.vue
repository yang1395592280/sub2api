<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-5">
      <header class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('openAIQuality.monitorTitle') }}</h1>
            <p class="mt-1 text-sm text-gray-500">{{ t('openAIQuality.monitorDescription') }}</p>
          </div>
          <div class="flex gap-2">
            <router-link class="btn btn-secondary btn-sm" to="/admin/openai-quality/records"><Icon name="clock" size="sm" />{{ t('nav.openAIQualityRecords') }}</router-link>
            <router-link class="btn btn-secondary btn-sm" to="/admin/openai-quality/settings">{{ t('nav.openAIQualitySettings') }}</router-link>
            <button class="btn btn-secondary btn-sm" :disabled="busy" @click="reload">{{ t('common.refresh') }}</button>
          </div>
        </div>
        <p class="mt-3 rounded-xl bg-primary-50 px-3 py-2 text-xs text-primary-800 dark:bg-primary-900/20 dark:text-primary-200">{{ t('openAIQuality.scopeNote') }}</p>
      </header>

      <nav class="flex gap-1 border-b border-gray-200 dark:border-dark-700" :aria-label="t('openAIQuality.monitorViews')">
        <button v-for="view in monitorViews" :key="view" class="border-b-2 px-4 py-3 text-sm font-medium" :class="monitorView === view ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" :aria-pressed="monitorView === view" @click="monitorView = view">{{ t(`openAIQuality.${view === 'results' ? 'visualResults' : 'policyManagement'}`) }}</button>
      </nav>
      <QualityResultsPanel v-show="monitorView === 'results'" :groups="groups" :account-id="route.query.account_id ? Number(route.query.account_id) : undefined" :revision="resultRevision" @open="openRun" />
      <div v-show="monitorView === 'policies'" class="space-y-5">
      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <article v-for="item in cards" :key="item.label" class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <p class="text-xs text-gray-500">{{ item.label }}</p>
          <p class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{{ item.value }}</p>
        </article>
      </section>
      <section class="flex flex-wrap gap-x-6 gap-y-2 rounded-2xl bg-white px-4 py-3 text-sm shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <span>{{ t('openAIQuality.queueDepth') }}: {{ overview?.health?.queue_depth ?? '—' }}</span>
        <span>{{ t('openAIQuality.runningCount') }}: {{ overview?.health?.running_count ?? '—' }}</span>
        <span>{{ t('openAIQuality.storeStatus') }}: {{ label(overview?.health?.store_status || '') }}</span>
        <span>{{ t('openAIQuality.lastSync') }}: {{ formatDate(overview?.health?.last_sync_at ?? null) }}</span>
      </section>

      <section class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
          <div><h2 class="font-semibold">{{ t('openAIQuality.policies') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.groupedPoliciesHelp') }}</p></div>
          <div class="flex flex-wrap gap-2">
            <input v-model="search" class="input w-48" :aria-label="t('openAIQuality.search')" :placeholder="t('openAIQuality.search')" />
            <Select v-model="sourceGroupFilter" class="w-52" :options="sourceGroupOptions" searchable :aria-label="t('openAIQuality.sourceGroupFilter')" />
            <Select v-model="stateFilter" class="w-36" :options="stateOptions" :aria-label="t('openAIQuality.stateFilter')" />
            <Select v-model="freshnessFilter" class="w-36" :options="freshnessOptions" :aria-label="t('openAIQuality.freshnessFilter')" />
          </div>
        </div>
        <div v-if="selectedPolicyIds.length" class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-4 py-3 text-sm dark:border-dark-700">
          <span>{{ t('openAIQuality.selectedPolicies', { count: selectedPolicyIds.length }) }}</span>
          <span class="text-xs text-gray-500">{{ t('openAIQuality.batchLimit') }}</span>
          <button class="btn btn-primary px-3 py-1.5 text-xs" :disabled="busy || !selectedRunnablePolicies.length" @click="runSelectedNow">{{ t('openAIQuality.runSelected') }}</button>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" @click="selectedPolicyIds = []">{{ t('common.cancel') }}</button>
        </div>
        <div v-if="busy && !overview" class="p-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1080px] text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900">
              <tr>
                <th class="p-4">{{ t('openAIQuality.account') }} / {{ t('openAIQuality.sourceGroup') }}</th>
                <th class="p-3"><label class="flex items-center gap-3"><input type="checkbox" class="checkbox" :checked="allVisibleSelected" :indeterminate="someVisibleSelected && !allVisibleSelected" :aria-label="t('openAIQuality.selectAll')" @change="toggleVisible" />{{ t('openAIQuality.model') }}</label></th>
                <th class="p-3">{{ t('openAIQuality.state') }}</th>
                <th class="p-3">{{ t('openAIQuality.control') }}</th>
                <th class="p-3">{{ t('openAIQuality.lastCheck') }}</th>
                <th class="p-3">{{ t('openAIQuality.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="row in visibleRows" :key="row.key">
                <tr v-for="(p, index) in row.policies" :key="p.id" class="border-t border-gray-100 transition-colors hover:bg-gray-50/60 dark:border-dark-700 dark:hover:bg-dark-700/30" :class="index === 0 ? 'border-t-gray-200 dark:border-t-dark-600' : ''">
                  <td v-if="index === 0" :rowspan="row.policies.length" class="min-w-52 max-w-64 p-4 align-top">
                    <p class="break-all font-semibold text-gray-900 dark:text-dark-100">{{ row.accountName || `#${row.accountId}` }}</p>
                    <span class="mt-2 inline-block rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300">{{ groupName(p) }}</span>
                    <router-link class="mt-3 flex items-center gap-1 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" :to="{ path: '/admin/openai-quality/records', query: { account_id: row.accountId, source_group_id: row.sourceGroupId } }"><Icon name="clock" size="xs" />{{ t('openAIQuality.executionHistory') }}<Icon name="chevronRight" size="xs" /></router-link>
                  </td>
                  <td class="p-3"><label class="flex items-center gap-3"><input v-model="selectedPolicyIds" class="checkbox" type="checkbox" :value="p.id" :disabled="!selectedPolicyIds.includes(p.id) && selectedPolicyIds.length >= 100" :aria-label="t('openAIQuality.selectPolicy', { account: `${p.account_name} · ${p.condition.model_id}` })" /><span class="whitespace-nowrap font-medium">{{ p.condition.model_id }}</span></label><p v-if="p.effective_upstream_model !== p.condition.model_id" class="ml-7 mt-1 text-xs text-gray-500">→ {{ p.effective_upstream_model }}</p></td>
                  <td class="p-3"><span :class="stateClass(p.state.state)" class="whitespace-nowrap rounded-lg px-2.5 py-1 text-xs font-medium">{{ stateLabel(p.state.state) }}</span><p class="mt-2 whitespace-nowrap text-xs text-gray-500">{{ label(p.state.freshness) }}<span v-if="p.state.failure_streak"> · {{ t('openAIQuality.failureStreak', { count: p.state.failure_streak }) }}</span></p></td>
                  <td class="p-3"><span class="whitespace-nowrap text-xs font-medium" :class="p.control.action === 'deny' ? 'text-red-600' : p.control.action === 'fallback' ? 'text-amber-600' : 'text-gray-600 dark:text-dark-300'">{{ label(p.control.action) }}</span><p class="mt-1 max-w-40 text-xs text-gray-400">{{ label(p.control.reason) }}</p></td>
                  <td class="p-3"><span class="text-xs font-medium" :class="p.state.last_verdict === 'incorrect' ? 'text-red-600' : p.state.last_verdict === 'passed' ? 'text-emerald-600' : 'text-gray-500'">{{ label(p.state.last_verdict) }}</span><p class="mt-1 whitespace-nowrap text-xs tabular-nums text-gray-400">{{ formatDate(p.state.last_checked_at) }}</p></td>
                  <td class="p-3"><div class="flex gap-2 whitespace-nowrap"><button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="busy || p.eligibility !== 'eligible'" @click="runNow(p)">{{ t('openAIQuality.runNow') }}</button><button class="btn btn-ghost px-2 py-1.5 text-xs text-primary-600 dark:text-primary-400" @click="selectPolicy(p)">{{ t('openAIQuality.timeline') }}</button></div></td>
                </tr>
              </template>
              <tr v-if="!filtered.length"><td colspan="6" class="p-12 text-center text-sm text-gray-500">{{ t('openAIQuality.noPolicies') }}<router-link class="mt-3 block text-primary-600" to="/admin/openai-quality/settings">{{ t('openAIQuality.newPolicy') }}</router-link></td></tr>
            </tbody>
          </table>
        </div>
        <Pagination :total="groupedRows.length" v-model:page="page" v-model:page-size="pageSize" />
      </section>

      </div>
      <BaseDialog :show="!!selected" :title="`${selected?.account_name || ''} · ${selected?.condition.model_id || ''}`" width="extra-wide" @close="closePolicy">
      <section v-if="selected" class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div><p class="text-sm font-medium">{{ groupName(selected) }} · {{ t('openAIQuality.last24h') }}</p><p class="mt-1 text-xs text-gray-500">{{ label(selected.condition.protocol) }} · {{ label(selected.condition.transport) }} · {{ label(selected.effective_reasoning_effort) }} · {{ label(selected.effective_service_tier) }}</p></div>
          <router-link class="btn btn-secondary px-3 py-2 text-xs" :to="{ path: '/admin/openai-quality/records', query: { policy_id: selected.id } }">{{ t('openAIQuality.allHistory') }}<Icon name="arrowRight" size="xs" /></router-link>
        </div>
        <nav class="flex gap-2 border-b border-gray-100 dark:border-dark-700" :aria-label="t('openAIQuality.policyDetails')">
          <button class="border-b-2 px-3 py-2 text-sm" :class="activeTab === 'timeline' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" @click="activeTab = 'timeline'">{{ t('openAIQuality.timeline') }}</button>
          <button class="border-b-2 px-3 py-2 text-sm" :class="activeTab === 'events' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" @click="showEvents">{{ t('openAIQuality.statusEvents') }}</button>
        </nav>

        <template v-if="activeTab === 'timeline'">
          <p v-if="!monitor" class="py-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
          <div class="flex flex-wrap gap-4 text-sm"><span>{{ t('openAIQuality.total') }}: {{ monitor?.summary.total ?? 0 }}</span><span>{{ t('openAIQuality.passed') }}: {{ monitor?.summary.passed ?? 0 }}</span><span>{{ t('openAIQuality.incorrect') }}: {{ monitor?.summary.incorrect ?? 0 }}</span><span>{{ t('openAIQuality.coverage') }}: {{ percent(monitor?.summary.answer_coverage) }}</span><span>{{ t('openAIQuality.passRate') }}: {{ percent(monitor?.summary.pass_rate) }}</span></div>
          <div class="grid grid-cols-12 gap-1 sm:grid-cols-[repeat(24,minmax(0,1fr))]" :aria-label="t('openAIQuality.last24h')">
            <button v-for="b in timelineBuckets" :key="b.index" :title="bucketLabel(b)" :aria-label="bucketLabel(b)" class="h-8 min-w-0 rounded focus:outline-none focus:ring-2 focus:ring-blue-500" :class="bucketClass(b.status)" @click="loadBucket(b.index)"></button>
          </div>
          <div class="flex justify-between text-xs text-gray-400"><span>{{ monitor ? formatDate(new Date(Date.parse(monitor.as_of) - 86400000).toISOString()) : '—' }}</span><span>{{ monitor ? formatDate(monitor.as_of) : '—' }}</span></div>
          <div class="flex flex-wrap gap-4 text-xs text-gray-500"><span v-for="value in ['passed','incorrect','error','unknown','empty']" :key="value" class="flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-sm" :class="bucketClass(value)"></span>{{ label(value) }}</span></div>
          <p v-if="!bucketRuns" class="text-center text-xs text-gray-400">{{ t('openAIQuality.clickTimeline') }}</p>
          <div v-if="bucketRuns" class="overflow-x-auto rounded-xl border border-gray-100 dark:border-dark-700">
            <table class="w-full text-xs"><thead><tr class="text-left text-gray-500"><th class="p-2">ID</th><th class="p-2">{{ t('openAIQuality.time') }}</th><th class="p-2">{{ t('openAIQuality.runStatus') }}</th><th class="p-2">{{ t('openAIQuality.answer') }}</th><th class="p-2">{{ t('openAIQuality.reason') }}</th><th class="p-2">{{ t('openAIQuality.actions') }}</th></tr></thead>
              <tbody><tr v-for="r in bucketRuns.items" :key="r.id" class="border-t border-gray-100 dark:border-dark-700"><td class="p-2">{{ r.id }}</td><td class="p-2">{{ formatDate(r.started_at || r.queued_at) }}</td><td class="p-2">{{ label(r.verdict || r.status) }}</td><td class="p-2">{{ r.result?.final_answer ?? '—' }} / {{ r.result?.expected_answer ?? '—' }}</td><td class="p-2">{{ label(r.reason_code) }}</td><td class="p-2"><button class="text-primary-600 hover:underline" @click="openRun(r.id)">{{ t('openAIQuality.viewDetails') }}</button><button v-if="isCancellable(r)" class="ml-3 text-red-600 hover:underline" @click="cancelRun(r)">{{ t('openAIQuality.cancelRun') }}</button></td></tr></tbody>
            </table>
            <div v-if="bucketRuns.items.length < bucketRuns.total" class="p-2 text-center"><button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="bucketBusy" @click="loadBucket(selectedBucketIndex!, bucketRuns.page + 1, true)">{{ bucketBusy ? t('common.loading') : t('openAIQuality.loadMoreHistory') }}</button></div>
          </div>
        </template>

        <div v-else class="space-y-3">
          <div v-if="eventsBusy && !events.length" class="p-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
          <div v-else-if="!events.length" class="p-6 text-center text-sm text-gray-500">{{ t('openAIQuality.noEvents') }}</div>
          <ol v-else class="space-y-2">
            <li v-for="event in events" :key="event.id" class="flex flex-wrap items-start justify-between gap-3 rounded-xl border border-gray-100 p-3 text-sm dark:border-dark-700">
              <div><p class="font-medium">{{ t(`openAIQuality.eventTypes.${event.event_type}`, event.event_type) }} <span v-if="event.old_state || event.new_state" class="font-normal text-gray-500">{{ event.old_state || '—' }} → {{ event.new_state || '—' }}</span></p><p class="mt-1 text-xs text-gray-500">{{ event.reason_code }} · {{ t('openAIQuality.revision') }} {{ event.revision }} · {{ t('openAIQuality.controlEpoch') }} {{ event.policy_control_epoch }} · {{ t('openAIQuality.actor') }} {{ event.actor_user_id || '—' }}</p></div>
              <div class="text-right text-xs text-gray-500"><time>{{ formatDate(event.created_at) }}</time><button v-if="event.run_id" class="ml-3 text-blue-600 hover:underline" @click="openRun(event.run_id)">{{ t('openAIQuality.viewRun', { id: event.run_id }) }}</button></div>
            </li>
          </ol>
          <div v-if="eventPage < eventPages" class="text-center"><button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="eventsBusy" @click="loadEvents(eventPage + 1, true)">{{ eventsBusy ? t('common.loading') : t('openAIQuality.loadMoreHistory') }}</button></div>
        </div>
      </section>
      <template v-if="selected" #footer><div class="flex flex-wrap justify-between gap-3"><span class="text-xs text-gray-400">{{ t('openAIQuality.serverAggregate') }} {{ formatDate(monitor?.as_of || null) }}</span><div class="flex gap-2"><button v-if="['quarantined','recovering'].includes(selected.state.state)" class="btn btn-secondary px-3 py-2 text-xs" @click="release(selected)">{{ t('openAIQuality.release') }}</button><button class="btn btn-ghost px-3 py-2 text-xs text-red-600" @click="archive(selected)">{{ t('openAIQuality.archive') }}</button></div></div></template>
      </BaseDialog>
      <QualityRunDetailDialog :detail="runDetail" :group-name="runDetail?.run.input_snapshot ? groupName(runDetail.run.input_snapshot) : ''" @close="runDetail = null" />

      <p v-if="error" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityRunDetailDialog from '@/components/admin/account-quality/QualityRunDetailDialog.vue'
import QualityResultsPanel from '@/components/admin/account-quality/QualityResultsPanel.vue'
import { groupQualityPolicies, qualityOutcomeClass } from '@/components/admin/account-quality/qualityPresentation'
import { getAllIncludingInactive } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { adaptServerQualityBuckets, type AdaptedServerTimelineBucket } from '@/components/admin/account-quality/qualityMonitor'
import { cancelQualityRun, getQualityEvents, getQualityOverview, getAllQualityPolicies, getQualityMonitor, getQualityBucketRuns, getQualityRun, submitQualityRuns, qualityPolicyAction, type QualityEvent, type QualityPolicy, type QualityMonitor, type QualityRun, type QualityRunDetail } from '@/api/admin/openaiAccountQuality'

const { t, te } = useI18n()
const route = useRoute()
const monitorViews = ['results', 'policies'] as const
const monitorView = ref<'results' | 'policies'>('results')
const resultRevision = ref(0)
const overview = ref<any>(null)
const policies = ref<QualityPolicy[]>([])
const groups = ref<AdminGroup[]>([])
const page = ref(1)
const pageSize = ref(10)
const selected = ref<QualityPolicy | null>(null)
const monitor = ref<QualityMonitor | null>(null)
const timelineBuckets = ref<AdaptedServerTimelineBucket[]>([])
const bucketRuns = ref<any>(null)
const selectedBucketIndex = ref<number | null>(null)
const selectedPolicyIds = ref<number[]>([])
const sourceGroupFilter = ref('')
const stateFilter = ref('')
const freshnessFilter = ref('')
const activeTab = ref<'timeline' | 'events'>('timeline')
const events = ref<QualityEvent[]>([])
const eventPage = ref(1)
const eventPages = ref(1)
const eventsBusy = ref(false)
const runDetail = ref<QualityRunDetail | null>(null)
const bucketBusy = ref(false)
const search = ref('')
const busy = ref(false)
const error = ref('')
let reloadRequest = 0
let monitorRequest = 0
let bucketRequest = 0
let eventRequest = 0
let runRequest = 0
let pollTimer: number | undefined
let pollFailures = 0
let disposed = false
let active = true
let activatedOnce = false

const filtered = computed(() => policies.value.filter(p => `${p.account_name} ${p.account_id} ${p.condition.model_id} ${p.effective_upstream_model}`.toLowerCase().includes(search.value.toLowerCase())))
const groupedRows = computed(() => groupQualityPolicies(filtered.value))
const visibleRows = computed(() => groupedRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const visiblePolicies = computed(() => visibleRows.value.flatMap(row => row.policies))
const sourceGroupOptions = computed(() => [{ value: '', label: t('openAIQuality.allSourceGroups') }, ...groups.value.filter(g => g.platform === 'openai' && g.group_role !== 'self_hosted_pool').map(g => ({ value: String(g.id), label: g.name }))])
const states = ['untested', 'observing', 'normal', 'quarantined', 'recovering']
const stateOptions = computed(() => [{ value: '', label: t('openAIQuality.allStates') }, ...states.map(value => ({ value, label: stateLabel(value) }))])
const freshnessOptions = computed(() => [{ value: '', label: t('openAIQuality.allFreshness') }, ...['fresh', 'stale', 'never'].map(value => ({ value, label: label(value) }))])
const selectedRunnablePolicies = computed(() => filtered.value.filter(p => selectedPolicyIds.value.includes(p.id) && p.eligibility === 'eligible'))
const allVisibleSelected = computed(() => visiblePolicies.value.length > 0 && visiblePolicies.value.every(p => selectedPolicyIds.value.includes(p.id)))
const someVisibleSelected = computed(() => visiblePolicies.value.some(p => selectedPolicyIds.value.includes(p.id)))
const cards = computed(() => [
  { label: t('openAIQuality.eligibleAccounts'), value: overview.value?.inventory?.eligible_account_count ?? 0 },
  { label: t('openAIQuality.configuredAccounts'), value: overview.value?.inventory?.configured_account_count ?? 0 },
  { label: t('openAIQuality.isolated'), value: overview.value?.policies?.state_counts?.quarantined ?? 0 },
  { label: t('openAIQuality.budget'), value: `${overview.value?.budget?.consumed ?? overview.value?.budget?.consumed_calls ?? 0} / ${overview.value?.budget?.limit ?? overview.value?.budget?.global_limit ?? '—'}` },
])

function errorMessage(value: unknown) { return extractApiErrorMessage(value, t('common.error')) }
function label(value: string) { return te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
function groupName(policy: QualityPolicy) { return groups.value.find(g => g.id === policy.source_group_id)?.name || policy.source_group_name || t('openAIQuality.unavailableGroup') }
async function reload() {
  const request = ++reloadRequest
  busy.value = true
  error.value = ''
  try {
    const query: Record<string, unknown> = {
      ...(route.query.account_id ? { account_id: Number(route.query.account_id) } : {}),
      ...(sourceGroupFilter.value ? { source_group_id: Number(sourceGroupFilter.value) } : {}),
      ...(stateFilter.value ? { state: stateFilter.value } : {}),
      ...(freshnessFilter.value ? { freshness: freshnessFilter.value } : {}),
    }
    const [o, p] = await Promise.all([getQualityOverview(query), getAllQualityPolicies(query)])
    if (request === reloadRequest) {
      overview.value = o
      policies.value = p.items
      resultRevision.value++
      selectedPolicyIds.value = selectedPolicyIds.value.filter(id => p.items.some(policy => policy.id === id))
      if (selected.value && !p.items.some(policy => policy.id === selected.value?.id)) closePolicy()
      else if (selected.value) selected.value = p.items.find(policy => policy.id === selected.value?.id) || null
      page.value = Math.min(page.value, Math.max(1, Math.ceil(groupedRows.value.length / pageSize.value)))
    }
    return request === reloadRequest
  } catch (e) { if (request === reloadRequest) error.value = errorMessage(e); return false }
  finally { if (request === reloadRequest) busy.value = false }
}
function closePolicy() {
  selected.value = null
  monitor.value = null
  timelineBuckets.value = []
  bucketRuns.value = null
  events.value = []
  ++monitorRequest
  ++bucketRequest
  ++eventRequest
}
async function selectPolicy(policy: QualityPolicy) {
  const request = ++monitorRequest
  selected.value = policy
  monitor.value = null
  timelineBuckets.value = []
  bucketRuns.value = null
  events.value = []
  eventPage.value = 1
  eventPages.value = 1
  selectedBucketIndex.value = null
  activeTab.value = 'timeline'
  error.value = ''
  try {
    const result = await getQualityMonitor(policy.id)
    const adaptedBuckets = adaptServerQualityBuckets(result)
    if (request === monitorRequest && selected.value?.id === policy.id) {
      monitor.value = result
      timelineBuckets.value = adaptedBuckets
    }
  } catch (e) { if (request === monitorRequest) error.value = errorMessage(e) }
}
async function loadBucket(index: number, page = 1, append = false) {
  if (!selected.value || !monitor.value) return
  const request = ++bucketRequest
  const policy = selected.value
  const currentMonitor = monitor.value
  bucketBusy.value = true
  try {
    const result = await getQualityBucketRuns(policy, currentMonitor, index, page)
    if (request !== bucketRequest || selected.value?.id !== policy.id || monitor.value !== currentMonitor) return
    if (append && selectedBucketIndex.value === index && bucketRuns.value) bucketRuns.value = { ...result, items: [...bucketRuns.value.items, ...result.items] }
    else { bucketRuns.value = result; selectedBucketIndex.value = index }
  } catch (e) { if (request === bucketRequest) error.value = errorMessage(e) }
  finally { if (request === bucketRequest) bucketBusy.value = false }
}
async function showEvents() {
  activeTab.value = 'events'
  if (events.value.length === 0) await loadEvents()
}
async function loadEvents(page = 1, append = false) {
  if (!selected.value) return
  const request = ++eventRequest
  const policyId = selected.value.id
  eventsBusy.value = true
  try {
    const result = await getQualityEvents({ policy_id: policyId, page, page_size: 50 })
    if (request !== eventRequest || selected.value?.id !== policyId) return
    events.value = append ? [...events.value, ...result.items] : result.items
    eventPage.value = result.page
    eventPages.value = result.pages
  } catch (e) { if (request === eventRequest) error.value = errorMessage(e) }
  finally { if (request === eventRequest) eventsBusy.value = false }
}
async function openRun(id: number) {
  const request = ++runRequest
  runDetail.value = null
  error.value = ''
  try {
    const result = await getQualityRun(id, true)
    if (request === runRequest) runDetail.value = result
  } catch (e) { if (request === runRequest) error.value = errorMessage(e) }
}
function isCancellable(run: QualityRun) { return ['queued', 'running'].includes(run.status) }
async function cancelRun(run: QualityRun) {
  const reason = window.prompt(t('openAIQuality.cancelRunReason'))
  if (!reason?.trim()) return
  try {
    await cancelQualityRun(run.id, reason.trim())
    if (selectedBucketIndex.value !== null) await loadBucket(selectedBucketIndex.value)
    if (activeTab.value === 'events') await loadEvents()
  } catch (e) { error.value = errorMessage(e) }
}
function toggleVisible(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  const visibleIds = new Set(visiblePolicies.value.map(p => p.id))
  selectedPolicyIds.value = checked
    ? [...new Set([...selectedPolicyIds.value, ...visibleIds])].slice(0, 100)
    : selectedPolicyIds.value.filter(id => !visibleIds.has(id))
}
async function runNow(policy: QualityPolicy) {
  busy.value = true
  error.value = ''
  try { await submitQualityRuns([policy.id]); await reload() }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
async function runSelectedNow() {
  if (!selectedRunnablePolicies.value.length) return
  busy.value = true
  error.value = ''
  try { await submitQualityRuns(selectedRunnablePolicies.value.map(policy => policy.id)); await reload() }
  catch (e) { error.value = errorMessage(e) }
  finally { busy.value = false }
}
async function release(policy: QualityPolicy) {
  const reason = window.prompt(t('openAIQuality.releaseReason'))
  if (!reason) return
  try { await qualityPolicyAction(policy.id, 'release', policy.revision, reason); await reload() }
  catch (e) { error.value = errorMessage(e) }
}
async function archive(policy: QualityPolicy) {
  if (!window.confirm(t('openAIQuality.archiveConfirm'))) return
  const reason = window.prompt(t('openAIQuality.archiveReason'))
  if (!reason) return
  try { await qualityPolicyAction(policy.id, 'archive', policy.revision, reason); await reload() }
  catch (e) { error.value = errorMessage(e) }
}
function formatDate(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function percent(value: number | null | undefined) { return value == null ? '—' : `${value.toFixed(1)}%` }
function stateLabel(value: string) { return t(`openAIQuality.states.${value}`) }
function stateClass(value: string) { return qualityOutcomeClass(value) }
function bucketClass(value: string) { return value === 'passed' ? 'bg-green-500' : value === 'incorrect' ? 'bg-red-500' : value === 'error' ? 'bg-rose-300' : value === 'running' ? 'bg-blue-400' : value === 'queued' ? 'bg-blue-200' : value === 'unknown' ? 'bg-amber-300' : value === 'empty' ? 'bg-gray-100 dark:bg-dark-700' : 'bg-gray-300' }
function bucketLabel(bucket: AdaptedServerTimelineBucket) { return `${formatDate(new Date(bucket.start_ms).toISOString())} · ${label(bucket.status)} · ${bucket.total}` }

function schedulePoll(delay: number) {
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
  if (document.hidden || disposed || !active) return
  pollTimer = window.setTimeout(() => { void refreshAndSchedule() }, delay)
}
async function refreshAndSchedule() {
  if (document.hidden || disposed || !active) return
  const succeeded = await reload()
  if (disposed || !active) return
  if (succeeded) pollFailures = 0
  else pollFailures++
  const hasActiveRuns = Number(overview.value?.health?.queue_depth || 0) + Number(overview.value?.health?.running_count || 0) > 0
  const normalDelay = hasActiveRuns ? 3000 : 30000
  const delay = succeeded ? normalDelay : Math.min(30000, 3000 * 2 ** Math.min(pollFailures, 3))
  schedulePoll(delay)
}
function visibilityChanged() {
  if (document.hidden) {
    if (pollTimer !== undefined) window.clearTimeout(pollTimer)
    pollTimer = undefined
  } else {
    void refreshAndSchedule()
  }
}
onMounted(() => {
  void getAllIncludingInactive().then(result => { groups.value = result }).catch(e => { error.value = errorMessage(e) })
  document.addEventListener('visibilitychange', visibilityChanged)
  void refreshAndSchedule()
})
watch([sourceGroupFilter, stateFilter, freshnessFilter], () => { page.value = 1; if (active) void reload() })
watch([search, pageSize], () => { page.value = 1 })
watch(() => route.query.account_id, () => { if (active && route.path === '/admin/openai-quality') { page.value = 1; void reload() } })
onActivated(() => {
  active = true
  if (activatedOnce) void refreshAndSchedule()
  activatedOnce = true
})
onDeactivated(() => {
  active = false
  closePolicy()
  runDetail.value = null
  ++runRequest
  ++reloadRequest
  busy.value = false
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
})
onUnmounted(() => {
  disposed = true
  ++reloadRequest
  ++monitorRequest
  ++bucketRequest
  ++eventRequest
  ++runRequest
  document.removeEventListener('visibilitychange', visibilityChanged)
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
})
</script>
