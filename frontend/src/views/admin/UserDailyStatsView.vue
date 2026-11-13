<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t(`${p}.title`) }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`${p}.description`) }}</p>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.timezone`) }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary" :disabled="loading" data-test="refresh" @click="load()">{{ t('common.refresh') }}</button>
          <button class="btn btn-primary" :disabled="loading || !data?.items.length" data-test="export" @click="exportPage">{{ t(`${p}.exportPage`) }}</button>
        </div>
      </header>

      <form class="card grid gap-4 p-4 sm:grid-cols-2 xl:grid-cols-4" @submit.prevent="applyFilters">
        <div>
          <label for="daily-date" class="daily-label">{{ t(`${p}.date`) }}</label>
          <input id="daily-date" v-model="date" type="date" class="input w-full" required :max="beijingToday" data-test="date" @change="applyFilters" />
          <div class="mt-2 flex gap-3 text-xs text-primary-600 dark:text-primary-400">
            <button type="button" data-test="today" @click="selectDay(0)">{{ t(`${p}.today`) }}</button>
            <button type="button" data-test="yesterday" @click="selectDay(-1)">{{ t(`${p}.yesterday`) }}</button>
          </div>
        </div>
        <div>
          <label for="daily-search" class="daily-label">{{ t(`${p}.user`) }}</label>
          <div class="flex gap-2">
            <input id="daily-search" v-model="search" class="input min-w-0 flex-1" :placeholder="t(`${p}.search`)" maxlength="200" data-test="search" />
            <button class="btn btn-secondary shrink-0" type="submit">{{ t(`${p}.searchButton`) }}</button>
          </div>
        </div>
        <div>
          <label for="daily-activity" class="daily-label">{{ t(`${p}.activity`) }}</label>
          <select id="daily-activity" v-model="activity" class="input w-full" data-test="activity" @change="applyFilters">
            <option v-for="option in activityOptions" :key="option.value" :value="option.value">{{ t(`${p}.${option.label}`) }}</option>
          </select>
        </div>
        <div>
          <label for="daily-sort" class="daily-label">{{ t(`${p}.sort`) }}</label>
          <div class="flex gap-2">
            <select id="daily-sort" v-model="sortBy" class="input min-w-0 flex-1" data-test="sort" @change="applyFilters">
              <option v-for="option in sortOptions" :key="option.value" :value="option.value">{{ t(`${p}.${option.label}`) }}</option>
            </select>
            <button type="button" class="btn btn-secondary shrink-0" data-test="sort-order" @click="toggleOrder">{{ t(`${p}.${sortOrder === 'desc' ? 'descending' : 'ascending'}`) }}</button>
          </div>
        </div>
      </form>

      <div v-if="error" role="alert" class="card flex items-center justify-between gap-3 p-4 text-red-600 dark:text-red-400">
        {{ t(`${p}.loadError`) }}
        <button class="btn btn-secondary" @click="load()">{{ t(`${p}.retry`) }}</button>
      </div>
      <div v-else-if="loading" class="flex justify-center py-16" role="status"><LoadingSpinner /></div>

      <template v-else-if="data">
        <section aria-labelledby="daily-summary">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2 text-sm">
            <h2 id="daily-summary" class="font-medium text-gray-900 dark:text-gray-100">{{ t(`${p}.filteredSummary`) }}</h2>
            <span class="text-gray-500 dark:text-gray-400">{{ t(`${p}.people`, { users: data.summary.users, active: data.summary.active_users, spending: data.summary.spending_users }) }}</span>
          </div>
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="card border-t-2 border-t-primary-400 p-4">
              <h3 class="daily-label">{{ t(`${p}.todayCost`) }} · {{ data.date }}</h3>
              <UserDailyUsageBreakdown :usage="data.summary.today_usage" data-test="summary-today" />
            </div>
            <div class="card p-4">
              <h3 class="daily-label">{{ t(`${p}.previousCost`) }} · {{ data.previous_date }}</h3>
              <UserDailyUsageBreakdown :usage="data.summary.previous_usage" />
            </div>
            <div class="card space-y-2 p-4 text-sm tabular-nums">
              <h3 class="daily-label">{{ t(`${p}.activityIncome`) }}</h3>
              <div class="daily-pair"><span>{{ t(`${p}.checkinNet`) }}</span><strong>${{ money(data.summary.activity.checkin_net) }}</strong></div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.checkinPeople`, { users: data.summary.checkin_users, count: data.summary.activity.checkin_count }) }}</p>
              <div class="daily-pair"><span>{{ t(`${p}.liyuNet`) }}</span><strong>${{ money(data.summary.activity.liyu_net) }}</strong></div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.liyuPeople`, { users: data.summary.liyu_users, count: data.summary.activity.liyu_draws }) }}</p>
              <div class="daily-pair text-xs"><span>{{ t(`${p}.giftCredit`) }}</span><span>${{ money(data.summary.activity.gift_credit) }}</span></div>
              <div class="daily-pair text-xs"><span>{{ t(`${p}.adjustment`) }}</span><span>${{ money(data.summary.activity.welfare_adjustment) }}</span></div>
            </div>
            <div class="card space-y-3 p-4 text-sm tabular-nums">
              <h3 class="daily-label">{{ t(`${p}.currentBalances`) }}</h3>
              <div><div class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.paidBalance`) }}</div><strong class="text-lg">${{ money(data.summary.balance) }}</strong></div>
              <div><div class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.welfareBalance`) }}</div><strong class="text-lg text-emerald-700 dark:text-emerald-400">${{ money(data.summary.welfare_balance) }}</strong></div>
            </div>
          </div>
        </section>

        <div class="card overflow-hidden" data-test="user-table">
          <DataTable :columns="columns" :data="data.items" row-key="user_id" :sticky-first-column="false" :expandable-actions="false" :virtualize-threshold="200">
            <template #cell-user="{ row }">
              <div class="max-w-56 whitespace-normal break-all font-medium">{{ row.username || row.email }}</div>
              <div v-if="row.username" class="max-w-56 whitespace-normal break-all text-xs text-gray-500">{{ row.email }}</div>
              <div class="mt-1 text-xs text-gray-500">#{{ row.user_id }} <span v-if="row.deleted" class="text-red-600">{{ t(`${p}.deleted`) }}</span><span v-else-if="row.status !== 'active'" class="text-amber-600">{{ t(`${p}.disabled`) }}</span></div>
            </template>
            <template #cell-rank="{ row }"><span class="font-semibold text-primary-600 dark:text-primary-400">{{ row.spending_rank ? `#${row.spending_rank}` : t(`${p}.noRank`) }}</span></template>
            <template #cell-checkin="{ row }">
              <div class="space-y-1 tabular-nums">
                <div class="font-semibold" :class="amountClass(row.activity.checkin_net)">${{ money(row.activity.checkin_net) }}</div>
                <div class="daily-small">{{ t(`${p}.baseReward`) }} ${{ money(row.activity.checkin_base) }}</div>
                <div class="daily-small">{{ t(`${p}.bonus`) }} ${{ money(row.activity.checkin_bonus) }}</div>
                <div class="daily-small">{{ t(`${p}.checkinCount`) }} {{ row.activity.checkin_count }}</div>
              </div>
            </template>
            <template #cell-liyu="{ row }">
              <div class="space-y-1 tabular-nums">
                <div class="font-semibold" :class="amountClass(row.activity.liyu_net)">${{ money(row.activity.liyu_net) }}</div>
                <div class="daily-small">{{ t(`${p}.draws`, { count: row.activity.liyu_draws }) }} · {{ t(`${p}.reward`) }} ${{ money(row.activity.liyu_reward) }}</div>
                <div class="daily-small">{{ t(`${p}.lucky`) }} ${{ money(row.activity.liyu_lucky) }}</div>
                <div class="daily-small">{{ t(`${p}.guess`) }} ${{ money(row.activity.liyu_guess) }}</div>
                <div class="daily-small">{{ t(`${p}.ticket`) }} ${{ money(row.activity.liyu_ticket_face_value) }}</div>
              </div>
            </template>
            <template #cell-today="{ row }"><UserDailyUsageBreakdown :usage="row.today_usage" /></template>
            <template #cell-previous="{ row }"><UserDailyUsageBreakdown :usage="row.previous_usage" /></template>
            <template #cell-balance="{ row }">
              <div class="space-y-1 tabular-nums"><div class="font-semibold">${{ money(row.balance) }}</div><div class="daily-small">{{ t(`${p}.frozen`) }} ${{ money(row.frozen_balance) }}</div><div class="daily-small">{{ t(`${p}.totalRecharged`) }} ${{ money(row.total_recharged) }}</div></div>
            </template>
            <template #cell-welfare="{ row }">
              <div class="space-y-1 tabular-nums">
                <div class="font-semibold" :class="amountClass(row.welfare_balance)">${{ money(row.welfare_balance) }}</div>
                <div v-for="source in welfareSources" :key="source.field" class="daily-small">{{ t(`${p}.${source.label}`) }} ${{ money(row[source.field]) }}</div>
                <details class="daily-small"><summary class="cursor-pointer py-1">{{ t(`${p}.adjustment`) }}</summary><div class="py-1">{{ t(`${p}.giftCredit`) }} ${{ money(row.activity.gift_credit) }}</div><div>{{ t(`${p}.adjustment`) }} ${{ money(row.activity.welfare_adjustment) }}</div></details>
              </div>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary btn-sm" :data-test="`activities-${row.user_id}`" @click="openActivities(row)">{{ t(`${p}.activities`) }}</button>
                <button class="btn btn-secondary btn-sm" :data-test="`usage-${row.user_id}`" @click="openUsage(row)">{{ t(`${p}.usage`) }}</button>
              </div>
            </template>
            <template #empty><p class="py-8 text-center text-sm text-gray-500">{{ t(`${p}.empty`) }}</p></template>
          </DataTable>
          <Pagination v-if="data.total > 0" :page="page" :total="data.total" :page-size="pageSize" :page-size-options="[10, 20, 50, 100]" @update:page="changePage" @update:page-size="changePageSize" />
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${p}.generated`, { time: time(data.generated_at) }) }}</p>
      </template>

      <details class="card px-4 py-3 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
        <summary class="cursor-pointer font-medium text-gray-700 dark:text-gray-200">{{ t(`${p}.rulesTitle`) }}</summary>
        <div class="mt-2 space-y-2"><p>{{ t(`${p}.balanceNote`) }}</p><p>{{ t(`${p}.usageNote`) }}</p><p>{{ t(`${p}.activityNote`) }}</p></div>
      </details>
    </div>

    <BaseDialog :show="selectedUser !== null" :title="`${t(`${p}.activities`)} · ${selectedUser?.email ?? ''} · ${data?.date ?? ''}`" width="extra-wide" @close="closeActivities">
      <div v-if="detailLoading" class="flex justify-center py-10" role="status"><LoadingSpinner /></div>
      <div v-else-if="detailError" role="alert" class="text-sm text-red-600">{{ t(`${p}.detailError`) }} <button class="btn btn-secondary" data-test="retry-details" @click="loadActivities()">{{ t(`${p}.retry`) }}</button></div>
      <template v-else-if="details">
        <DataTable :columns="detailColumns" :data="details.items" :sticky-first-column="false" :expandable-actions="false">
          <template #cell-kind="{ row }">{{ translatedValue('kinds', row.kind) }} <span class="text-xs text-gray-400">#{{ row.record_id }}</span></template>
          <template #cell-created_at="{ row }">{{ time(row.created_at) }}</template>
          <template #cell-amount="{ row }"><span class="tabular-nums" :class="amountClass(row.amount)">{{ row.amount > 0 ? '+' : '' }}${{ money(row.amount) }}</span></template>
          <template #cell-ticket_face_value="{ row }">{{ row.kind === 'liyu_draw' ? `$${money(row.ticket_face_value)}` : '—' }}</template>
          <template #cell-outcome="{ row }">{{ translatedValue('outcomes', row.outcome) }}</template>
          <template #cell-description="{ row }"><span class="inline-block max-w-64 whitespace-normal break-words">{{ row.description || '—' }}</span></template>
          <template #empty><p class="py-8 text-center text-gray-500">{{ t(`${p}.noActivities`) }}</p></template>
        </DataTable>
        <Pagination v-if="details.total > 0" :page="detailPage" :total="details.total" :page-size="20" :show-page-size-selector="false" @update:page="changeDetailPage" />
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { saveAs } from 'file-saver'
import { userDailyStatsAPI, type DailyActivityFilter, type DailySort, type DailyPage, type UserDailyActivity, type UserDailyStatsResponse, type UserDailyStatsRow } from '@/api/admin/userDailyStats'
import { formatCostFixed } from '@/utils/format'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDailyUsageBreakdown from '@/components/admin/UserDailyUsageBreakdown.vue'

const p = 'admin.userDailyStats'
const { t, te, locale } = useI18n()
const router = useRouter()
function beijingDate(offset = 0): string {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date(Date.now() + offset * 86400000))
  return ['year', 'month', 'day'].map(kind => parts.find(part => part.type === kind)!.value).join('-')
}
const beijingToday = beijingDate()
const date = ref(beijingToday)
const search = ref('')
const activity = ref<DailyActivityFilter>('all')
const sortBy = ref<DailySort>('today_cost')
const sortOrder = ref<'asc' | 'desc'>('desc')
const page = ref(1)
const pageSize = ref(Math.min(100, getPersistedPageSize()))
const loading = ref(false)
const error = ref(false)
const data = ref<UserDailyStatsResponse | null>(null)
let queryVersion = 0
let listController: AbortController | undefined
const selectedUser = ref<UserDailyStatsRow | null>(null)
const details = ref<DailyPage<UserDailyActivity> | null>(null)
const detailLoading = ref(false)
const detailError = ref(false)
const detailPage = ref(1)
let detailVersion = 0
let detailController: AbortController | undefined

const activityOptions = [
  { value: 'all', label: 'all' }, { value: 'active', label: 'active' },
  { value: 'checkin', label: 'checkinFilter' }, { value: 'liyu', label: 'liyuFilter' }, { value: 'usage', label: 'usageFilter' }
]
const sortOptions = [
  { value: 'today_cost', label: 'todayCost' }, { value: 'previous_cost', label: 'previousCost' },
  { value: 'checkin_net', label: 'checkinNet' }, { value: 'liyu_net', label: 'liyuNet' },
  { value: 'balance', label: 'paidBalance' }, { value: 'welfare_balance', label: 'welfareBalance' }, { value: 'requests', label: 'requests' }
]
const welfareSources = [
  { field: 'checkin_balance', label: 'checkin' }, { field: 'liyu_balance', label: 'liyu' }, { field: 'gift_balance', label: 'gift' }
]
const columns = computed(() => [
  { key: 'user', label: t(`${p}.user`) }, { key: 'rank', label: t(`${p}.rank`) },
  { key: 'checkin', label: t(`${p}.checkinNet`) }, { key: 'liyu', label: t(`${p}.liyuNet`) },
  { key: 'today', label: `${t(`${p}.todayCost`)} (${data.value?.date ?? date.value})` },
  { key: 'previous', label: `${t(`${p}.previousCost`)} (${data.value?.previous_date ?? ''})` },
  { key: 'balance', label: t(`${p}.paidBalance`) }, { key: 'welfare', label: t(`${p}.welfareBalance`) }, { key: 'actions', label: t(`${p}.actions`) }
])
const detailColumns = computed(() => [
  { key: 'created_at', label: t(`${p}.eventTime`) }, { key: 'kind', label: t(`${p}.eventType`) },
  { key: 'amount', label: t(`${p}.amount`) }, { key: 'ticket_face_value', label: t(`${p}.ticket`) },
  { key: 'outcome', label: t(`${p}.outcome`) }, { key: 'description', label: t(`${p}.detail`) }
])
const money = (value: number) => formatCostFixed(value, 8)
const amountClass = (amount: number) => amount < 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-400'
const time = (value: string) => new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-GB', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }).format(new Date(value))
function translatedValue(group: string, value: string): string {
  return value ? (te(`${p}.${group}.${value}`) ? t(`${p}.${group}.${value}`) : value) : '—'
}

async function load(): Promise<void> {
  const version = ++queryVersion
  listController?.abort()
  listController = new AbortController()
  closeActivities()
  loading.value = true
  error.value = false
  data.value = null
  try {
    const result = await userDailyStatsAPI.list({ date: date.value, search: search.value.trim(), activity: activity.value, sort_by: sortBy.value, sort_order: sortOrder.value, page: page.value, page_size: pageSize.value }, listController.signal)
    if (version === queryVersion) data.value = result
  } catch {
    if (version === queryVersion) error.value = true
  } finally {
    if (version === queryVersion) loading.value = false
  }
}
function applyFilters(): void { page.value = 1; void load() }
function selectDay(offset: number): void { date.value = beijingDate(offset); applyFilters() }
function toggleOrder(): void { sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'; applyFilters() }
function changePage(value: number): void { page.value = value; void load() }
function changePageSize(value: number): void { pageSize.value = Math.min(100, value); setPersistedPageSize(pageSize.value); applyFilters() }
function openUsage(row: UserDailyStatsRow): void {
  void router.push({ path: '/admin/usage', query: { user_id: String(row.user_id), start_date: data.value!.date, end_date: data.value!.date } })
}
function closeActivities(): void {
  ++detailVersion
  detailController?.abort()
  selectedUser.value = null
  details.value = null
}
function openActivities(row: UserDailyStatsRow): void { selectedUser.value = row; detailPage.value = 1; void loadActivities() }
function changeDetailPage(value: number): void { detailPage.value = value; void loadActivities() }
async function loadActivities(): Promise<void> {
  if (!selectedUser.value || !data.value) return
  const version = ++detailVersion
  detailController?.abort()
  detailController = new AbortController()
  detailLoading.value = true
  detailError.value = false
  details.value = null
  try {
    const result = await userDailyStatsAPI.activities(selectedUser.value.user_id, { date: data.value.date, page: detailPage.value, page_size: 20 }, detailController.signal)
    if (version === detailVersion) details.value = result
  } catch {
    if (version === detailVersion) detailError.value = true
  } finally {
    if (version === detailVersion) detailLoading.value = false
  }
}

function exportPage(): void {
  if (!data.value || loading.value) return
  const usageKeys = ['total_cost', 'paid_cost', 'welfare_cost', 'checkin_cost', 'liyu_cost', 'gift_cost', 'subscription_cost', 'unclassified_cost', 'requests', 'tokens'] as const
  const usageLabels = ['todayCost', 'paidCost', 'welfareCost', 'checkin', 'liyu', 'gift', 'subscription', 'unclassified', 'requestCount', 'tokens']
  const activityKeys = ['checkin_count', 'checkin_base', 'checkin_bonus', 'checkin_net', 'liyu_draws', 'liyu_reward', 'liyu_lucky', 'liyu_guess', 'liyu_net', 'liyu_ticket_face_value', 'gift_credit', 'welfare_adjustment'] as const
  const activityLabels = ['checkinCount', 'baseReward', 'bonus', 'checkinNet', 'drawCount', 'reward', 'lucky', 'guess', 'liyuNet', 'ticket', 'giftCredit', 'adjustment']
  const headers = [t(`${p}.date`), 'User ID', t(`${p}.user`), 'Email', t(`${p}.rank`), t(`${p}.paidBalance`), t(`${p}.welfareBalance`), ...welfareSources.map(s => t(`${p}.${s.label}`)), t(`${p}.frozen`), t(`${p}.totalRecharged`), ...activityLabels.map(label => t(`${p}.${label}`, { count: '' })), ...['todayCost', 'previousCost'].flatMap(day => usageLabels.map(label => `${t(`${p}.${day}`)} / ${t(`${p}.${label}`, { requests: '', tokens: '' })}`))]
  const safeText = (value: string) => /^[\s]*[=+\-@]/.test(value) ? `'${value}` : value
  const rows = data.value.items.map(row => [data.value!.date, row.user_id, safeText(row.username), safeText(row.email), row.spending_rank ?? '', money(row.balance), money(row.welfare_balance), money(row.checkin_balance), money(row.liyu_balance), money(row.gift_balance), money(row.frozen_balance), money(row.total_recharged), ...activityKeys.map(key => key === 'liyu_draws' || key === 'checkin_count' ? row.activity[key] : money(row.activity[key])), ...[row.today_usage, row.previous_usage].flatMap(usage => usageKeys.map(key => key === 'requests' || key === 'tokens' ? usage[key] : money(usage[key])))])
  // Quote every cell and neutralize spreadsheet formulas in user-controlled text.
  const csvCell = (value: string | number) => `"${String(value).replace(/"/g, '""')}"`
  const csv = '\uFEFF' + [headers, ...rows].map(row => row.map(csvCell).join(',')).join('\r\n')
  saveAs(new Blob([csv], { type: 'text/csv;charset=utf-8' }), `user_daily_stats_${data.value.date}_page_${data.value.page}.csv`)
}

onMounted(() => { void load() })
onUnmounted(() => { ++queryVersion; listController?.abort(); closeActivities() })
</script>

<style scoped>
.daily-label { @apply mb-2 block text-xs font-medium text-gray-600 dark:text-gray-400; }
.daily-small { @apply text-xs text-gray-500 dark:text-gray-400; }
.daily-pair { @apply flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1; }
</style>
