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
          <button
            v-for="item in tabs"
            :key="item.id"
            type="button"
            :data-testid="`zenxiang-tab-${item.id}`"
            class="shrink-0 px-4 py-2 text-sm font-medium"
            :class="activeTab === item.id ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400' : 'text-gray-500 dark:text-dark-400'"
            @click="selectTab(item.id)"
          >
            {{ item.label }}
          </button>
        </div>
      </section>

      <section v-if="activeTab === 'settings'" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.settingsTitle') }}</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.settingsHint') }}</p>
            </div>
            <Toggle v-model="settingsForm.global_enabled" />
          </div>
          <div class="mt-4 grid gap-3 md:grid-cols-4">
            <label><span class="input-label">{{ t('admin.zenxiangLiyu.ticketUsageThreshold') }}</span><input v-model.number="settingsForm.ticket_usage_threshold" type="number" min="0.01" step="0.01" class="input" /></label>
            <label><span class="input-label">{{ t('admin.zenxiangLiyu.dailyTicketLimit') }}</span><input v-model.number="settingsForm.daily_ticket_limit" type="number" min="1" step="1" class="input" /></label>
            <label><span class="input-label">{{ t('admin.zenxiangLiyu.unitSalePrice') }}</span><input v-model.number="settingsForm.unit_sale_price" type="number" min="0" step="0.0001" class="input" /></label>
            <label><span class="input-label">{{ t('admin.zenxiangLiyu.unitCostPrice') }}</span><input v-model.number="settingsForm.unit_cost_price" type="number" min="0" step="0.0001" class="input" /></label>
          </div>
          <div class="mt-4 grid gap-3 md:grid-cols-4">
            <div class="rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <div class="flex items-center justify-between gap-3">
                <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.luckyCoinEnabled') }}</span>
                <Toggle v-model="settingsForm.lucky_coin_enabled" />
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.luckyCoinHint') }}</p>
            </div>
            <label>
              <span class="input-label">{{ t('admin.zenxiangLiyu.luckyCoinDoubleProbability') }}</span>
              <input v-model.number="settingsForm.lucky_coin_double_probability" type="number" min="0" max="100" step="0.01" class="input" />
            </label>
          </div>
          <div class="mt-4 grid gap-3 md:grid-cols-4">
            <div class="rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.expectedRewardPerTicket') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatAmount(theoreticalExpense) }}</p>
            </div>
            <div class="rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.rewardRate') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatPercent(theoreticalRewardRate) }}</p>
            </div>
            <div class="rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.grossProfitAfterReward') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatPercent(theoreticalGrossProfitRateAfterReward) }}</p>
            </div>
            <div class="rounded-md bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.tenConsumptionReward') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatAmount(theoreticalRewardForTenConsumption) }}</p>
            </div>
          </div>
          <div class="mt-4 flex justify-end">
            <button data-testid="zenxiang-save-settings" class="btn btn-primary" :disabled="savingSettings" @click="saveSettings">{{ t('admin.zenxiangLiyu.saveSettings') }}</button>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.grants') }}</h2>
          <div class="mt-3">
            <div class="relative w-full md:w-80">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              />
              <input
                v-model.trim="grantSearch"
                type="search"
                class="input pl-10"
                :placeholder="t('admin.zenxiangLiyu.grantSearchPlaceholder')"
                @input="handleGrantSearchInput"
              />
            </div>
          </div>
          <div v-if="grantSearchResults.length" class="mt-3 divide-y divide-gray-200 rounded-md border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
            <div v-for="user in grantSearchResults" :key="user.id" class="flex items-center justify-between gap-3 px-3 py-2 text-sm">
              <span class="min-w-0 truncate text-gray-700 dark:text-dark-200">{{ user.email || `#${user.id}` }}</span>
              <div class="flex shrink-0 flex-wrap justify-end gap-2">
                <button class="btn btn-secondary" @click="selectGiftUser(user.id, user.email || `#${user.id}`)">{{ t('admin.zenxiangLiyu.giftTickets') }}</button>
                <button class="btn btn-primary" @click="createGrant(user.id)">{{ t('admin.zenxiangLiyu.grant') }}</button>
              </div>
            </div>
          </div>
          <div class="mt-4 rounded-md border border-primary-100 bg-primary-50/60 p-3 dark:border-primary-500/30 dark:bg-primary-500/10">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-end">
              <div class="min-w-0 flex-1">
                <span class="input-label">{{ t('admin.zenxiangLiyu.giftUser') }}</span>
                <div class="mt-1 truncate rounded-md border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200">
                  {{ selectedGiftUser ? selectedGiftUser.label : t('admin.zenxiangLiyu.noGiftUserSelected') }}
                </div>
              </div>
              <label class="w-full lg:w-36">
                <span class="input-label">{{ t('admin.zenxiangLiyu.giftTicketCount') }}</span>
                <input v-model.number="giftForm.ticket_count" type="number" min="1" max="1000" step="1" class="input" />
              </label>
              <label class="min-w-0 flex-1">
                <span class="input-label">{{ t('admin.zenxiangLiyu.giftNotes') }}</span>
                <input v-model.trim="giftForm.notes" type="text" class="input" :placeholder="t('admin.zenxiangLiyu.giftNotesPlaceholder')" />
              </label>
              <button class="btn btn-primary" :disabled="giftingTickets || !selectedGiftUser || giftForm.ticket_count <= 0" @click="giftTicketsToSelectedUser">
                {{ t('admin.zenxiangLiyu.confirmGiftTickets') }}
              </button>
            </div>
          </div>
          <div class="mt-4 divide-y divide-gray-200 dark:divide-dark-700">
            <div v-for="grant in grants" :key="grant.user_id" class="flex items-center justify-between gap-3 py-2 text-sm">
              <span>{{ grant.user_email || `#${grant.user_id}` }}</span>
              <div class="flex shrink-0 flex-wrap justify-end gap-2">
                <button class="btn btn-secondary" @click="selectGiftUser(grant.user_id, grant.user_email || `#${grant.user_id}`)">{{ t('admin.zenxiangLiyu.giftTickets') }}</button>
                <button class="btn btn-secondary" :disabled="isResettingGrant(grant.user_id)" @click="resetGrantDailyPlays(grant.user_id)">{{ t('admin.zenxiangLiyu.resetDailyPlays') }}</button>
                <button class="btn btn-secondary" @click="removeGrant(grant.user_id)">{{ t('admin.zenxiangLiyu.remove') }}</button>
              </div>
            </div>
            <p v-if="grants.length === 0" class="py-2 text-sm text-gray-500">{{ t('admin.zenxiangLiyu.noGrants') }}</p>
          </div>
        </div>
      </section>

      <section v-else-if="activeTab === 'prizes'" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.prizeTitle') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.prizeHint') }}</p>
          </div>
          <button class="btn btn-secondary" @click="addFormalPrize">{{ t('admin.zenxiangLiyu.addPrize') }}</button>
        </div>
        <PrizeMetrics class="mt-4" :probability-total="probabilityTotal" :expense="theoreticalExpense" :profit="theoreticalProfit" :profit-rate="theoreticalProfitRate" />
        <p v-if="probabilityTotal !== 100" class="mt-3 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
          {{ t('admin.zenxiangLiyu.probabilityWarning', { total: formatNumber(probabilityTotal) }) }}
        </p>
        <PrizeEditor v-model:prizes="prizes" class="mt-4" />
        <div class="mt-4 flex justify-end">
          <button data-testid="zenxiang-save-prizes" class="btn btn-primary" :disabled="savingPrizes || probabilityTotal !== 100" @click="savePrizes">{{ t('admin.zenxiangLiyu.savePrizeConfiguration') }}</button>
        </div>
      </section>

      <section v-else-if="activeTab === 'stats'" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <label class="block max-w-52">
            <span class="input-label">{{ t('admin.zenxiangLiyu.statsDate') }}</span>
            <input v-model="statsDate" type="date" class="input" @change="loadStats" />
          </label>
        </div>
        <div data-testid="zenxiang-stats-tables" class="grid gap-4">
          <div class="overflow-hidden rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.userStats') }}</h2>
            <div class="mt-3 overflow-x-auto">
              <table class="w-full min-w-[720px] text-sm">
                <thead>
                  <tr>
                    <th v-for="header in [t('admin.zenxiangLiyu.user'), t('admin.zenxiangLiyu.availableBalance'), t('admin.zenxiangLiyu.usageAmount'), t('admin.zenxiangLiyu.plays'), t('admin.zenxiangLiyu.rewardTotal'), t('admin.zenxiangLiyu.netTotal')]" :key="header" class="p-2 text-left text-xs text-gray-500 dark:text-dark-400">{{ header }}</th>
                  </tr>
                </thead>
                <tbody>
                  <template v-for="row in userStats" :key="row.user_id">
                    <tr class="border-t border-gray-200 dark:border-dark-700">
                      <td class="p-2">
                        <button
                          type="button"
                          :data-testid="`zenxiang-user-stats-toggle-${row.user_id}`"
                          class="inline-flex max-w-56 items-center gap-1.5 text-left font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                          :aria-expanded="expandedUserID === row.user_id"
                          :title="expandedUserID === row.user_id ? t('admin.zenxiangLiyu.userRecordsCollapse') : t('admin.zenxiangLiyu.userRecordsExpand')"
                          @click="toggleUserDetails(row.user_id)"
                        >
                          <Icon :name="expandedUserID === row.user_id ? 'chevronDown' : 'chevronRight'" size="xs" class="shrink-0" />
                          <span class="truncate">{{ row.user_email || String(row.user_id) }}</span>
                        </button>
                      </td>
                      <td class="p-2 text-gray-700 dark:text-dark-200">{{ formatAmount(row.balance) }}</td>
                      <td class="p-2 text-gray-700 dark:text-dark-200">{{ formatAmount(row.usage_amount) }}</td>
                      <td class="p-2 text-gray-700 dark:text-dark-200">{{ row.play_count }}</td>
                      <td class="p-2 text-gray-700 dark:text-dark-200">{{ formatAmount(row.reward_amount) }}</td>
                      <td class="p-2 text-gray-700 dark:text-dark-200">{{ formatAmount(row.user_net_amount) }}</td>
                    </tr>
                    <tr v-if="expandedUserID === row.user_id" :data-testid="`zenxiang-user-stats-details-${row.user_id}`">
                      <td colspan="6" class="border-t border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/70">
                        <p v-if="userRecordLoading" class="py-4 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.userRecordsLoading') }}</p>
                        <div v-else-if="userRecordError" class="flex min-h-16 flex-wrap items-center justify-center gap-3 text-sm text-rose-600 dark:text-rose-300">
                          <span>{{ userRecordError }}</span>
                          <button :data-testid="`zenxiang-user-records-retry-${row.user_id}`" type="button" class="btn btn-secondary" @click="loadUserRecords(row.user_id)">{{ t('admin.zenxiangLiyu.userRecordsRetry') }}</button>
                        </div>
                        <p v-else-if="expandedUserRecords.length === 0" class="py-4 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.userRecordsEmpty') }}</p>
                        <div v-else class="overflow-x-auto">
                          <table class="w-full min-w-[760px] table-fixed text-xs">
                            <thead>
                              <tr class="text-gray-500 dark:text-dark-400">
                                <th class="w-40 p-2 text-left font-medium">{{ t('admin.zenxiangLiyu.userRecordPlayedAt') }}</th>
                                <th class="p-2 text-left font-medium">{{ t('admin.zenxiangLiyu.prizeName') }}</th>
                                <th class="w-28 p-2 text-right font-medium">{{ t('admin.zenxiangLiyu.userRecordOriginalReward') }}</th>
                                <th class="w-28 p-2 text-left font-medium">{{ t('admin.zenxiangLiyu.userRecordLuckyResult') }}</th>
                                <th class="w-28 p-2 text-right font-medium">{{ t('admin.zenxiangLiyu.userRecordAdjustment') }}</th>
                                <th class="w-28 p-2 text-right font-medium">{{ t('admin.zenxiangLiyu.userRecordFinalNet') }}</th>
                              </tr>
                            </thead>
                            <tbody>
                              <tr v-for="record in expandedUserRecords" :key="record.id" class="border-t border-gray-200 dark:border-dark-700">
                                <td class="whitespace-nowrap p-2 text-gray-600 dark:text-dark-300">{{ formatDateTime(record.played_at) }}</td>
                                <td class="truncate p-2 font-medium text-gray-800 dark:text-dark-100" :title="record.prize_name">{{ record.prize_name }}</td>
                                <td class="p-2 text-right tabular-nums text-gray-700 dark:text-dark-200">{{ formatAmount(record.reward_amount) }}</td>
                                <td class="p-2" :class="luckyCoinResultClass(record)">{{ luckyCoinResultLabel(record) }}</td>
                                <td class="p-2 text-right tabular-nums" :class="amountClass(record.lucky_coin_adjustment)">{{ formatSignedAmount(record.lucky_coin_adjustment) }}</td>
                                <td class="p-2 text-right font-semibold tabular-nums" :class="amountClass(record.user_net_amount)">{{ formatSignedAmount(record.user_net_amount) }}</td>
                              </tr>
                            </tbody>
                          </table>
                        </div>
                      </td>
                    </tr>
                  </template>
                </tbody>
              </table>
            </div>
          </div>
          <StatsTable :title="t('admin.zenxiangLiyu.prizeStats')" :headers="[t('admin.zenxiangLiyu.prizeName'), t('admin.zenxiangLiyu.configuredProbability'), t('admin.zenxiangLiyu.actualRate'), t('admin.zenxiangLiyu.probabilityDiff'), t('admin.zenxiangLiyu.hitCount')]" :rows="prizeStatsRows" />
        </div>
      </section>

      <section v-else class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.simulatorTitle') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.zenxiangLiyu.simulatorHint') }}</p>
          <div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <label v-for="field in simulatorFields" :key="field.key"><span class="input-label">{{ field.label }}</span><input v-model.number="simulationForm[field.key]" type="number" :min="field.min" :step="field.step" class="input" /></label>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.simulationPrizeConfig') }}</h3>
            <button class="btn btn-secondary" @click="addSimulationPrize">{{ t('admin.zenxiangLiyu.addPrize') }}</button>
          </div>
          <PrizeEditor v-model:prizes="simulationPrizes" class="mt-4" />
          <div class="mt-4 flex flex-wrap justify-end gap-2">
            <button data-testid="zenxiang-recommend" class="btn btn-secondary" :disabled="simulating" @click="recommendConfiguration">{{ t('admin.zenxiangLiyu.recommend') }}</button>
            <button data-testid="zenxiang-simulate" class="btn btn-primary" :disabled="simulating" @click="runSimulation">{{ t('admin.zenxiangLiyu.runSimulation') }}</button>
          </div>
        </div>

        <div v-if="simulationResult" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <Metric :label="t('admin.zenxiangLiyu.totalPlays')" :value="simulationResult.total_plays" />
          <Metric :label="t('admin.zenxiangLiyu.systemRevenue')" :value="formatAmount(simulationResult.total_revenue)" />
          <Metric :label="t('admin.zenxiangLiyu.systemExpense')" :value="formatAmount(simulationResult.total_expense)" />
          <Metric :label="t('admin.zenxiangLiyu.systemProfit')" :value="formatAmount(simulationResult.net_profit)" />
          <Metric :label="t('admin.zenxiangLiyu.profitRate')" :value="formatPercent(simulationResult.profit_rate)" />
        </div>
        <div v-if="recommendationPlans.length" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.zenxiangLiyu.recommendations') }}</h2>
          <div class="mt-3 space-y-2">
            <div v-for="(plan, index) in recommendationPlans" :key="index" class="flex flex-col gap-2 rounded-md border border-gray-200 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
              <span class="text-sm text-gray-600 dark:text-dark-300">{{ t('admin.zenxiangLiyu.planSummary', { profit: formatAmount(plan.theory_profit), rate: formatPercent(plan.theory_profit_rate) }) }}</span>
              <button :data-testid="`zenxiang-apply-recommendation-${index}`" class="btn btn-secondary" :disabled="savingPrizes" @click="applyRecommendation(plan.prizes)">{{ t('admin.zenxiangLiyu.applyRecommendation') }}</button>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import type { ZenxiangLiyuGrant, ZenxiangLiyuPrizeInput, ZenxiangLiyuPrizeStats, ZenxiangLiyuSimulationResult, ZenxiangLiyuUserStats } from '@/api/admin/zenxiangLiyu'
