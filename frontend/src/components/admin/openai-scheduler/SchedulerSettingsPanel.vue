<template>
  <form class="space-y-6" @submit.prevent="submit">
    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.runtime') }}</h3>
      <div class="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <label class="flex items-center justify-between gap-4 md:col-span-2 xl:col-span-1">
          <span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.globalScheduler') }}</span>
          <Toggle :modelValue="form.enabled" @update:modelValue="form.enabled = $event" />
        </label>
        <div><label class="input-label" for="scheduler-mode">{{ t('admin.openaiAutoScheduler.settings.mode') }}</label><select id="scheduler-mode" v-model="form.mode" class="input" @change="applyModePreset"><option value="legacy">{{ t('admin.openaiAutoScheduler.modes.legacy') }}</option><option value="balanced">{{ t('admin.openaiAutoScheduler.modes.balanced') }}</option><option value="performance_first">{{ t('admin.openaiAutoScheduler.modes.performance') }}</option><option value="cost_first">{{ t('admin.openaiAutoScheduler.modes.cost') }}</option><option value="efficiency">{{ t('admin.openaiAutoScheduler.modes.efficiency') }}</option></select></div>
        <label class="flex items-center justify-between gap-4 md:col-span-2 xl:col-span-1">
          <span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.shadowMode') }}</span>
          <Toggle :modelValue="form.shadow_mode" @update:modelValue="form.shadow_mode = $event" />
        </label>
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.firstOutput') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2">
        <label class="flex items-center justify-between gap-4 sm:col-span-2">
          <span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.earlySSEPreambleFlush') }}</span>
          <Toggle :modelValue="form.early_sse_preamble_flush_enabled" @update:modelValue="form.early_sse_preamble_flush_enabled = $event" />
        </label>
        <NumberField id="scheduler-first-output-timeout" v-model="form.first_output_timeout_seconds" :label="t('admin.openaiAutoScheduler.settings.firstOutputTimeout')" :min="0" :max="600" />
        <NumberField id="scheduler-high-effort-first-output-timeout" v-model="form.high_effort_first_output_timeout_seconds" :label="t('admin.openaiAutoScheduler.settings.highEffortFirstOutputTimeout')" :min="0" :max="1800" />
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.balancedSelection') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <NumberField id="scheduler-top-k" v-model="form.top_k" :label="t('admin.openaiAutoScheduler.settings.topK')" :min="1" :max="10" />
        <label class="flex items-center justify-between gap-4"><span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.adaptiveTopK') }}</span><Toggle :modelValue="form.adaptive_top_k_enabled" @update:modelValue="form.adaptive_top_k_enabled = $event" /></label>
        <NumberField id="scheduler-exploration" v-model="form.exploration_rate" :label="t('admin.openaiAutoScheduler.settings.explorationRate')" :min="0" :max="0.1" :step="0.01" />
        <NumberField id="scheduler-exploration-budget" v-model="form.exploration_budget" :label="t('admin.openaiAutoScheduler.settings.explorationBudget')" :min="0" :max="0.1" :step="0.01" />
        <NumberField id="scheduler-exploration-interval" v-model="form.exploration_min_interval_seconds" :label="t('admin.openaiAutoScheduler.settings.explorationMinInterval')" :min="30" :max="3600" />
        <NumberField id="scheduler-exploration-max-hour" v-model="form.exploration_max_real_samples_per_hour" :label="t('admin.openaiAutoScheduler.settings.explorationMaxPerHour')" :min="1" :max="60" />
        <label class="flex items-center justify-between gap-4"><span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.staleOpenRequiresProbe') }}</span><Toggle :modelValue="form.stale_open_requires_probe" @update:modelValue="form.stale_open_requires_probe = $event" /></label>
        <NumberField id="scheduler-session-gap" v-model="form.session_escape_min_gap_ms" :label="t('admin.openaiAutoScheduler.settings.sessionEscapeGap')" :min="0" :max="30000" />
        <NumberField id="scheduler-session-ratio" v-model="form.session_escape_ratio" :label="t('admin.openaiAutoScheduler.settings.sessionEscapeRatio')" :min="0" :max="2" :step="0.05" />
        <NumberField id="scheduler-cost-weight" v-model="form.cost_weight" :label="t('admin.openaiAutoScheduler.settings.costWeight')" :min="0" :max="1" :step="0.05" />
        <NumberField id="scheduler-temperature" v-model="form.temperature" :label="t('admin.openaiAutoScheduler.settings.temperature')" :min="0.01" :max="1" :step="0.01" />
        <NumberField id="scheduler-max-share" v-model="form.max_account_share" :label="t('admin.openaiAutoScheduler.settings.maxAccountShare')" :min="0.01" :max="1" :step="0.05" />
        <NumberField id="scheduler-low-confidence-share" v-model="form.low_confidence_max_share" :label="t('admin.openaiAutoScheduler.settings.lowConfidenceShare')" :min="0.01" :max="1" :step="0.05" />
        <NumberField id="scheduler-latency-budget" v-model="form.latency_budget_ms" :label="t('admin.openaiAutoScheduler.settings.latencyBudget')" :min="1" :max="30000" />
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.policyWeights') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <NumberField id="scheduler-weight-latency" v-model="form.weights.latency" :label="t('admin.openaiAutoScheduler.ranking.scores.latency')" :min="0" :max="1" :step="0.01" />
        <NumberField id="scheduler-weight-reliability" v-model="form.weights.reliability" :label="t('admin.openaiAutoScheduler.ranking.scores.reliability')" :min="0" :max="1" :step="0.01" />
        <NumberField id="scheduler-weight-cost" v-model="form.weights.cost" :label="t('admin.openaiAutoScheduler.ranking.scores.cost')" :min="0" :max="1" :step="0.01" />
        <NumberField id="scheduler-weight-capacity" v-model="form.weights.capacity" :label="t('admin.openaiAutoScheduler.ranking.scores.capacity')" :min="0" :max="1" :step="0.01" />
        <NumberField id="scheduler-weight-quota" v-model="form.weights.quota" :label="t('admin.openaiAutoScheduler.ranking.scores.quota')" :min="0" :max="1" :step="0.01" />
        <NumberField id="scheduler-weight-priority" v-model="form.weights.priority" :label="t('admin.openaiAutoScheduler.ranking.scores.priority')" :min="0" :max="1" :step="0.01" />
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.breaker') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <NumberField id="scheduler-slow-threshold" v-model="form.slow_threshold_ms" :label="t('admin.openaiAutoScheduler.settings.slowThreshold')" :min="1" />
        <NumberField id="scheduler-severe-threshold" v-model="form.severe_slow_threshold_ms" :label="t('admin.openaiAutoScheduler.settings.severeThreshold')" :min="1" />
        <NumberField id="scheduler-slow-breaker" v-model="form.consecutive_slow_breaker_threshold" :label="t('admin.openaiAutoScheduler.settings.consecutiveSlow')" :min="1" />
        <NumberField id="scheduler-error-breaker" v-model="form.consecutive_error_breaker_threshold" :label="t('admin.openaiAutoScheduler.settings.consecutiveErrors')" :min="1" />
        <NumberField id="scheduler-cooldown" v-model="form.cooldown_seconds" :label="t('admin.openaiAutoScheduler.settings.cooldown')" :min="1" />
        <NumberField id="scheduler-half-open" v-model="form.half_open_success_threshold" :label="t('admin.openaiAutoScheduler.settings.recoverySamples')" :min="1" />
        <NumberField id="scheduler-recovery-step" v-model="form.recovery_step" :label="t('admin.openaiAutoScheduler.settings.recoveryStep')" :min="1" />
      </div>
    </section>

    <section>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.healthProbe') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div><label class="input-label" for="scheduler-probe-model">{{ t('admin.openaiAutoScheduler.settings.probeModel') }}</label><input id="scheduler-probe-model" v-model.trim="form.probe_model" class="input" /></div>
        <NumberField id="scheduler-probe-interval" v-model="form.probe_interval_seconds" :label="t('admin.openaiAutoScheduler.settings.probeInterval')" :min="1" />
        <NumberField id="scheduler-probe-jitter" v-model="form.probe_jitter_seconds" :label="t('admin.openaiAutoScheduler.settings.probeJitter')" :min="0" />
        <NumberField id="scheduler-health-ttl" v-model="form.health_ttl_seconds" :label="t('admin.openaiAutoScheduler.settings.healthTTL')" :min="60" :max="86400" />
        <NumberField id="scheduler-real-fresh" v-model="form.real_sample_fresh_seconds" :label="t('admin.openaiAutoScheduler.settings.realFreshness')" :min="30" :max="3600" />
      </div>
    </section>

    <p v-if="validationError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ validationError }}</p>
    <div class="flex justify-end">
      <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('admin.openaiAutoScheduler.settings.saving') : t('admin.openaiAutoScheduler.settings.save') }}</button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { defineComponent, h, reactive, ref, watch } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import type { OpenAIAutoSchedulerSettings } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ modelValue: OpenAIAutoSchedulerSettings; saving: boolean }>()
