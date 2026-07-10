<template>
  <AppLayout>
    <div class="space-y-4">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.description') }}</p>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="reloadCurrentTab">{{ t('admin.zenxiangLiyu.refresh') }}</button>
        </div>
        <div class="mt-4 flex overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <button v-for="item in tabs" :key="item.id" type="button" :data-testid="`zenxiang-tab-${item.id}`" class="shrink-0 px-4 py-2 text-sm font-medium" :class="activeTab === item.id ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400' : 'text-gray-500 dark:text-dark-400'" @click="selectTab(item.id)">{{ item.label }}</button>
        </div>
      </section>

      <section v-if="activeTab === 'settings'" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex items-center justify-between gap-4"><div><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.settingsTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.settingsHint') }}</p></div><Toggle v-model="settingsForm.global_enabled" /></div>
        <div class="mt-4 grid gap-3 md:grid-cols-3">
          <label><span class="input-label">{{ t('admin.zenxiangLiyu.ticketAmount') }}</span><input v-model.number="settingsForm.ticket_amount" type="number" min="0" step="0.01" class="input" /></label>
          <label><span class="input-label">{{ t('admin.zenxiangLiyu.minimumBalance') }}</span><input v-model.number="settingsForm.minimum_balance" type="number" min="0" step="0.01" class="input" /></label>
          <label><span class="input-label">{{ t('admin.zenxiangLiyu.dailyPlayLimit') }}</span><input v-model.number="settingsForm.daily_play_limit" type="number" min="1" step="1" class="input" /></label>
        </div>
        <div class="mt-4 flex justify-end"><button data-testid="zenxiang-save-settings" class="btn btn-primary" :disabled="savingSettings" @click="saveSettings">{{ t('admin.zenxiangLiyu.saveSettings') }}</button></div>
      </section>

      <section v-else-if="activeTab === 'prizes'" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-start justify-between gap-3"><div><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.prizeTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.prizeHint') }}</p></div><button class="btn btn-secondary" @click="addPrize">{{ t('admin.zenxiangLiyu.addPrize') }}</button></div>
        <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Metric :label="t('admin.zenxiangLiyu.probabilityTotal')" :value="formatNumber(probabilityTotal) + '%'" :warn="probabilityTotal !== 100" />
          <Metric :label="t('admin.zenxiangLiyu.theoreticalExpense')" :value="formatAmount(theoreticalExpense)" />
          <Metric :label="t('admin.zenxiangLiyu.theoreticalProfit')" :value="formatAmount(theoreticalProfit)" />
          <Metric :label="t('admin.zenxiangLiyu.theoreticalProfitRate')" :value="formatPercent(theoreticalProfitRate)" />
        </div>
        <p v-if="probabilityTotal !== 100" class="mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-500/10 dark:text-amber-300">{{ t('admin.zenxiangLiyu.probabilityWarning', { total: formatNumber(probabilityTotal) }) }}</p>
        <div class="mt-4 overflow-x-auto"><table class="w-full min-w-[840px] divide-y divide-gray-200 dark:divide-dark-700"><thead><tr class="text-left text-xs text-gray-500 dark:text-dark-400"><th class="p-2">{{ t('admin.zenxiangLiyu.prizeName') }}</th><th class="p-2">{{ t('admin.zenxiangLiyu.rewardAmount') }}</th><th class="p-2">{{ t('admin.zenxiangLiyu.probability') }}</th><th class="p-2">{{ t('admin.zenxiangLiyu.enabled') }}</th><th class="p-2">{{ t('admin.zenxiangLiyu.sortOrder') }}</th><th class="p-2"></th></tr></thead><tbody class="divide-y divide-gray-200 dark:divide-dark-700"><tr v-for="(prize, index) in prizes" :key="prize.id ?? `new-${index}`"><td class="p-2"><input v-model.trim="prize.name" class="input" /></td><td class="p-2"><input v-model.number="prize.reward_amount" type="number" min="0" step="0.01" class="input" /></td><td class="p-2"><input v-model.number="prize.probability" type="number" min="0" max="100" step="0.01" class="input" /></td><td class="p-2"><Toggle v-model="prize.enabled" /></td><td class="p-2"><input v-model.number="prize.sort_order" type="number" min="0" step="1" class="input" /></td><td class="p-2 text-right"><button class="btn btn-secondary" @click="prizes.splice(index, 1)">{{ t('admin.zenxiangLiyu.remove') }}</button></td></tr></tbody></table></div>
        <div class="mt-4 flex justify-end"><button data-testid="zenxiang-save-prizes" class="btn btn-primary" :disabled="savingPrizes || probabilityTotal !== 100" @click="savePrizes">{{ t('admin.zenxiangLiyu.savePrizeConfiguration') }}</button></div>
      </section>

      <section v-else-if="activeTab === 'stats'" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"><div class="flex flex-wrap items-end justify-between gap-3"><div class="grid gap-3 sm:grid-cols-2"><label><span class="input-label">{{ t('admin.zenxiangLiyu.startDate') }}</span><input v-model="statsDates.start" type="date" class="input" /></label><label><span class="input-label">{{ t('admin.zenxiangLiyu.endDate') }}</span><input v-model="statsDates.end" type="date" class="input" /></label></div><button class="btn btn-secondary" @click="loadStats">{{ t('admin.zenxiangLiyu.refreshStats') }}</button></div></div>
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5"><Metric :label="t('admin.zenxiangLiyu.systemRevenue')" :value="formatAmount(overview.total_revenue)" /><Metric :label="t('admin.zenxiangLiyu.systemExpense')" :value="formatAmount(overview.total_expense)" /><Metric :label="t('admin.zenxiangLiyu.systemProfit')" :value="formatAmount(overview.net_profit)" /><Metric :label="t('admin.zenxiangLiyu.profitRate')" :value="formatPercent(overviewProfitRate)" /><Metric :label="t('admin.zenxiangLiyu.participantsAndPlays')" :value="`${overview.participating_users} / ${overview.total_plays}`" /></div>
        <div class="grid gap-4 xl:grid-cols-2"><StatsTable :title="t('admin.zenxiangLiyu.userStats')" :headers="[t('admin.zenxiangLiyu.user'), t('admin.zenxiangLiyu.plays'), t('admin.zenxiangLiyu.ticketTotal'), t('admin.zenxiangLiyu.rewardTotal'), t('admin.zenxiangLiyu.netTotal')]" :rows="userStats.map(row => [row.user_email || String(row.user_id), row.play_count, formatAmount(row.ticket_amount), formatAmount(row.reward_amount), formatAmount(row.user_net_amount)])" /><StatsTable :title="t('admin.zenxiangLiyu.prizeStats')" :headers="[t('admin.zenxiangLiyu.prizeName'), t('admin.zenxiangLiyu.probability'), t('admin.zenxiangLiyu.hitCount'), t('admin.zenxiangLiyu.actualRate')]" :rows="prizeStats.map(row => [row.prize_name, `${row.probability}%`, row.hit_count, formatPercent(overview.total_plays ? row.hit_count / overview.total_plays : 0)])" /></div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.grants') }}</h2><div class="mt-3 flex gap-2"><input v-model.number="grantUserId" type="number" min="1" class="input" :placeholder="t('admin.zenxiangLiyu.grantUserId')" /><button class="btn btn-primary" :disabled="!grantUserId" @click="createGrant">{{ t('admin.zenxiangLiyu.grant') }}</button></div><div class="mt-3 divide-y divide-gray-200 dark:divide-dark-700"><div v-for="grant in grants" :key="grant.user_id" class="flex items-center justify-between gap-3 py-2 text-sm"><span>{{ grant.user_email || `#${grant.user_id}` }}</span><button class="btn btn-secondary" @click="removeGrant(grant.user_id)">{{ t('admin.zenxiangLiyu.remove') }}</button></div><p v-if="grants.length === 0" class="py-2 text-sm text-gray-500">{{ t('admin.zenxiangLiyu.noGrants') }}</p></div></div>
      </section>

      <section v-else class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.simulatorTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.simulatorHint') }}</p><div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4"><label v-for="field in simulatorFields" :key="field.key"><span class="input-label">{{ field.label }}</span><input v-model.number="simulationForm[field.key]" type="number" :min="field.min" :step="field.step" class="input" /></label></div><div class="mt-4 flex flex-wrap justify-end gap-2"><button data-testid="zenxiang-recommend" class="btn btn-secondary" :disabled="simulating" @click="recommendConfiguration">{{ t('admin.zenxiangLiyu.recommend') }}</button><button data-testid="zenxiang-simulate" class="btn btn-primary" :disabled="simulating" @click="runSimulation">{{ t('admin.zenxiangLiyu.runSimulation') }}</button></div></div>
        <div v-if="simulationResult" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5"><Metric :label="t('admin.zenxiangLiyu.totalPlays')" :value="simulationResult.total_plays" /><Metric :label="t('admin.zenxiangLiyu.systemRevenue')" :value="formatAmount(simulationResult.total_revenue)" /><Metric :label="t('admin.zenxiangLiyu.systemExpense')" :value="formatAmount(simulationResult.total_expense)" /><Metric :label="t('admin.zenxiangLiyu.systemProfit')" :value="formatAmount(simulationResult.net_profit)" /><Metric :label="t('admin.zenxiangLiyu.profitRate')" :value="formatPercent(simulationResult.profit_rate)" /></div>
        <div v-if="recommendationPlans.length" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.recommendations') }}</h2><div class="mt-3 space-y-2"><div v-for="(plan, index) in recommendationPlans" :key="index" class="flex flex-col gap-2 rounded-md border border-gray-200 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700"><span class="text-sm text-gray-600 dark:text-dark-300">{{ t('admin.zenxiangLiyu.planSummary', { profit: formatAmount(plan.theory_profit), rate: formatPercent(plan.theory_profit_rate) }) }}</span><button :data-testid="`zenxiang-apply-recommendation-${index}`" class="btn btn-secondary" @click="applyRecommendation(plan.prizes)">{{ t('admin.zenxiangLiyu.applyToLocalPrizes') }}</button></div></div></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { ZenxiangLiyuGrant, ZenxiangLiyuOverviewStats, ZenxiangLiyuPrizeInput, ZenxiangLiyuPrizeStats, ZenxiangLiyuSimulationResult, ZenxiangLiyuUserStats } from '@/api/admin/zenxiangLiyu'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const activeTab = ref('settings')
