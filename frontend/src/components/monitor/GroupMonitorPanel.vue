<template>
  <section class="space-y-5">
    <header class="flex flex-wrap items-start justify-between gap-4">
      <div><h2 class="text-lg font-semibold">{{ t('groupMonitor.title') }}</h2><p class="mt-1 max-w-3xl text-sm text-gray-500">{{ t('groupMonitor.description') }}</p></div>
      <div class="flex gap-2"><button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('groupMonitor.refresh') }}</button><button v-if="admin" class="btn btn-primary" @click="configure()">{{ t('groupMonitor.configure') }}</button></div>
    </header>
    <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950">{{ error }}</p>
    <p v-if="notice" role="status" class="text-sm text-primary-600">{{ notice }}</p>
    <p v-if="!loading && !plans.length" class="card p-8 text-center text-gray-500">{{ t('groupMonitor.empty') }}</p>
    <div class="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
      <article v-for="plan in plans" :key="plan.id" class="card min-w-0 space-y-4 p-5">
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0"><h3 class="truncate font-semibold">{{ plan.group_name }}</h3><p class="mt-1 text-xs text-gray-500">{{ plan.platform }} · #{{ plan.group_id }}</p></div>
          <span v-if="plan.config && !plan.config.enabled" class="rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-500 dark:bg-dark-700">{{ t('groupMonitor.paused') }}</span>
          <span v-else-if="isRunning(plan)" class="text-xs text-primary-500">{{ t('groupMonitor.running') }}</span>
        </div>
        <div v-for="model in models(plan)" :key="model" class="space-y-2">
          <div class="flex items-center justify-between gap-2 text-sm"><span class="truncate font-mono" :title="model">{{ model }}</span><span class="shrink-0" :class="statusClass(latest(plan, model)?.status)">{{ statusText(latest(plan, model)?.status) }}</span></div>
          <div class="flex items-center justify-between text-xs text-gray-500"><span>{{ t('groupMonitor.latency') }} {{ latency(latest(plan, model)) }}</span><span>{{ t('groupMonitor.ttft') }} {{ firstContent(latest(plan, model)) }}</span></div>
          <div class="flex h-5 gap-1" :aria-label="t('groupMonitor.history')"><span v-for="r in timeline(plan, model)" :key="r.id" class="min-w-1 flex-1 rounded-sm" :class="r.status === 'success' ? 'bg-emerald-400' : r.status === 'running' ? 'bg-gray-300' : 'bg-rose-400'" :title="`${date(r.created_at)} · ${statusText(r.status)}`" /></div>
        </div>
        <div v-if="admin" class="space-y-1 text-xs text-gray-500"><p>{{ t('groupMonitor.budget', { used: plan.requests_used, limit: plan.config?.daily_request_limit }) }}</p><p>{{ t('groupMonitor.next') }} {{ date(plan.next_probe_at) }}</p></div>
        <div class="flex flex-wrap gap-2 border-t border-gray-100 pt-3 dark:border-dark-700">
          <button class="btn btn-secondary btn-sm" @click="openHistory(plan, 'probe')">{{ t('groupMonitor.history') }}</button>
          <button v-if="admin || plan.pelican_visible" class="btn btn-secondary btn-sm" @click="openHistory(plan, 'pelican')">{{ t('groupMonitor.pelican') }}</button>
          <button v-if="admin" class="btn btn-primary btn-sm" :disabled="busy || isRunning(plan)" @click="run(plan, 'probe')">{{ t('groupMonitor.probe') }}</button>
          <button v-if="admin" class="btn btn-secondary btn-sm" @click="configure(plan)">{{ t('groupMonitor.edit') }}</button>
        </div>
      </article>
    </div>
    <BaseDialog :show="showConfig" :title="t('groupMonitor.configure')" width="wide" @close="showConfig = false">
      <form class="space-y-6" @submit.prevent="save">
        <p class="rounded-xl bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-200">{{ t('groupMonitor.newPaused') }}</p>
        <fieldset><legend class="mb-3 font-semibold">{{ t('groupMonitor.groups') }}</legend><div class="grid max-h-48 gap-3 overflow-auto sm:grid-cols-2"><label v-for="g in groups" :key="g.id" class="flex items-center gap-2 text-sm"><input v-model="selectedGroups" type="checkbox" :value="g.id" /><span>{{ g.name }} <span class="text-gray-400">{{ g.platform }}</span></span></label></div></fieldset>
        <fieldset class="space-y-3"><legend class="mb-3 font-semibold">{{ t('groupMonitor.models') }}</legend>
          <button class="btn btn-secondary btn-sm" type="button" :disabled="!selectedGroups.length || busy" @click="loadCandidates">{{ t('groupMonitor.candidates') }}</button>
          <div v-if="candidates.length" class="flex max-h-32 flex-wrap gap-2 overflow-auto"><button v-for="model in candidates" :key="model" type="button" class="rounded-lg border px-2 py-1 text-xs" :class="modelNames.includes(model) ? 'border-primary-500 bg-primary-50 text-primary-700' : ''" @click="toggleModel(model)">{{ model }}</button></div>
          <label class="block"><span class="mb-2 block text-xs text-gray-500">{{ t('groupMonitor.modelHint') }}</span><textarea v-model="modelDraft" class="input w-full font-mono" rows="3" required /></label>
        </fieldset>
        <fieldset class="space-y-4"><legend class="mb-3 font-semibold">{{ t('groupMonitor.schedule') }}</legend>
          <div class="grid gap-4 sm:grid-cols-2"><label v-for="field in numericFields" :key="field.key" class="space-y-2 text-sm"><span>{{ t(`groupMonitor.${field.label}`) }}</span><input v-model.number="config[field.key]" type="number" class="input w-full" :min="field.min" :max="field.max" required /></label></div>
          <label class="flex items-center gap-2 text-sm"><input v-model="config.pelican_enabled" type="checkbox" />{{ t('groupMonitor.pelicanEnabled') }}</label>
          <label class="block space-y-2 text-sm"><span>{{ t('groupMonitor.prompt') }}</span><textarea v-model="config.pelican_prompt" class="input w-full" rows="4" maxlength="12000" :required="config.pelican_enabled" /></label>
          <label class="flex items-center gap-2 text-sm"><input v-model="config.enabled" type="checkbox" />{{ t('groupMonitor.enabled') }}</label>
          <p class="text-sm text-primary-600">{{ t('groupMonitor.estimate', { count: estimate }) }}</p><p class="text-xs text-gray-500">{{ t('groupMonitor.disabledHint') }}</p>
        </fieldset>
        <p v-if="dialogError" role="alert" class="text-sm text-red-500">{{ dialogError }}</p>
        <div class="flex justify-end gap-2"><button v-if="editingPlan" type="button" class="btn btn-secondary text-red-600" :disabled="busy" @click="remove">{{ t('groupMonitor.delete') }}</button><button class="btn btn-primary" type="submit" :disabled="busy || !selectedGroups.length || modelNames.length < 1 || modelNames.length > 5">{{ t('groupMonitor.save') }}</button></div>
      </form>
    </BaseDialog>
    <BaseDialog :show="!!historyPlan" :title="`${historyPlan?.group_name ?? ''} · ${t(`groupMonitor.${historyKind === 'pelican' ? 'pelican' : 'history'}`)}`" width="extra-wide" @close="historyPlan = null">
      <div class="space-y-4">
        <div class="flex flex-wrap justify-between gap-3"><select v-model="historyModel" class="input" @change="loadHistory"><option value="">{{ t('groupMonitor.allModels') }}</option><option v-for="m in historyPlan ? models(historyPlan) : []" :key="m" :value="m">{{ m }}</option></select><div class="flex gap-2"><button class="btn btn-secondary" :disabled="historyLoading" @click="loadHistory">{{ t('groupMonitor.refresh') }}</button><button v-if="admin && historyKind === 'pelican'" class="btn btn-primary" :disabled="busy || (!!historyPlan && isRunning(historyPlan))" @click="historyPlan && run(historyPlan, 'pelican')">{{ t('groupMonitor.pelican') }}</button></div></div>
        <p v-if="historyError" role="alert" class="text-sm text-red-500">{{ historyError }}</p>
        <p v-if="historyKind === 'pelican'" class="text-sm text-gray-500">{{ t('groupMonitor.reviewNote') }}</p>
        <div class="grid gap-4 md:grid-cols-[220px_1fr]">
          <div class="max-h-[640px] space-y-2 overflow-auto"><button v-for="r in history" :key="r.id" class="w-full rounded-xl border p-3 text-left text-xs" :class="selectedRun?.id === r.id ? 'border-primary-400 bg-primary-50 dark:bg-dark-700' : 'border-gray-100 dark:border-dark-700'" @click="selectRun(r)"><span class="block truncate font-mono">{{ r.model }}</span><span class="mt-2 block">{{ date(r.created_at) }}</span><span :class="statusClass(r.status)">{{ statusText(r.status) }}</span><span v-if="historyKind === 'pelican'" class="ml-2">{{ t(`groupMonitor.${detail(r).review || 'unreviewed'}`) }}</span></button><p v-if="!history.length" class="py-4 text-sm text-gray-500">{{ t('groupMonitor.noData') }}</p></div>
          <div v-if="selectedRun" class="min-w-0 space-y-4">
            <div v-if="admin" class="space-y-2 rounded-xl bg-gray-50 p-4 text-xs dark:bg-dark-900">
              <p>{{ t('groupMonitor.account') }}: {{ selectedRun.detail?.account_name || '—' }} <span v-if="selectedRun.detail?.account_id">#{{ selectedRun.detail.account_id }}</span></p>
              <p class="break-all">{{ t('groupMonitor.messageId') }}: <code>{{ selectedRun.detail?.upstream_message_id || t('groupMonitor.notReturned') }}</code></p>
              <p class="break-all">{{ t('groupMonitor.requestId') }}: <code>{{ selectedRun.detail?.upstream_request_id || t('groupMonitor.notReturned') }}</code></p>
              <p class="break-all">{{ t('groupMonitor.traceId') }}: <code>{{ selectedRun.detail?.trace_id || '—' }}</code></p>
              <p>{{ selectedRun.model }} → {{ selectedRun.detail?.mapped_model || '—' }} · {{ t('groupMonitor.latency') }} {{ latency(selectedRun) }} · {{ t('groupMonitor.tokens') }} {{ selectedRun.detail?.input_tokens ?? '—' }} / {{ selectedRun.detail?.output_tokens ?? '—' }}</p>
              <p v-if="selectedRun.detail?.error" class="break-words text-red-500">{{ selectedRun.detail.error }}</p>
              <button class="btn btn-secondary btn-sm" @click="copyFeedback">{{ t('groupMonitor.feedback') }}</button>
            </div>
            <div v-if="historyKind === 'pelican'" class="space-y-3">
              <div v-if="admin" class="flex flex-wrap items-center gap-2 text-sm"><label for="pelican-review">{{ t('groupMonitor.review') }}</label><select id="pelican-review" class="input" :value="detail(selectedRun).review || 'unreviewed'" :disabled="busy || selectedRun.status === 'running'" @change="review(($event.target as HTMLSelectElement).value)"><option v-for="value in reviewValues" :key="value" :value="value">{{ t(`groupMonitor.${value}`) }}</option></select></div>
              <iframe v-if="detail(selectedRun).html" :key="selectedRun.id" :title="t('groupMonitor.preview')" :srcdoc="buildPelicanPreviewSource(detail(selectedRun).html || '')" sandbox="allow-scripts" referrerpolicy="no-referrer" class="h-[440px] w-full rounded-xl border bg-white" />
              <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('groupMonitor.noPreview') }}</p>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getAll, getModelAllowlistCandidates } from '@/api/admin/groups'