const emit = defineEmits<{ (event: 'save', value: OpenAIAutoSchedulerSettings): void }>()
const { t } = useI18n()

const NumberField = defineComponent({
  props: { id: { type: String, required: true }, label: { type: String, required: true }, modelValue: { type: Number, required: true }, min: Number, max: Number, step: Number },
  emits: ['update:modelValue'],
  setup(fieldProps, { emit: fieldEmit }) {
    return () => h('div', [
      h('label', { class: 'input-label', for: fieldProps.id }, fieldProps.label),
      h('input', { id: fieldProps.id, class: 'input', type: 'number', min: fieldProps.min, max: fieldProps.max, step: fieldProps.step ?? 1, value: fieldProps.modelValue, onInput: (event: Event) => fieldEmit('update:modelValue', Number((event.target as HTMLInputElement).value)) }),
    ])
  },
})

const validationError = ref('')
const form = reactive(normalizeSettings(props.modelValue))

watch(() => props.modelValue, (value) => Object.assign(form, normalizeSettings(value)), { deep: true })

function normalizeSettings(value: OpenAIAutoSchedulerSettings) {
  return {
    ...value,
    mode: value.mode || 'balanced',
    shadow_mode: value.shadow_mode ?? true,
    top_k: value.top_k ?? 3,
    adaptive_top_k_enabled: value.adaptive_top_k_enabled ?? true,
    exploration_rate: value.exploration_rate ?? 0.03,
    exploration_budget: value.exploration_budget ?? 0.05,
    exploration_min_interval_seconds: value.exploration_min_interval_seconds ?? 600,
    exploration_max_real_samples_per_hour: value.exploration_max_real_samples_per_hour ?? 6,
    stale_open_requires_probe: value.stale_open_requires_probe ?? true,
    session_escape_min_gap_ms: value.session_escape_min_gap_ms ?? 1000,
    session_escape_ratio: value.session_escape_ratio ?? 0.25,
    health_ttl_seconds: value.health_ttl_seconds ?? 1800,
    real_sample_fresh_seconds: value.real_sample_fresh_seconds ?? 300,
    probe_jitter_seconds: value.probe_jitter_seconds ?? 0,
    temperature: value.temperature ?? 0.18,
    max_account_share: value.max_account_share ?? 0.7,
    low_confidence_max_share: value.low_confidence_max_share ?? 0.1,
    latency_budget_ms: value.latency_budget_ms ?? 1000,
    early_sse_preamble_flush_enabled: value.early_sse_preamble_flush_enabled ?? false,
    first_output_timeout_seconds: value.first_output_timeout_seconds ?? 0,
    high_effort_first_output_timeout_seconds: value.high_effort_first_output_timeout_seconds ?? 0,
    weights: { ...(value.weights || { latency: 0.35, reliability: 0.25, cost: 0.15, capacity: 0.15, quota: 0.05, priority: 0.05 }) },
  }
}

