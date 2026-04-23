<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-700">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.sizeBet.title') }}</h1>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.sizeBet.description') }}</p>
            </div>
            <nav class="flex flex-wrap gap-2">
              <button v-for="tab in tabs" :key="tab" :data-test="`tab-${tab}`" type="button" class="rounded-full px-4 py-2 text-sm font-medium transition" :class="activeTab === tab ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'" @click="activeTab = tab">
                {{ t(`admin.sizeBet.tabs.${tab}`) }}
              </button>
            </nav>
          </div>
        </div>

        <div v-if="activeTab === 'settings'" class="space-y-6 p-6">
          <div v-if="settingsStatus === 'loading'" class="flex justify-center py-10"><LoadingSpinner /></div>
          <div v-else-if="settingsStatus === 'error'" class="px-4 py-10">
            <EmptyState :title="t('admin.sizeBet.settingsLoadFailedTitle')" :description="settingsErrorMessage || t('admin.sizeBet.loadFailed')">
              <template #action>
                <button data-test="settings-load-retry" type="button" class="btn btn-primary" @click="loadSettings">{{ t('common.retry') }}</button>
              </template>
            </EmptyState>
          </div>
          <template v-else>
            <div class="rounded-2xl bg-gray-50 p-4 ring-1 ring-gray-100 dark:bg-dark-800 dark:ring-dark-700">
              <p class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.sizeBet.probabilitySummary', form.probabilities) }}</p>
              <p class="mt-2 text-sm text-gray-700 dark:text-gray-200">{{ t('admin.sizeBet.oddsSummary', form.odds) }}</p>
            </div>

            <div class="grid gap-4 lg:grid-cols-2">
              <div class="space-y-4 rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                <div class="flex items-center justify-between gap-4">
                  <div>
                    <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.sizeBet.enabled') }}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.sizeBet.enabledHint') }}</p>
                  </div>
                  <Toggle v-model="form.enabled" />
                </div>
                <div class="grid gap-4 sm:grid-cols-2">
                  <div><label class="input-label">{{ t('admin.sizeBet.roundDuration') }}</label><input data-test="settings-round-duration" v-model.number="form.round_duration_seconds" type="number" min="1" class="input" /></div>
                  <div><label class="input-label">{{ t('admin.sizeBet.betCloseOffset') }}</label><input data-test="settings-bet-close-offset" v-model.number="form.bet_close_offset_seconds" type="number" min="0" class="input" /></div>
                </div>
                <div><label class="input-label">{{ t('admin.sizeBet.allowedStakes') }}</label><input data-test="settings-allowed-stakes" v-model="allowedStakesText" type="text" class="input" :placeholder="t('admin.sizeBet.allowedStakesPlaceholder')" /></div>
              </div>

              <div class="grid gap-4">
                <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                  <p class="mb-4 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.sizeBet.probabilitiesTitle') }}</p>
                  <div class="grid gap-4 sm:grid-cols-3">
                    <div><label class="input-label">{{ t('sizeBet.directions.small') }}</label><input data-test="settings-prob-small" v-model.number="form.probabilities.small" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.mid') }}</label><input data-test="settings-prob-mid" v-model.number="form.probabilities.mid" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.big') }}</label><input data-test="settings-prob-big" v-model.number="form.probabilities.big" type="number" step="0.1" class="input" /></div>
                  </div>
                </div>
                <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                  <p class="mb-4 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.sizeBet.oddsTitle') }}</p>
                  <div class="grid gap-4 sm:grid-cols-3">
                    <div><label class="input-label">{{ t('sizeBet.directions.small') }}</label><input data-test="settings-odds-small" v-model.number="form.odds.small" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.mid') }}</label><input data-test="settings-odds-mid" v-model.number="form.odds.mid" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.big') }}</label><input data-test="settings-odds-big" v-model.number="form.odds.big" type="number" step="0.1" class="input" /></div>
                  </div>
                </div>
              </div>
            </div>

            <div><label class="input-label">{{ t('admin.sizeBet.rulesMarkdown') }}</label><textarea data-test="settings-rules-markdown" v-model="form.rules_markdown" rows="8" class="input"></textarea></div>
            <p v-if="validationError" class="text-sm text-red-600 dark:text-red-400">{{ validationError }}</p>
            <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700"><button data-test="save-settings" type="button" class="btn btn-primary" :disabled="savingSettings || !settingsReady" @click="saveSettings">{{ savingSettings ? t('common.saving') : t('common.save') }}</button></div>
          </template>
        </div>

        <div v-else class="space-y-4 p-6">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t(`admin.sizeBet.tabs.${activeTab}`) }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`admin.sizeBet.tabDescriptions.${activeTab}`) }}</p>
            </div>
            <div v-if="activeTab !== 'rounds'" class="flex flex-wrap items-end gap-3">
              <template v-if="activeTab === 'bets'">
                <div><label class="input-label">{{ t('admin.sizeBet.filters.roundId') }}</label><input data-test="filter-round-id" v-model="betFilters.round_id" type="text" inputmode="numeric" class="input w-28" /></div>
                <div><label class="input-label">{{ t('admin.sizeBet.filters.userId') }}</label><input data-test="filter-user-id" v-model="betFilters.user_id" type="text" inputmode="numeric" class="input w-28" /></div>
              </template>
              <template v-else>
                <div><label class="input-label">{{ t('admin.sizeBet.filters.roundId') }}</label><input data-test="filter-round-id" v-model="ledgerFilters.round_id" type="text" inputmode="numeric" class="input w-28" /></div>
                <div><label class="input-label">{{ t('admin.sizeBet.filters.userId') }}</label><input data-test="filter-user-id" v-model="ledgerFilters.user_id" type="text" inputmode="numeric" class="input w-28" /></div>
              </template>
              <div v-if="activeTab === 'bets'"><label class="input-label">{{ t('admin.sizeBet.filters.status') }}</label><Select data-test="filter-status" v-model="betFilters.status" :options="betStatusOptions" class="w-36" /></div>
              <div v-else><label class="input-label">{{ t('admin.sizeBet.filters.entryType') }}</label><Select data-test="filter-entry-type" v-model="ledgerFilters.entry_type" :options="ledgerEntryTypeOptions" class="w-40" /></div>
              <button data-test="apply-filters" type="button" class="btn btn-primary" :disabled="currentTable.loading" @click="applyFilters">{{ t('common.apply') }}</button>
              <button type="button" class="btn btn-secondary" :disabled="currentTable.loading" @click="resetFilters">{{ t('common.reset') }}</button>
            </div>
            <button v-else type="button" class="btn btn-secondary" :disabled="roundsState.loading" @click="loadActiveTab(true)">{{ t('common.refresh') }}</button>
          </div>

          <DataTable :columns="currentColumns" :data="currentTable.items" :loading="currentTable.loading">
            <template #cell-server_seed_hash="{ value }"><span class="font-mono text-xs text-gray-700 dark:text-gray-200">{{ value || '-' }}</span></template>
            <template #cell-server_seed="{ value }"><span class="font-mono text-xs text-gray-700 dark:text-gray-200">{{ value || '-' }}</span></template>
            <template #cell-actions="{ row }"><button v-if="isRoundRefundable(row)" :data-test="`refund-round-${row.id}`" type="button" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click="openRefundDialog(row)">{{ t('admin.sizeBet.refundAction') }}</button></template>
          </DataTable>

          <Pagination v-if="currentTable.pagination.total > 0" :page="currentTable.pagination.page" :total="currentTable.pagination.total" :page-size="currentTable.pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
        </div>
      </section>
    </div>

    <ConfirmDialog :show="refundDialogOpen" :title="t('admin.sizeBet.refundConfirmTitle')" :message="t('admin.sizeBet.refundConfirmMessage', { round: refundTarget?.round_no ?? '-' })" danger @confirm="confirmRefund" @cancel="refundDialogOpen = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as sizeBetAdminAPI from '@/api/admin/sizeBet'