import type { ZenxiangLiyuRecord } from '@/api/zenxiangLiyu'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const activeTab = ref<'settings' | 'prizes' | 'stats' | 'simulator'>('settings')
const loading = ref(false)
const savingSettings = ref(false)
const savingPrizes = ref(false)
const simulating = ref(false)
const searchingUsers = ref(false)
const giftingTickets = ref(false)
const resettingGrantIds = ref<Set<number>>(new Set())
const settingsForm = reactive({
  global_enabled: false,
  ticket_amount: 0,
  minimum_balance: 0,
  daily_play_limit: 1,
  ticket_usage_threshold: 5,
  daily_ticket_limit: 3,
  unit_sale_price: 0.1,
  unit_cost_price: 0.05,
  lucky_coin_enabled: true,
  lucky_coin_double_probability: 50
})
const prizes = ref<ZenxiangLiyuPrizeInput[]>([])
const simulationPrizes = ref<ZenxiangLiyuPrizeInput[]>([])
const grants = ref<ZenxiangLiyuGrant[]>([])
const grantSearch = ref('')
const grantSearchResults = ref<AdminUser[]>([])
const selectedGiftUser = ref<{ id: number, label: string } | null>(null)
const giftForm = reactive({ request_id: newGiftRequestId(), ticket_count: 1, notes: '' })
const userStats = ref<ZenxiangLiyuUserStats[]>([])
const prizeStats = ref<ZenxiangLiyuPrizeStats[]>([])
const statsDate = ref(todayString())
const expandedUserID = ref<number | null>(null)
const userRecordCache = ref<Record<string, ZenxiangLiyuRecord[]>>({})
const userRecordLoading = ref(false)
const userRecordError = ref('')
let userRecordRequestVersion = 0
const simulationForm = reactive({ user_count: 100, plays_per_user: 3, initial_balance: 100, ticket_amount: 2, minimum_balance: 10, daily_play_limit: 3, target_profit_rate: 0.1 })
const simulationResult = ref<ZenxiangLiyuSimulationResult | null>(null)
const recommendationPlans = ref<Array<{ prizes: ZenxiangLiyuPrizeInput[], theory_profit: number, theory_profit_rate: number }>>([])
const tabs = computed(() => [
  { id: 'settings' as const, label: t('admin.zenxiangLiyu.settings') },
  { id: 'prizes' as const, label: t('admin.zenxiangLiyu.prizes') },
  { id: 'stats' as const, label: t('admin.zenxiangLiyu.stats') },
  { id: 'simulator' as const, label: t('admin.zenxiangLiyu.simulator') },
])
const simulatorFields = computed(() => [
  { key: 'user_count' as const, label: t('admin.zenxiangLiyu.userCount'), min: 1, step: 1 },
  { key: 'plays_per_user' as const, label: t('admin.zenxiangLiyu.playsPerUser'), min: 1, step: 1 },
  { key: 'initial_balance' as const, label: t('admin.zenxiangLiyu.initialBalance'), min: 0, step: 0.01 },
  { key: 'ticket_amount' as const, label: t('admin.zenxiangLiyu.ticketAmount'), min: 0, step: 0.01 },
  { key: 'minimum_balance' as const, label: t('admin.zenxiangLiyu.minimumBalance'), min: 0, step: 0.01 },
  { key: 'daily_play_limit' as const, label: t('admin.zenxiangLiyu.dailyPlayLimit'), min: 1, step: 1 },
  { key: 'target_profit_rate' as const, label: t('admin.zenxiangLiyu.targetProfitRate'), min: 0, step: 0.01 },
])
const enabledPrizes = computed(() => prizes.value.filter((prize) => prize.enabled))
const probabilityTotal = computed(() => enabledPrizes.value.reduce((sum, prize) => sum + Number(prize.probability || 0), 0))
const theoreticalExpense = computed(() => enabledPrizes.value.reduce((sum, prize) => sum + Number(prize.reward_amount || 0) * Number(prize.probability || 0) / 100, 0))
const theoreticalProfit = computed(() => Number(settingsForm.ticket_usage_threshold || 0) - theoreticalExpense.value)
const theoreticalProfitRate = computed(() => settingsForm.ticket_usage_threshold > 0 ? theoreticalProfit.value / settingsForm.ticket_usage_threshold : 0)
const theoreticalRewardRate = computed(() => {
  const threshold = Number(settingsForm.ticket_usage_threshold || 0)
  return threshold > 0 ? theoreticalExpense.value / threshold : 0
})
const theoreticalGrossProfitRateBeforeReward = computed(() => {
  const sale = Number(settingsForm.unit_sale_price || 0)
  if (sale <= 0) return 0
  return (sale - Number(settingsForm.unit_cost_price || 0)) / sale
})
const theoreticalGrossProfitRateAfterReward = computed(() => theoreticalGrossProfitRateBeforeReward.value - theoreticalRewardRate.value)
const theoreticalRewardForTenConsumption = computed(() => {
  const threshold = Number(settingsForm.ticket_usage_threshold || 0)
  const limit = Math.max(0, Number(settingsForm.daily_ticket_limit || 0))
  if (threshold <= 0) return 0
  return Math.min(Math.floor(10 / threshold), limit) * theoreticalExpense.value
})
const prizeStatsTotalHits = computed(() => prizeStats.value.reduce((sum, row) => sum + row.hit_count, 0))
const prizeStatsRows = computed(() => prizeStats.value.map((row) => {
  const actualRate = prizeStatsTotalHits.value ? row.hit_count / prizeStatsTotalHits.value : 0
  const configuredRate = Number(row.probability || 0) / 100
  return [row.prize_name, `${formatNumber(row.probability)}%`, formatPercent(actualRate), formatSignedPercent(actualRate - configuredRate), row.hit_count]
}))
const expandedUserRecords = computed(() => {
  if (expandedUserID.value === null) return []
  return userRecordCache.value[userRecordCacheKey(expandedUserID.value)] ?? []
})

