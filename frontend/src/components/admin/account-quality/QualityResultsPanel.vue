<template>
  <div class="space-y-5">
    <section class="rounded-2xl bg-white p-5 ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('openAIQuality.resultsHeading') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.recentScope') }}</p></div>
        <div class="flex flex-wrap gap-2">
          <Select v-model="sourceGroup" class="w-44" :options="groupOptions" searchable :aria-label="t('openAIQuality.sourceGroupFilter')" />
          <input v-model="model" class="input w-44" :placeholder="t('openAIQuality.exactModel')" :aria-label="t('openAIQuality.model')" @keyup.enter="refresh" />
          <button class="btn btn-secondary btn-sm" :disabled="busy" @click="refresh">{{ t('openAIQuality.queryRecords') }}</button>
        </div>
      </div>
      <div class="mt-5 grid grid-cols-2 gap-4 border-y border-gray-100 py-5 sm:grid-cols-4 dark:border-dark-700">
        <div><p class="text-xs text-gray-500">{{ t('openAIQuality.passRate') }}</p><p class="mt-1 font-mono text-3xl font-semibold text-emerald-600 dark:text-emerald-400">{{ passRate }}</p></div>
        <div><p class="text-xs text-gray-500">{{ t('openAIQuality.passed') }}</p><p class="mt-1 font-mono text-3xl font-semibold">{{ passed }}<span class="ml-1 text-sm font-normal text-gray-400">/ {{ recent.length }}</span></p></div>
        <div><p class="text-xs text-gray-500">{{ t('openAIQuality.failedChecks') }}</p><p class="mt-1 font-mono text-3xl font-semibold text-red-500">{{ incorrect }}</p></div>
        <div><p class="text-xs text-gray-500">{{ t('openAIQuality.noVerdict') }}</p><p class="mt-1 font-mono text-3xl font-semibold text-gray-400">{{ recent.length - passed - incorrect }}</p></div>
      </div>
      <div class="mt-4 flex items-center justify-between text-xs"><h3 class="font-medium">{{ t('openAIQuality.recentChecks') }}</h3><span class="text-gray-400">{{ t('openAIQuality.oldToNew') }}</span></div>
      <div class="mt-3 grid grid-cols-[repeat(20,minmax(0,1fr))] gap-1.5 sm:grid-cols-[repeat(30,minmax(0,1fr))] xl:grid-cols-[repeat(60,minmax(0,1fr))]" :aria-label="t('openAIQuality.recentChecks')">
        <button v-for="run in [...recent].reverse()" :key="run.id" class="h-7 rounded-sm ring-offset-2 transition-opacity hover:opacity-70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500" :class="tileClass(run.verdict || run.status)" :title="runLabel(run)" :aria-label="runLabel(run)" @click="$emit('open', run.id)" />
        <span v-if="!recent.length" class="col-span-full py-4 text-center text-sm text-gray-400">{{ busy ? t('common.loading') : t('openAIQuality.noRecords') }}</span>
      </div>
      <div class="mt-3 flex flex-wrap gap-4 text-xs text-gray-500"><span v-for="state in ['passed','incorrect','unknown','error','running']" :key="state" class="flex items-center gap-1.5"><i class="h-2 w-2 rounded-sm" :class="tileClass(state)" />{{ label(state) }}</span></div>
    </section>

    <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }} <button class="underline" @click="refresh">{{ t('common.refresh') }}</button></p>
    <div class="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
      <section class="min-w-0">
        <div class="mb-3 flex items-center justify-between"><div><h2 class="text-base font-semibold">{{ t('openAIQuality.pelicanGallery') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.pelicanGalleryHelp') }}</p></div><span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-500 dark:bg-dark-700">{{ galleryTotal }}</span></div>
        <div v-if="busy && !gallery.length" class="grid gap-4 sm:grid-cols-2"><div v-for="n in 4" :key="n" class="h-64 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-700" /></div>
        <div v-else-if="!gallery.length" class="flex min-h-80 flex-col items-center justify-center rounded-2xl border border-dashed border-gray-200 px-6 text-center dark:border-dark-600"><Icon name="sparkles" size="xl" class="mb-4 text-gray-300" /><h3 class="text-sm font-medium">{{ t('openAIQuality.noPelicans') }}</h3><p class="mt-2 max-w-sm text-xs leading-relaxed text-gray-500">{{ t('openAIQuality.noPelicansHelp') }}</p><router-link class="btn btn-secondary btn-sm mt-4" to="/admin/openai-quality/settings">{{ t('openAIQuality.configureDual') }}</router-link></div>
        <div v-else class="grid gap-4 sm:grid-cols-2">
          <button v-for="run in gallery" :key="run.id" class="group overflow-hidden rounded-2xl bg-white text-left ring-1 ring-gray-900/5 transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:bg-dark-800 dark:ring-dark-700" :aria-label="runLabel(run)" @click="$emit('open', run.id)">
            <div class="relative aspect-[3/2] overflow-hidden bg-gray-50 dark:bg-dark-900">
              <QualitySVGPreview v-if="run.evidence?.svg" :svg="run.evidence.svg" :alt="t('openAIQuality.pelicanArtwork')" />
              <div v-else class="flex h-full flex-col items-center justify-center gap-2 p-6 text-center"><Icon name="sparkles" size="lg" class="text-gray-300" /><p class="text-xs text-gray-500">{{ placeholder(run) }}</p></div>
              <span class="absolute -right-9 top-5 w-36 rotate-45 py-1.5 text-center text-xs font-semibold shadow-sm" :class="ribbonClass(pelicanOutcome(run))">{{ artworkLabel(pelicanOutcome(run)) }}</span>
              <span v-if="run.evidence?.svg" class="absolute bottom-2 left-2 rounded-md bg-white/90 px-2 py-1 text-[10px] font-medium text-gray-500">SVG · {{ t('openAIQuality.animation') }}</span>
            </div>
            <div class="p-4"><div class="flex items-center justify-between gap-2"><p class="truncate text-sm font-semibold">{{ run.input_snapshot?.account_name || `#${run.account_id}` }}</p><span class="shrink-0 text-[11px] text-gray-400">{{ date(run.started_at || run.queued_at) }}</span></div><p class="mt-1 truncate text-xs text-gray-500">{{ run.input_snapshot?.condition.model_id }} · {{ run.input_snapshot?.source_group_name || groupName(run) }}</p><div class="mt-3 flex justify-between border-t border-gray-100 pt-2.5 font-mono text-[11px] text-gray-400 dark:border-dark-700"><span>{{ seconds(run.result.pelican?.latency_ms) }} s</span><span>{{ tokens(run.result.pelican?.total_tokens) }} Token</span><span>{{ cost(run.result.pelican?.estimated_cost_usd) }}</span></div></div>
          </button>
        </div>
        <div v-if="galleryTotal > 12" class="mt-4 flex items-center justify-center gap-4 text-xs"><button class="btn btn-secondary btn-sm" :disabled="galleryPage === 1 || busy" @click="changePage(-1)">{{ t('openAIQuality.prevPage') }}</button><span>{{ galleryPage }} / {{ Math.ceil(galleryTotal / 12) }}</span><button class="btn btn-secondary btn-sm" :disabled="galleryPage * 12 >= galleryTotal || busy" @click="changePage(1)">{{ t('openAIQuality.nextPage') }}</button></div>
      </section>
      <aside class="min-w-0 rounded-2xl bg-white p-4 ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="mb-4 flex items-center justify-between"><h2 class="font-semibold">{{ t('openAIQuality.candyResults') }}</h2><router-link class="text-xs text-primary-600 dark:text-primary-400" to="/admin/openai-quality/records">{{ t('openAIQuality.showAllRecords') }} →</router-link></div>
        <div v-if="!recent.length" class="py-12 text-center text-sm text-gray-400">{{ t('openAIQuality.noRecords') }}</div>
        <div class="max-h-[1160px] space-y-3 overflow-y-auto pr-1">
          <button v-for="run in recent.slice(0, 20)" :key="run.id" class="w-full rounded-xl border p-3 text-left transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700" :class="candy(run).verdict === 'incorrect' ? 'border-red-200 dark:border-red-900' : 'border-gray-100 dark:border-dark-700'" @click="$emit('open', run.id)">
            <div class="flex items-center justify-between gap-2"><span class="rounded-md px-2 py-0.5 text-xs font-semibold" :class="qualityOutcomeClass(candy(run).verdict)">{{ label(candy(run).verdict) }}</span><span class="font-mono text-xs text-gray-500">{{ candy(run).final_answer ?? '—' }} / 21</span></div>
            <p class="mt-2.5 truncate text-sm font-medium">{{ run.input_snapshot?.account_name || `#${run.account_id}` }}</p><p class="mt-1 truncate text-[11px] text-gray-500">{{ run.input_snapshot?.condition.model_id }} · {{ run.input_snapshot?.condition.benchmark_id === 'dual' ? t('openAIQuality.dualCheck') : t('openAIQuality.candyOnly') }}</p>
            <div class="mt-2 flex items-center justify-between text-[11px] text-gray-400"><time>{{ date(run.started_at || run.queued_at) }}</time><span class="font-mono">{{ seconds(candy(run).latency_ms) }} s</span></div>
            <div class="mt-2 flex justify-between border-t border-gray-100 pt-2 font-mono text-[10px] text-gray-500 dark:border-dark-700"><span>{{ tokens(candy(run).total_tokens) }} Token</span><span>{{ candy(run).tokens_per_second?.toFixed(1) ?? '—' }} TPS</span><span>{{ cost(candy(run).estimated_cost_usd) }}</span></div>
          </button>
        </div>
      </aside>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { getQualityGallery, getQualityRuns, type QualityCheck, type QualityRun } from '@/api/admin/openaiAccountQuality'
import { extractApiErrorMessage } from '@/utils/apiError'
import QualitySVGPreview from './QualitySVGPreview.vue'
import { qualityOutcomeClass } from './qualityPresentation'
const props = defineProps<{ groups: { id: number; name: string; platform: string; group_role?: string }[]; accountId?: number; revision?: number }>()
defineEmits<{ (event: 'open', id: number): void }>()
const { t, te } = useI18n()
const recent = ref<QualityRun[]>([]), gallery = ref<QualityRun[]>([])
const galleryTotal = ref(0), galleryPage = ref(1), sourceGroup = ref(''), model = ref(''), busy = ref(false), error = ref('')
let request = 0
let activeScope = ''
const groupOptions = computed(() => [{ value: '', label: t('openAIQuality.allSourceGroups') }, ...props.groups.filter(g => g.platform === 'openai' && g.group_role !== 'self_hosted_pool').map(g => ({ value: String(g.id), label: g.name }))])
const passed = computed(() => recent.value.filter(r => r.verdict === 'passed').length)
const incorrect = computed(() => recent.value.filter(r => r.verdict === 'incorrect').length)
const passRate = computed(() => passed.value + incorrect.value ? `${(100 * passed.value / (passed.value + incorrect.value)).toFixed(1)}%` : '—')
function label(value: string) { return value === 'incorrect' ? t('openAIQuality.failedChecks') : te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
function candy(run: QualityRun): QualityCheck {
  if (run.result.candy) return run.result.candy
  if (run.input_snapshot?.condition.benchmark_id === 'dual') return { verdict: ['queued', 'running'].includes(run.status) ? run.status : 'unknown', reason_code: 'dual_incomplete', latency_ms: 0, input_tokens: null, output_tokens: null, total_tokens: null, estimated_cost_usd: null, response_truncated: false }
  return { ...run.result, verdict: run.verdict || run.status }
}
function pelicanOutcome(run: QualityRun) { return run.result.pelican?.verdict || run.status }
function ribbonClass(value: string) { return value === 'passed' ? 'bg-emerald-500 text-white' : value === 'incorrect' ? 'bg-red-500 text-white' : value === 'unknown' ? 'bg-amber-500 text-white' : 'bg-gray-500 text-white' }
function artworkLabel(value: string) { return value === 'incorrect' ? t('openAIQuality.degraded') : label(value) }
function date(value: string) { return new Date(value).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function groupName(run: QualityRun) { return props.groups.find(g => g.id === run.input_snapshot?.source_group_id)?.name || t('openAIQuality.unavailableGroup') }
function seconds(ms?: number) { return ms == null ? '—' : (ms / 1000).toFixed(1) }
function tokens(value?: number | null) { return value == null ? '—' : value.toLocaleString() }
function cost(value?: number | null) { return value == null ? '—' : `$${value.toFixed(4)}` }
function runLabel(run: QualityRun) { return `${run.input_snapshot?.account_name} · ${run.input_snapshot?.condition.model_id} · ${date(run.started_at || run.queued_at)} · ${label(run.verdict || run.status)}` }
function placeholder(run: QualityRun) { return run.response_expires_at && Date.parse(run.response_expires_at) <= Date.now() ? t('openAIQuality.artworkExpired') : label(run.result.pelican?.reason_code || run.reason_code || run.status) }
function tileClass(value: string) { return value === 'passed' ? 'bg-emerald-500' : value === 'incorrect' ? 'bg-red-500' : value === 'unknown' ? 'bg-amber-400' : ['running', 'queued'].includes(value) ? 'bg-primary-400' : 'bg-gray-300 dark:bg-dark-500' }
async function refresh() {
  const current = ++request
  busy.value = true; error.value = ''
  const scope = JSON.stringify([props.accountId, sourceGroup.value, model.value.trim()])
  if (scope !== activeScope) {
    activeScope = scope; galleryPage.value = 1
    recent.value = []; gallery.value = []; galleryTotal.value = 0
  }
  const until = new Date()
  const query = { ...(props.accountId ? { account_id: props.accountId } : {}), ...(sourceGroup.value ? { source_group_id: Number(sourceGroup.value) } : {}), ...(model.value.trim() ? { model_id: model.value.trim() } : {}), from: new Date(until.getTime() - 180 * 86400000).toISOString(), to: until.toISOString() }
  try {
    const [r, g] = await Promise.all([getQualityRuns({ ...query, page_size: 60 }), getQualityGallery({ ...query, page_size: 12, page: galleryPage.value })])
    if (current !== request) return
    recent.value = r.items; gallery.value = g.items; galleryTotal.value = g.total
  } catch (e) { if (current === request) error.value = extractApiErrorMessage(e, t('common.error')) }
  finally { if (current === request) busy.value = false }
}
function changePage(delta: number) { galleryPage.value += delta; void refresh() }
watch(sourceGroup, () => { void refresh() })
watch(() => [props.accountId, props.revision], () => { void refresh() })
onMounted(refresh)
onDeactivated(() => { ++request; busy.value = false })
onBeforeUnmount(() => { ++request })
</script>