import { groupMonitorAPI, type GroupMonitor, type GroupMonitorConfig, type MonitorRun, type MonitorRunDetail } from '@/api/groupMonitor'
import type { AdminGroup } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildPelicanPreviewSource } from '@/utils/pelicanPreview'
const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { t } = useI18n()
const plans = ref<GroupMonitor[]>([]), groups = ref<AdminGroup[]>([]), selectedGroups = ref<number[]>([])
const loading = ref(false), busy = ref(false), showConfig = ref(false), error = ref(''), dialogError = ref(''), notice = ref('')
const editingPlan = ref<GroupMonitor>(), modelDraft = ref(''), candidates = ref<string[]>([])
const config = ref<GroupMonitorConfig>(defaultConfig())
const historyPlan = ref<GroupMonitor | null>(null), historyKind = ref<'probe' | 'pelican'>('pelican'), historyModel = ref('')
const history = ref<MonitorRun[]>([]), selectedRun = ref<MonitorRun>(), historyLoading = ref(false), historyError = ref('')
const reviewValues = ['unreviewed', 'pass', 'fail', 'uncertain'] as const
const numericFields = [
  { key: 'probe_interval_seconds', label: 'probeInterval', min: 60, max: 86400 },
  { key: 'pelican_interval_seconds', label: 'pelicanInterval', min: 600, max: 604800 },
  { key: 'daily_request_limit', label: 'dailyLimit', min: 1, max: 10000 },
  { key: 'keep_history', label: 'keep', min: 10, max: 500 }
] as const
const modelNames = computed(() => [...new Set(modelDraft.value.split(/[\n,，]/).map(m => m.trim()).filter(Boolean))])
const estimate = computed(() => Math.ceil(selectedGroups.value.length * Math.min(config.value.daily_request_limit, modelNames.value.length * (86400 / Math.max(60, config.value.probe_interval_seconds) + (config.value.pelican_enabled ? 86400 / Math.max(600, config.value.pelican_interval_seconds) : 0)))))
function defaultConfig(): GroupMonitorConfig { return { models: [], enabled: false, probe_interval_seconds: 300, pelican_enabled: false, pelican_interval_seconds: 3600, pelican_prompt: t('groupMonitor.defaultPrompt'), daily_request_limit: 500, keep_history: 100 } }
function models(p: GroupMonitor) { return p.config?.models ?? p.models ?? [] }
function latest(p: GroupMonitor, model: string) { return p.latest_runs.find(r => r.kind === 'probe' && r.model === model) }
function timeline(p: GroupMonitor, model: string) { return p.latest_runs.filter(r => r.kind === 'probe' && r.model === model).slice(0, 30).reverse() }
function detail(r: MonitorRun): MonitorRunDetail { return r.detail ?? { latency_ms: r.latency_ms ?? 0, ttft_ms: r.ttft_ms, html: r.html, review: r.review ?? 'unreviewed' } }
function latency(r?: MonitorRun) { return r && r.status !== 'running' ? `${(detail(r).latency_ms / 1000).toFixed(1)}s` : '—' }
function firstContent(r?: MonitorRun) { const ms = r ? detail(r).ttft_ms : undefined; return ms == null ? '—' : `${(ms / 1000).toFixed(1)}s` }
function isRunning(p: GroupMonitor) { return !!p.lease_until && new Date(p.lease_until).getTime() > Date.now() }
function statusText(status?: string) { return t(`groupMonitor.${status === 'success' ? 'success' : status === 'failed' ? 'failed' : status === 'running' ? 'running' : 'noData'}`) }
function statusClass(status?: string) { return status === 'success' ? 'text-emerald-600' : status === 'failed' ? 'text-rose-500' : 'text-gray-400' }
function date(value?: string) { return value ? new Date(value).toLocaleString() : '—' }
async function load() { if (loading.value) return; loading.value = true; error.value = ''; try { plans.value = await groupMonitorAPI.list(props.admin) } catch (e) { error.value = extractApiErrorMessage(e, t('common.error')) } finally { loading.value = false } }
async function configure(plan?: GroupMonitor) {
  editingPlan.value = plan; selectedGroups.value = plan ? [plan.group_id] : []; config.value = plan?.config ? JSON.parse(JSON.stringify(plan.config)) : defaultConfig(); modelDraft.value = config.value.models.join('\n'); candidates.value = []; dialogError.value = ''; showConfig.value = true
  try { groups.value = (await getAll()).filter(g => g.platform !== 'composite' && g.group_role !== 'self_hosted_pool') } catch (e) { dialogError.value = extractApiErrorMessage(e, t('common.error')) }
}
function toggleModel(model: string) { const names = modelNames.value; modelDraft.value = (names.includes(model) ? names.filter(m => m !== model) : [...names, model]).join('\n') }
async function loadCandidates() { busy.value = true; try { candidates.value = [...new Set((await Promise.all(selectedGroups.value.map(id => getModelAllowlistCandidates(id)))).flat())] } catch (e) { dialogError.value = extractApiErrorMessage(e, t('common.error')) } finally { busy.value = false } }
async function save() { busy.value = true; dialogError.value = ''; try { await groupMonitorAPI.save(selectedGroups.value, { ...config.value, models: modelNames.value }); showConfig.value = false; await load() } catch (e) { dialogError.value = extractApiErrorMessage(e, t('common.error')) } finally { busy.value = false } }
async function remove() { if (!editingPlan.value || !window.confirm(t('groupMonitor.deleteConfirm'))) return; busy.value = true; try { await groupMonitorAPI.remove(editingPlan.value.id); showConfig.value = false; await load() } catch (e) { dialogError.value = extractApiErrorMessage(e, t('common.error')) } finally { busy.value = false } }
async function run(plan: GroupMonitor, kind: string) { busy.value = true; error.value = ''; historyError.value = ''; try { await groupMonitorAPI.run(plan.id, kind); notice.value = t('groupMonitor.runStarted'); await load(); if (historyPlan.value) await loadHistory() } catch (e) { const msg = extractApiErrorMessage(e, t('common.error')); if (historyPlan.value) historyError.value = msg; else error.value = msg } finally { busy.value = false } }
async function openHistory(plan: GroupMonitor, kind: 'probe' | 'pelican') { historyPlan.value = plan; historyKind.value = kind; historyModel.value = ''; history.value = []; selectedRun.value = undefined; await loadHistory() }
let historyRequest = 0
async function loadHistory() {
  if (!historyPlan.value) return
  const request = ++historyRequest; historyLoading.value = true; historyError.value = ''
  try { const result = await groupMonitorAPI.history(props.admin, historyPlan.value.id, historyKind.value, historyModel.value); if (request !== historyRequest) return; history.value = result; const next = result.find(r => r.id === selectedRun.value?.id) ?? result[0]; if (!next) { selectedRun.value = undefined; return }; if (next.id === selectedRun.value?.id && next.status === selectedRun.value?.status && next.status !== 'running') { if (next.detail) next.detail.html = selectedRun.value?.detail?.html; else next.html = selectedRun.value?.html; selectedRun.value = next } else { await selectRun(next) } }
  catch (e) { if (request === historyRequest) historyError.value = extractApiErrorMessage(e, t('common.error')) }
  finally { if (request === historyRequest) historyLoading.value = false }
}
async function selectRun(run: MonitorRun) {
 selectedRun.value = run
 if (!historyPlan.value || run.kind !== 'pelican' || run.status === 'running' || detail(run).html) return
 const planID = historyPlan.value.id
 try { const full = await groupMonitorAPI.getRun(props.admin, planID, run.id); if (historyPlan.value?.id === planID && selectedRun.value?.id === run.id) selectedRun.value = full } catch (e) { historyError.value = extractApiErrorMessage(e, t('common.error')) }
}
async function review(value: string) { if (!historyPlan.value || !selectedRun.value) return; busy.value = true; try { await groupMonitorAPI.review(historyPlan.value.id, selectedRun.value.id, value); await loadHistory() } catch (e) { historyError.value = extractApiErrorMessage(e, t('common.error')) } finally { busy.value = false } }
async function copyFeedback() { if (!selectedRun.value || !historyPlan.value) return; const r = selectedRun.value, d = detail(r); const text = [`${historyPlan.value.group_name} · ${r.kind} · ${date(r.created_at)}`, `model: ${r.model} → ${d.mapped_model || '—'}`, `account: ${d.account_name || '—'} #${d.account_id || '—'}`, `status: ${r.status} · review: ${d.review}`, `message/response ID: ${d.upstream_message_id || t('groupMonitor.notReturned')}`, `upstream request ID: ${d.upstream_request_id || t('groupMonitor.notReturned')}`, `trace ID: ${d.trace_id || '—'}`, `latency: ${d.latency_ms}ms`].join('\n'); try { await navigator.clipboard.writeText(text); notice.value = t('groupMonitor.copied') } catch (e) { historyError.value = extractApiErrorMessage(e, t('common.error')) } }
let timer: ReturnType<typeof setInterval>
onMounted(() => { void load(); timer = setInterval(() => { if (!document.hidden && !busy.value && !showConfig.value) { void load(); if (historyPlan.value && !historyLoading.value) void loadHistory() } }, 15000) })
onBeforeUnmount(() => { clearInterval(timer); historyRequest++ })
</script>