const Metric = defineComponent({
  props: { label: { type: String, required: true }, value: { type: [String, Number], required: true }, warn: Boolean },
  setup(props) {
    return () => h('div', { class: ['rounded-md border p-3', props.warn ? 'border-amber-300 bg-amber-50 dark:border-amber-700 dark:bg-amber-500/10' : 'border-gray-200 dark:border-dark-700'] }, [
      h('p', { class: 'text-xs text-gray-500 dark:text-dark-400' }, props.label),
      h('p', { class: 'mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white' }, String(props.value)),
    ])
  },
})

const PrizeMetrics = defineComponent({
  props: { probabilityTotal: { type: Number, required: true }, expense: { type: Number, required: true }, profit: { type: Number, required: true }, profitRate: { type: Number, required: true } },
  setup(props) {
    return () => h('div', { class: 'grid gap-3 sm:grid-cols-2 xl:grid-cols-4' }, [
      h(Metric, { label: t('admin.zenxiangLiyu.probabilityTotal'), value: `${formatNumber(props.probabilityTotal)}%`, warn: props.probabilityTotal !== 100 }),
      h(Metric, { label: t('admin.zenxiangLiyu.theoreticalExpense'), value: formatAmount(props.expense) }),
      h(Metric, { label: t('admin.zenxiangLiyu.theoreticalProfit'), value: formatAmount(props.profit) }),
      h(Metric, { label: t('admin.zenxiangLiyu.theoreticalProfitRate'), value: formatPercent(props.profitRate) }),
    ])
  },
})

