<template>
  <form class="space-y-6" @submit.prevent="submit">
    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.runtime') }}</h3>
      <div class="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <label class="flex items-center justify-between gap-4 md:col-span-2 xl:col-span-1">
          <span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.globalScheduler') }}</span>
          <Toggle :modelValue="form.enabled" @update:modelValue="form.enabled = $event" />
        </label>
        <div><label class="input-label" for="scheduler-mode">{{ t('admin.openaiAutoScheduler.settings.mode') }}</label><select id="scheduler-mode" v-model="form.mode" class="input"><option value="legacy">{{ t('admin.openaiAutoScheduler.modes.legacy') }}</option><option value="balanced">{{ t('admin.openaiAutoScheduler.modes.balanced') }}</option></select></div>
        <label class="flex items-center justify-between gap-4 md:col-span-2 xl:col-span-1">
          <span class="input-label mb-0">{{ t('admin.openaiAutoScheduler.settings.shadowMode') }}</span>
          <Toggle :modelValue="form.shadow_mode" @update:modelValue="form.shadow_mode = $event" />
        </label>
      </div>
    </section>

    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.settings.balancedSelection') }}</h3>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <NumberField id="scheduler-top-k" v-model="form.top_k" :label="t('admin.openaiAutoScheduler.settings.topK')" :min="1" :max="10" />
        <NumberField id="scheduler-exploration" v-model="form.exploration_rate" :label="t('admin.openaiAutoScheduler.settings.explorationRate')" :min="0" :max="0.1" :step="0.01" />
        <NumberField id="scheduler-session-gap" v-model="form.session_escape_min_gap_ms" :label="t('admin.openaiAutoScheduler.settings.sessionEscapeGap')" :min="0" :max="30000" />
        <NumberField id="scheduler-session-ratio" v-model="form.session_escape_ratio" :label="t('admin.openaiAutoScheduler.settings.sessionEscapeRatio')" :min="0" :max="2" :step="0.05" />
        <NumberField id="scheduler-cost-weight" v-model="form.cost_weight" :label="t('admin.openaiAutoScheduler.settings.costWeight')" :min="0" :max="1" :step="0.05" />
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
    exploration_rate: value.exploration_rate ?? 0.03,
    session_escape_min_gap_ms: value.session_escape_min_gap_ms ?? 1000,
    session_escape_ratio: value.session_escape_ratio ?? 0.25,
    health_ttl_seconds: value.health_ttl_seconds ?? 1800,
    real_sample_fresh_seconds: value.real_sample_fresh_seconds ?? 300,
    probe_jitter_seconds: value.probe_jitter_seconds ?? 0,
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
    exploration_rate: Number(form.exploration_rate),
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
  })
}

function validate(): string {
  if (!form.probe_model.trim()) return t('admin.openaiAutoScheduler.settings.errors.probeModelRequired')
  if (form.top_k < 1 || form.top_k > 10) return t('admin.openaiAutoScheduler.settings.errors.topKRange')
  if (form.exploration_rate < 0 || form.exploration_rate > 0.1) return t('admin.openaiAutoScheduler.settings.errors.explorationRange')
  if (form.session_escape_min_gap_ms < 0 || form.session_escape_min_gap_ms > 30000) return t('admin.openaiAutoScheduler.settings.errors.sessionGapRange')
  if (form.session_escape_ratio < 0 || form.session_escape_ratio > 2) return t('admin.openaiAutoScheduler.settings.errors.sessionRatioRange')
  if (form.cost_weight < 0 || form.cost_weight > 1) return t('admin.openaiAutoScheduler.settings.errors.costWeightRange')
  if (form.probe_interval_seconds <= 0 || form.slow_threshold_ms <= 0 || form.consecutive_slow_breaker_threshold <= 0 || form.consecutive_error_breaker_threshold <= 0 || form.cooldown_seconds <= 0 || form.half_open_success_threshold <= 0 || form.recovery_step <= 0) return t('admin.openaiAutoScheduler.settings.errors.positiveRequired')
  if (form.severe_slow_threshold_ms < form.slow_threshold_ms) return t('admin.openaiAutoScheduler.settings.errors.severeThreshold')
  if (form.probe_jitter_seconds > form.probe_interval_seconds / 2) return t('admin.openaiAutoScheduler.settings.errors.jitterRange')
  if (form.health_ttl_seconds < 60 || form.health_ttl_seconds > 86400) return t('admin.openaiAutoScheduler.settings.errors.healthTTLRange')
  if (form.real_sample_fresh_seconds < 30 || form.real_sample_fresh_seconds > 3600) return t('admin.openaiAutoScheduler.settings.errors.realFreshnessRange')
  return ''
}
</script>