const loading = ref(false), savingSettings = ref(false), savingPrizes = ref(false), simulating = ref(false)
const settingsForm = reactive({ global_enabled: false, ticket_amount: 0, minimum_balance: 0, daily_play_limit: 1 })
const prizes = ref<ZenxiangLiyuPrizeInput[]>([])
const grants = ref<ZenxiangLiyuGrant[]>([]), grantUserId = ref<number | null>(null)
const overview = ref<ZenxiangLiyuOverviewStats>({ total_plays: 0, total_revenue: 0, total_expense: 0, net_profit: 0, participating_users: 0 })
const userStats = ref<ZenxiangLiyuUserStats[]>([]), prizeStats = ref<ZenxiangLiyuPrizeStats[]>([])
const statsDates = reactive({ start: '', end: '' })
const simulationForm = reactive({ user_count: 100, plays_per_user: 3, initial_balance: 100, ticket_amount: 2, minimum_balance: 10, daily_play_limit: 3, target_profit_rate: 0.1 })
const simulationResult = ref<ZenxiangLiyuSimulationResult | null>(null)
const recommendationPlans = ref<Array<{ prizes: ZenxiangLiyuPrizeInput[], theory_profit: number, theory_profit_rate: number }>>([])
const tabs = computed(() => [{ id: 'settings', label: t('admin.zenxiangLiyu.settings') }, { id: 'prizes', label: t('admin.zenxiangLiyu.prizes') }, { id: 'stats', label: t('admin.zenxiangLiyu.stats') }, { id: 'simulator', label: t('admin.zenxiangLiyu.simulator') }])
const simulatorFields = computed(() => [{ key: 'user_count' as const, label: t('admin.zenxiangLiyu.userCount'), min: 1, step: 1 }, { key: 'plays_per_user' as const, label: t('admin.zenxiangLiyu.playsPerUser'), min: 1, step: 1 }, { key: 'initial_balance' as const, label: t('admin.zenxiangLiyu.initialBalance'), min: 0, step: 0.01 }, { key: 'ticket_amount' as const, label: t('admin.zenxiangLiyu.ticketAmount'), min: 0, step: 0.01 }, { key: 'minimum_balance' as const, label: t('admin.zenxiangLiyu.minimumBalance'), min: 0, step: 0.01 }, { key: 'daily_play_limit' as const, label: t('admin.zenxiangLiyu.dailyPlayLimit'), min: 1, step: 1 }, { key: 'target_profit_rate' as const, label: t('admin.zenxiangLiyu.targetProfitRate'), min: 0, step: 0.01 }])
const enabledPrizes = computed(() => prizes.value.filter((prize) => prize.enabled))
const probabilityTotal = computed(() => enabledPrizes.value.reduce((sum, prize) => sum + Number(prize.probability || 0), 0))
const theoreticalExpense = computed(() => enabledPrizes.value.reduce((sum, prize) => sum + Number(prize.reward_amount || 0) * Number(prize.probability || 0) / 100, 0))
const theoreticalProfit = computed(() => Number(settingsForm.ticket_amount || 0) - theoreticalExpense.value)
const theoreticalProfitRate = computed(() => settingsForm.ticket_amount > 0 ? theoreticalProfit.value / settingsForm.ticket_amount : 0)
const overviewProfitRate = computed(() => overview.value.total_revenue ? overview.value.net_profit / overview.value.total_revenue : 0)
const Metric = defineComponent({ props: { label: { type: String, required: true }, value: { type: [String, Number], required: true }, warn: Boolean }, setup(props) { return () => h('div', { class: ['rounded-md border p-3', props.warn ? 'border-amber-300 bg-amber-50 dark:border-amber-700 dark:bg-amber-500/10' : 'border-gray-200 dark:border-dark-700'] }, [h('p', { class: 'text-xs text-gray-500 dark:text-dark-400' }, props.label), h('p', { class: 'mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white' }, String(props.value))]) } })
const StatsTable = defineComponent({ props: { title: { type: String, required: true }, headers: { type: Array as () => string[], required: true }, rows: { type: Array as () => Array<Array<string | number>>, required: true } }, setup(props) { return () => h('div', { class: 'overflow-x-auto rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900' }, [h('h2', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title), h('table', { class: 'mt-3 w-full text-sm' }, [h('thead', props.headers.map((header) => h('th', { class: 'p-2 text-left text-xs text-gray-500 dark:text-dark-400' }, header))), h('tbody', props.rows.map((row) => h('tr', { class: 'border-t border-gray-200 dark:border-dark-700' }, row.map((cell) => h('td', { class: 'p-2 text-gray-700 dark:text-dark-200' }, String(cell))))))])]) } })
function formatNumber(value: number) { return Number(value || 0).toFixed(2).replace(/\.00$/, '') }
function formatAmount(value: number) { return formatNumber(value) }
function formatPercent(value: number) { return `${formatNumber(value * 100)}%` }
function copyPrizes(value: ZenxiangLiyuPrizeInput[]) { return value.map((prize) => ({ ...prize })) }
async function loadCore() { loading.value = true; try { const [settings, prizeList] = await Promise.all([adminAPI.zenxiangLiyu.getSettings(), adminAPI.zenxiangLiyu.listPrizes()]); Object.assign(settingsForm, settings); Object.assign(simulationForm, { ticket_amount: settings.ticket_amount, minimum_balance: settings.minimum_balance, daily_play_limit: settings.daily_play_limit }); prizes.value = copyPrizes(prizeList) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.loadFailed'))) } finally { loading.value = false } }
async function loadStats() { loading.value = true; try { const [overviewResult, usersResult, prizesResult, grantsResult] = await Promise.all([adminAPI.zenxiangLiyu.getOverviewStats(), adminAPI.zenxiangLiyu.listUserStats(), adminAPI.zenxiangLiyu.listPrizeStats(), adminAPI.zenxiangLiyu.listGrants()]); overview.value = overviewResult; userStats.value = usersResult.items; prizeStats.value = prizesResult; grants.value = grantsResult.items } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.statsLoadFailed'))) } finally { loading.value = false } }
async function selectTab(tab: string) { activeTab.value = tab; if (tab === 'stats') await loadStats() }
async function reloadCurrentTab() { if (activeTab.value === 'stats') await loadStats(); else await loadCore() }
async function saveSettings() { savingSettings.value = true; try { const saved = await adminAPI.zenxiangLiyu.updateSettings({ ...settingsForm }); Object.assign(settingsForm, saved); appStore.showSuccess(t('admin.zenxiangLiyu.settingsSaved')) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.settingsSaveFailed'))) } finally { savingSettings.value = false } }
function addPrize() { prizes.value.push({ name: t('admin.zenxiangLiyu.newPrize'), reward_amount: 0, probability: 0, enabled: true, sort_order: prizes.value.length + 1 }) }
async function savePrizes() { if (probabilityTotal.value !== 100) return; savingPrizes.value = true; try { prizes.value = copyPrizes(await adminAPI.zenxiangLiyu.replacePrizes(copyPrizes(prizes.value))); appStore.showSuccess(t('admin.zenxiangLiyu.prizesSaved')) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.prizesSaveFailed'))) } finally { savingPrizes.value = false } }
async function createGrant() { if (!grantUserId.value) return; try { await adminAPI.zenxiangLiyu.createGrant({ user_id: grantUserId.value, enabled: true }); grantUserId.value = null; await loadStats(); appStore.showSuccess(t('admin.zenxiangLiyu.grantSaved')) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.grantSaveFailed'))) } }
async function removeGrant(userId: number) { try { await adminAPI.zenxiangLiyu.deleteGrant(userId); grants.value = grants.value.filter((grant) => grant.user_id !== userId); appStore.showSuccess(t('admin.zenxiangLiyu.grantRemoved')) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.grantRemoveFailed'))) } }
async function runSimulation() { simulating.value = true; try { simulationResult.value = await adminAPI.zenxiangLiyu.simulate({ ...simulationForm, prizes: copyPrizes(prizes.value) }) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.simulationFailed'))) } finally { simulating.value = false } }
async function recommendConfiguration() { simulating.value = true; try { const result = await adminAPI.zenxiangLiyu.recommend({ target_profit_rate: simulationForm.target_profit_rate, ticket_amount: simulationForm.ticket_amount, prizes: copyPrizes(prizes.value) }); recommendationPlans.value = result.plans.map((plan) => ({ prizes: copyPrizes(plan.prizes), theory_profit: plan.theory_profit, theory_profit_rate: plan.theory_profit_rate })) } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.recommendationFailed'))) } finally { simulating.value = false } }
function applyRecommendation(planPrizes: ZenxiangLiyuPrizeInput[]) { prizes.value = copyPrizes(planPrizes); appStore.showSuccess(t('admin.zenxiangLiyu.recommendationApplied')) }
onMounted(loadCore)
</script>