const PrizeEditor = defineComponent({
  props: { prizes: { type: Array as () => ZenxiangLiyuPrizeInput[], required: true } },
  emits: ['update:prizes'],
  setup(props, { emit, attrs }) {
    const update = (index: number, field: keyof ZenxiangLiyuPrizeInput, value: string | number | boolean) => {
      const next = copyPrizes(props.prizes)
      ;(next[index] as Record<string, unknown>)[field] = value
      emit('update:prizes', next)
    }
    const remove = (index: number) => {
      const next = copyPrizes(props.prizes)
      next.splice(index, 1)
      emit('update:prizes', next)
    }
    return () => h('div', { ...attrs, class: ['overflow-x-auto', attrs.class] }, [
      h('table', { class: 'w-full min-w-[840px] divide-y divide-gray-200 dark:divide-dark-700' }, [
        h('thead', h('tr', { class: 'text-left text-xs text-gray-500 dark:text-dark-400' }, [
          t('admin.zenxiangLiyu.prizeName'), t('admin.zenxiangLiyu.rewardAmount'), t('admin.zenxiangLiyu.probability'), t('admin.zenxiangLiyu.enabled'), t('admin.zenxiangLiyu.sortOrder'), '',
        ].map((header) => h('th', { class: 'p-2' }, header)))),
        h('tbody', { class: 'divide-y divide-gray-200 dark:divide-dark-700' }, props.prizes.map((prize, index) => h('tr', { key: prize.id ?? `new-${index}` }, [
          h('td', { class: 'p-2' }, h('input', { class: 'input', value: prize.name, onInput: (event: Event) => update(index, 'name', (event.target as HTMLInputElement).value.trim()) })),
          h('td', { class: 'p-2' }, h('input', { class: 'input', type: 'number', min: 0, step: 0.01, value: prize.reward_amount, onInput: (event: Event) => update(index, 'reward_amount', Number((event.target as HTMLInputElement).value)) })),
          h('td', { class: 'p-2' }, h('input', { class: 'input', type: 'number', min: 0, max: 100, step: 0.01, value: prize.probability, onInput: (event: Event) => update(index, 'probability', Number((event.target as HTMLInputElement).value)) })),
          h('td', { class: 'p-2' }, h(Toggle, { modelValue: prize.enabled, 'onUpdate:modelValue': (value: boolean) => update(index, 'enabled', value) })),
          h('td', { class: 'p-2' }, h('input', { class: 'input', type: 'number', min: 0, step: 1, value: prize.sort_order, onInput: (event: Event) => update(index, 'sort_order', Number((event.target as HTMLInputElement).value)) })),
          h('td', { class: 'p-2 text-right' }, h('button', { class: 'btn btn-secondary', onClick: () => remove(index) }, t('admin.zenxiangLiyu.remove'))),
        ]))),
      ]),
    ])
  },
})