import type { SizeBetAdminBet, SizeBetAdminLedgerEntry, SizeBetAdminRound, SizeBetAdminSettings } from '@/api/admin/sizeBet'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import type { Column } from '@/components/common/types'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

type AuditTab = 'settings' | 'rounds' | 'bets' | 'ledger'
type LoadState = 'loading' | 'ready' | 'error'
type PaginationState = { page: number; page_size: number; total: number; pages: number }
type TableState<T> = { items: T[]; loading: boolean; loaded: boolean; pagination: PaginationState }
type BetFilters = { round_id: string; user_id: string; status: string }
type LedgerFilters = { round_id: string; user_id: string; entry_type: string }

const pageSize = getPersistedPageSize()
const { t } = useI18n()
const appStore = useAppStore()
const tabs: AuditTab[] = ['settings', 'rounds', 'bets', 'ledger']
const activeTab = ref<AuditTab>('settings')
const settingsStatus = ref<LoadState>('loading')
const settingsErrorMessage = ref('')
const savingSettings = ref(false)
const validationError = ref('')
const allowedStakesText = ref('')
const refundDialogOpen = ref(false)
const refundTarget = ref<SizeBetAdminRound | null>(null)
const form = reactive<SizeBetAdminSettings>({ enabled: true, round_duration_seconds: 60, bet_close_offset_seconds: 50, allowed_stakes: [2, 5, 10, 20], probabilities: { small: 45, mid: 10, big: 45 }, odds: { small: 2, mid: 10, big: 2 }, rules_markdown: '' })
const roundsState = reactive<TableState<SizeBetAdminRound>>(newTableState())
const betsState = reactive<TableState<SizeBetAdminBet>>(newTableState())
const ledgerState = reactive<TableState<SizeBetAdminLedgerEntry>>(newTableState())
const betFilters = reactive<BetFilters>({ round_id: '', user_id: '', status: '' })
const ledgerFilters = reactive<LedgerFilters>({ round_id: '', user_id: '', entry_type: '' })

