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
              <button
                v-for="tab in tabs"
                :key="tab"
                type="button"
                class="rounded-full px-4 py-2 text-sm font-medium transition"
                :class="activeTab === tab ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'"
                @click="activeTab = tab"
              >
                {{ t(`admin.sizeBet.tabs.${tab}`) }}
              </button>
            </nav>
          </div>
        </div>

        <div v-if="activeTab === 'settings'" class="space-y-6 p-6">
          <div v-if="loadingSettings" class="flex justify-center py-10"><LoadingSpinner /></div>
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
                  <div><label class="input-label">{{ t('admin.sizeBet.roundDuration') }}</label><input v-model.number="form.round_duration_seconds" type="number" min="1" class="input" /></div>
                  <div><label class="input-label">{{ t('admin.sizeBet.betCloseOffset') }}</label><input v-model.number="form.bet_close_offset_seconds" type="number" min="0" class="input" /></div>
                </div>
                <div>
                  <label class="input-label">{{ t('admin.sizeBet.allowedStakes') }}</label>
                  <input v-model="allowedStakesText" type="text" class="input" :placeholder="t('admin.sizeBet.allowedStakesPlaceholder')" />
                </div>
              </div>

              <div class="grid gap-4">
                <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                  <p class="mb-4 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.sizeBet.probabilitiesTitle') }}</p>
                  <div class="grid gap-4 sm:grid-cols-3">
                    <div><label class="input-label">{{ t('sizeBet.directions.small') }}</label><input v-model.number="form.probabilities.small" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.mid') }}</label><input v-model.number="form.probabilities.mid" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.big') }}</label><input v-model.number="form.probabilities.big" type="number" step="0.1" class="input" /></div>
                  </div>
                </div>
                <div class="rounded-2xl border border-gray-100 p-5 dark:border-dark-700">
                  <p class="mb-4 text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.sizeBet.oddsTitle') }}</p>
                  <div class="grid gap-4 sm:grid-cols-3">
                    <div><label class="input-label">{{ t('sizeBet.directions.small') }}</label><input v-model.number="form.odds.small" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.mid') }}</label><input v-model.number="form.odds.mid" type="number" step="0.1" class="input" /></div>
                    <div><label class="input-label">{{ t('sizeBet.directions.big') }}</label><input v-model.number="form.odds.big" type="number" step="0.1" class="input" /></div>
                  </div>
                </div>
              </div>
            </div>

            <div>
              <label class="input-label">{{ t('admin.sizeBet.rulesMarkdown') }}</label>
              <textarea v-model="form.rules_markdown" rows="8" class="input"></textarea>
            </div>

            <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
              <button data-test="save-settings" type="button" class="btn btn-primary" :disabled="savingSettings" @click="saveSettings">
                {{ savingSettings ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </template>
        </div>

        <div v-else class="space-y-4 p-6">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t(`admin.sizeBet.tabs.${activeTab}`) }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`admin.sizeBet.tabDescriptions.${activeTab}`) }}</p>
            </div>
            <button type="button" class="btn btn-secondary" :disabled="currentTable.loading" @click="loadActiveTab(true)">{{ t('common.refresh') }}</button>
          </div>

          <DataTable :columns="currentColumns" :data="currentTable.items" :loading="currentTable.loading" />
          <Pagination
            v-if="currentTable.pagination.total > 0"
            :page="currentTable.pagination.page"
            :total="currentTable.pagination.total"
            :page-size="currentTable.pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import * as sizeBetAdminAPI from '@/api/admin/sizeBet'
import type { SizeBetAdminBet, SizeBetAdminLedgerEntry, SizeBetAdminRound, SizeBetAdminSettings } from '@/api/admin/sizeBet'

type AuditTab = 'settings' | 'rounds' | 'bets' | 'ledger'
type TableState<T> = { items: T[]; loading: boolean; loaded: boolean; pagination: { page: number; page_size: number; total: number; pages: number } }

const pageSize = getPersistedPageSize()
const { t } = useI18n()
const appStore = useAppStore()
const tabs: AuditTab[] = ['settings', 'rounds', 'bets', 'ledger']
const activeTab = ref<AuditTab>('settings')
const loadingSettings = ref(false)
const savingSettings = ref(false)
const allowedStakesText = ref('2, 5, 10, 20')
const form = reactive<SizeBetAdminSettings>({ enabled: true, round_duration_seconds: 60, bet_close_offset_seconds: 50, allowed_stakes: [2, 5, 10, 20], probabilities: { small: 45, mid: 10, big: 45 }, odds: { small: 2, mid: 10, big: 2 }, rules_markdown: '' })
const roundsState = reactive<TableState<SizeBetAdminRound>>(createTableState())
const betsState = reactive<TableState<SizeBetAdminBet>>(createTableState())
const ledgerState = reactive<TableState<SizeBetAdminLedgerEntry>>(createTableState())