const StatsTable = defineComponent({
  props: { title: { type: String, required: true }, headers: { type: Array as () => string[], required: true }, rows: { type: Array as () => Array<Array<string | number>>, required: true } },
  setup(props) {
    return () => h('div', { class: 'overflow-x-auto rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900' }, [
      h('h2', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title),
      h('table', { class: 'mt-3 w-full text-sm' }, [
        h('thead', h('tr', props.headers.map((header) => h('th', { class: 'p-2 text-left text-xs text-gray-500 dark:text-dark-400' }, header)))),
        h('tbody', props.rows.map((row) => h('tr', { class: 'border-t border-gray-200 dark:border-dark-700' }, row.map((cell) => h('td', { class: 'p-2 text-gray-700 dark:text-dark-200' }, String(cell)))))),
      ]),
    ])
  },
})

function formatNumber(value: number) { return Number(value || 0).toFixed(2).replace(/\.00$/, '') }
function formatAmount(value: number) { return `${formatNumber(value)} ${t('admin.zenxiangLiyu.pointsUnit')}` }
function formatSignedAmount(value: number) { return `${Number(value || 0) > 0 ? '+' : ''}${formatAmount(value)}` }
function formatPercent(value: number) { return `${formatNumber(value * 100)}%` }
function formatSignedPercent(value: number) { return `${value > 0 ? '+' : ''}${formatPercent(value)}` }
function amountClass(value: number) {
  if (Number(value || 0) > 0) return 'text-emerald-700 dark:text-emerald-300'
  if (Number(value || 0) < 0) return 'text-rose-700 dark:text-rose-300'
  return 'text-gray-500 dark:text-dark-400'
}
function luckyCoinResultLabel(record: ZenxiangLiyuRecord) {
  if (!record.lucky_coin_played) return t('admin.zenxiangLiyu.userRecordLuckyNotPlayed')
  return record.lucky_coin_outcome === 'double'
    ? t('admin.zenxiangLiyu.userRecordLuckyDoubled')
    : t('admin.zenxiangLiyu.userRecordLuckyMissed')
}
function luckyCoinResultClass(record: ZenxiangLiyuRecord) {
  if (!record.lucky_coin_played) return 'text-gray-500 dark:text-dark-400'
  return record.lucky_coin_outcome === 'double'
    ? 'text-emerald-700 dark:text-emerald-300'
    : 'text-rose-700 dark:text-rose-300'
}
function todayString() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
function copyPrizes(value: ZenxiangLiyuPrizeInput[]) { return value.map((prize) => ({ ...prize })) }
function newPrize(sortOrder: number): ZenxiangLiyuPrizeInput {
  return { name: t('admin.zenxiangLiyu.newPrize'), reward_amount: 0, probability: 0, enabled: true, sort_order: sortOrder }
}
function addFormalPrize() {
  prizes.value = [...prizes.value, newPrize(prizes.value.length + 1)]
}
function addSimulationPrize() {
  simulationPrizes.value = [...simulationPrizes.value, newPrize(simulationPrizes.value.length + 1)]
}
function syncSimulationPrizes() {
  simulationPrizes.value = copyPrizes(prizes.value)
}

