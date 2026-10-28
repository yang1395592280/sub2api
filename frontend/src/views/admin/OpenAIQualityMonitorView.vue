<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-5 p-4 lg:p-7">
      <header class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('openAIQuality.monitorTitle') }}</h1>
            <p class="mt-1 text-sm text-gray-500">{{ t('openAIQuality.monitorDescription') }}</p>
          </div>
          <div class="flex gap-2">
            <router-link class="btn-secondary" to="/admin/openai-quality/settings">{{ t('nav.openAIQualitySettings') }}</router-link>
            <button class="btn-secondary" :disabled="busy" @click="reload">{{ t('common.refresh') }}</button>
          </div>
        </div>
        <p class="mt-3 rounded-xl bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">{{ t('openAIQuality.scopeNote') }}</p>
      </header>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <article v-for="item in cards" :key="item.label" class="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <p class="text-xs text-gray-500">{{ item.label }}</p>
          <p class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{{ item.value }}</p>
        </article>
      </section>
      <section class="flex flex-wrap gap-x-6 gap-y-2 rounded-2xl bg-white px-4 py-3 text-sm shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <span>{{ t('openAIQuality.queueDepth') }}: {{ overview?.health?.queue_depth ?? '—' }}</span>
        <span>{{ t('openAIQuality.runningCount') }}: {{ overview?.health?.running_count ?? '—' }}</span>
        <span>{{ t('openAIQuality.storeStatus') }}: {{ overview?.health?.store_status ?? '—' }}</span>
        <span>{{ t('openAIQuality.lastSync') }}: {{ formatDate(overview?.health?.last_sync_at ?? null) }}</span>
      </section>

      <section class="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
          <h2 class="font-semibold">{{ t('openAIQuality.policies') }}</h2>
          <div class="flex flex-wrap gap-2">
            <input v-model="search" class="input w-56" :placeholder="t('openAIQuality.search')" />
            <select v-model="sourceGroupFilter" class="input w-44" :aria-label="t('openAIQuality.sourceGroupFilter')"><option value="">{{ t('openAIQuality.allSourceGroups') }}</option><option v-for="id in sourceGroupOptions" :key="id" :value="String(id)">#{{ id }}</option></select>
            <select v-model="stateFilter" class="input w-40" :aria-label="t('openAIQuality.stateFilter')"><option value="">{{ t('openAIQuality.allStates') }}</option><option v-for="state in states" :key="state" :value="state">{{ stateLabel(state) }}</option></select>
            <select v-model="freshnessFilter" class="input w-40" :aria-label="t('openAIQuality.freshnessFilter')"><option value="">{{ t('openAIQuality.allFreshness') }}</option><option value="fresh">{{ t('openAIQuality.fresh') }}</option><option value="stale">{{ t('openAIQuality.stale') }}</option><option value="never">{{ t('openAIQuality.never') }}</option></select>
          </div>
        </div>
        <div v-if="selectedPolicyIds.length" class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-4 py-3 text-sm dark:border-dark-700">
          <span>{{ t('openAIQuality.selectedPolicies', { count: selectedPolicyIds.length }) }}</span>
          <span class="text-xs text-gray-500">{{ t('openAIQuality.batchLimit') }}</span>
          <button class="btn-primary btn-xs" :disabled="busy || !selectedRunnablePolicies.length" @click="runSelectedNow">{{ t('openAIQuality.runSelected') }}</button>
          <button class="btn-secondary btn-xs" @click="selectedPolicyIds = []">{{ t('common.cancel') }}</button>
        </div>
        <div v-if="busy && !overview" class="p-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900">
              <tr>
                <th class="w-10 p-3"><input type="checkbox" :checked="allVisibleSelected" :indeterminate="someVisibleSelected && !allVisibleSelected" :aria-label="t('openAIQuality.selectAll')" @change="toggleVisible" /></th>
                <th class="p-3">{{ t('openAIQuality.account') }}</th>
                <th class="p-3">{{ t('openAIQuality.condition') }}</th>
                <th class="p-3">{{ t('openAIQuality.state') }}</th>
                <th class="p-3">{{ t('openAIQuality.control') }}</th>
                <th class="p-3">{{ t('openAIQuality.lastCheck') }}</th>
                <th class="p-3">{{ t('openAIQuality.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in filtered" :key="p.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="p-3"><input v-model="selectedPolicyIds" type="checkbox" :value="p.id" :disabled="!selectedPolicyIds.includes(p.id) && selectedPolicyIds.length >= 100" :aria-label="t('openAIQuality.selectPolicy', { account: p.account_name || p.account_id })" /></td>
                <td class="p-3"><button class="font-medium text-blue-600 hover:underline" @click="selectPolicy(p)">{{ p.account_name || `#${p.account_id}` }}</button><p class="text-xs text-gray-500">{{ t('openAIQuality.group') }} #{{ p.source_group_id }}</p></td>
                <td class="p-3"><span>{{ p.effective_upstream_model }}</span><p class="text-xs text-gray-500">{{ p.condition.protocol }} · {{ p.condition.transport }} · {{ p.effective_reasoning_effort }} · {{ p.effective_service_tier }}</p></td>
                <td class="p-3"><span :class="stateClass(p.state.state)" class="rounded-full px-2 py-1 text-xs">{{ stateLabel(p.state.state) }}</span><p class="mt-1 text-xs text-gray-500">{{ p.state.freshness }} · {{ p.state.failure_streak }}/{{ p.rules.failure_threshold }}</p></td>
                <td class="p-3"><span :class="p.control.effective ? 'text-green-600' : 'text-gray-500'">{{ p.control.action }}</span><p class="text-xs text-gray-500">{{ p.control.reason }}</p></td>
                <td class="p-3 text-xs text-gray-500">{{ p.state.last_checked_at ? formatDate(p.state.last_checked_at) : '—' }}<p>{{ p.state.last_verdict }}</p></td>
                <td class="p-3"><div class="flex gap-2"><button class="btn-secondary btn-xs" :disabled="busy || p.eligibility !== 'eligible'" @click="runNow(p)">{{ t('openAIQuality.runNow') }}</button><button v-if="['quarantined','recovering'].includes(p.state.state)" class="btn-secondary btn-xs" @click="release(p)">{{ t('openAIQuality.release') }}</button><button class="btn-secondary btn-xs" @click="archive(p)">{{ t('openAIQuality.archive') }}</button></div></td>
              </tr>
              <tr v-if="!filtered.length"><td colspan="7" class="p-8 text-center text-sm text-gray-500">{{ t('openAIQuality.noPolicies') }}</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="selected" class="space-y-4 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div><h2 class="font-semibold">{{ selected.account_name }} · {{ t('openAIQuality.last24h') }}</h2><p class="text-xs text-gray-500">{{ t('openAIQuality.serverAggregate') }} {{ monitor?.as_of }}</p></div>
          <button class="text-sm text-gray-500" :aria-label="t('common.close')" @click="closePolicy">×</button>
        </div>
        <nav class="flex gap-2 border-b border-gray-100 dark:border-dark-700" :aria-label="t('openAIQuality.policyDetails')">
          <button class="border-b-2 px-3 py-2 text-sm" :class="activeTab === 'timeline' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500'" @click="activeTab = 'timeline'">{{ t('openAIQuality.currentStatus') }} / {{ t('openAIQuality.executionHistory') }}</button>
          <button class="border-b-2 px-3 py-2 text-sm" :class="activeTab === 'events' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500'" @click="showEvents">{{ t('openAIQuality.statusEvents') }}</button>
        </nav>

        <template v-if="activeTab === 'timeline'">
          <div class="flex flex-wrap gap-4 text-sm"><span>{{ t('openAIQuality.total') }}: {{ monitor?.summary.total ?? 0 }}</span><span>{{ t('openAIQuality.passed') }}: {{ monitor?.summary.passed ?? 0 }}</span><span>{{ t('openAIQuality.incorrect') }}: {{ monitor?.summary.incorrect ?? 0 }}</span><span>{{ t('openAIQuality.coverage') }}: {{ percent(monitor?.summary.answer_coverage) }}</span><span>{{ t('openAIQuality.passRate') }}: {{ percent(monitor?.summary.pass_rate) }}</span></div>
          <div class="grid grid-cols-12 gap-1 sm:grid-cols-24" :aria-label="t('openAIQuality.last24h')">
            <button v-for="b in timelineBuckets" :key="b.index" :title="bucketLabel(b)" :aria-label="bucketLabel(b)" class="h-8 min-w-0 rounded focus:outline-none focus:ring-2 focus:ring-blue-500" :class="bucketClass(b.status)" @click="loadBucket(b.index)"></button>
          </div>
          <div v-if="bucketRuns" class="overflow-x-auto rounded-xl border border-gray-100 dark:border-dark-700">
            <table class="w-full text-xs"><thead><tr class="text-left text-gray-500"><th class="p-2">ID</th><th class="p-2">{{ t('openAIQuality.time') }}</th><th class="p-2">{{ t('openAIQuality.runStatus') }}</th><th class="p-2">{{ t('openAIQuality.answer') }}</th><th class="p-2">{{ t('openAIQuality.reason') }}</th><th class="p-2">{{ t('openAIQuality.actions') }}</th></tr></thead>
              <tbody><tr v-for="r in bucketRuns.items" :key="r.id" class="border-t border-gray-100 dark:border-dark-700"><td class="p-2">{{ r.id }}</td><td class="p-2">{{ formatDate(r.started_at || r.queued_at) }}</td><td class="p-2">{{ r.status }}</td><td class="p-2">{{ r.result?.final_answer ?? '—' }} / {{ r.result?.expected_answer ?? '—' }}</td><td class="p-2">{{ r.reason_code }}</td><td class="p-2"><button class="text-blue-600 hover:underline" @click="openRun(r.id)">{{ t('openAIQuality.viewDetails') }}</button><button v-if="isCancellable(r)" class="ml-3 text-red-600 hover:underline" @click="cancelRun(r)">{{ t('openAIQuality.cancelRun') }}</button></td></tr></tbody>
            </table>
            <div v-if="bucketRuns.items.length < bucketRuns.total" class="p-2 text-center"><button class="btn-secondary btn-xs" :disabled="bucketBusy" @click="loadBucket(selectedBucketIndex!, bucketRuns.page + 1, true)">{{ bucketBusy ? t('common.loading') : t('openAIQuality.loadMoreHistory') }}</button></div>
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
          <div v-if="eventPage < eventPages" class="text-center"><button class="btn-secondary btn-xs" :disabled="eventsBusy" @click="loadEvents(eventPage + 1, true)">{{ eventsBusy ? t('common.loading') : t('openAIQuality.loadMoreHistory') }}</button></div>
        </div>
      </section>

      <div v-if="runDetail" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="presentation" @click.self="runDetail = null">
        <section class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-5 shadow-xl dark:bg-dark-800" role="dialog" aria-modal="true" :aria-label="t('openAIQuality.runDetailsTitle', { id: runDetail.run.id })">
          <div class="flex items-start justify-between gap-3"><div><h2 class="text-lg font-semibold">{{ t('openAIQuality.runDetailsTitle', { id: runDetail.run.id }) }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.trigger') }}: {{ runDetail.run.trigger_source }} · {{ formatDate(runDetail.run.finished_at || runDetail.run.queued_at) }}</p></div><button class="text-gray-500" :aria-label="t('common.close')" @click="runDetail = null">×</button></div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2"><p>{{ t('openAIQuality.runStatus') }}: {{ runDetail.run.status }} / {{ runDetail.run.verdict || '—' }}</p><p>{{ t('openAIQuality.reason') }}: {{ runDetail.run.result.reason_code || runDetail.run.reason_code || '—' }}</p><p>{{ t('openAIQuality.answer') }}: {{ runDetail.run.result.final_answer ?? '—' }} / {{ runDetail.run.result.expected_answer }}</p><p>{{ t('openAIQuality.latency') }}: {{ runDetail.run.result.latency_ms }} ms</p><p>{{ t('openAIQuality.tokenUsage') }}: {{ runDetail.run.result.input_tokens ?? '—' }} / {{ runDetail.run.result.output_tokens ?? '—' }} / {{ runDetail.run.result.total_tokens ?? '—' }}</p><p>{{ t('openAIQuality.estimatedCost') }}: {{ runDetail.run.result.estimated_cost_usd ?? '—' }}</p><p>{{ t('openAIQuality.stateParticipation') }}: {{ runDetail.run.state_applied ? t('openAIQuality.appliedToState') : (runDetail.run.state_skip_reason || t('openAIQuality.notAppliedToState')) }}</p></div>
          <p class="mt-3 rounded-xl bg-blue-50 p-3 text-xs text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">{{ t('openAIQuality.scopeNote') }}</p>
          <div class="mt-4"><h3 class="mb-2 font-medium">{{ t('openAIQuality.finalResponse') }}</h3><p v-if="runDetail.response_expired" class="text-sm text-gray-500">{{ t('openAIQuality.responseExpired') }}</p><p v-else-if="!runDetail.response_available" class="text-sm text-gray-500">{{ t('openAIQuality.responseUnavailable') }}</p><pre v-else class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-900">{{ runDetail.run.response_text }}</pre><p v-if="runDetail.run.result.response_truncated" class="mt-2 text-xs text-amber-700">{{ t('openAIQuality.responseTruncated') }}</p></div>
        </section>
      </div>

      <p v-if="error" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adaptServerQualityBuckets, type AdaptedServerTimelineBucket } from '@/components/admin/account-quality/qualityMonitor'
import { cancelQualityRun, getQualityEvents, getQualityOverview, getAllQualityPolicies, getQualityMonitor, getQualityBucketRuns, getQualityRun, submitQualityRuns, qualityPolicyAction, type QualityEvent, type QualityPolicy, type QualityMonitor, type QualityRun, type QualityRunDetail } from '@/api/admin/openaiAccountQuality'

const { t } = useI18n()
const route = useRoute()
const overview = ref<any>(null)
const policies = ref<QualityPolicy[]>([])
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

const filtered = computed(() => policies.value.filter(p => `${p.account_name} ${p.account_id} ${p.effective_upstream_model}`.toLowerCase().includes(search.value.toLowerCase())))
const sourceGroupOptions = computed(() => [...new Set(policies.value.flatMap(policy => [policy.source_group_id, ...policy.compatible_source_group_ids]))].sort((a, b) => a - b))
const states = ['untested', 'observing', 'normal', 'quarantined', 'recovering']
const selectedRunnablePolicies = computed(() => filtered.value.filter(p => selectedPolicyIds.value.includes(p.id) && p.eligibility === 'eligible'))
const allVisibleSelected = computed(() => filtered.value.length > 0 && filtered.value.every(p => selectedPolicyIds.value.includes(p.id)))
const someVisibleSelected = computed(() => filtered.value.some(p => selectedPolicyIds.value.includes(p.id)))
const cards = computed(() => [
  { label: t('openAIQuality.eligibleAccounts'), value: overview.value?.inventory?.eligible_account_count ?? 0 },
  { label: t('openAIQuality.configuredAccounts'), value: overview.value?.inventory?.configured_account_count ?? 0 },
  { label: t('openAIQuality.isolated'), value: overview.value?.policies?.state_counts?.quarantined ?? 0 },
  { label: t('openAIQuality.budget'), value: `${overview.value?.budget?.consumed ?? overview.value?.budget?.consumed_calls ?? 0} / ${overview.value?.budget?.limit ?? overview.value?.budget?.global_limit ?? '—'}` },
])

function errorMessage(errorValue: unknown) { return errorValue instanceof Error ? errorValue.message : String(errorValue) }
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
      selectedPolicyIds.value = selectedPolicyIds.value.filter(id => p.items.some(policy => policy.id === id))
      if (selected.value && !p.items.some(policy => policy.id === selected.value?.id)) closePolicy()
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
  const visibleIds = new Set(filtered.value.map(p => p.id))
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
function stateClass(value: string) { return value === 'quarantined' || value === 'recovering' ? 'bg-red-100 text-red-700' : value === 'observing' ? 'bg-amber-100 text-amber-700' : value === 'normal' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600' }
function bucketClass(value: string) { return value === 'passed' ? 'bg-green-500' : value === 'incorrect' ? 'bg-red-500' : value === 'error' ? 'bg-rose-300' : value === 'running' ? 'bg-blue-400' : value === 'queued' ? 'bg-blue-200' : value === 'unknown' ? 'bg-amber-300' : value === 'empty' ? 'bg-gray-100 dark:bg-dark-700' : 'bg-gray-300' }
function bucketLabel(bucket: AdaptedServerTimelineBucket) { return `${formatDate(new Date(bucket.start_ms).toISOString())} · ${bucket.status} · ${bucket.total}` }

function schedulePoll(delay: number) {
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
  if (document.hidden) return
  pollTimer = window.setTimeout(() => { void refreshAndSchedule() }, delay)
}
async function refreshAndSchedule() {
  if (document.hidden) return
  const succeeded = await reload()
  if (succeeded) pollFailures = 0
  else pollFailures++
  const active = Number(overview.value?.health?.queue_depth || 0) + Number(overview.value?.health?.running_count || 0) > 0
  const normalDelay = active ? 3000 : 30000
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
  document.addEventListener('visibilitychange', visibilityChanged)
  void refreshAndSchedule()
})
watch([sourceGroupFilter, stateFilter, freshnessFilter], () => { void reload() })
onUnmounted(() => {
  document.removeEventListener('visibilitychange', visibilityChanged)
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
})
</script>
