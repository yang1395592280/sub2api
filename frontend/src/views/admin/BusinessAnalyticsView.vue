<template>
  <AppLayout>
    <div data-test="business-analytics-page" class="space-y-4">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.businessAnalytics.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.businessAnalytics.description') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" :disabled="loading" @click="reloadActiveTab">
            <Icon name="refresh" size="sm" />
            <span>{{ t('common.refresh') }}</span>
          </button>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(280px,auto)_160px_160px_1fr] xl:items-end">
          <div>
            <label class="input-label">{{ t('admin.dashboard.timeRange') }}</label>
            <DateRangePicker
              v-model:start-date="filters.start_date"
              v-model:end-date="filters.end_date"
              @change="handleFiltersChange"
            />
          </div>
          <div>
            <label class="input-label" for="business-analytics-platform">{{ t('admin.businessAnalytics.platform') }}</label>
            <input
              id="business-analytics-platform"
              v-model.trim="filters.platform"
              class="input"
              :placeholder="t('admin.businessAnalytics.allPlatforms')"
              @keyup.enter="handleFiltersChange"
            />
          </div>
          <div>
            <label class="input-label" for="business-analytics-granularity">{{ t('admin.businessAnalytics.granularity') }}</label>
            <Select
              id="business-analytics-granularity"
              v-model="filters.granularity"
              :options="granularityOptions"
              @change="handleFiltersChange"
            />
          </div>
          <div class="flex flex-wrap gap-2 xl:justify-end">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="handleFiltersChange">
              {{ t('admin.businessAnalytics.applyFilters') }}
            </button>
          </div>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="overflow-x-auto border-b border-gray-200 px-3 dark:border-dark-700">
          <div class="flex min-w-max gap-1">
            <button
              v-for="tab in tabs"
              :key="tab"
              type="button"
              class="tab"
              :class="{ 'tab-active': activeTab === tab }"
              :data-test="`tab-${tab}`"
              @click="switchTab(tab)"
            >
              {{ t(`admin.businessAnalytics.tabs.${tab}`) }}
            </button>
          </div>
        </div>

        <div class="p-4">
          <div v-if="activeTab === 'overview'" class="space-y-4">
            <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div
                v-for="metric in overviewMetrics"
                :key="metric.key"
                class="rounded-md border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/60"
                :data-test="`metric-${metric.key}`"
              >
                <div class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ metric.label }}</div>
                <div class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
              </div>
            </div>

            <div class="overflow-x-auto">
              <table class="w-full min-w-[860px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.date') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.requests') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.activeUsers') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.revenue') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.channelCost') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.grossProfit') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.profitMargin') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr v-for="point in overview?.trend || []" :key="point.date">
                    <td class="analytics-td font-medium">{{ point.date }}</td>
                    <td class="analytics-td text-right">{{ formatNumber(point.requests) }}</td>
                    <td class="analytics-td text-right">{{ formatNumber(point.active_users) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(point.revenue) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(point.channel_cost) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(point.gross_profit) }}</td>
                    <td class="analytics-td text-right">{{ formatPercent(point.profit_margin) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else-if="activeTab === 'groups'" class="overflow-x-auto">
            <table class="w-full min-w-[1180px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800/60">
                <tr>
                  <th class="analytics-th">{{ t('admin.businessAnalytics.columns.group') }}</th>
                  <th class="analytics-th">{{ t('admin.businessAnalytics.columns.platform') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.currentRate') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.averageRate') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.activeUsers') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.requests') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.revenue') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.channelCost') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.grossProfit') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.profitMargin') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.previousChange') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-for="row in groups" :key="row.group_id">
                  <td class="analytics-td font-medium">{{ row.group_name }}</td>
                  <td class="analytics-td">{{ row.platform || '-' }}</td>
                  <td class="analytics-td text-right">{{ formatRate(row.current_rate_multiplier) }}</td>
                  <td class="analytics-td text-right text-gray-500 dark:text-dark-400">{{ t('admin.businessAnalytics.notProvidedByApi') }}</td>
                  <td class="analytics-td text-right">{{ formatNumber(row.active_users) }}</td>
                  <td class="analytics-td text-right">{{ formatNumber(row.requests) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.revenue) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.channel_cost) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.gross_profit) }}</td>
                  <td class="analytics-td text-right">{{ formatPercent(row.profit_margin) }}</td>
                  <td class="analytics-td text-right">{{ formatPercent(row.revenue_change_rate) }}</td>
                </tr>
              </tbody>
            </table>
            <EmptyRows v-if="!groups.length" :text="t('admin.businessAnalytics.empty.groups')" />
          </div>

          <div v-else-if="activeTab === 'channels'" class="overflow-x-auto">
            <table class="w-full min-w-[1180px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800/60">
                <tr>
                  <th class="analytics-th">{{ t('admin.businessAnalytics.columns.channel') }}</th>
                  <th class="analytics-th">{{ t('admin.businessAnalytics.columns.platform') }}</th>
                  <th class="analytics-th">{{ t('admin.businessAnalytics.columns.status') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.currentPrice') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.averagePrice') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.activeUsers') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.requests') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.revenue') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.channelCost') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.grossProfit') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.profitMargin') }}</th>
                  <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.missingPrice') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-for="row in channels" :key="`${row.channel_id}-${row.account_id}`">
                  <td class="analytics-td font-medium">{{ row.account_name }}</td>
                  <td class="analytics-td">{{ row.platform || '-' }}</td>
                  <td class="analytics-td">{{ row.status || '-' }}</td>
                  <td class="analytics-td text-right">{{ formatRate(row.current_channel_price) }}</td>
                  <td class="analytics-td text-right text-gray-500 dark:text-dark-400">{{ t('admin.businessAnalytics.notProvidedByApi') }}</td>
                  <td class="analytics-td text-right">{{ formatNumber(row.active_users) }}</td>
                  <td class="analytics-td text-right">{{ formatNumber(row.requests) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.revenue) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.channel_cost) }}</td>
                  <td class="analytics-td text-right">{{ formatMoney(row.gross_profit) }}</td>
                  <td class="analytics-td text-right">{{ formatPercent(row.profit_margin) }}</td>
                  <td class="analytics-td text-right">{{ formatNumber(row.missing_channel_price_records) }}</td>
                </tr>
              </tbody>
            </table>
            <EmptyRows v-if="!channels.length" :text="t('admin.businessAnalytics.empty.channels')" />
          </div>

          <div v-else-if="activeTab === 'priceImpact'" class="space-y-4">
            <div class="grid gap-3 md:grid-cols-3 xl:grid-cols-[180px_180px_220px_1fr] xl:items-end">
              <div>
                <label class="input-label" for="price-impact-group">{{ t('admin.businessAnalytics.group') }}</label>
                <select
                  id="price-impact-group"
                  v-model.number="priceImpactFilters.group_id"
                  data-test="price-impact-group-select"
                  class="input"
                  :disabled="!groups.length || loading"
                  @change="loadPriceImpact"
                >
                  <option v-if="!groups.length" :value="0">
                    {{ t('admin.businessAnalytics.noGroupsForSelection') }}
                  </option>
                  <option v-for="group in groups" :key="group.group_id" :value="group.group_id">
                    {{ group.group_name || `#${group.group_id}` }}
                  </option>
                </select>
              </div>
              <div>
                <label class="input-label" for="price-impact-date">{{ t('admin.businessAnalytics.changeDate') }}</label>
                <input id="price-impact-date" v-model="priceImpactFilters.change_date" type="date" class="input" @change="loadPriceImpact" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.businessAnalytics.windowDays') }}</label>
                <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-800">
                  <button
                    v-for="days in [3, 7, 14]"
                    :key="days"
                    type="button"
                    class="rounded-md px-3 py-1.5 text-sm font-medium"
                    :class="priceImpactFilters.days === days ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-dark-300'"
                    @click="setImpactWindow(days)"
                  >
                    {{ days }}
                  </button>
                </div>
              </div>
              <div class="xl:text-right">
                <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPriceImpact">
                  {{ t('admin.businessAnalytics.applyFilters') }}
                </button>
              </div>
            </div>

            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.metric') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.before') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.after') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.delta') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr>
                    <td class="analytics-td font-medium">{{ t('admin.businessAnalytics.columns.revenue') }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(priceImpact?.before_revenue) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(priceImpact?.after_revenue) }}</td>
                    <td data-test="price-impact-delta" class="analytics-td text-right">{{ formatMoney(priceImpact?.revenue_delta) }}</td>
                  </tr>
                  <tr>
                    <td class="analytics-td font-medium">{{ t('admin.businessAnalytics.columns.grossProfit') }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(priceImpact?.before_gross_profit) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(priceImpact?.after_gross_profit) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(priceImpact?.gross_profit_delta) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.businessAnalytics.priceImpactUserGap') }}
            </p>
          </div>

          <div v-else class="space-y-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <p
                v-if="missingChannelPriceRecordCount > 0"
                data-test="records-approx-summary"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ t('admin.businessAnalytics.recordsApproxHint', { count: overview?.missing_channel_price_records || 0 }) }}
                <span class="ml-2 rounded-full bg-amber-50 px-2 py-0.5 font-medium text-amber-700 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30">
                  {{ t('admin.businessAnalytics.historicalApproximation') }}
                </span>
              </p>
              <button data-test="reload-records" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadRecords">
                {{ t('common.refresh') }}
              </button>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[1480px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.time') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.user') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.apiKey') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.group') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.channelAccount') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.model') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.tokensRequests') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.groupRateSnapshot') }}</th>
                    <th class="analytics-th">{{ t('admin.businessAnalytics.columns.channelPriceSnapshot') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.revenue') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.channelCost') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.grossProfit') }}</th>
                    <th class="analytics-th text-right">{{ t('admin.businessAnalytics.columns.profitMargin') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr v-for="row in records.items" :key="row.id">
                    <td class="analytics-td whitespace-nowrap">{{ formatDateTime(row.created_at) }}</td>
                    <td class="analytics-td">{{ row.user_email || `#${row.user_id}` }}</td>
                    <td class="analytics-td">{{ row.api_key_name || `#${row.api_key_id}` }}</td>
                    <td class="analytics-td">{{ row.group_name || `#${row.group_id}` }}</td>
                    <td class="analytics-td">{{ row.account_name || `#${row.account_id}` }}</td>
                    <td class="analytics-td">{{ row.model || '-' }}</td>
                    <td class="analytics-td text-right">{{ formatNumber(row.total_tokens) }} / {{ formatNumber(row.requests) }}</td>
                    <td class="analytics-td text-gray-500 dark:text-dark-400">{{ t('admin.businessAnalytics.snapshotUnavailable') }}</td>
                    <td class="analytics-td">
                      <span v-if="isApproximateRecord(row)" :data-test="`record-approximate-${row.id}`" class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30">
                        {{ t('admin.businessAnalytics.historicalApproximation') }}
                      </span>
                      <span v-else class="text-gray-500 dark:text-dark-400">{{ t('admin.businessAnalytics.snapshotUnavailable') }}</span>
                    </td>
                    <td class="analytics-td text-right">{{ formatMoney(row.revenue) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(row.channel_cost) }}</td>
                    <td class="analytics-td text-right">{{ formatMoney(row.gross_profit) }}</td>
                    <td class="analytics-td text-right">{{ formatPercent(recordProfitMargin(row)) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="!records.items.length" data-test="records-empty">
              <EmptyRows :text="t('admin.businessAnalytics.empty.records')" />
            </div>
            <Pagination
              v-if="records.total > 0"
              :page="records.page"
              :total="records.total"
              :page-size="records.page_size"
              @update:page="handleRecordsPageChange"
              @update:pageSize="handleRecordsPageSizeChange"
            />
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  BusinessAnalyticsFilter,
  BusinessChannelRow,
  BusinessGroupRow,
  BusinessOverview,
  BusinessPriceChangeImpact,
  BusinessRecordRow,
  BusinessRecordsResponse,
} from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

type TabKey = 'overview' | 'groups' | 'channels' | 'priceImpact' | 'records'

const EmptyRows = defineComponent({
  props: {
    text: { type: String, required: true },
  },
  setup(props) {
    return () =>
      h(
        'div',
        { class: 'flex min-h-32 items-center justify-center p-6 text-sm text-gray-500 dark:text-dark-400' },
        props.text
      )
  },
})

const { t } = useI18n()
const appStore = useAppStore()

const tabs: TabKey[] = ['overview', 'groups', 'channels', 'priceImpact', 'records']
const activeTab = ref<TabKey>('overview')
const loading = ref(false)
const overview = ref<BusinessOverview | null>(null)
const groups = ref<BusinessGroupRow[]>([])
const channels = ref<BusinessChannelRow[]>([])
const priceImpact = ref<BusinessPriceChangeImpact | null>(null)
const records = reactive<BusinessRecordsResponse>({
  items: [],
  total: 0,
  page: 1,
  page_size: getPersistedPageSize(),
})

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const defaultEnd = new Date()
const defaultStart = new Date(defaultEnd.getTime() - 6 * 24 * 60 * 60 * 1000)

const filters = reactive<BusinessAnalyticsFilter>({
  start_date: formatLocalDate(defaultStart),
  end_date: formatLocalDate(defaultEnd),
  granularity: 'day',
  platform: '',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
})

const priceImpactFilters = reactive({
  group_id: 0,
  change_date: formatLocalDate(defaultEnd),
  days: 7,
})

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.businessAnalytics.day') },
  { value: 'week', label: t('admin.businessAnalytics.week') },
])

const overviewMetrics = computed(() => [
  { key: 'revenue', label: t('admin.businessAnalytics.metrics.revenue'), value: formatMoney(overview.value?.revenue) },
  { key: 'channelCost', label: t('admin.businessAnalytics.metrics.channelCost'), value: formatMoney(overview.value?.channel_cost) },
  { key: 'grossProfit', label: t('admin.businessAnalytics.metrics.grossProfit'), value: formatMoney(overview.value?.gross_profit) },
  { key: 'profitMargin', label: t('admin.businessAnalytics.metrics.profitMargin'), value: formatPercent(overview.value?.profit_margin) },
  { key: 'activeUsers', label: t('admin.businessAnalytics.metrics.activeUsers'), value: formatNumber(overview.value?.active_users) },
  { key: 'requests', label: t('admin.businessAnalytics.metrics.requests'), value: formatNumber(overview.value?.requests) },
  { key: 'revenuePerActiveUser', label: t('admin.businessAnalytics.metrics.revenuePerActiveUser'), value: formatMoney(overview.value?.revenue_per_active_user) },
  { key: 'profitPerActiveUser', label: t('admin.businessAnalytics.metrics.profitPerActiveUser'), value: formatMoney(overview.value?.profit_per_active_user) },
])

const missingChannelPriceRecordCount = computed(() => overview.value?.missing_channel_price_records || 0)

const priceImpactGroupOptions = computed(() => groups.value.map((group) => group.group_id))

function normalizedFilters(): BusinessAnalyticsFilter {
  return {
    start_date: filters.start_date,
    end_date: filters.end_date,
    granularity: filters.granularity,
    platform: filters.platform || undefined,
    timezone: filters.timezone,
  }
}

function formatNumber(value: number | null | undefined): string {
  return Number(value || 0).toLocaleString()
}

function formatMoney(value: number | null | undefined): string {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatPercent(value: number | null | undefined): string {
  if (value == null || Number.isNaN(Number(value))) return '-'
  return `${(Number(value) * 100).toFixed(2)}%`
}

function formatRate(value: number | null | undefined): string {
  if (value == null || Number.isNaN(Number(value))) return '-'
  return Number(value).toFixed(4)
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function recordProfitMargin(row: BusinessRecordRow): number | null {
  if (!row.revenue) return null
  return row.gross_profit / row.revenue
}

function isApproximateRecord(row: BusinessRecordRow): boolean {
  return Boolean((row as BusinessRecordRow & { channel_price_snapshot_missing?: boolean }).channel_price_snapshot_missing)
}

async function withLoading(work: () => Promise<void>): Promise<void> {
  loading.value = true
  try {
    await work()
  } catch (error) {
    console.error('Failed to load business analytics:', error)
    appStore.showError(t('admin.businessAnalytics.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadOverview(): Promise<void> {
  await withLoading(async () => {
    overview.value = await adminAPI.businessAnalytics.getOverview(normalizedFilters())
  })
}

async function loadGroups(): Promise<void> {
  await withLoading(async () => {
    groups.value = await adminAPI.businessAnalytics.getGroups(normalizedFilters())
    if (!priceImpactGroupOptions.value.includes(priceImpactFilters.group_id)) {
      priceImpactFilters.group_id = groups.value[0]?.group_id || 0
    }
  })
}

async function loadChannels(): Promise<void> {
  await withLoading(async () => {
    channels.value = await adminAPI.businessAnalytics.getChannels(normalizedFilters())
  })
}

async function loadPriceImpact(): Promise<void> {
  if (!groups.value.length) {
    await loadGroups()
  }
  if (!priceImpactFilters.group_id) {
    priceImpact.value = null
    return
  }
  await withLoading(async () => {
    priceImpact.value = await adminAPI.businessAnalytics.getPriceChangeImpact({
      group_id: Number(priceImpactFilters.group_id),
      change_date: priceImpactFilters.change_date,
      days: priceImpactFilters.days,
      timezone: filters.timezone,
    })
  })
}

async function loadRecords(): Promise<void> {
  await withLoading(async () => {
    const response = await adminAPI.businessAnalytics.getRecords({
      ...normalizedFilters(),
      page: records.page,
      page_size: records.page_size,
    })
    records.items = response.items || []
    records.total = response.total || 0
    records.page = response.page || records.page
    records.page_size = response.page_size || records.page_size
  })
}

function reloadActiveTab(): void {
  if (activeTab.value === 'overview') void loadOverview()
  if (activeTab.value === 'groups') void loadGroups()
  if (activeTab.value === 'channels') void loadChannels()
  if (activeTab.value === 'priceImpact') void loadPriceImpact()
  if (activeTab.value === 'records') void loadRecords()
}

function switchTab(tab: TabKey): void {
  activeTab.value = tab
  reloadActiveTab()
}

function handleFiltersChange(): void {
  records.page = 1
  reloadActiveTab()
}

function setImpactWindow(days: number): void {
  priceImpactFilters.days = days
  void loadPriceImpact()
}

function handleRecordsPageChange(page: number): void {
  records.page = page
  void loadRecords()
}

function handleRecordsPageSizeChange(pageSize: number): void {
  records.page = 1
  records.page_size = pageSize
  void loadRecords()
}

onMounted(() => {
  void loadOverview()
})
</script>

<style scoped>
.analytics-th {
  @apply px-4 py-3 text-left text-xs font-semibold text-gray-500 dark:text-dark-400;
}

.analytics-td {
  @apply px-4 py-3 text-sm text-gray-700 dark:text-dark-200;
}
</style>