const settingsReady = computed(() => settingsStatus.value === 'ready')
const currentTable = computed(() => activeTab.value === 'rounds' ? roundsState : activeTab.value === 'bets' ? betsState : ledgerState)
const betStatusOptions = computed(() => [{ value: '', label: t('common.all') }, { value: 'placed', label: t('admin.sizeBet.status.placed') }, { value: 'won', label: t('admin.sizeBet.status.won') }, { value: 'lost', label: t('admin.sizeBet.status.lost') }, { value: 'refunded', label: t('admin.sizeBet.status.refunded') }])
const ledgerEntryTypeOptions = computed(() => [{ value: '', label: t('common.all') }, { value: 'bet_debit', label: t('admin.sizeBet.entryType.bet_debit') }, { value: 'bet_payout', label: t('admin.sizeBet.entryType.bet_payout') }, { value: 'bet_refund', label: t('admin.sizeBet.entryType.bet_refund') }])
const roundColumns = computed<Column[]>(() => [{ key: 'round_no', label: t('admin.sizeBet.columns.round') }, { key: 'status', label: t('admin.sizeBet.columns.status'), formatter: (value) => statusLabel(String(value)) }, { key: 'result_number', label: t('admin.sizeBet.columns.result'), formatter: (_, row) => row.result_number == null || !row.result_direction ? '-' : `${row.result_number} / ${directionLabel(String(row.result_direction))}` }, { key: 'server_seed_hash', label: t('admin.sizeBet.columns.serverSeedHash') }, { key: 'server_seed', label: t('admin.sizeBet.columns.serverSeed') }, { key: 'starts_at', label: t('admin.sizeBet.columns.schedule'), formatter: (_, row) => `${formatDateTime(row.starts_at)} -> ${formatDateTime(row.settles_at)}` }, { key: 'actions', label: t('admin.sizeBet.columns.actions') }])
const betColumns = computed<Column[]>(() => [{ key: 'round_no', label: t('admin.sizeBet.columns.round') }, { key: 'username', label: t('admin.sizeBet.columns.user'), formatter: (_, row) => `${row.username} (#${row.user_id})` }, { key: 'direction', label: t('admin.sizeBet.columns.direction'), formatter: (value) => directionLabel(String(value)) }, { key: 'stake_amount', label: t('admin.sizeBet.columns.stake'), formatter: (value) => formatAmount(Number(value)) }, { key: 'payout_amount', label: t('admin.sizeBet.columns.payout'), formatter: (value) => formatAmount(Number(value)) }, { key: 'net_result_amount', label: t('admin.sizeBet.columns.net'), formatter: (value) => formatAmount(Number(value)) }, { key: 'status', label: t('admin.sizeBet.columns.status'), formatter: (value) => statusLabel(String(value)) }, { key: 'placed_at', label: t('admin.sizeBet.columns.createdAt'), formatter: (value) => formatDateTime(value) }])
const ledgerColumns = computed<Column[]>(() => [{ key: 'user_id', label: t('admin.sizeBet.columns.user'), formatter: (value) => `#${value}` }, { key: 'entry_type', label: t('admin.sizeBet.columns.entryType'), formatter: (value) => entryTypeLabel(String(value)) }, { key: 'direction', label: t('admin.sizeBet.columns.direction'), formatter: (value) => value ? directionLabel(String(value)) : '-' }, { key: 'stake_amount', label: t('admin.sizeBet.columns.stake'), formatter: (value) => formatAmount(Number(value)) }, { key: 'delta_amount', label: t('admin.sizeBet.columns.delta'), formatter: (value) => formatAmount(Number(value)) }, { key: 'balance_before', label: t('admin.sizeBet.columns.balanceWindow'), formatter: (_, row) => `${formatAmount(row.balance_before)} -> ${formatAmount(row.balance_after)}` }, { key: 'reason', label: t('admin.sizeBet.columns.reason'), formatter: (value) => value || '-' }, { key: 'created_at', label: t('admin.sizeBet.columns.createdAt'), formatter: (value) => formatDateTime(value) }])
const currentColumns = computed(() => activeTab.value === 'rounds' ? roundColumns.value : activeTab.value === 'bets' ? betColumns.value : ledgerColumns.value)