function submit(): void {
  validationError.value = validate()
  if (validationError.value) return
  emit('save', {
    ...form,
    enabled: Boolean(form.enabled),
    mode: form.mode,
    shadow_mode: Boolean(form.shadow_mode),
    probe_model: String(form.probe_model || '').trim(),
    top_k: Number(form.top_k),
    adaptive_top_k_enabled: Boolean(form.adaptive_top_k_enabled),
    exploration_rate: Number(form.exploration_rate),
    exploration_budget: Number(form.exploration_budget),
    exploration_min_interval_seconds: Number(form.exploration_min_interval_seconds),
    exploration_max_real_samples_per_hour: Number(form.exploration_max_real_samples_per_hour),
    stale_open_requires_probe: Boolean(form.stale_open_requires_probe),
    session_escape_min_gap_ms: Number(form.session_escape_min_gap_ms),
    session_escape_ratio: Number(form.session_escape_ratio),
    health_ttl_seconds: Number(form.health_ttl_seconds),
    real_sample_fresh_seconds: Number(form.real_sample_fresh_seconds),
    probe_jitter_seconds: Number(form.probe_jitter_seconds),
    probe_interval_seconds: Number(form.probe_interval_seconds),
    slow_threshold_ms: Number(form.slow_threshold_ms),
    severe_slow_threshold_ms: Number(form.severe_slow_threshold_ms),
    consecutive_slow_breaker_threshold: Number(form.consecutive_slow_breaker_threshold),
    consecutive_error_breaker_threshold: Number(form.consecutive_error_breaker_threshold),
    cooldown_seconds: Number(form.cooldown_seconds),
    half_open_success_threshold: Number(form.half_open_success_threshold),
    cost_weight: Number(form.cost_weight),
    recovery_step: Number(form.recovery_step),
    temperature: Number(form.temperature),
    max_account_share: Number(form.max_account_share),
    low_confidence_max_share: Number(form.low_confidence_max_share),
    latency_budget_ms: Number(form.latency_budget_ms),
    early_sse_preamble_flush_enabled: Boolean(form.early_sse_preamble_flush_enabled),
    first_output_timeout_seconds: Number(form.first_output_timeout_seconds),
    high_effort_first_output_timeout_seconds: Number(form.high_effort_first_output_timeout_seconds),
    weights: {
      latency: Number(form.weights.latency), reliability: Number(form.weights.reliability), cost: Number(form.weights.cost),
      capacity: Number(form.weights.capacity), quota: Number(form.weights.quota), priority: Number(form.weights.priority),
    },
  })
}

