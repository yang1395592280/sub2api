<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.checkinAnalytics.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.checkinAnalytics.description') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary" @click="loadAnalytics">
            {{ t('common.refresh') }}
          </button>
        </div>

        <div class="mt-5 flex flex-wrap gap-2">
          <button
            v-for="option in presets"
            :key="option.value"
            type="button"
            class="rounded-full border px-4 py-2 text-sm font-medium transition"
            :class="selectedPreset === option.value
              ? 'border-primary-500 bg-primary-50 text-primary-600 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-400'
              : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-700 dark:text-gray-300'"
            @click="selectedPreset = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <input
            v-model="searchQuery"
            type="text"
            class="input"
            :placeholder="t('admin.checkinAnalytics.filters.searchPlaceholder')"
          />
          <input
            v-model="timezone"
            type="text"
            class="input"
            :placeholder="t('admin.checkinAnalytics.filters.timezonePlaceholder')"
          />
          <input
            v-model="customStartDate"
            type="date"
            class="input"
            :disabled="selectedPreset !== 'custom'"
          />
          <input
            v-model="customEndDate"
            type="date"
            class="input"
            :disabled="selectedPreset !== 'custom'"
          />
        </div>
      </div>

      <div v-if="analyticsError" class="rounded-2xl border border-red-200 bg-red-50 p-6 text-center text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/20 dark:text-red-300">
        <p>{{ t('admin.checkinAnalytics.failedToLoad') }}</p>
        <button type="button" class="btn btn-secondary mt-4" @click="loadAnalytics">
          {{ t('common.retry') }}
        </button>
      </div>

      <template v-else>
        <CheckinAnalyticsOverviewCards
          :overview="analytics?.overview"
          :loading="analyticsLoading"
        />

        <div class="grid gap-6 xl:grid-cols-[2fr_1fr]">
          <CheckinTrendChart
            :points="analytics?.trend ?? []"
            :loading="analyticsLoading"
            @select-date="handleTrendSelect"
          />
          <CheckinTopUsersCard
            :users="analytics?.top_users ?? []"
            :loading="analyticsLoading"
            @select-user="handleUserSelect"
          />
        </div>

        <CheckinRewardDistributionChart
          :buckets="analytics?.reward_distribution ?? []"
          :loading="analyticsLoading"
        />

        <CheckinDetailsTable
          :filters="detailFilters"
          :linked-date="linkedDate"
          :linked-user="linkedUser"
          @clear-linkage="clearLinkage"
        />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api/admin'
import type { AdminCheckinAnalyticsResponse, AdminCheckinTopUser } from '@/api/admin/checkins'
import { useAppStore } from '@/stores'
import CheckinAnalyticsOverviewCards from './checkin-analytics/components/CheckinAnalyticsOverviewCards.vue'
import CheckinTrendChart from './checkin-analytics/components/CheckinTrendChart.vue'
import CheckinRewardDistributionChart from './checkin-analytics/components/CheckinRewardDistributionChart.vue'
import CheckinTopUsersCard from './checkin-analytics/components/CheckinTopUsersCard.vue'
import CheckinDetailsTable from './checkin-analytics/components/CheckinDetailsTable.vue'

defineOptions({ name: 'CheckinAnalyticsView' })

type PresetValue = '7d' | '30d' | 'month' | 'custom'

const { t } = useI18n()
const appStore = useAppStore()

const analytics = ref<AdminCheckinAnalyticsResponse | null>(null)
const analyticsLoading = ref(false)
const analyticsError = ref(false)

const selectedPreset = ref<PresetValue>('30d')
const searchQuery = ref('')
const timezone = ref('')
const customStartDate = ref('')
const customEndDate = ref('')
const linkedDate = ref('')
const linkedUser = ref<AdminCheckinTopUser | null>(null)

const presets = computed(() => [
  { value: '7d' as const, label: t('admin.checkinAnalytics.filters.last7Days') },
  { value: '30d' as const, label: t('admin.checkinAnalytics.filters.last30Days') },
  { value: 'month' as const, label: t('admin.checkinAnalytics.filters.thisMonth') },
  { value: 'custom' as const, label: t('admin.checkinAnalytics.filters.custom') }
])

const dateRange = computed(() => {
  const now = new Date()
  const end = formatDateInput(now)

  if (selectedPreset.value === 'custom') {
    return {
      start: customStartDate.value || end,
      end: customEndDate.value || end
    }
  }

  if (selectedPreset.value === 'month') {
    return {
      start: formatDateInput(new Date(now.getFullYear(), now.getMonth(), 1)),
      end
    }
  }

  const offset = selectedPreset.value === '7d' ? 6 : 29
  const startDate = new Date(now)
  startDate.setDate(now.getDate() - offset)

  return {
    start: formatDateInput(startDate),
    end
  }
})

const detailFilters = computed(() => ({
  search: linkedUser.value?.user_email || linkedUser.value?.user_name || searchQuery.value || undefined,
  date: linkedDate.value || undefined,
  timezone: timezone.value || undefined
}))

async function loadAnalytics() {
  analyticsLoading.value = true
  analyticsError.value = false
  try {
    analytics.value = await adminAPI.checkins.getAnalytics({
      start_date: dateRange.value.start,
      end_date: dateRange.value.end,
      search: searchQuery.value || undefined,
      timezone: timezone.value || undefined,
      top_limit: 10
    })
  } catch (error) {
    console.error('Error loading checkin analytics:', error)
    analyticsError.value = true
    appStore.showError(t('admin.checkinAnalytics.failedToLoad'))
  } finally {
    analyticsLoading.value = false
  }
}

function handleTrendSelect(date: string) {
  linkedDate.value = date
}

function handleUserSelect(user: AdminCheckinTopUser) {
  linkedUser.value = user
}

function clearLinkage() {
  linkedDate.value = ''
  linkedUser.value = null
}

watch(
  () => [selectedPreset.value, searchQuery.value, timezone.value, customStartDate.value, customEndDate.value],
  () => {
    clearLinkage()
    void loadAnalytics()
  },
  { immediate: true }
)

function formatDateInput(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
</script>
