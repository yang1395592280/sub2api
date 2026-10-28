<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-5">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('openAIQuality.settingsTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('openAIQuality.settingsDescription') }}</p></div>
        <div class="flex gap-2"><router-link class="btn btn-secondary" to="/admin/openai-quality/records"><Icon name="clock" size="sm" />{{ t('nav.openAIQualityRecords') }}</router-link><router-link class="btn btn-secondary" to="/admin/openai-quality">{{ t('nav.openAIQuality') }}</router-link></div>
      </header>
      <div class="rounded-xl bg-primary-50 px-4 py-3 text-xs leading-relaxed text-primary-800 dark:bg-primary-900/20 dark:text-primary-200">{{ t('openAIQuality.scopeNote') }}</div>
      <nav class="flex flex-wrap gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800" :aria-label="t('nav.openAIQualitySettings')">
        <button v-for="item in sections" :key="item.value" class="rounded-lg px-5 py-2.5 text-sm font-medium transition-colors" :class="activeSection === item.value ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 hover:text-gray-800 dark:hover:text-dark-200'" :aria-current="activeSection === item.value ? 'page' : undefined" @click="activeSection = item.value">{{ item.label }}<span v-if="item.value === 'policies'" class="ml-2 rounded-md bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-dark-600">{{ policies.length }}</span></button>
      </nav>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <p v-if="notice" role="status" class="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">{{ notice }}</p>

      <div v-show="activeSection === 'create'" class="grid items-start gap-5 lg:grid-cols-3">
        <div class="space-y-5 lg:col-span-2">
          <section class="card space-y-5 p-5 sm:p-6">
            <div class="flex gap-3"><span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">1</span><div><h2 class="font-semibold">{{ t('openAIQuality.chooseGroupAndAccounts') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.chooseGroupBeforeAccounts') }}</p></div></div>
            <label class="block space-y-2"><span class="input-label">{{ t('openAIQuality.sourceGroup') }}</span><Select v-model="selectedSourceGroupId" :options="sourceGroupOptions" searchable :aria-label="t('openAIQuality.sourceGroup')" :placeholder="t('openAIQuality.chooseSourceGroup')" /></label>
            <div v-if="selectedSourceGroupId" class="space-y-3">
              <div class="flex flex-wrap items-center justify-between gap-2"><span class="text-sm font-medium">{{ t('openAIQuality.selectedTargets', { count: selectedAccountIds.length }) }}<span class="ml-2 text-xs font-normal text-gray-400">/ {{ maxSelectableAccounts }}</span></span><span class="rounded-lg px-2 py-1 text-xs" :class="selectedGroupCanEnforce ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">{{ t(selectedGroupCanEnforce ? 'openAIQuality.canControlGroup' : 'openAIQuality.probeOnlyGroup') }}</span></div>
              <div class="relative"><Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" /><input v-model="targetSearch" class="input pl-9" :aria-label="t('openAIQuality.searchTarget')" :placeholder="t('openAIQuality.searchTarget')" /></div>
              <div v-if="filteredTargets.length" class="grid max-h-72 gap-2 overflow-y-auto rounded-xl border border-gray-200 p-2 dark:border-dark-700 sm:grid-cols-2">
                <label v-for="target in filteredTargets" :key="target.account_id" class="flex min-h-14 cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors" :class="selectedAccountIds.includes(target.account_id) ? 'border-primary-300 bg-primary-50/70 dark:border-primary-700 dark:bg-primary-900/20' : 'border-transparent hover:bg-gray-50 dark:hover:bg-dark-700'">
                  <input v-model="selectedAccountIds" class="checkbox mt-0.5" type="checkbox" :value="target.account_id" :disabled="!target.can_probe || (!selectedAccountIds.includes(target.account_id) && selectedAccountIds.length >= maxSelectableAccounts)" />
                  <span class="min-w-0 text-sm"><span class="block break-all font-medium">{{ target.account_name }}</span><small v-if="!target.can_probe" class="mt-1 block text-amber-700 dark:text-amber-300">{{ target.reasons.map(label).join(' · ') || label(target.eligibility) }}</small></span>
                </label>
              </div>
              <p v-else class="rounded-xl bg-gray-50 p-6 text-center text-sm text-gray-500 dark:bg-dark-900">{{ t('openAIQuality.noAccountsInGroup') }}</p>
            </div>
            <div v-else class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-400 dark:border-dark-600">{{ t('openAIQuality.chooseGroupBeforeAccounts') }}</div>
          </section>

          <section class="card space-y-5 p-5 sm:p-6">
            <div class="flex gap-3"><span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">2</span><div><h2 class="font-semibold">{{ t('openAIQuality.configureModels') }}</h2><p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('openAIQuality.modelSelectionHelp') }}</p></div></div>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2"><span class="input-label">{{ t('openAIQuality.modelOne') }}</span><input v-model.trim="modelOne" class="input" placeholder="gpt-5.5" /></label>
              <label v-if="modelTwoEnabled" class="block space-y-2"><span class="input-label">{{ t('openAIQuality.modelTwo') }}</span><span class="flex gap-2"><input v-model.trim="modelTwo" class="input min-w-0 flex-1" placeholder="gpt-5.4" /><button type="button" class="btn btn-ghost px-3" :aria-label="t('openAIQuality.removeSecondModel')" @click="removeSecondModel"><Icon name="x" size="sm" /></button></span></label>
              <button v-else type="button" class="flex min-h-11 items-center justify-center gap-2 self-end rounded-xl border border-dashed border-gray-300 px-4 py-2.5 text-sm text-primary-600 hover:border-primary-400 dark:border-dark-600 dark:text-primary-400" :disabled="selectedAccountIds.length > 50" @click="addSecondModel"><Icon name="plus" size="sm" />{{ t('openAIQuality.addSecondModel') }}</button>
            </div>
            <div class="space-y-2 border-t border-gray-100 pt-4 dark:border-dark-700"><span class="input-label">{{ t('openAIQuality.detectionSuite') }}</span><div class="grid gap-3 sm:grid-cols-2"><label v-for="suite in suites" :key="suite.value" class="flex cursor-pointer items-start gap-3 rounded-xl border p-4" :class="condition.benchmark_id === suite.value ? 'border-primary-300 bg-primary-50/40 dark:border-primary-700 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700'"><input v-model="condition.benchmark_id" type="radio" name="quality-suite" :value="suite.value" class="mt-1" /><span><b class="text-sm">{{ suite.label }}</b><small class="mt-1 block text-xs leading-relaxed text-gray-500">{{ suite.help }}</small></span></label></div><p v-if="condition.benchmark_id === 'dual' && (!settings?.reviewer || !health.renderer_configured)" class="text-xs text-amber-700 dark:text-amber-300">{{ t('openAIQuality.dualNotReady') }} <button class="underline" @click="activeSection = 'global'">{{ t('openAIQuality.configureReviewer') }}</button></p></div>
            <details class="border-t border-gray-100 pt-4 dark:border-dark-700"><summary class="cursor-pointer text-sm font-medium text-gray-600 dark:text-dark-300">{{ t('openAIQuality.requestParameters') }}</summary>
              <div class="mt-4 grid gap-4 sm:grid-cols-2">
                <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.protocol') }}</span><Select v-model="condition.protocol" :options="protocolOptions" :aria-label="t('openAIQuality.protocol')" /></label>
                <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.transport') }}</span><Select v-model="condition.transport" :options="transportOptions" :aria-label="t('openAIQuality.transport')" /></label>
                <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.reasoningEffort') }}</span><Select v-model="condition.reasoning_effort" :options="effortOptions" :aria-label="t('openAIQuality.reasoningEffort')" /></label>
                <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.serviceTier') }}</span><Select v-model="condition.service_tier" :options="tierOptions" :aria-label="t('openAIQuality.serviceTier')" /></label>
              </div>
            </details>
          </section>
        </div>

        <section class="card space-y-5 p-5 lg:sticky lg:top-6">
          <div class="flex gap-3"><span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">3</span><div><h2 class="font-semibold">{{ t('openAIQuality.checkAndSave') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t(condition.benchmark_id === 'dual' ? 'openAIQuality.dualHelp' : 'openAIQuality.candyBenchmark') }}</p></div></div>
          <div class="space-y-3 rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-900/60"><div class="flex justify-between gap-3"><span class="text-gray-500">{{ t('openAIQuality.sourceGroup') }}</span><span class="text-right font-medium">{{ selectedGroup?.name || '—' }}</span></div><div class="flex justify-between gap-3"><span class="text-gray-500">{{ t('openAIQuality.policyList') }}</span><span class="font-medium">{{ t('openAIQuality.accountModelCount', { accounts: selectedAccountIds.length, models: selectedModels.length, count: createPolicyCount }) }}</span></div></div>
          <div class="space-y-4">
            <label class="flex cursor-pointer items-start gap-3"><input v-model="rules.auto_check_enabled" type="checkbox" class="checkbox mt-0.5" /><span class="text-sm font-medium">{{ t('openAIQuality.autoCheck') }}</span></label>
            <label class="flex items-start gap-3"><input v-model="rules.control_enabled" type="checkbox" class="checkbox mt-0.5" :disabled="!canEnforceAll" /><span class="text-sm"><span class="font-medium">{{ t('openAIQuality.controlPolicy') }}</span><small v-if="!canEnforceAll" class="mt-1 block leading-relaxed text-gray-500">{{ t('openAIQuality.probeOnlyHelp') }}</small></span></label>
          </div>
          <div class="grid gap-3"><label v-for="field in basicRuleFields" :key="field.key" class="space-y-1.5"><span class="input-label">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="rules[field.key]" class="input" type="number" :min="field.min" :max="field.max" /></label></div>
          <details class="border-t border-gray-100 pt-4 dark:border-dark-700"><summary class="cursor-pointer text-sm font-medium text-gray-600 dark:text-dark-300">{{ t('openAIQuality.advancedRules') }}</summary><div class="mt-4 space-y-3"><label v-for="field in advancedRuleFields" :key="field.key" class="block space-y-1.5"><span class="input-label">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="rules[field.key]" class="input" type="number" :min="field.min" :max="field.max" /></label></div></details>
          <div v-if="previewResult" class="rounded-xl p-3 text-sm" :class="previewResult.valid ? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-amber-50 text-amber-800 dark:bg-amber-900/20 dark:text-amber-300'"><p class="font-medium">{{ previewResult.valid ? t('openAIQuality.previewReady') : t('openAIQuality.previewInvalid') }}</p><p class="mt-1 text-xs">{{ t('openAIQuality.dailyCalls') }}: {{ previewResult.daily_calls_estimate }} / {{ previewResult.global_budget }}</p><p v-for="warning in previewResult.warnings || []" :key="warning.code" class="mt-2 text-xs">{{ warning.message }}</p></div>
          <div class="grid gap-2 border-t border-gray-100 pt-4 dark:border-dark-700"><button class="btn btn-secondary w-full" :disabled="busy" @click="preview">{{ t('openAIQuality.preview') }}</button><div class="grid grid-cols-2 gap-2"><button class="btn btn-secondary" :disabled="!canCreate" @click="create(false)">{{ t('openAIQuality.createPolicy') }}</button><button class="btn btn-primary" :disabled="!canCreate" @click="create(true)">{{ busy ? t('common.loading') : t('openAIQuality.saveAndRun') }}</button></div></div>
        </section>
      </div>

      <section v-show="activeSection === 'policies'" class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700"><h2 class="font-semibold">{{ t('openAIQuality.policyList') }}</h2><div class="flex flex-wrap gap-2"><input v-model="policySearch" class="input w-56" :placeholder="t('openAIQuality.search')" /><button class="btn btn-secondary" :disabled="!selectedPolicies.length" @click="beginBulkEdit()">{{ t('openAIQuality.editSelected', { count: selectedPolicies.length }) }}</button></div></div>
        <div class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="p in filteredPolicies" :key="p.id" class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
            <label class="flex min-w-0 items-start gap-3"><input v-model="selectedPolicyIds" type="checkbox" class="checkbox mt-1" :value="p.id" :disabled="!selectedPolicyIds.includes(p.id) && selectedPolicyIds.length >= 100" /><span class="min-w-0 text-sm"><span class="block break-all font-semibold">{{ p.account_name }} <span class="ml-2 font-normal text-gray-500">{{ p.condition.model_id }}</span></span><small class="mt-1 block text-gray-500">{{ groupName(p) }} · {{ t(`openAIQuality.states.${p.state.state}`) }} · {{ p.rules.auto_check_enabled ? t('openAIQuality.automatic') : t('openAIQuality.manualOnly') }}</small></span></label>
            <div class="flex gap-2"><router-link class="btn btn-ghost px-3 py-2 text-xs text-primary-600" :to="{ path: '/admin/openai-quality/records', query: { policy_id: p.id } }">{{ t('openAIQuality.executionHistory') }}</router-link><button v-if="p.condition.benchmark_id !== 'dual'" class="btn btn-secondary px-3 py-2 text-xs" :disabled="busy" @click="previewUpgrade(p)">{{ t('openAIQuality.upgradeDual') }}</button><button class="btn btn-secondary px-3 py-2 text-xs" @click="beginEdit(p)">{{ t('openAIQuality.editRules') }}</button></div>
          </div>
        </div>
        <p v-if="!filteredPolicies.length" class="p-10 text-center text-sm text-gray-500">{{ t('openAIQuality.noPolicies') }}</p>
      </section>

      <form v-if="settings" v-show="activeSection === 'global'" class="card space-y-6 p-5 sm:p-6" @submit.prevent="save">
        <div><h2 class="font-semibold">{{ t('openAIQuality.globalSettings') }}</h2><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.settingsDescription') }}</p></div>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="flex items-start gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700"><input v-model="settings.automation_enabled" type="checkbox" class="checkbox mt-1" /><span><b class="text-sm">{{ t('openAIQuality.automation') }}</b><small class="mt-1 block text-gray-500">{{ t('openAIQuality.automationHelp') }}</small></span></label>
          <label class="flex items-start gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700"><input v-model="settings.control_enabled" type="checkbox" class="checkbox mt-1" /><span><b class="text-sm">{{ t('openAIQuality.globalControl') }}</b><small class="mt-1 block text-gray-500">{{ t('openAIQuality.controlHelp') }}</small></span></label>
          <label v-for="field in numberFields" :key="field.key" class="space-y-1.5"><span class="input-label">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="settings[field.key]" class="input" type="number" :min="field.min" :max="field.max" /><small v-if="field.key === 'default_policy_daily_call_limit'" class="block text-xs text-gray-500">{{ t('openAIQuality.policyDefaultLimitHelp') }}</small></label>
        </div>
        <section class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3"><div><h3 class="text-sm font-semibold">{{ t('openAIQuality.configureReviewer') }}</h3><p class="mt-1 max-w-3xl text-xs leading-relaxed text-gray-500">{{ t('openAIQuality.reviewerHelp') }}</p></div><label class="flex items-center gap-2 text-sm"><input type="checkbox" class="checkbox" :checked="!!settings.reviewer" @change="toggleReviewer" />{{ t('openAIQuality.enableReviewer') }}</label></div>
          <p class="text-xs" :class="health.renderer_configured ? 'text-emerald-600' : 'text-amber-600'">{{ t(health.renderer_configured ? 'openAIQuality.rendererConfigured' : 'openAIQuality.rendererMissing') }}</p>
          <div v-if="settings.reviewer" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.sourceGroup') }}</span><Select v-model="settings.reviewer.source_group_id" :options="sourceGroupOptions" searchable :aria-label="t('openAIQuality.reviewerGroup')" /></label>
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.reviewerAccount') }}</span><Select v-model="settings.reviewer.account_id" :options="reviewerAccountOptions" searchable :aria-label="t('openAIQuality.reviewerAccount')" /></label>
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.reviewerModel') }}</span><input v-model.trim="settings.reviewer.condition.model_id" class="input" :aria-label="t('openAIQuality.reviewerModel')" placeholder="gpt-5.5" /></label>
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.protocol') }}</span><Select v-model="settings.reviewer.condition.protocol" :options="protocolOptions" :aria-label="t('openAIQuality.reviewerProtocol')" /></label>
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.reasoningEffort') }}</span><Select v-model="settings.reviewer.condition.reasoning_effort" :options="effortOptions" /></label>
            <label class="space-y-2"><span class="input-label">{{ t('openAIQuality.serviceTier') }}</span><Select v-model="settings.reviewer.condition.service_tier" :options="tierOptions" /></label>
          </div>
        </section>
        <div v-if="settingsPreview" class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-900"><p>{{ settingsPreview.valid ? t('openAIQuality.previewReady') : t('openAIQuality.previewInvalid') }}</p><p class="mt-1 text-xs text-gray-500">{{ t('openAIQuality.dailyCalls') }}: {{ settingsPreview.daily_calls_estimate }} / {{ settingsPreview.global_budget }}</p><p v-for="warning in settingsPreview.warnings || []" :key="warning.code" class="mt-1 text-amber-700">{{ warning.message }}</p></div>
        <div class="flex flex-wrap justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-700"><div class="flex gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="previewSettings">{{ t('openAIQuality.preview') }}</button><button class="btn btn-primary" :disabled="saving || !settingsPreview?.valid || settingsPreviewFingerprint !== currentSettingsFingerprint()">{{ saving ? t('common.loading') : t('common.save') }}</button></div><button type="button" class="btn btn-ghost text-red-600" :disabled="saving" @click="emergencyDisable">{{ t('openAIQuality.emergencyDisable') }}</button></div>
        <div class="flex flex-wrap gap-x-6 gap-y-2 text-xs text-gray-400"><span>{{ t('openAIQuality.queueDepth') }}: {{ health.queue_depth }}</span><span>{{ t('openAIQuality.runningCount') }}: {{ health.running_count }}</span><span>{{ t('openAIQuality.storeStatus') }}: {{ label(health.store_status) }}</span><span>{{ t('openAIQuality.budget') }}: {{ budget.consumed }} / {{ budget.limit }}</span><span>{{ t('openAIQuality.lastSync') }}: {{ formatDate(health.last_sync_at) }}</span></div>
      </form>
      <BaseDialog :show="!!upgradingPolicy" :title="t('openAIQuality.upgradeDual')" @close="upgradingPolicy = null">
        <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-300">{{ t('openAIQuality.upgradeHelp') }}</p>
        <p v-if="upgradingPolicy" class="mt-4 text-sm font-medium">{{ upgradingPolicy.account_name }} · {{ upgradingPolicy.condition.model_id }}</p>
        <p v-for="issue in upgradePreview?.validation_errors || []" :key="issue.code" class="mt-3 text-sm text-red-600">{{ issue.message }}</p>
        <template #footer><button class="btn btn-primary" :disabled="busy || !upgradePreview?.valid" @click="upgrade">{{ t('openAIQuality.confirmUpgrade') }}</button></template>
      </BaseDialog>
      <BaseDialog :show="editingPolicies.length > 0" :title="t('openAIQuality.editSelected', { count: editingPolicies.length })" width="wide" @close="editingPolicies = []; editingPreview = null">
        <p class="mb-5 text-xs leading-relaxed text-gray-500">{{ t('openAIQuality.bulkEditHelp') }}</p>
        <div class="grid gap-4 sm:grid-cols-2"><label v-for="field in ruleFields" :key="field.key" class="space-y-1.5"><span class="input-label">{{ t(`openAIQuality.${field.label}`) }}</span><input v-model.number="editRules[field.key]" class="input" type="number" :min="field.min" :max="field.max" /></label><label class="flex items-center gap-3 text-sm"><input v-model="editRules.auto_check_enabled" type="checkbox" class="checkbox" />{{ t('openAIQuality.autoCheck') }}</label><label class="flex items-center gap-3 text-sm"><input v-model="editRules.control_enabled" type="checkbox" class="checkbox" :disabled="!canEnforceEdits" />{{ t('openAIQuality.controlPolicy') }}</label></div>
        <p v-if="!canEnforceEdits" class="mt-4 text-xs text-gray-500">{{ t('openAIQuality.probeOnlyHelp') }}</p>
        <div v-if="editingPreview" class="mt-4 rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-900"><p>{{ editingPreview.valid ? t('openAIQuality.previewReady') : t('openAIQuality.previewInvalid') }}</p><p v-for="warning in editingPreview.warnings || []" :key="warning.code" class="mt-1 text-amber-700">{{ warning.message }}</p></div>
        <template #footer><div class="flex justify-end gap-2"><button class="btn btn-secondary" :disabled="busy" @click="previewEdit">{{ t('openAIQuality.preview') }}</button><button class="btn btn-primary" :disabled="busy || !editingPreview?.valid || editingPreviewFingerprint !== currentEditFingerprint()" @click="saveEdit">{{ t('common.save') }}</button></div></template>
      </BaseDialog>
    </main>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onActivated, onDeactivated, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getAllIncludingInactive as getAllAdminGroups } from '@/api/admin/groups'