async function loadCore() {
  loading.value = true
  try {
    const [settings, prizeList, grantsResult] = await Promise.all([
      adminAPI.zenxiangLiyu.getSettings(),
      adminAPI.zenxiangLiyu.listPrizes(),
      adminAPI.zenxiangLiyu.listGrants({ page_size: 100 }),
    ])
    Object.assign(settingsForm, settings)
    Object.assign(simulationForm, { ticket_amount: settings.ticket_amount, minimum_balance: settings.minimum_balance, daily_play_limit: settings.daily_play_limit })
    prizes.value = copyPrizes(prizeList)
    grants.value = grantsResult.items
    syncSimulationPrizes()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadGrants() {
  const grantsResult = await adminAPI.zenxiangLiyu.listGrants({ page_size: 100 })
  grants.value = grantsResult.items
}

async function loadStats() {
  resetUserRecordDetails()
  loading.value = true
  try {
    const [usersResult, prizesResult, grantsResult] = await Promise.all([
      adminAPI.zenxiangLiyu.listUserStats({ page_size: 100, date: statsDate.value }),
      adminAPI.zenxiangLiyu.listPrizeStats(),
      adminAPI.zenxiangLiyu.listGrants({ page_size: 100 }),
    ])
    userStats.value = usersResult.items
    prizeStats.value = prizesResult
    grants.value = grantsResult.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.statsLoadFailed')))
  } finally {
    loading.value = false
  }
}

function userRecordCacheKey(userID: number, date = statsDate.value) {
  return `${date}:${userID}`
}

function resetUserRecordDetails() {
  userRecordRequestVersion++
  expandedUserID.value = null
  userRecordCache.value = {}
  userRecordLoading.value = false
  userRecordError.value = ''
}

async function toggleUserDetails(userID: number) {
  if (expandedUserID.value === userID) {
    userRecordRequestVersion++
    expandedUserID.value = null
    userRecordLoading.value = false
    userRecordError.value = ''
    return
  }
  userRecordRequestVersion++
  expandedUserID.value = userID
  userRecordLoading.value = false
  userRecordError.value = ''
  if (userRecordCache.value[userRecordCacheKey(userID)]) return
  await loadUserRecords(userID)
}

async function loadUserRecords(userID: number) {
  const date = statsDate.value
  const cacheKey = userRecordCacheKey(userID, date)
  const requestVersion = ++userRecordRequestVersion
  expandedUserID.value = userID
  userRecordLoading.value = true
  userRecordError.value = ''
  try {
    const result = await adminAPI.zenxiangLiyu.listUserRecords(userID, { date, page: 1, page_size: 100 })
    if (requestVersion !== userRecordRequestVersion || expandedUserID.value !== userID || statsDate.value !== date) return
    userRecordCache.value = { ...userRecordCache.value, [cacheKey]: result.items }
  } catch (error) {
    if (requestVersion !== userRecordRequestVersion || expandedUserID.value !== userID || statsDate.value !== date) return
    userRecordError.value = extractApiErrorMessage(error, t('admin.zenxiangLiyu.userRecordsLoadFailed'))
  } finally {
    if (requestVersion === userRecordRequestVersion) userRecordLoading.value = false
  }
}

async function selectTab(tab: typeof activeTab.value) {
  activeTab.value = tab
  if (tab === 'stats') await loadStats()
}
async function reloadCurrentTab() { if (activeTab.value === 'stats') await loadStats(); else await loadCore() }
async function saveSettings() {
  savingSettings.value = true
  try {
    const saved = await adminAPI.zenxiangLiyu.updateSettings({ ...settingsForm })
    Object.assign(settingsForm, saved)
    appStore.showSuccess(t('admin.zenxiangLiyu.settingsSaved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.settingsSaveFailed')))
  } finally {
    savingSettings.value = false
  }
}
async function savePrizes() {
  if (probabilityTotal.value !== 100) return
  savingPrizes.value = true
  try {
    prizes.value = copyPrizes(await adminAPI.zenxiangLiyu.replacePrizes(copyPrizes(prizes.value)))
    syncSimulationPrizes()
    appStore.showSuccess(t('admin.zenxiangLiyu.prizesSaved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.prizesSaveFailed')))
  } finally {
    savingPrizes.value = false
  }
}
async function searchGrantUsers() {
  if (!grantSearch.value) {
    grantSearchResults.value = []
    return
  }
  searchingUsers.value = true
  try {
    const result = await adminAPI.users.list(1, 10, { role: 'user', search: grantSearch.value })
    grantSearchResults.value = result.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.userSearchFailed')))
  } finally {
    searchingUsers.value = false
  }
}