const roundColumns = computed<Column[]>(() => [
  { key: 'round_no', label: t('admin.sizeBet.columns.round') },
  { key: 'status', label: t('admin.sizeBet.columns.status'), formatter: (value) => formatStatus(String(value)) },
  { key: 'result_number', label: t('admin.sizeBet.columns.result'), formatter: (_, row) => row.result_number == null || !row.result_direction ? '-' : `${row.result_number} / ${formatDirection(row.result_direction)}` },
  { key: 'starts_at', label: t('admin.sizeBet.columns.schedule'), formatter: (_, row) => `${formatDateTime(row.starts_at)} → ${formatDateTime(row.settles_at)}` },
  { key: 'prob_small', label: t('admin.sizeBet.columns.probabilities'), formatter: (_, row) => `${row.prob_small}/${row.prob_mid}/${row.prob_big}` },
  { key: 'odds_small', label: t('admin.sizeBet.columns.odds'), formatter: (_, row) => `${formatAmount(row.odds_small)}/${formatAmount(row.odds_mid)}/${formatAmount(row.odds_big)}` }
])
const betColumns = computed<Column[]>(() => [
  { key: 'round_no', label: t('admin.sizeBet.columns.round') },
  { key: 'username', label: t('admin.sizeBet.columns.user'), formatter: (_, row) => `${row.username} (#${row.user_id})` },
  { key: 'direction', label: t('admin.sizeBet.columns.direction'), formatter: (value) => formatDirection(String(value)) },
  { key: 'stake_amount', label: t('admin.sizeBet.columns.stake'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'payout_amount', label: t('admin.sizeBet.columns.payout'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'net_result_amount', label: t('admin.sizeBet.columns.net'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'status', label: t('admin.sizeBet.columns.status'), formatter: (value) => formatStatus(String(value)) },
  { key: 'placed_at', label: t('admin.sizeBet.columns.createdAt'), formatter: (value) => formatDateTime(value) }
])
const ledgerColumns = computed<Column[]>(() => [
  { key: 'user_id', label: t('admin.sizeBet.columns.user'), formatter: (value) => `#${value}` },
  { key: 'entry_type', label: t('admin.sizeBet.columns.entryType'), formatter: (value) => formatEntryType(String(value)) },
  { key: 'direction', label: t('admin.sizeBet.columns.direction'), formatter: (value) => value ? formatDirection(String(value)) : '-' },
  { key: 'stake_amount', label: t('admin.sizeBet.columns.stake'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'delta_amount', label: t('admin.sizeBet.columns.delta'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'balance_before', label: t('admin.sizeBet.columns.balanceWindow'), formatter: (_, row) => `${formatAmount(row.balance_before)} → ${formatAmount(row.balance_after)}` },
  { key: 'reason', label: t('admin.sizeBet.columns.reason'), formatter: (value) => value || '-' },
  { key: 'created_at', label: t('admin.sizeBet.columns.createdAt'), formatter: (value) => formatDateTime(value) }
])

const currentTable = computed(() => activeTab.value === 'rounds' ? roundsState : activeTab.value === 'bets' ? betsState : ledgerState)
const currentColumns = computed(() => activeTab.value === 'rounds' ? roundColumns.value : activeTab.value === 'bets' ? betColumns.value : ledgerColumns.value)

onMounted(() => { void loadSettings() })
watch(activeTab, (tab) => { if (tab !== 'settings') void loadActiveTab() })

function createTableState<T>(): TableState<T> { return { items: [], loading: false, loaded: false, pagination: { page: 1, page_size: pageSize, total: 0, pages: 1 } } }
function applySettings(settings: SizeBetAdminSettings) { form.enabled = settings.enabled; form.round_duration_seconds = settings.round_duration_seconds; form.bet_close_offset_seconds = settings.bet_close_offset_seconds; form.allowed_stakes = [...settings.allowed_stakes]; form.probabilities = { ...settings.probabilities }; form.odds = { ...settings.odds }; form.rules_markdown = settings.rules_markdown; allowedStakesText.value = settings.allowed_stakes.join(', ') }
function parseAllowedStakes() { return Array.from(new Set(allowedStakesText.value.split(',').map(item => Number(item.trim())).filter(item => Number.isInteger(item) && item > 0))) }
function formatAmount(value: number) { return Number.isInteger(value) ? String(value) : value.toFixed(2) }
function formatDirection(value: string) { return t(`sizeBet.directions.${value}`) }
function formatStatus(value: string) { const key = `admin.sizeBet.status.${value}`; const translated = t(key); return translated === key ? value : translated }
function formatEntryType(value: string) { const key = `admin.sizeBet.entryType.${value}`; const translated = t(key); return translated === key ? value : translated }

async function loadSettings() {
  loadingSettings.value = true
  try {
    applySettings(await sizeBetAdminAPI.getSettings())
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.sizeBet.loadFailed'))
  } finally {
    loadingSettings.value = false
  }
}

async function saveSettings() {
  const allowedStakes = parseAllowedStakes()
  if (!allowedStakes.length) { appStore.showError(t('admin.sizeBet.invalidAllowedStakes')); return }
  savingSettings.value = true
  try {
    await sizeBetAdminAPI.updateSettings({ ...form, allowed_stakes: allowedStakes })
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
    const loader = activeTab.value === 'rounds' ? sizeBetAdminAPI.listRounds : activeTab.value === 'bets' ? sizeBetAdminAPI.listBets : sizeBetAdminAPI.listLedger
    const response = await loader(state.pagination.page, state.pagination.page_size)
    state.items = response.items
    state.pagination.total = response.total
    state.pagination.pages = response.pages
    state.loaded = true
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.sizeBet.loadFailed'))
  } finally {
    state.loading = false
  }
}

function handlePageChange(page: number) { currentTable.value.pagination.page = page; void loadActiveTab(true) }
function handlePageSizeChange(nextPageSize: number) { currentTable.value.pagination.page_size = nextPageSize; currentTable.value.pagination.page = 1; void loadActiveTab(true) }
</script>
