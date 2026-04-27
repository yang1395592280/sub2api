<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-700">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.luckyWheel.title') }}</h1>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.luckyWheel.description') }}</p>
            </div>
            <nav class="flex flex-wrap gap-2">
              <button v-for="tab in tabs" :key="tab" type="button" class="rounded-full px-4 py-2 text-sm font-medium transition" :class="activeTab === tab ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'" @click="activeTab = tab">
                {{ t(`admin.luckyWheel.tabs.${tab}`) }}
              </button>
            </nav>
          </div>
        </div>

        <div v-if="activeTab === 'settings'" class="space-y-6 p-6">
          <div v-if="loadState === 'loading'" class="flex justify-center py-12"><LoadingSpinner /></div>
          <div v-else-if="loadState === 'error'" class="px-4 py-10">
            <EmptyState :title="t('admin.luckyWheel.loadFailedTitle')" :description="errorMessage || t('admin.luckyWheel.loadFailed')">
              <template #action><button type="button" class="btn btn-primary" @click="loadSettings">{{ t('common.retry') }}</button></template>
            </EmptyState>
          </div>
          <template v-else>
            <div class="grid gap-4 lg:grid-cols-2">
              <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                <div class="flex items-center justify-between gap-4">
                  <div><p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.luckyWheel.enabled') }}</p><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.luckyWheel.enabledHint') }}</p></div>
                  <Toggle v-model="form.enabled" />
                </div>
                <div class="mt-4"><label class="input-label">{{ t('admin.luckyWheel.dailySpinLimit') }}</label><input v-model.number="form.daily_spin_limit" type="number" min="1" max="20" class="input" /></div>
                <p class="mt-4 text-sm text-gray-600 dark:text-gray-300">{{ t('admin.luckyWheel.summary', { probability: totalProbability.toFixed(2), floor: minPointsRequired }) }}</p>
              </div>
              <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                <div class="flex items-center justify-between"><p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.luckyWheel.prizesTitle') }}</p><button type="button" class="btn btn-secondary btn-sm" @click="addPrize">{{ t('admin.luckyWheel.addPrize') }}</button></div>
                <div class="mt-4 space-y-3">
                  <div v-for="(prize, index) in form.prizes" :key="prize.key" class="grid gap-3 rounded-2xl bg-gray-50 p-4 ring-1 ring-gray-100 dark:bg-dark-800 dark:ring-dark-700 md:grid-cols-[1.1fr_0.9fr_0.7fr_auto]">
                    <div><label class="input-label">{{ t('admin.luckyWheel.prizeLabel') }}</label><input v-model="prize.label" type="text" class="input" /></div>
                    <div><label class="input-label">{{ t('admin.luckyWheel.prizeType') }}</label><select v-model="prize.type" class="input"><option v-for="option in prizeTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></div>
                    <div class="grid grid-cols-2 gap-3"><div><label class="input-label">{{ t('admin.luckyWheel.deltaPoints') }}</label><input v-model.number="prize.delta_points" type="number" class="input" /></div><div><label class="input-label">{{ t('admin.luckyWheel.probability') }}</label><input v-model.number="prize.probability" type="number" min="0" step="0.1" class="input" /></div></div>
                    <div class="flex items-end"><button type="button" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" :disabled="form.prizes.length <= 4" @click="removePrize(index)">{{ t('common.delete') }}</button></div>
                  </div>
                </div>
              </div>
            </div>
            <div><label class="input-label">{{ t('admin.luckyWheel.rulesMarkdown') }}</label><textarea v-model="form.rules_markdown" rows="8" class="input"></textarea></div>
            <p v-if="validationError" class="text-sm text-red-600 dark:text-red-400">{{ validationError }}</p>
            <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700"><button type="button" class="btn btn-primary" :disabled="saving || Boolean(validationError)" @click="saveSettings">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
          </template>
        </div>

        <div v-else class="space-y-4 p-6">
          <div class="flex flex-wrap items-end gap-3">
            <div><label class="input-label">{{ t('admin.luckyWheel.filters.userId') }}</label><input v-model="filters.user_id" type="text" inputmode="numeric" class="input w-28" /></div>
            <div><label class="input-label">{{ t('admin.luckyWheel.filters.startDate') }}</label><input v-model="filters.start_date" type="date" class="input w-40" /></div>
            <div><label class="input-label">{{ t('admin.luckyWheel.filters.endDate') }}</label><input v-model="filters.end_date" type="date" class="input w-40" /></div>
            <button type="button" class="btn btn-primary" :disabled="spinsLoading" @click="loadSpins(1)">{{ t('common.apply') }}</button>
          </div>
          <DataTable :columns="columns" :data="spins.items" :loading="spinsLoading" />
          <Pagination v-if="spins.total > 0" :page="spins.page" :total="spins.total" :page-size="spins.page_size" @update:page="loadSpins" @update:pageSize="changePageSize" />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as luckyWheelAdminAPI from '@/api/admin/luckyWheel'
