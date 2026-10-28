<template>
  <AppLayout>
    <main class="mx-auto max-w-5xl space-y-5 p-4 lg:p-7">
      <header class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700"><h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ t('openAIQuality.settingsTitle') }}</h1><p class="mt-1 text-sm text-gray-500">{{ t('openAIQuality.settingsDescription') }}</p><p class="mt-3 rounded-xl bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">{{ t('openAIQuality.scopeNote') }}</p></header>
      <form v-if="settings" class="space-y-5" @submit.prevent="save">
        <section class="grid gap-4 rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:grid-cols-2">
          <label class="flex items-center gap-3"><input v-model="settings.automation_enabled" type="checkbox" class="checkbox" /><span><b>{{ t('openAIQuality.automation') }}</b><small class="block text-gray-500">{{ t('openAIQuality.automationHelp') }}</small></span></label>
          <label class="flex items-center gap-3"><input v-model="settings.control_enabled" type="checkbox" class="checkbox" /><span><b>{{ t('openAIQuality.globalControl') }}</b><small class="block text-gray-500">{{ t('openAIQuality.controlHelp') }}</small></span></label>
          <label v-for="field in numberFields" :key="field.key" class="block"><span class="mb-1 block text-sm font-medium">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="settings[field.key]" class="input" type="number" :min="field.min" :max="field.max" /><small v-if="field.key === 'default_policy_daily_call_limit'" class="mt-1 block text-xs text-gray-500">{{ t('openAIQuality.policyDefaultLimitHelp') }}</small></label>
          <div class="sm:col-span-2 flex flex-wrap gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="previewSettings">{{ t('openAIQuality.preview') }}</button><button class="btn btn-primary" :disabled="saving || !settingsPreview?.valid || settingsPreviewFingerprint !== currentSettingsFingerprint()">{{ saving ? t('common.loading') : t('common.save') }}</button><button type="button" class="btn btn-secondary text-red-600" :disabled="saving" @click="emergencyDisable">{{ t('openAIQuality.emergencyDisable') }}</button></div>
          <div v-if="settingsPreview" class="sm:col-span-2 rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-900"><p>{{ settingsPreview.valid ? t('openAIQuality.previewValid') : t('openAIQuality.previewInvalid') }}</p><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.dailyCalls') }}: {{ settingsPreview.daily_calls_estimate }} / {{ settingsPreview.global_budget }}</p><p v-for="warning in settingsPreview.warnings || []" :key="warning.code" class="mt-1 text-amber-700">{{ warning.message }}</p></div>
        </section>
      </form>
      <section v-if="settings" class="flex flex-wrap gap-x-6 gap-y-2 rounded-2xl bg-white px-4 py-3 text-sm shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700"><span>{{ t('openAIQuality.queueDepth') }}: {{ health.queue_depth }}</span><span>{{ t('openAIQuality.runningCount') }}: {{ health.running_count }}</span><span>{{ t('openAIQuality.storeStatus') }}: {{ health.store_status }}</span><span>{{ t('openAIQuality.lastSync') }}: {{ formatDate(health.last_sync_at) }}</span><span>{{ t('openAIQuality.budget') }}: {{ budget.consumed }} / {{ budget.limit }}</span></section>
      <section class="space-y-4 rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div><h2 class="font-semibold">{{ t('openAIQuality.newPolicy') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.policyScopeHelp') }}</p></div>
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="block sm:col-span-2"><span class="mb-1 block text-sm font-medium">{{ t('openAIQuality.sourceGroup') }}</span><select v-model.number="selectedSourceGroupId" class="input"><option :value="0">{{ t('openAIQuality.chooseSourceGroup') }}</option><option v-for="group in sourceGroups" :key="group.id" :value="group.id">{{ group.name }} (#{{ group.id }})</option></select></label>
          <fieldset v-if="selectedSourceGroupId" class="sm:col-span-2"><legend class="mb-2 text-sm font-medium">{{ t('openAIQuality.account') }} · {{ t('openAIQuality.selectedTargets', { count: selectedAccountIds.length }) }}</legend><p class="mb-2 text-xs text-gray-500">{{ t('openAIQuality.batchLimit') }}</p><input v-model="targetSearch" class="input mb-2" :placeholder="t('openAIQuality.searchTarget')" /><div v-if="filteredTargets.length" class="max-h-48 space-y-2 overflow-y-auto rounded-xl border border-gray-200 p-3 dark:border-dark-700"><label v-for="target in filteredTargets" :key="target.account_id" class="flex items-start gap-2 text-sm"><input v-model="selectedAccountIds" class="mt-1" type="checkbox" :value="target.account_id" :disabled="!target.can_probe || (!selectedAccountIds.includes(target.account_id) && selectedAccountIds.length >= maxSelectableAccounts)" /><span>{{ target.account_name }} (#{{ target.account_id }})<small v-if="!target.can_probe" class="block text-amber-700">{{ target.reasons.join(' · ') || target.eligibility }}</small></span></label></div><p v-else class="rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">{{ t('openAIQuality.noAccountsInGroup') }}</p></fieldset>
          <p v-else class="sm:col-span-2 rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">{{ t('openAIQuality.chooseGroupBeforeAccounts') }}</p>
          <fieldset class="sm:col-span-2 space-y-3"><legend class="text-sm font-medium">{{ t('openAIQuality.model') }}</legend><p class="text-xs text-gray-500">{{ t('openAIQuality.modelSelectionHelp') }}</p><div class="grid gap-3 sm:grid-cols-2"><label class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.modelOne') }}</span><input v-model.trim="modelOne" class="input" placeholder="gpt-5.5" /></label><label v-if="modelTwoEnabled" class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.modelTwo') }}</span><div class="flex gap-2"><input v-model.trim="modelTwo" class="input min-w-0 flex-1" placeholder="gpt-5.4" /><button type="button" class="btn btn-ghost shrink-0 px-2 py-1 text-xs" @click="removeSecondModel">{{ t('openAIQuality.removeSecondModel') }}</button></div></label></div><button v-if="!modelTwoEnabled" type="button" class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="selectedAccountIds.length > 50" @click="addSecondModel">{{ t('openAIQuality.addSecondModel') }}</button><p v-if="selectedAccountIds.length" class="text-xs text-gray-500">{{ t('openAIQuality.policyCount', { count: createPolicyCount }) }}</p></fieldset>
          <label class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.protocol') }}</span><select v-model="condition.protocol" class="input"><option value="responses">Responses</option><option value="chat_completions">Chat Completions</option></select></label>
          <label class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.transport') }}</span><select v-model="condition.transport" class="input"><option value="http">HTTP / SSE</option><option v-if="condition.protocol === 'responses'" value="websocket_v2">Responses WebSocket v2</option></select></label>
          <label class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.reasoningEffort') }}</span><select v-model="condition.reasoning_effort" class="input"><option v-for="v in ['upstream_default','none','minimal','low','medium','high','xhigh']" :key="v" :value="v">{{ v }}</option></select></label>
          <label class="block"><span class="mb-1 block text-sm">{{ t('openAIQuality.serviceTier') }}</span><select v-model="condition.service_tier" class="input"><option v-for="v in ['upstream_default','default','priority','flex','auto','scale','ultrafast']" :key="v" :value="v">{{ v }}</option></select></label>
          <label v-for="field in ruleFields" :key="field.key" class="block"><span class="mb-1 block text-sm">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="rules[field.key]" class="input" type="number" :min="field.min" :max="field.max" /></label>
          <label class="flex items-center gap-3"><input v-model="rules.auto_check_enabled" type="checkbox" class="checkbox" />{{ t('openAIQuality.autoCheck') }}</label>
          <label class="flex items-center gap-3"><input v-model="rules.control_enabled" type="checkbox" class="checkbox" :disabled="!canEnforceAll" />{{ t('openAIQuality.controlPolicy') }} <span v-if="!canEnforceAll" class="text-xs text-amber-600">{{ t('openAIQuality.unsupportedControl') }}</span></label>
        </div>
        <div class="flex flex-wrap gap-2"><button class="btn btn-secondary" :disabled="busy" @click="preview">{{ t('openAIQuality.preview') }}</button><button class="btn btn-primary" :disabled="busy || !previewResult?.valid || previewFingerprint !== JSON.stringify(currentCreateItems())" @click="create(false)">{{ t('openAIQuality.createPolicy') }}</button><button class="btn btn-secondary" :disabled="busy || !previewResult?.valid || previewFingerprint !== JSON.stringify(currentCreateItems())" @click="create(true)">{{ t('openAIQuality.saveAndRun') }}</button></div>
        <div v-if="previewResult" class="rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-900"><p>{{ previewResult.valid ? t('openAIQuality.previewValid') : t('openAIQuality.previewInvalid') }}</p><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.dailyCalls') }}: {{ previewResult.daily_calls_estimate }} / {{ previewResult.global_budget }}</p><p v-for="warning in previewResult.warnings || []" :key="warning.code" class="mt-1 text-amber-700">{{ warning.message }}</p></div>
      </section>
      <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3"><h2 class="font-semibold">{{ t('openAIQuality.policyList') }}</h2><button class="btn-secondary btn-xs" :disabled="!selectedPolicies.length" @click="beginBulkEdit()">{{ t('openAIQuality.editSelected', { count: selectedPolicies.length }) }}</button></div>
        <label v-for="p in policies" :key="p.id" class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 py-3 text-sm dark:border-dark-700"><span class="flex items-center gap-3"><input v-model="selectedPolicyIds" type="checkbox" :value="p.id" /><span>{{ p.account_name }} · {{ p.effective_upstream_model }} · {{ p.condition.protocol }} / {{ p.condition.transport }}<small class="block text-xs text-gray-500">{{ p.state.state }} · {{ p.rules.auto_check_enabled ? t('openAIQuality.automatic') : t('openAIQuality.manualOnly') }}</small></span></span><button class="text-blue-600 hover:underline" @click="beginEdit(p)">{{ t('openAIQuality.editRules') }}</button></label>
        <p v-if="!policies.length" class="text-sm text-gray-500">{{ t('openAIQuality.noPolicies') }}</p>
        <div v-if="editingPolicies.length" class="mt-4 space-y-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex items-center justify-between"><h3 class="font-semibold">{{ t('openAIQuality.editSelected', { count: editingPolicies.length }) }}</h3><button class="text-sm text-gray-500" @click="editingPolicies = []; editingPreview = null">{{ t('common.cancel') }}</button></div><p class="text-xs text-amber-700">{{ t('openAIQuality.bulkEditHelp') }}</p>
          <div class="grid gap-3 sm:grid-cols-2"><label v-for="field in ruleFields" :key="field.key" class="block"><span class="mb-1 block text-sm">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="editRules[field.key]" class="input" type="number" :min="field.min" :max="field.max" /></label><label class="flex items-center gap-3"><input v-model="editRules.auto_check_enabled" type="checkbox" class="checkbox" />{{ t('openAIQuality.autoCheck') }}</label><label class="flex items-center gap-3"><input v-model="editRules.control_enabled" type="checkbox" class="checkbox" />{{ t('openAIQuality.controlPolicy') }}</label></div>
          <div class="flex flex-wrap gap-2"><button class="btn btn-secondary" :disabled="busy" @click="previewEdit">{{ t('openAIQuality.preview') }}</button><button class="btn btn-primary" :disabled="busy || !editingPreview?.valid || editingPreviewFingerprint !== currentEditFingerprint()" @click="saveEdit">{{ t('common.save') }}</button></div>
          <div v-if="editingPreview" class="rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-900"><p>{{ editingPreview.valid ? t('openAIQuality.previewValid') : t('openAIQuality.previewInvalid') }}</p><p v-for="warning in editingPreview.warnings || []" :key="warning.code" class="mt-1 text-amber-700">{{ warning.message }}</p></div>
        </div>
      </section>
      <p v-if="error" class="rounded-xl bg-red-50 p-3 text-sm text-red-700">{{ error }}</p>
    </main>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getAll as getAllAdminGroups } from '@/api/admin/groups'