let grantSearchTimeout: number | null = null
function handleGrantSearchInput() {
  if (grantSearchTimeout) window.clearTimeout(grantSearchTimeout)
  if (!grantSearch.value.trim()) {
    grantSearchResults.value = []
    return
  }
  grantSearchTimeout = window.setTimeout(() => {
    void searchGrantUsers()
  }, 300)
}

async function createGrant(userId: number) {
  try {
    await adminAPI.zenxiangLiyu.createGrant({ user_id: userId, enabled: true })
    grantSearchResults.value = []
    grantSearch.value = ''
    await loadGrants()
    appStore.showSuccess(t('admin.zenxiangLiyu.grantSaved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.grantSaveFailed')))
  }
}
function selectGiftUser(userId: number, label: string) {
  selectedGiftUser.value = { id: userId, label }
}
function newGiftRequestId() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `zenxiang-gift-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
async function giftTicketsToSelectedUser() {
  if (!selectedGiftUser.value || giftForm.ticket_count <= 0) return
  giftingTickets.value = true
  try {
    const gift = await adminAPI.zenxiangLiyu.giftTickets({
      request_id: giftForm.request_id,
      user_id: selectedGiftUser.value.id,
      ticket_count: giftForm.ticket_count,
      notes: giftForm.notes,
    })
    giftForm.request_id = newGiftRequestId()
    giftForm.ticket_count = 1
    giftForm.notes = ''
    appStore.showSuccess(t('admin.zenxiangLiyu.giftTicketsSuccess', { count: gift.ticket_count, user: gift.user_email || selectedGiftUser.value.label }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.giftTicketsFailed')))
  } finally {
    giftingTickets.value = false
  }
}
async function removeGrant(userId: number) {
  try {
    await adminAPI.zenxiangLiyu.deleteGrant(userId)
    grants.value = grants.value.filter((grant) => grant.user_id !== userId)
    appStore.showSuccess(t('admin.zenxiangLiyu.grantRemoved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.grantRemoveFailed')))
  }
}
function isResettingGrant(userId: number) {
  return resettingGrantIds.value.has(userId)
}
async function resetGrantDailyPlays(userId: number) {
  resettingGrantIds.value = new Set(resettingGrantIds.value).add(userId)
  try {
    const result = await adminAPI.zenxiangLiyu.resetGrantDailyPlays(userId)
    appStore.showSuccess(t('admin.zenxiangLiyu.resetDailyPlaysSuccess', { count: result.previous_play_count, remaining: result.remaining_plays }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.resetDailyPlaysFailed')))
  } finally {
    const next = new Set(resettingGrantIds.value)
    next.delete(userId)
    resettingGrantIds.value = next
  }
}
async function runSimulation() {
  simulating.value = true
  try {
    simulationResult.value = await adminAPI.zenxiangLiyu.simulate({ ...simulationForm, prizes: copyPrizes(simulationPrizes.value) })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.simulationFailed')))
  } finally {
    simulating.value = false
  }
}
async function recommendConfiguration() {
  simulating.value = true
  try {
    const result = await adminAPI.zenxiangLiyu.recommend({ target_profit_rate: simulationForm.target_profit_rate, ticket_amount: simulationForm.ticket_amount, prizes: copyPrizes(simulationPrizes.value) })
    recommendationPlans.value = result.plans.map((plan) => ({ prizes: copyPrizes(plan.prizes), theory_profit: plan.theory_profit, theory_profit_rate: plan.theory_profit_rate }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.recommendationFailed')))
  } finally {
    simulating.value = false
  }
}
async function applyRecommendation(planPrizes: ZenxiangLiyuPrizeInput[]) {
  savingPrizes.value = true
  try {
    prizes.value = copyPrizes(await adminAPI.zenxiangLiyu.applySimulation(copyPrizes(planPrizes)))
    syncSimulationPrizes()
    appStore.showSuccess(t('admin.zenxiangLiyu.recommendationApplied'))
    activeTab.value = 'prizes'
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.zenxiangLiyu.prizesSaveFailed')))
  } finally {
    savingPrizes.value = false
  }
}
onMounted(loadCore)
onBeforeUnmount(() => {
  if (grantSearchTimeout) window.clearTimeout(grantSearchTimeout)
})
</script>