import type { LuckyWheelAdminSettings, LuckyWheelSpinRecord } from '@/types/luckyWheel'
import type { BasePaginationResponse } from '@/types'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const tabs = ['settings', 'spins'] as const
const activeTab = ref<typeof tabs[number]>('settings')
const loadState = ref<'loading' | 'ready' | 'error'>('loading')
const errorMessage = ref('')
const saving = ref(false)
const validationError = ref('')
const spinsLoading = ref(false)
const form = reactive<LuckyWheelAdminSettings>({ enabled: true, daily_spin_limit: 5, prizes: [], rules_markdown: '' })
const spins = ref<BasePaginationResponse<LuckyWheelSpinRecord>>({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
const filters = reactive({ user_id: '', start_date: '', end_date: '' })

const prizeTypeOptions = computed(() => [
  { value: 'reward', label: t('admin.luckyWheel.prizeTypes.reward') },
  { value: 'penalty', label: t('admin.luckyWheel.prizeTypes.penalty') },
  { value: 'thanks', label: t('admin.luckyWheel.prizeTypes.thanks') },
])
const totalProbability = computed(() => form.prizes.reduce((sum, item) => sum + Number(item.probability || 0), 0))
const minPointsRequired = computed(() => Math.max(...form.prizes.filter(item => item.delta_points < 0).map(item => Math.abs(item.delta_points)), 0))
const columns = computed<Column[]>(() => [
  { key: 'username', label: t('admin.luckyWheel.columns.user'), formatter: (_, row) => `${row.username || row.email} (#${row.user_id})` },
  { key: 'spin_index', label: t('admin.luckyWheel.columns.spinIndex'), formatter: (_, row) => `${row.spin_date} / ${row.spin_index}` },
  { key: 'prize_label', label: t('admin.luckyWheel.columns.prize') },
  { key: 'delta_points', label: t('admin.luckyWheel.columns.delta'), formatter: value => formatSigned(Number(value)) },
  { key: 'points_after', label: t('admin.luckyWheel.columns.pointsAfter'), formatter: value => String(value) },
  { key: 'created_at', label: t('admin.luckyWheel.columns.createdAt'), formatter: value => formatDateTime(String(value)) },
])

onMounted(() => { void loadSettings() })
watch(activeTab, (tab) => { if (tab === 'spins' && !spins.value.items.length) void loadSpins(1) })
watch(form, () => { validationError.value = validateForm() }, { deep: true })

async function loadSettings(): Promise<void> {
  loadState.value = 'loading'
  try {
    applySettings(await luckyWheelAdminAPI.getSettings())
    loadState.value = 'ready'
  } catch (error: any) {
    loadState.value = 'error'
    errorMessage.value = error?.message || t('admin.luckyWheel.loadFailed')
    appStore.showError(errorMessage.value)
  }
}

async function saveSettings(): Promise<void> {
  validationError.value = validateForm()
  if (validationError.value) return
  saving.value = true
  try {
    await luckyWheelAdminAPI.updateSettings({ ...form, prizes: form.prizes.map(item => ({ ...item })) })
    appStore.showSuccess(t('admin.luckyWheel.saveSuccess'))
    await loadSettings()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.luckyWheel.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function loadSpins(page = spins.value.page): Promise<void> {
  spinsLoading.value = true
  try {
    spins.value = await luckyWheelAdminAPI.listSpins({
      page,
      page_size: spins.value.page_size,
      user_id: parsePositiveInt(filters.user_id),
      start_date: filters.start_date || undefined,
      end_date: filters.end_date || undefined,
    })
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.luckyWheel.loadFailed'))
  } finally {
    spinsLoading.value = false
  }
}

function applySettings(settings: LuckyWheelAdminSettings): void {
  Object.assign(form, { enabled: settings.enabled, daily_spin_limit: settings.daily_spin_limit, rules_markdown: settings.rules_markdown, prizes: settings.prizes.map(item => ({ ...item })) })
  validationError.value = ''
}

function addPrize(): void {
  form.prizes.push({ key: `segment_${Date.now()}_${form.prizes.length}`, label: t('admin.luckyWheel.newPrize'), type: 'reward', delta_points: 18, probability: 5 })
}

function removePrize(index: number): void {
  form.prizes.splice(index, 1)
  validationError.value = validateForm()
}

function changePageSize(pageSize: number): void {
  spins.value = { ...spins.value, page_size: pageSize }
  void loadSpins(1)
}

function parsePositiveInt(value: string): number | undefined {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

function formatSigned(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`
}

function validateForm(): string {
  const hasReward = form.prizes.some(item => item.type === 'reward' && item.delta_points > 0)
  const hasPenalty = form.prizes.some(item => item.type === 'penalty' && item.delta_points < 0)
  const hasThanks = form.prizes.some(item => item.type === 'thanks' && item.delta_points === 0)
  const validKeyCount = new Set(form.prizes.map(item => item.key.trim())).size === form.prizes.length
  const validLabels = form.prizes.every(item => item.label.trim())
  const sumValid = Math.abs(totalProbability.value - 100) <= 0.0001
  if (form.daily_spin_limit < 1 || form.daily_spin_limit > 20) return t('admin.luckyWheel.validation.dailySpinLimit')
  if (!form.rules_markdown.trim()) return t('admin.luckyWheel.validation.rulesMarkdown')
  if (!validKeyCount || !validLabels || !hasReward || !hasPenalty || !hasThanks || !sumValid) return t('admin.luckyWheel.validation.prizes')
  if (form.prizes.some(item => item.type === 'reward' && item.delta_points <= 0)) return t('admin.luckyWheel.validation.prizes')
  if (form.prizes.some(item => item.type === 'penalty' && item.delta_points >= 0)) return t('admin.luckyWheel.validation.prizes')
  if (form.prizes.some(item => item.type === 'thanks' && item.delta_points !== 0)) return t('admin.luckyWheel.validation.prizes')
  return ''
}
</script>
