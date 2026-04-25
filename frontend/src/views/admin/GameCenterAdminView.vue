<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card border border-gray-100 p-6 dark:border-dark-700">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.description') }}</p>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadData">
            {{ t('common.refresh') }}
          </button>
        </div>
      </section>

      <section v-if="loading" class="card border border-gray-100 p-6 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-300">
        {{ t('common.loading') }}
      </section>

      <template v-else>
        <section data-test="claim-config-section" class="card space-y-4 border border-gray-100 p-6 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.sections.claim') }}</h2>
          <div class="grid gap-4 lg:grid-cols-2">
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.gameCenter.switches.gameCenterEnabled') }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.switches.gameCenterEnabledHint') }}</p>
                </div>
                <Toggle v-model="form.game_center_enabled" />
              </div>
            </div>
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.gameCenter.switches.claimEnabled') }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.switches.claimEnabledHint') }}</p>
                </div>
                <Toggle v-model="form.claim_enabled" />
              </div>
            </div>
          </div>

          <div class="flex justify-end">
            <button type="button" class="btn btn-secondary btn-sm" @click="addClaimBatch">
              {{ t('admin.gameCenter.actions.addBatch') }}
            </button>
          </div>

          <div class="space-y-3">
            <div
              v-for="(batch, index) in form.claim_schedule"
              :key="`${batch.batch_key}-${index}`"
              class="grid gap-3 rounded-xl border border-gray-100 p-4 sm:grid-cols-4 dark:border-dark-700"
            >
              <div>
                <label class="input-label">{{ t('admin.gameCenter.claim.batchKey') }}</label>
                <input v-model="batch.batch_key" :data-test="`claim-batch-key-${index}`" type="text" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.gameCenter.claim.claimTime') }}</label>
                <input v-model="batch.claim_time" :data-test="`claim-time-${index}`" type="time" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.gameCenter.claim.pointsAmount') }}</label>
                <input v-model.number="batch.points_amount" :data-test="`claim-points-${index}`" type="number" min="0" class="input" />
              </div>
              <div class="flex items-end justify-between gap-3">
                <div>
                  <p class="input-label">{{ t('admin.gameCenter.claim.enabled') }}</p>
                  <Toggle v-model="batch.enabled" />
                </div>
                <button type="button" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click="removeClaimBatch(index)">
                  {{ t('common.delete') }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <section data-test="exchange-config-section" class="card space-y-4 border border-gray-100 p-6 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.sections.exchange') }}</h2>
          <div class="grid gap-4 lg:grid-cols-2">
            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-4">
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.gameCenter.exchange.balanceToPoints') }}</p>
                <Toggle v-model="form.exchange.balance_to_points_enabled" />
              </div>
              <div class="mt-3 grid gap-3 sm:grid-cols-2">
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.exchange.rate') }}</label>
                  <input v-model.number="form.exchange.balance_to_points_rate" data-test="exchange-balance-rate" type="number" min="0" class="input" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.exchange.minAmount') }}</label>
                  <input v-model.number="form.exchange.min_balance_amount" data-test="exchange-balance-min" type="number" min="0" class="input" />
                </div>
              </div>
            </div>

            <div class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-4">
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.gameCenter.exchange.pointsToBalance') }}</p>
                <Toggle v-model="form.exchange.points_to_balance_enabled" />
              </div>
              <div class="mt-3 grid gap-3 sm:grid-cols-2">
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.exchange.rate') }}</label>
                  <input v-model.number="form.exchange.points_to_balance_rate" data-test="exchange-points-rate" type="number" min="0" class="input" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.exchange.minAmount') }}</label>
                  <input v-model.number="form.exchange.min_points_amount" data-test="exchange-points-min" type="number" min="0" class="input" />
                </div>
              </div>
            </div>
          </div>
        </section>

        <section data-test="catalog-config-section" class="card space-y-4 border border-gray-100 p-6 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.sections.catalog') }}</h2>

          <div v-if="!catalogItems.length" class="rounded-xl border border-dashed border-gray-200 p-5 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
            {{ t('admin.gameCenter.catalog.empty') }}
          </div>

          <div v-else class="space-y-3">
            <article
              v-for="item in catalogItems"
              :key="item.game_key"
              :data-test="`catalog-item-${item.game_key}`"
              class="rounded-xl border border-gray-100 p-4 dark:border-dark-700"
            >
              <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</h3>
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ item.subtitle || item.description }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.enabled') }}</span>
                  <Toggle v-model="item.enabled" />
                </div>
              </div>

              <div class="mt-3 grid gap-3 sm:grid-cols-3">
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.catalog.sortOrder') }}</label>
                  <input v-model.number="item.sort_order" type="number" class="input" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.gameCenter.catalog.openMode') }}</label>
                  <select v-model="item.default_open_mode" class="input">
                    <option value="dual">{{ t('admin.gameCenter.catalog.openModes.dual') }}</option>
                    <option value="embed">{{ t('admin.gameCenter.catalog.openModes.embed') }}</option>
                    <option value="standalone">{{ t('admin.gameCenter.catalog.openModes.standalone') }}</option>
                  </select>
                </div>
                <div class="grid grid-cols-2 gap-3">
                  <div class="rounded-lg border border-gray-100 px-3 py-2 dark:border-dark-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.catalog.supportsEmbed') }}</p>
                    <Toggle v-model="item.supports_embed" />
                  </div>
                  <div class="rounded-lg border border-gray-100 px-3 py-2 dark:border-dark-700">
                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.catalog.supportsStandalone') }}</p>
                    <Toggle v-model="item.supports_standalone" />
                  </div>
                </div>
              </div>

              <div class="mt-3 flex justify-end">
                <RouterLink
                  v-if="item.game_key === 'size_bet'"
                  class="btn btn-secondary btn-sm mr-2"
                  to="/admin/games/size-bet"
                >
                  {{ t('admin.gameCenter.catalog.openGameSettings') }}
                </RouterLink>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :data-test="`save-catalog-${item.game_key}`"
                  :disabled="savingCatalogKey === item.game_key"
                  @click="saveCatalog(item)"
                >
                  {{ t('common.save') }}
                </button>
              </div>
            </article>
          </div>
        </section>

        <section data-test="operations-section" class="card space-y-4 border border-gray-100 p-6 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.sections.operations') }}</h2>
          <div class="grid gap-4 lg:grid-cols-[1fr_auto]">
            <div class="grid gap-3 sm:grid-cols-3">
              <div>
                <label class="input-label">{{ t('admin.gameCenter.operations.userId') }}</label>
                <input v-model.number="adjustForm.user_id" data-test="adjust-user-id" type="number" min="1" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.gameCenter.operations.deltaPoints') }}</label>
                <input v-model.number="adjustForm.delta_points" data-test="adjust-delta" type="number" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.gameCenter.operations.reason') }}</label>
                <input v-model="adjustForm.reason" data-test="adjust-reason" type="text" class="input" />
              </div>
            </div>
            <div class="flex items-end">
              <button
                type="button"
                class="btn btn-primary"
                data-test="submit-adjust"
                :disabled="adjustingPoints"
                @click="submitAdjustPoints"
              >
                {{ adjustingPoints ? t('common.saving') : t('admin.gameCenter.operations.submit') }}
              </button>
            </div>
          </div>
        </section>

        <section data-test="audit-section" class="card space-y-5 border border-gray-100 p-6 dark:border-dark-700">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.sections.audit') }}</h2>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.audit.description') }}</p>
            </div>
            <div class="flex flex-wrap items-end gap-3">
              <div>
                <label class="input-label">{{ t('admin.gameCenter.operations.userId') }}</label>
                <input v-model.number="auditUserID" type="number" min="1" class="input w-32" />
              </div>
              <button type="button" class="btn btn-secondary" @click="loadAuditData">{{ t('common.refresh') }}</button>
            </div>
          </div>

          <div class="grid gap-4 xl:grid-cols-3">
            <article class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.audit.ledger') }}</h3>
              <div class="mt-3 space-y-2">
                <div v-for="item in ledgerItems" :key="`ledger-${item.id}`" class="rounded-lg bg-gray-50 px-3 py-3 text-sm dark:bg-dark-800">
                  <div class="flex items-center justify-between gap-3">
                    <span class="font-medium text-gray-900 dark:text-white">#{{ item.user_id }} · {{ item.entry_type }}</span>
                    <span :class="item.delta_points >= 0 ? 'text-emerald-600' : 'text-rose-600'">{{ formatSigned(item.delta_points) }}</span>
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.reason || '--' }}</p>
                </div>
                <p v-if="!ledgerItems.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.audit.empty') }}</p>
              </div>
            </article>

            <article class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.audit.claims') }}</h3>
              <div class="mt-3 space-y-2">
                <div v-for="item in claimItems" :key="`claim-${item.id}`" class="rounded-lg bg-gray-50 px-3 py-3 text-sm dark:bg-dark-800">
                  <div class="flex items-center justify-between gap-3">
                    <span class="font-medium text-gray-900 dark:text-white">#{{ item.user_id }} · {{ item.batch_key }}</span>
                    <span class="text-emerald-600">+{{ item.points_amount }}</span>
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.claim_date }}</p>
                </div>
                <p v-if="!claimItems.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.audit.empty') }}</p>
              </div>
            </article>

            <article class="rounded-xl border border-gray-100 p-4 dark:border-dark-700">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.gameCenter.audit.exchanges') }}</h3>
              <div class="mt-3 space-y-2">
                <div v-for="item in exchangeItems" :key="`exchange-${item.id}`" class="rounded-lg bg-gray-50 px-3 py-3 text-sm dark:bg-dark-800">
                  <div class="flex items-center justify-between gap-3">
                    <span class="font-medium text-gray-900 dark:text-white">#{{ item.user_id }} · {{ item.direction }}</span>
                    <span class="text-sky-600">{{ item.rate }}</span>
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.reason || item.status }}</p>
                </div>
                <p v-if="!exchangeItems.length" class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.gameCenter.audit.empty') }}</p>
              </div>
            </article>
          </div>
        </section>

        <div class="flex justify-end">
          <button data-test="save-settings" type="button" class="btn btn-primary" :disabled="savingSettings" @click="saveSettings">
            {{ savingSettings ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import * as gameCenterAdminAPI from '@/api/admin/gameCenter'
import type {
  GameCenterAdminSettings,
  GameCenterAdminLedgerItem,
  GameCenterCatalogItem,
  GameCenterClaimRecord,
  GameCenterExchangeRecord,
  UpdateGameCenterCatalogRequest,
  UpdateGameCenterSettingsRequest
} from '@/api/admin/gameCenter'
import Toggle from '@/components/common/Toggle.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const savingSettings = ref(false)
const savingCatalogKey = ref('')
const adjustingPoints = ref(false)
const auditUserID = ref<number | null>(null)

const form = reactive<UpdateGameCenterSettingsRequest>(defaultSettings())
const catalogItems = ref<GameCenterCatalogItem[]>([])
const ledgerItems = ref<GameCenterAdminLedgerItem[]>([])
const claimItems = ref<GameCenterClaimRecord[]>([])
const exchangeItems = ref<GameCenterExchangeRecord[]>([])
const adjustForm = reactive({
  user_id: 0,
  delta_points: 0,
  reason: ''
})

onMounted(() => {
  void loadData()
})

function defaultSettings(): UpdateGameCenterSettingsRequest {
  return {
    game_center_enabled: false,
    claim_enabled: false,
    claim_schedule: [],
    exchange: {
      balance_to_points_enabled: false,
      points_to_balance_enabled: false,
      balance_to_points_rate: 0,
      points_to_balance_rate: 0,
      min_balance_amount: 0,
      min_points_amount: 0
    }
  }
}

function cloneSettings(source: GameCenterAdminSettings): UpdateGameCenterSettingsRequest {
  const claimSchedule = Array.isArray(source.claim_schedule) ? source.claim_schedule : []
  const exchange = source.exchange ?? {
    balance_to_points_enabled: false,
    points_to_balance_enabled: false,
    balance_to_points_rate: 0,
    points_to_balance_rate: 0,
    min_balance_amount: 0,
    min_points_amount: 0
  }
  return {
    game_center_enabled: Boolean(source.game_center_enabled),
    claim_enabled: Boolean(source.claim_enabled),
    claim_schedule: claimSchedule.map(item => ({
      batch_key: item.batch_key,
      claim_time: item.claim_time,
      points_amount: Number(item.points_amount) || 0,
      enabled: Boolean(item.enabled)
    })),
    exchange: {
      balance_to_points_enabled: Boolean(exchange.balance_to_points_enabled),
      points_to_balance_enabled: Boolean(exchange.points_to_balance_enabled),
      balance_to_points_rate: Number(exchange.balance_to_points_rate) || 0,
      points_to_balance_rate: Number(exchange.points_to_balance_rate) || 0,
      min_balance_amount: Number(exchange.min_balance_amount) || 0,
      min_points_amount: Number(exchange.min_points_amount) || 0
    }
  }
}

function applySettings(source: GameCenterAdminSettings) {
  const next = cloneSettings(source)
  Object.assign(form, next)
}

function normalizeCatalogItem(item: GameCenterCatalogItem): GameCenterCatalogItem {
  return {
    game_key: item.game_key,
    name: item.name,
    subtitle: item.subtitle ?? '',
    cover_image: item.cover_image ?? '',
    description: item.description ?? '',
    enabled: Boolean(item.enabled),
    sort_order: Number(item.sort_order) || 0,
    default_open_mode: item.default_open_mode ?? 'dual',
    supports_embed: item.supports_embed !== false,
    supports_standalone: item.supports_standalone !== false
  }
}

function addClaimBatch() {
  form.claim_schedule.push({
    batch_key: `batch_${form.claim_schedule.length + 1}`,
    claim_time: '00:00',
    points_amount: 0,
    enabled: true
  })
}

function removeClaimBatch(index: number) {
  form.claim_schedule.splice(index, 1)
}

async function loadData() {
  loading.value = true
  try {
    const [settings, catalog] = await Promise.all([
      gameCenterAdminAPI.getSettings(),
      gameCenterAdminAPI.getCatalog()
    ])
    applySettings(settings)
    catalogItems.value = catalog.map(normalizeCatalogItem)
    await loadAuditData()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.gameCenter.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadAuditData() {
  try {
    const userID = auditUserID.value && auditUserID.value > 0 ? auditUserID.value : undefined
    const [ledger, claims, exchanges] = await Promise.all([
      gameCenterAdminAPI.listLedger(userID),
      gameCenterAdminAPI.listClaims(userID),
      gameCenterAdminAPI.listExchanges(userID)
    ])
    ledgerItems.value = ledger.items
    claimItems.value = claims.items
    exchangeItems.value = exchanges.items
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.gameCenter.loadFailed'))
  }
}

async function saveSettings() {
  savingSettings.value = true
  try {
    await gameCenterAdminAPI.updateSettings(cloneSettings(form))
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('admin.gameCenter.saveSettingsSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.gameCenter.saveSettingsFailed'))
  } finally {
    savingSettings.value = false
  }
}

async function saveCatalog(item: GameCenterCatalogItem) {
  const payload: UpdateGameCenterCatalogRequest = {
    enabled: item.enabled,
    sort_order: item.sort_order,
    default_open_mode: item.default_open_mode,
    supports_embed: item.supports_embed,
    supports_standalone: item.supports_standalone
  }

  savingCatalogKey.value = item.game_key
  try {
    await gameCenterAdminAPI.updateCatalog(item.game_key, payload)
    appStore.showSuccess(t('admin.gameCenter.saveCatalogSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.gameCenter.saveCatalogFailed'))
  } finally {
    savingCatalogKey.value = ''
  }
}

async function submitAdjustPoints() {
  if (!adjustForm.user_id || !adjustForm.delta_points || !adjustForm.reason.trim()) {
    appStore.showError(t('admin.gameCenter.operations.validation'))
    return
  }
  adjustingPoints.value = true
  try {
    await gameCenterAdminAPI.adjustPoints(adjustForm.user_id, {
      delta_points: adjustForm.delta_points,
      reason: adjustForm.reason.trim()
    })
    appStore.showSuccess(t('admin.gameCenter.operations.success'))
    adjustForm.delta_points = 0
    adjustForm.reason = ''
    await loadAuditData()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.gameCenter.operations.failed'))
  } finally {
    adjustingPoints.value = false
  }
}

function formatSigned(value: number) {
  return value >= 0 ? `+${value}` : `${value}`
}
</script>