function validate(): string {
  if (!form.probe_model.trim()) return t('admin.openaiAutoScheduler.settings.errors.probeModelRequired')
  if (form.top_k < 1 || form.top_k > 10) return t('admin.openaiAutoScheduler.settings.errors.topKRange')
  if (form.exploration_rate < 0 || form.exploration_rate > 0.1) return t('admin.openaiAutoScheduler.settings.errors.explorationRange')
  if (form.exploration_budget < 0 || form.exploration_budget > 0.1 || form.exploration_min_interval_seconds < 30 || form.exploration_min_interval_seconds > 3600) return t('admin.openaiAutoScheduler.settings.errors.explorationPolicyRange')
  if (form.exploration_max_real_samples_per_hour < 1 || form.exploration_max_real_samples_per_hour > 60) return t('admin.openaiAutoScheduler.settings.errors.explorationMaxPerHourRange')
  if (form.session_escape_min_gap_ms < 0 || form.session_escape_min_gap_ms > 30000) return t('admin.openaiAutoScheduler.settings.errors.sessionGapRange')
  if (form.session_escape_ratio < 0 || form.session_escape_ratio > 2) return t('admin.openaiAutoScheduler.settings.errors.sessionRatioRange')
  if (form.cost_weight < 0 || form.cost_weight > 1) return t('admin.openaiAutoScheduler.settings.errors.costWeightRange')
  if (form.temperature <= 0 || form.temperature > 1 || form.max_account_share <= 0 || form.max_account_share > 1 || form.low_confidence_max_share <= 0 || form.low_confidence_max_share > 1 || form.latency_budget_ms <= 0 || form.latency_budget_ms > 30000) return t('admin.openaiAutoScheduler.settings.errors.policyRange')
  if (Object.values(form.weights).some((value) => value < 0 || value > 1) || Object.values(form.weights).reduce((sum, value) => sum + value, 0) <= 0) return t('admin.openaiAutoScheduler.settings.errors.weightsRange')
  if (form.probe_interval_seconds <= 0 || form.slow_threshold_ms <= 0 || form.consecutive_slow_breaker_threshold <= 0 || form.consecutive_error_breaker_threshold <= 0 || form.cooldown_seconds <= 0 || form.half_open_success_threshold <= 0 || form.recovery_step <= 0) return t('admin.openaiAutoScheduler.settings.errors.positiveRequired')
  if (form.severe_slow_threshold_ms < form.slow_threshold_ms) return t('admin.openaiAutoScheduler.settings.errors.severeThreshold')
  if (form.probe_jitter_seconds > form.probe_interval_seconds / 2) return t('admin.openaiAutoScheduler.settings.errors.jitterRange')
  if (form.health_ttl_seconds < 60 || form.health_ttl_seconds > 86400) return t('admin.openaiAutoScheduler.settings.errors.healthTTLRange')
  if (form.real_sample_fresh_seconds < 30 || form.real_sample_fresh_seconds > 3600) return t('admin.openaiAutoScheduler.settings.errors.realFreshnessRange')
  if (form.first_output_timeout_seconds < 0 || form.first_output_timeout_seconds > 600 || (form.first_output_timeout_seconds > 0 && form.first_output_timeout_seconds < 5)) return t('admin.openaiAutoScheduler.settings.errors.firstOutputTimeoutRange')
  if (form.high_effort_first_output_timeout_seconds < 0 || form.high_effort_first_output_timeout_seconds > 1800 || (form.high_effort_first_output_timeout_seconds > 0 && form.high_effort_first_output_timeout_seconds < 30)) return t('admin.openaiAutoScheduler.settings.errors.highEffortFirstOutputTimeoutRange')
  return ''
}

function applyModePreset(): void {
  const presets = {
    balanced: { temperature: 0.18, max: 0.7, weights: { latency: 0.35, reliability: 0.25, cost: 0.15, capacity: 0.15, quota: 0.05, priority: 0.05 } },
    performance_first: { temperature: 0.1, max: 0.85, weights: { latency: 0.55, reliability: 0.25, cost: 0.05, capacity: 0.1, quota: 0.03, priority: 0.02 } },
    cost_first: { temperature: 0.16, max: 0.75, weights: { latency: 0.2, reliability: 0.25, cost: 0.4, capacity: 0.08, quota: 0.04, priority: 0.03 } },
    efficiency: { temperature: 0.14, max: 0.8, weights: { latency: 0.35, reliability: 0.25, cost: 0.25, capacity: 0.08, quota: 0.04, priority: 0.03 } },
  } as const
  const preset = presets[form.mode as keyof typeof presets]
  if (!preset) return
  form.temperature = preset.temperature
  form.max_account_share = preset.max
  Object.assign(form.weights, preset.weights)
}
</script>