onMounted(() => { void loadSettings() })
watch(activeTab, (tab) => { if (tab !== 'settings') void loadActiveTab() })

function newTableState<T>(): TableState<T> { return { items: [], loading: false, loaded: false, pagination: { page: 1, page_size: pageSize, total: 0, pages: 1 } } }
function applySettings(settings: SizeBetAdminSettings) { Object.assign(form, { enabled: settings.enabled, round_duration_seconds: settings.round_duration_seconds, bet_close_offset_seconds: settings.bet_close_offset_seconds, allowed_stakes: [...settings.allowed_stakes], probabilities: { ...settings.probabilities }, odds: { ...settings.odds }, rules_markdown: settings.rules_markdown }); allowedStakesText.value = settings.allowed_stakes.join(', '); validationError.value = '' }
function parsePositiveInt(value: string) { const trimmed = value.trim(); if (!trimmed) return undefined; const parsed = Number(trimmed); return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined }
function parseAllowedStakes() { return Array.from(new Set(allowedStakesText.value.split(',').map(item => Number(item.trim())).filter(item => Number.isInteger(item) && item > 0))) }
function formatAmount(value: number) { return Number.isInteger(value) ? String(value) : value.toFixed(2) }
function directionLabel(value: string) { const key = `sizeBet.directions.${value}`; const translated = t(key); return translated === key ? value : translated }
function statusLabel(value: string) { const key = `admin.sizeBet.status.${value}`; const translated = t(key); return translated === key ? value : translated }
function entryTypeLabel(value: string) { const key = `admin.sizeBet.entryType.${value}`; const translated = t(key); return translated === key ? value : translated }
function isRoundRefundable(round: SizeBetAdminRound) { return round.status !== 'settled' || round.result_number == null }
function currentQueryFilters() { return activeTab.value === 'bets' ? { round_id: parsePositiveInt(betFilters.round_id), user_id: parsePositiveInt(betFilters.user_id), status: betFilters.status || undefined } : { round_id: parsePositiveInt(ledgerFilters.round_id), user_id: parsePositiveInt(ledgerFilters.user_id), entry_type: ledgerFilters.entry_type || undefined } }
function validateForm() { if (form.round_duration_seconds <= 0) return t('admin.sizeBet.validation.roundDuration'); if (form.bet_close_offset_seconds < 0 || form.bet_close_offset_seconds >= form.round_duration_seconds) return t('admin.sizeBet.validation.betCloseOffset'); if (!parseAllowedStakes().length) return t('admin.sizeBet.invalidAllowedStakes'); if (form.odds.small <= 0 || form.odds.mid <= 0 || form.odds.big <= 0) return t('admin.sizeBet.validation.odds'); return '' }