import { createQualityPolicies, emergencyDisableQuality, getAllQualityPolicies, getAllQualityTargets, getQualitySettings, previewQuality, previewQualitySettings, saveQualitySettings, submitQualityRuns, updateQualityPolicies, type QualityCondition, type QualityPolicy, type QualityRules, type QualitySettings, type QualityTarget, type QualityRuntimeHealth } from '@/api/admin/openaiAccountQuality'
import type { AdminGroup } from '@/types'

const { t, te } = useI18n()
const activeSection = ref('create')
const policySearch = ref('')
const notice = ref('')
let activatedOnce = false
const sections = computed(() => [{ value: 'create', label: t('openAIQuality.newPolicy') }, { value: 'policies', label: t('openAIQuality.policyList') }, { value: 'global', label: t('openAIQuality.globalSettings') }])
const settings = ref<QualitySettings | null>(null)
const targets = ref<QualityTarget[]>([])
const groups = ref<AdminGroup[]>([])
const policies = ref<QualityPolicy[]>([])
const health = ref<QualityRuntimeHealth>({ queue_depth: 0, running_count: 0, store_status: 'unknown', last_sync_at: '' })
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
const condition = reactive<QualityCondition>({ model_id: '', protocol: 'responses', transport: 'http', reasoning_effort: 'upstream_default', service_tier: 'upstream_default', benchmark_id: 'dual', benchmark_version: '1', scorer_version: 'dual-visual-v1' })
const rules = reactive<QualityRules>({ auto_check_enabled: false, control_enabled: false, interval_seconds: 1800, jitter_seconds: 60, failure_threshold: 3, recovery_threshold: 2, cooldown_seconds: 1800, recovery_interval_seconds: 600, evidence_window_seconds: 7200, timeout_seconds: 420, max_output_tokens: 8192, daily_call_limit: 100 })
const editRules = reactive<QualityRules>({ auto_check_enabled: false, control_enabled: false, interval_seconds: 1800, jitter_seconds: 60, failure_threshold: 3, recovery_threshold: 2, cooldown_seconds: 1800, recovery_interval_seconds: 600, evidence_window_seconds: 7200, timeout_seconds: 180, max_output_tokens: 4096, daily_call_limit: 100 })
const upgradingPolicy = ref<QualityPolicy | null>(null)
const upgradePreview = ref<{ valid: boolean; validation_errors?: { code: string; message: string }[] } | null>(null)
const suites = computed(() => [{ value: 'dual', label: t('openAIQuality.dualCheck'), help: t('openAIQuality.dualHelp') }, { value: 'candy', label: t('openAIQuality.candyOnly'), help: t('openAIQuality.candyHelp') }])
const reviewerAccountOptions = computed(() => targets.value.filter(target => target.can_probe && target.compatible_source_group_ids.includes(settings.value?.reviewer?.source_group_id || 0)).map(target => ({ value: target.account_id, label: target.account_name })))
watch(() => condition.benchmark_id, value => { condition.scorer_version = value === 'dual' ? 'dual-visual-v1' : 'strict-json-v1' })
watch(() => settings.value?.reviewer?.source_group_id, () => { if (settings.value?.reviewer && !reviewerAccountOptions.value.some(account => account.value === settings.value?.reviewer?.account_id)) settings.value.reviewer.account_id = 0 })
function toggleReviewer() {
  if (!settings.value) return
  settings.value.reviewer = settings.value.reviewer ? null : { account_id: 0, source_group_id: 0, condition: { model_id: '', protocol: 'responses', transport: 'http', reasoning_effort: 'upstream_default', service_tier: 'upstream_default', benchmark_id: 'candy', benchmark_version: '1', scorer_version: 'strict-json-v1' } }
}
function upgradeItem(policy: QualityPolicy) { return { account_id: policy.account_id, source_group_id: policy.source_group_id, condition: { ...policy.condition, benchmark_id: 'dual' as const, scorer_version: 'dual-visual-v1' as const }, rules: { ...policy.rules, timeout_seconds: Math.max(policy.rules.timeout_seconds, 420), max_output_tokens: Math.max(policy.rules.max_output_tokens, 8192) }, replaces_policy_id: policy.id, replaces_expected_revision: policy.revision } }
async function previewUpgrade(policy: QualityPolicy) {
  upgradingPolicy.value = policy; upgradePreview.value = null; busy.value = true
  try { upgradePreview.value = await previewQuality([upgradeItem(policy)]) }
  catch (e: unknown) { error.value = e instanceof Error ? e.message : String(e) }
  finally { busy.value = false }
}
async function upgrade() {
  if (!upgradingPolicy.value || !upgradePreview.value?.valid) return
  busy.value = true
  try { await createQualityPolicies([upgradeItem(upgradingPolicy.value)]); upgradingPolicy.value = null; await load() }
  catch (e: unknown) { error.value = e instanceof Error ? e.message : String(e) }
  finally { busy.value = false }
}
const sourceGroups = computed(() => groups.value.filter(group => group.platform === 'openai' && group.group_role !== 'self_hosted_pool' && group.status === 'active').sort((a, b) => a.name.localeCompare(b.name)))
const sourceGroupOptions = computed(() => sourceGroups.value.map(group => ({ value: group.id, label: group.name })))
const selectedGroup = computed(() => sourceGroups.value.find(group => group.id === selectedSourceGroupId.value))
const selectedGroupCanEnforce = computed(() => !!selectedGroup.value?.allow_auto_cheapest_scheduling)
const targetsInSelectedGroup = computed(() => targets.value.filter(target => target.compatible_source_group_ids.includes(selectedSourceGroupId.value)))
const selectedTargets = computed(() => targetsInSelectedGroup.value.filter(target => selectedAccountIds.value.includes(target.account_id)))
const filteredTargets = computed(() => targetsInSelectedGroup.value.filter(target => `${target.account_name} ${target.account_id}`.toLowerCase().includes(targetSearch.value.toLowerCase())))
const selectedModels = computed(() => [modelOne.value.trim(), ...(modelTwoEnabled.value ? [modelTwo.value.trim()] : [])].filter(Boolean))
const maxSelectableAccounts = computed(() => modelTwoEnabled.value ? 50 : 100)
const createPolicyCount = computed(() => selectedTargets.value.length * selectedModels.value.length)
const canEnforceAll = computed(() => selectedGroupCanEnforce.value && selectedTargets.value.length > 0 && selectedTargets.value.every(target => target.can_enforce))
const selectedPolicies = computed(() => policies.value.filter(policy => selectedPolicyIds.value.includes(policy.id)))
const filteredPolicies = computed(() => policies.value.filter(policy => `${policy.account_name} ${policy.condition.model_id} ${groupName(policy)}`.toLowerCase().includes(policySearch.value.toLowerCase())))
const canEnforceEdits = computed(() => editingPolicies.value.length > 0 && editingPolicies.value.every(policy => policy.control_supported !== false && !!groups.value.find(group => group.id === policy.source_group_id)?.allow_auto_cheapest_scheduling))
const canCreate = computed(() => !busy.value && previewResult.value?.valid && previewFingerprint.value === JSON.stringify(currentCreateItems()))
function label(value: string) { return te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
function groupName(policy: QualityPolicy) { return groups.value.find(group => group.id === policy.source_group_id)?.name || policy.source_group_name || t('openAIQuality.unavailableGroup') }
const protocolOptions = [{ value: 'responses', label: 'Responses' }, { value: 'chat_completions', label: 'Chat Completions' }]
const transportOptions = computed(() => [{ value: 'http', label: 'HTTP / SSE' }, ...(condition.protocol === 'responses' ? [{ value: 'websocket_v2', label: 'Responses WebSocket v2' }] : [])])
const effortOptions = computed(() => ['upstream_default','none','minimal','low','medium','high','xhigh'].map(value => ({ value, label: label(value) })))
const tierOptions = computed(() => ['upstream_default','default','priority','flex','auto','scale','ultrafast'].map(value => ({ value, label: label(value) })))
function clearCreatePreview() {
  previewResult.value = null
  previewFingerprint.value = ''
}
watch(selectedSourceGroupId, () => {
  selectedAccountIds.value = []
  targetSearch.value = ''
  clearCreatePreview()
})
watch(canEnforceAll, value => { if (!value) rules.control_enabled = false })
watch(() => condition.protocol, value => { if (value === 'chat_completions') condition.transport = 'http' })
watch([selectedAccountIds, modelOne, modelTwo, modelTwoEnabled, condition, rules], clearCreatePreview, { deep: true })
watch([editRules, editingPolicies], () => { editingPreview.value = null; editingPreviewFingerprint.value = '' }, { deep: true })
watch(settings, () => { settingsPreview.value = null; settingsPreviewFingerprint.value = '' }, { deep: true })
type SettingsNumberKey = 'global_concurrency' | 'queue_limit' | 'queue_timeout_seconds' | 'global_daily_call_limit' | 'default_policy_daily_call_limit' | 'raw_retention_days' | 'metadata_retention_days'
type RulesNumberKey = 'interval_seconds' | 'jitter_seconds' | 'failure_threshold' | 'recovery_threshold' | 'cooldown_seconds' | 'recovery_interval_seconds' | 'evidence_window_seconds' | 'timeout_seconds' | 'max_output_tokens' | 'daily_call_limit'
type NumberField<K extends string> = { key: K; label: string; min: number; max: number }
const numberFields: NumberField<SettingsNumberKey>[] = [{ key: 'global_concurrency', label: 'globalConcurrency', min: 1, max: 10 }, { key: 'queue_limit', label: 'queueLimit', min: 100, max: 1000 }, { key: 'queue_timeout_seconds', label: 'queueTimeout', min: 60, max: 1800 }, { key: 'global_daily_call_limit', label: 'globalDailyLimit', min: 1, max: 100000 }, { key: 'default_policy_daily_call_limit', label: 'policyDefaultLimit', min: 1, max: 1000 }, { key: 'raw_retention_days', label: 'rawRetention', min: 1, max: 30 }, { key: 'metadata_retention_days', label: 'metadataRetention', min: 7, max: 180 }]
const ruleFields: NumberField<RulesNumberKey>[] = [{ key: 'interval_seconds', label: 'interval', min: 300, max: 86400 }, { key: 'jitter_seconds', label: 'jitter', min: 0, max: 300 }, { key: 'failure_threshold', label: 'failureThreshold', min: 1, max: 10 }, { key: 'recovery_threshold', label: 'recoveryThreshold', min: 1, max: 10 }, { key: 'cooldown_seconds', label: 'cooldown', min: 300, max: 86400 }, { key: 'recovery_interval_seconds', label: 'recoveryInterval', min: 300, max: 86400 }, { key: 'evidence_window_seconds', label: 'evidenceWindow', min: 600, max: 604800 }, { key: 'timeout_seconds', label: 'timeout', min: 30, max: 600 }, { key: 'max_output_tokens', label: 'maxOutput', min: 512, max: 16384 }, { key: 'daily_call_limit', label: 'dailyLimit', min: 1, max: 1000 }]
const basicRuleFields = ruleFields.filter(field => ['interval_seconds', 'daily_call_limit'].includes(field.key))
const advancedRuleFields = ruleFields.filter(field => !['interval_seconds', 'daily_call_limit'].includes(field.key))
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
  busy.value = true; error.value = ''; notice.value = ''; const fingerprint = JSON.stringify(payload)
  try { const result = await previewQuality(payload); if (fingerprint === JSON.stringify(currentCreateItems())) { previewResult.value = result; previewFingerprint.value = fingerprint } }
  catch (e: any) { error.value = e?.message || String(e) }
  finally { busy.value = false }
}
async function create(runAfterSave = false) {
  const payload = currentCreateItems()
  if (!previewResult.value?.valid || previewFingerprint.value !== JSON.stringify(payload)) return
  busy.value = true; error.value = ''; notice.value = ''
  try {
    const created = await createQualityPolicies(payload)
    previewResult.value = null
    previewFingerprint.value = ''
    notice.value = t('openAIQuality.policiesCreated', { count: created.items.length })
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
  if (!canEnforceEdits.value) editRules.control_enabled = false
  editingPreview.value = null
  editingPreviewFingerprint.value = ''
}
async function previewEdit() {
  const payload = currentEditItems()
  if (!payload.length) return
  busy.value = true; error.value = ''; notice.value = ''; const fingerprint = JSON.stringify(payload)
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
onActivated(() => { if (activatedOnce) void load(); activatedOnce = true })
onDeactivated(() => { editingPolicies.value = []; editingPreview.value = null })
</script>
