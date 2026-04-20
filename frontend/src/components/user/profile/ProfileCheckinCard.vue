<template>
  <div id="profile-checkin" class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-start justify-between gap-4">
        <div>
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300">
              <Icon name="calendar" size="lg" />
            </div>
            <div>
              <h2 class="text-lg font-medium text-gray-900 dark:text-white">
                {{ t('profile.checkin.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('profile.checkin.description', { min: formatReward(props.minReward), max: formatReward(props.maxReward) }) }}
              </p>
            </div>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-primary shrink-0"
          :disabled="submitting || loading || status?.stats.checked_in_today"
          @click="handleCheckinClick"
        >
          <Icon v-if="!submitting" name="gift" size="sm" class="mr-2" />
          <svg v-else class="mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{
            status?.stats.checked_in_today
              ? t('profile.checkin.checkedToday')
              : submitting
                ? t('profile.checkin.submitting')
                : t('profile.checkin.action')
          }}
        </button>
      </div>
    </div>

    <div class="px-6 py-6 space-y-6">
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('profile.checkin.totalCheckins') }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status?.stats.total_checkins || 0 }}</div>
        </div>
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('profile.checkin.monthReward') }}</div>
          <div class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">
            {{ formatReward(monthlyReward) }}
          </div>
        </div>
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('profile.checkin.totalReward') }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatReward(status?.stats.total_reward || 0) }}
          </div>
        </div>
      </div>

      <div
        v-if="showTurnstile && props.turnstileEnabled && props.turnstileSiteKey"
        class="rounded-2xl border border-primary-200 bg-primary-50 px-4 py-4 dark:border-primary-800/40 dark:bg-primary-900/10"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="font-medium text-primary-800 dark:text-primary-200">
              {{ t('profile.checkin.securityTitle') }}
            </div>
            <p class="mt-1 text-sm text-primary-700 dark:text-primary-300">
              {{ t('profile.checkin.securityDescription') }}
            </p>
          </div>
          <button type="button" class="text-sm text-primary-700 hover:text-primary-900 dark:text-primary-300" @click="closeTurnstile">
            {{ t('common.cancel') }}
          </button>
        </div>
        <div class="mt-4">
          <TurnstileWidget
            :key="turnstileWidgetKey"
            :site-key="props.turnstileSiteKey"
            @verify="handleTurnstileVerify"
            @expire="handleTurnstileExpire"
            @error="handleTurnstileError"
          />
        </div>
      </div>

      <div class="rounded-3xl border border-gray-100 p-4 dark:border-dark-700">
        <div class="mb-4 flex items-center justify-between">
          <button type="button" class="calendar-nav-btn" @click="shiftMonth(-1)">
            <Icon name="chevronLeft" size="sm" />
          </button>
          <div class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ monthLabel }}
          </div>
          <button type="button" class="calendar-nav-btn" @click="shiftMonth(1)">
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>

        <div class="grid grid-cols-7 gap-2 text-center text-xs font-medium uppercase tracking-[0.12em] text-gray-400">
          <div v-for="weekday in weekdays" :key="weekday">
            {{ weekday }}
          </div>
        </div>

        <div v-if="loading" class="py-12 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('profile.checkin.loading') }}
        </div>

        <div v-else class="mt-3 grid grid-cols-7 gap-2">
          <div
            v-for="day in calendarDays"
            :key="day.key"
            class="calendar-cell"
            :class="{
              'calendar-cell-empty': !day.inCurrentMonth,
              'calendar-cell-checked': !!day.record,
              'calendar-cell-today': day.isToday
            }"
          >
            <div class="text-xs font-medium">{{ day.date.getDate() }}</div>
            <div v-if="day.record" class="mt-1 flex items-center justify-center gap-1 text-[10px] font-medium text-emerald-600 dark:text-emerald-300">
              <Icon name="checkCircle" size="xs" />
              <span>{{ formatRewardCompact(day.record.reward_amount) }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 px-4 py-4 text-sm text-gray-600 dark:bg-dark-700/60 dark:text-gray-300">
        <ul class="space-y-2">
          <li>{{ t('profile.checkin.ruleOne') }}</li>
          <li>{{ t('profile.checkin.ruleTwo') }}</li>
          <li>{{ t('profile.checkin.ruleThree') }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import Icon from '@/components/icons/Icon.vue'
import { checkinAPI, type CheckinRecordSummary, type CheckinStatus } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  enabled: boolean
  minReward: number
  maxReward: number
  turnstileEnabled: boolean
  turnstileSiteKey: string
}>()

type CalendarDay = {
  key: string
  date: Date
  inCurrentMonth: boolean
  isToday: boolean
  record?: CheckinRecordSummary
}

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<CheckinStatus | null>(null)
const loading = ref(false)
const submitting = ref(false)
const currentMonth = ref(formatMonth(new Date()))
const showTurnstile = ref(false)
const turnstileToken = ref('')
const turnstileWidgetKey = ref(0)

const weekdays = computed(() => [
  t('profile.checkin.weekdays.mon'),
  t('profile.checkin.weekdays.tue'),
  t('profile.checkin.weekdays.wed'),
  t('profile.checkin.weekdays.thu'),
  t('profile.checkin.weekdays.fri'),
  t('profile.checkin.weekdays.sat'),
  t('profile.checkin.weekdays.sun')
])

const monthLabel = computed(() => {
  const date = parseMonth(currentMonth.value)
  return date.toLocaleDateString(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'long'
  })
})