async function loadSettings() {
  settingsStatus.value = 'loading'
  settingsErrorMessage.value = ''
  try {
    applySettings(await sizeBetAdminAPI.getSettings())
    settingsStatus.value = 'ready'
  } catch (error: any) {
    settingsStatus.value = 'error'
    settingsErrorMessage.value = error?.message || t('admin.sizeBet.loadFailed')
    appStore.showError(settingsErrorMessage.value)
  }
}

async function saveSettings() {
  validationError.value = validateForm()
  if (!settingsReady.value || validationError.value) return
  savingSettings.value = true
  try {
    await sizeBetAdminAPI.updateSettings({ ...form, allowed_stakes: parseAllowedStakes() })
    appStore.showSuccess(t('admin.sizeBet.saveSuccess'))
    await loadSettings()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.sizeBet.saveFailed'))
  } finally {
    savingSettings.value = false
  }
}

async function loadActiveTab(force = false) {
  const state = currentTable.value
  if (activeTab.value === 'settings' || state.loading || (!force && state.loaded)) return
  state.loading = true
  try {
    const response = activeTab.value === 'rounds' ? await sizeBetAdminAPI.listRounds(state.pagination.page, state.pagination.page_size) : activeTab.value === 'bets' ? await sizeBetAdminAPI.listBets(state.pagination.page, state.pagination.page_size, currentQueryFilters()) : await sizeBetAdminAPI.listLedger(state.pagination.page, state.pagination.page_size, currentQueryFilters())
    state.items = response.items
    Object.assign(state.pagination, { total: response.total, pages: response.pages, page: response.page, page_size: response.page_size })
    state.loaded = true
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.sizeBet.loadFailed'))
  } finally {
    state.loading = false
  }
}

function applyFilters() { currentTable.value.pagination.page = 1; void loadActiveTab(true) }
function resetFilters() { Object.assign(activeTab.value === 'bets' ? betFilters : ledgerFilters, activeTab.value === 'bets' ? { round_id: '', user_id: '', status: '' } : { round_id: '', user_id: '', entry_type: '' }); applyFilters() }
function handlePageChange(page: number) { currentTable.value.pagination.page = page; void loadActiveTab(true) }
function handlePageSizeChange(nextPageSize: number) { Object.assign(currentTable.value.pagination, { page: 1, page_size: nextPageSize }); void loadActiveTab(true) }
function openRefundDialog(round: SizeBetAdminRound) { refundTarget.value = round; refundDialogOpen.value = true }

async function confirmRefund() {
  if (!refundTarget.value) return
  try {
    const result = await sizeBetAdminAPI.refundRound(refundTarget.value.id)
    appStore.showSuccess(t('admin.sizeBet.refundSuccess', { count: result.refunded_count }))
    refundDialogOpen.value = false
    await loadActiveTab(true)
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.sizeBet.refundFailed'))
  }
}
</script>