import { createQualityPolicies, emergencyDisableQuality, getAllQualityPolicies, getAllQualityTargets, getQualitySettings, previewQuality, previewQualitySettings, saveQualitySettings, submitQualityRuns, updateQualityPolicies, type QualityCondition, type QualityPolicy, type QualityRules, type QualitySettings, type QualityTarget } from '@/api/admin/openaiAccountQuality'
import type { AdminGroup } from '@/types'

const { t } = useI18n()
const settings = ref<QualitySettings | null>(null)
const targets = ref<QualityTarget[]>([])
const groups = ref<AdminGroup[]>([])
const policies = ref<QualityPolicy[]>([])
const health = ref({ queue_depth: 0, running_count: 0, store_status: 'unknown', last_sync_at: '' })
const budget = ref({ consumed: 0, limit: 0 })
const selectedAccountIds = ref<number[]>([])
const selectedSourceGroupId = ref(0)
const selectedPolicyIds = ref<number[]>([])
const targetSearch = ref('')
const modelOne = ref('')
const modelTwo = ref('')
const modelTwoEnabled = ref(false)
const saving = ref(false)
const busy = ref(false)
const error = ref('')
const previewResult = ref<any>(null)
const previewFingerprint = ref('')
const settingsPreview = ref<any>(null)
const settingsPreviewFingerprint = ref('')
const editingPolicies = ref<QualityPolicy[]>([])
const editingPreview = ref<any>(null)
const editingPreviewFingerprint = ref('')
const condition = reactive<QualityCondition>({ model_id: '', protocol: 'responses', transport: 'http', reasoning_effort: 'upstream_default', service_tier: 'upstream_default', benchmark_id: 'candy', benchmark_version: '1', scorer_version: 'strict-json-v1' })
const rules = reactive<QualityRules>({ auto_check_enabled: false, control_enabled: false, interval_seconds: 1800, jitter_seconds: 60, failure_threshold: 3, recovery_threshold: 2, cooldown_seconds: 1800, recovery_interval_seconds: 600, evidence_window_seconds: 7200, timeout_seconds: 180, max_output_tokens: 4096, daily_call_limit: 100 })
const editRules = reactive<QualityRules>({ auto_check_enabled: false, control_enabled: false, interval_seconds: 1800, jitter_seconds: 60, failure_threshold: 3, recovery_threshold: 2, cooldown_seconds: 1800, recovery_interval_seconds: 600, evidence_window_seconds: 7200, timeout_seconds: 180, max_output_tokens: 4096, daily_call_limit: 100 })
const eligibleSourceGroupIDs = computed(() => new Set(targets.value.flatMap(target => target.compatible_source_group_ids)))
const sourceGroups = computed(() => groups.value.filter(group => eligibleSourceGroupIDs.value.has(group.id)).sort((a, b) => a.name.localeCompare(b.name)))
const targetsInSelectedGroup = computed(() => targets.value.filter(target => target.compatible_source_group_ids.includes(selectedSourceGroupId.value)))
const selectedTargets = computed(() => targetsInSelectedGroup.value.filter(target => selectedAccountIds.value.includes(target.account_id)))
const filteredTargets = computed(() => targetsInSelectedGroup.value.filter(target => `${target.account_name} ${target.account_id}`.toLowerCase().includes(targetSearch.value.toLowerCase())))
const selectedModels = computed(() => [modelOne.value.trim(), ...(modelTwoEnabled.value ? [modelTwo.value.trim()] : [])].filter(Boolean))
const maxSelectableAccounts = computed(() => modelTwoEnabled.value ? 50 : 100)
const createPolicyCount = computed(() => selectedTargets.value.length * selectedModels.value.length)
const canEnforceAll = computed(() => selectedTargets.value.length > 0 && selectedTargets.value.every(target => target.can_enforce))
const selectedPolicies = computed(() => policies.value.filter(policy => selectedPolicyIds.value.includes(policy.id)))
function clearCreatePreview() {
  previewResult.value = null
  previewFingerprint.value = ''
}
watch(selectedSourceGroupId, () => {
  selectedAccountIds.value = []
  targetSearch.value = ''
  clearCreatePreview()
})
watch([selectedAccountIds, modelOne, modelTwo, modelTwoEnabled, condition, rules], clearCreatePreview, { deep: true })
watch([editRules, editingPolicies], () => { editingPreview.value = null; editingPreviewFingerprint.value = '' }, { deep: true })
watch(settings, () => { settingsPreview.value = null; settingsPreviewFingerprint.value = '' }, { deep: true })
type SettingsNumberKey = 'global_concurrency' | 'queue_limit' | 'queue_timeout_seconds' | 'global_daily_call_limit' | 'default_policy_daily_call_limit' | 'raw_retention_days' | 'metadata_retention_days'
type RulesNumberKey = 'interval_seconds' | 'jitter_seconds' | 'failure_threshold' | 'recovery_threshold' | 'cooldown_seconds' | 'recovery_interval_seconds' | 'evidence_window_seconds' | 'timeout_seconds' | 'max_output_tokens' | 'daily_call_limit'
type NumberField<K extends string> = { key: K; label: string; min: number; max: number }
const numberFields: NumberField<SettingsNumberKey>[] = [{ key: 'global_concurrency', label: 'globalConcurrency', min: 1, max: 10 }, { key: 'queue_limit', label: 'queueLimit', min: 100, max: 1000 }, { key: 'queue_timeout_seconds', label: 'queueTimeout', min: 60, max: 1800 }, { key: 'global_daily_call_limit', label: 'globalDailyLimit', min: 1, max: 100000 }, { key: 'default_policy_daily_call_limit', label: 'policyDefaultLimit', min: 1, max: 1000 }, { key: 'raw_retention_days', label: 'rawRetention', min: 1, max: 30 }, { key: 'metadata_retention_days', label: 'metadataRetention', min: 7, max: 180 }]
const ruleFields: NumberField<RulesNumberKey>[] = [{ key: 'interval_seconds', label: 'interval', min: 300, max: 86400 }, { key: 'jitter_seconds', label: 'jitter', min: 0, max: 300 }, { key: 'failure_threshold', label: 'failureThreshold', min: 1, max: 10 }, { key: 'recovery_threshold', label: 'recoveryThreshold', min: 1, max: 10 }, { key: 'cooldown_seconds', label: 'cooldown', min: 300, max: 86400 }, { key: 'recovery_interval_seconds', label: 'recoveryInterval', min: 300, max: 86400 }, { key: 'evidence_window_seconds', label: 'evidenceWindow', min: 600, max: 604800 }, { key: 'timeout_seconds', label: 'timeout', min: 30, max: 600 }, { key: 'max_output_tokens', label: 'maxOutput', min: 512, max: 16384 }, { key: 'daily_call_limit', label: 'dailyLimit', min: 1, max: 1000 }]
function currentSettingsFingerprint() { if (!settings.value) return ''; const { revision, control_epoch: _controlEpoch, ...payload } = settings.value; return JSON.stringify({ ...payload, expected_revision: revision }) }
function currentCreateItems() { return selectedTargets.value.flatMap(target => selectedModels.value.map(modelId => ({ account_id: target.account_id, source_group_id: selectedSourceGroupId.value, condition: { ...condition, model_id: modelId }, rules: { ...rules } }))) }
function currentEditItems() { return editingPolicies.value.map(policy => ({ policy_id: policy.id, expected_revision: policy.revision, rules: { ...editRules } })) }
function currentEditFingerprint() { return JSON.stringify(currentEditItems()) }
function formatDate(value: string) { return value ? new Date(value).toLocaleString() : '—' }
function addSecondModel() { if (selectedAccountIds.value.length <= 50) { modelTwo.value = ''; modelTwoEnabled.value = true } }
function removeSecondModel() { modelTwo.value = ''; modelTwoEnabled.value = false }
async function load() {
  try {
    const [s, ts, ps, gs] = await Promise.all([getQualitySettings(), getAllQualityTargets(), getAllQualityPolicies(), getAllAdminGroups()])
    settings.value = s.settings
    health.value = s.health
    budget.value = s.budget
    targets.value = ts.items
    groups.value = gs
    policies.value = ps.items
    if (!sourceGroups.value.some(group => group.id === selectedSourceGroupId.value)) selectedSourceGroupId.value = 0
    const availableAccountIDs = new Set(targetsInSelectedGroup.value.filter(target => target.can_probe).map(target => target.account_id))
    selectedAccountIds.value = selectedAccountIds.value.filter(id => availableAccountIDs.has(id)).slice(0, maxSelectableAccounts.value)
    selectedPolicyIds.value = selectedPolicyIds.value.filter(id => policies.value.some(policy => policy.id === id))
  } catch (e: any) { error.value = e?.message || String(e) }
}
async function previewSettings() {
  if (!settings.value) return
  saving.value = true; error.value = ''; const fingerprint = currentSettingsFingerprint()
  try { const result = await previewQualitySettings(settings.value); if (fingerprint === currentSettingsFingerprint()) { settingsPreview.value = result; settingsPreviewFingerprint.value = fingerprint } }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { saving.value = false }
}
async function save() {
  if (!settings.value || !settingsPreview.value?.valid || settingsPreviewFingerprint.value !== currentSettingsFingerprint()) return
  saving.value = true; error.value = ''
  try { const { revision, control_epoch: _controlEpoch, ...payload } = settings.value; const result = await saveQualitySettings({ ...payload, expected_revision: revision }); settings.value = result.settings; await load() }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { saving.value = false }
}
async function emergencyDisable() {
  const reason = window.prompt(t('openAIQuality.emergencyReason'))
  if (!reason?.trim() || !window.confirm(t('openAIQuality.emergencyConfirm'))) return
  saving.value = true; error.value = ''
  try { await emergencyDisableQuality(reason.trim()); settingsPreview.value = null; await load() }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { saving.value = false }
}
async function preview() {
  const payload = currentCreateItems()
  const normalizedModels = selectedModels.value
  if (!selectedSourceGroupId.value || !selectedTargets.value.length || !modelOne.value.trim() || (modelTwoEnabled.value && !modelTwo.value.trim())) { error.value = t('openAIQuality.chooseAccountAndModel'); return }
  if (modelTwoEnabled.value && modelOne.value.trim() === modelTwo.value.trim()) { error.value = t('openAIQuality.duplicateModels'); return }
  if (!normalizedModels.length || payload.length > 100) { error.value = t('openAIQuality.batchPolicyLimit'); return }
  busy.value = true; error.value = ''; const fingerprint = JSON.stringify(payload)
  try { const result = await previewQuality(payload); if (fingerprint === JSON.stringify(currentCreateItems())) { previewResult.value = result; previewFingerprint.value = fingerprint } }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { busy.value = false }
}
async function create(runAfterSave = false) {
  const payload = currentCreateItems()
  if (!previewResult.value?.valid || previewFingerprint.value !== JSON.stringify(payload)) return
  busy.value = true; error.value = ''
  try {
    const created = await createQualityPolicies(payload)
    previewResult.value = null
    previewFingerprint.value = ''
    await load()
    if (runAfterSave && created.items.length) {
      try { await submitQualityRuns(created.items.map(policy => policy.id)); await load() }
      catch (e: any) { error.value = t('openAIQuality.savedButRunFailed', { reason: e?.message || String(e) }) }
    }
  }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { busy.value = false }
}
function beginEdit(policy: QualityPolicy) { beginBulkEdit([policy]) }
function beginBulkEdit(policiesToEdit = selectedPolicies.value) {
  if (!policiesToEdit.length) return
  editingPolicies.value = [...policiesToEdit]
  Object.assign(editRules, policiesToEdit[0].rules)
  editingPreview.value = null
  editingPreviewFingerprint.value = ''
}
async function previewEdit() {
  const payload = currentEditItems()
  if (!payload.length) return
  busy.value = true; error.value = ''; const fingerprint = JSON.stringify(payload)
  try { const result = await previewQuality(payload, 'update'); if (fingerprint === currentEditFingerprint()) { editingPreview.value = result; editingPreviewFingerprint.value = fingerprint } }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { busy.value = false }
}
async function saveEdit() {
  const payload = currentEditItems()
  const fingerprint = currentEditFingerprint()
  if (!payload.length || !editingPreview.value?.valid || editingPreviewFingerprint.value !== fingerprint) return
  busy.value = true; error.value = ''
  try { await updateQualityPolicies(payload); editingPolicies.value = []; editingPreview.value = null; selectedPolicyIds.value = []; await load() }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { busy.value = false }
}
onMounted(load)
</script>