const monthlyReward = computed(() => {
  return (status.value?.stats.records || []).reduce((sum, record) => sum + (record.reward_amount || 0), 0)
})

const calendarDays = computed<CalendarDay[]>(() => {
  const monthDate = parseMonth(currentMonth.value)
  const firstOfMonth = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1)
  const startOffset = (firstOfMonth.getDay() + 6) % 7
  const gridStart = new Date(firstOfMonth)
  gridStart.setDate(firstOfMonth.getDate() - startOffset)
  const today = formatDate(new Date())
  const recordMap = new Map((status.value?.stats.records || []).map((record) => [record.checkin_date, record]))
  const days: CalendarDay[] = []

  for (let index = 0; index < 42; index += 1) {
    const date = new Date(gridStart)
    date.setDate(gridStart.getDate() + index)
    const formatted = formatDate(date)
    days.push({
      key: `${formatted}-${index}`,
      date,
      inCurrentMonth: date.getMonth() === monthDate.getMonth(),
      isToday: formatted === today,
      record: recordMap.get(formatted)
    })
  }

  return days
})

watch(() => props.enabled, (value) => {
  if (value) {
    void fetchStatus()
  }
})

watch(currentMonth, () => {
  if (props.enabled) {
    void fetchStatus()
  }
})

onMounted(() => {
  if (props.enabled) {
    void fetchStatus()
  }
})

async function fetchStatus() {
  loading.value = true
  try {
    status.value = await checkinAPI.getStatus(currentMonth.value)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.checkin.loadFailed')))
  } finally {
    loading.value = false
  }
}

function handleCheckinClick() {
  if (props.turnstileEnabled && props.turnstileSiteKey) {
    showTurnstile.value = true
    turnstileToken.value = ''
    turnstileWidgetKey.value += 1
    return
  }
  void submitCheckin()
}

async function submitCheckin(token?: string) {
  submitting.value = true
  try {
    const result = await checkinAPI.doCheckin(token)
    appStore.showSuccess(t('profile.checkin.success', { reward: formatReward(result.reward_amount) }))
    showTurnstile.value = false
    turnstileToken.value = ''
    await Promise.all([fetchStatus(), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.checkin.submitFailed')))
    turnstileWidgetKey.value += 1
  } finally {
    submitting.value = false
  }
}

function handleTurnstileVerify(token: string) {
  turnstileToken.value = token
  void submitCheckin(token)
}

function handleTurnstileExpire() {
  turnstileToken.value = ''
  appStore.showError(t('auth.turnstileExpired'))
}

function handleTurnstileError() {
  turnstileToken.value = ''
  appStore.showError(t('auth.turnstileFailed'))
}

function closeTurnstile() {
  showTurnstile.value = false
  turnstileToken.value = ''
  turnstileWidgetKey.value += 1
}

function shiftMonth(offset: number) {
  const date = parseMonth(currentMonth.value)
  date.setMonth(date.getMonth() + offset)
  currentMonth.value = formatMonth(date)
}

function parseMonth(value: string): Date {
  const [year, month] = value.split('-').map(Number)
  return new Date(year, (month || 1) - 1, 1)
}

function formatMonth(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatReward(value: number): string {
  return `$${value < 0.1 ? value.toFixed(4) : value.toFixed(2)}`
}

function formatRewardCompact(value: number): string {
  return value < 0.1 ? value.toFixed(3) : value.toFixed(2)
}
</script>

<style scoped>
.calendar-nav-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 2rem;
  width: 2rem;
  border-radius: 9999px;
  border: 1px solid rgb(229 231 235 / 1);
  color: rgb(75 85 99 / 1);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.calendar-nav-btn:hover {
  background: rgb(243 244 246 / 1);
}

.calendar-cell {
  min-height: 5rem;
  border-radius: 1rem;
  border: 1px solid rgb(243 244 246 / 1);
  padding: 0.75rem 0.5rem;
  text-align: center;
  color: rgb(31 41 55 / 1);
  background: rgb(255 255 255 / 1);
}

.calendar-cell-empty {
  opacity: 0.35;
}

.calendar-cell-checked {
  border-color: rgb(167 243 208 / 1);
  background: rgb(236 253 245 / 1);
}

.calendar-cell-today {
  box-shadow: inset 0 0 0 1px rgb(59 130 246 / 0.35);
}

:global(.dark) .calendar-nav-btn {
  border-color: rgb(55 65 81 / 1);
  color: rgb(209 213 219 / 1);
}

:global(.dark) .calendar-nav-btn:hover {
  background: rgb(31 41 55 / 1);
}

:global(.dark) .calendar-cell {
  border-color: rgb(31 41 55 / 1);
  color: rgb(243 244 246 / 1);
  background: rgb(17 24 39 / 0.25);
}

:global(.dark) .calendar-cell-checked {
  border-color: rgb(6 95 70 / 1);
  background: rgb(6 78 59 / 0.2);
}
</style>
