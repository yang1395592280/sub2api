<template>
  <div v-if="status?.enabled || status?.risk" id="profile-checkin" class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-center justify-between gap-4">
        <div>
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300">
              <Icon name="calendar" size="lg" />
            </div>
            <div>
              <h2 class="text-lg font-medium text-gray-900 dark:text-white">
                {{ t('wallet.checkinTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ `${t('wallet.checkinHint')} ${t('wallet.rewardRange')}: ${formatReward(status?.min_reward || 0)} – ${formatReward(status?.max_reward || 0)}` }}
              </p>
            </div>
          </div>
        </div>
        <div v-if="!status?.risk" class="flex shrink-0 flex-col gap-2 sm:flex-row sm:items-center sm:gap-2.5">
          <button
            type="button"
            class="btn btn-primary"
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
                ? t('wallet.checked')
                : submitting
                  ? t('wallet.checkinAction')
                  : t('wallet.checkinAction')
            }}
          </button>
          <button
            v-if="status?.bonus_enabled"
            data-testid="checkin-bonus-button"
            type="button"
            class="lucky-bonus-btn whitespace-nowrap"
            :disabled="bonusButtonDisabled"
            @click="handleLuckyBonus"
          >
            <span class="lucky-bonus-btn__shine"></span>
            <span class="relative z-10 flex items-center">
              <svg v-if="bonusSubmitting" class="mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <Icon v-else name="gift" size="sm" class="mr-2" />
              {{ bonusSubmitting ? t('wallet.bonusPending') : t('wallet.bonus') }}
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="px-6 py-6 space-y-6">
      <div v-if="status?.risk" class="space-y-2 rounded-2xl bg-red-50 px-4 py-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
        <p class="font-semibold">{{ t('wallet.suspended') }}</p>
        <p>{{ status.risk.reason }}</p>
        <p>{{ t('wallet.clearedAmount') }}: {{ formatReward(status.risk.cleared_amount) }} · {{ t('wallet.caseID') }}: {{ status.risk.case_id }}</p>
        <p>{{ t('wallet.reviewHint') }}</p>
      </div>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('wallet.totalCheckins') }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status?.stats.total_checkins || 0 }}</div>
        </div>
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('wallet.monthReward') }}</div>
          <div class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">
            {{ formatReward(monthlyReward) }}
          </div>
        </div>
        <div class="rounded-2xl bg-gray-50 px-4 py-4 dark:bg-dark-700/60">
          <div class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('wallet.totalReward') }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatReward(status?.stats.total_reward || 0) }}
          </div>
        </div>
      </div>

      <div
        v-if="status?.bonus_enabled"
        class="lucky-bonus-panel"
        :class="{
          'lucky-bonus-panel-win': bonusResultRecord?.bonus_status === 'win',
          'lucky-bonus-panel-lose': bonusResultRecord?.bonus_status === 'lose',
          'lucky-bonus-panel-idle': !bonusResultRecord,
          'lucky-bonus-panel-drawing': bonusSubmitting
        }"
      >
        <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div class="flex items-center gap-3">
            <div class="bonus-flip-card" :class="{ 'bonus-flip-card-active': bonusSubmitting, 'bonus-flip-card-win': bonusResultRecord?.bonus_status === 'win', 'bonus-flip-card-lose': bonusResultRecord?.bonus_status === 'lose' }" aria-hidden="true">
              <Icon name="sparkles" size="sm" />
              <span v-if="bonusResultRecord && !bonusSubmitting">{{ bonusResultRecord.bonus_status === 'win' ? '×2' : '−½' }}</span>
            </div>
            <div>
            <div class="text-xs uppercase tracking-[0.2em] text-gray-500 dark:text-gray-400">
              {{ t('wallet.bonusResultTitle') }}
            </div>
            <div class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{
                bonusResultRecord
                  ? bonusResultRecord.bonus_status === 'win'
                    ? t('wallet.bonusWin')
                    : t('wallet.bonusLose')
                  : status?.bonus_available
                    ? t('wallet.bonus')
                    : status?.stats.checked_in_today
                      ? t('wallet.bonusAlreadyPlayed')
                      : t('wallet.bonusCheckinFirst')
              }}
            </div>
            </div>
          </div>
          <div class="grid min-w-[240px] grid-cols-3 gap-3 text-sm">
            <div class="rounded-2xl bg-white/80 px-3 py-3 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.baseReward') }}</div>
              <div class="mt-1 font-semibold text-gray-900 dark:text-white">
                {{ formatReward(status?.today_record?.base_reward_amount || 0) }}
              </div>
            </div>
            <div class="rounded-2xl bg-white/80 px-3 py-3 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.reward') }}</div>
              <div class="mt-1 font-semibold text-gray-900 dark:text-white">
                {{ formatReward(status?.today_record?.reward_amount || 0) }}
              </div>
            </div>
            <div class="rounded-2xl bg-white/80 px-3 py-3 dark:bg-dark-900/40">
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.bonusDelta') }}</div>
              <div
                class="mt-1 font-semibold"
                :class="(status?.today_record?.bonus_delta_amount || 0) >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'"
              >
                {{ formatSignedReward(status?.today_record?.bonus_delta_amount || 0) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!status?.risk && settings && (captchaProps.turnstileEnabled || captchaProps.tencentEnabled || captchaProps.aliyunEnabled)" class="rounded-2xl border border-primary-200 bg-primary-50 px-4 py-3 dark:border-primary-800/40 dark:bg-primary-900/10">
        <CaptchaChallenge
          ref="captcha"
          v-bind="captchaProps"
          @verify="onVerify"
          @expire="handleCaptchaExpire"
          @error="handleCaptchaExpire"
        />
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
          {{ t('common.loading') }}
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
          <li>{{ t('wallet.checkinHint') }}</li>
          <li>{{ t('wallet.bonusRule') }}</li>
          <li>{{ t('wallet.activityRule') }}</li>
          <li>{{ t('wallet.debt') }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatWalletAmount } from '@/utils/walletDisplay'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPublicSettings } from '@/api/auth'
import { checkinAPI, type CheckinProof, type CheckinRecord, type CheckinStatus } from '@/api/checkin'
import type { PublicSettings } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const app = useAppStore()
const auth = useAuthStore()
const status = ref<CheckinStatus | null>(null)
const settings = ref<PublicSettings | null>(null)
const captcha = ref<InstanceType<typeof CaptchaChallenge> | null>(null)
const proof = ref<CheckinProof>({})
const pendingAction = ref<boolean | null>(null)
const website = ref('')
const loading = ref(false)
const submitting = ref(false)
const bonusSubmitting = ref(false)
const currentMonth = ref(formatMonth(new Date()))
const selectedDate = ref('')
let disposed = false
let loadRequest = 0
let bonusAnimationTimer: number | undefined

interface CalendarDay {
  key: string
  date: Date
  inCurrentMonth: boolean
  isToday: boolean
  record?: CheckinRecord
}

const weekdays = computed(() => [
  t('wallet.weekdays.mon'), t('wallet.weekdays.tue'), t('wallet.weekdays.wed'),
  t('wallet.weekdays.thu'), t('wallet.weekdays.fri'), t('wallet.weekdays.sat'), t('wallet.weekdays.sun')
])
const monthLabel = computed(() => parseMonth(currentMonth.value).toLocaleDateString(locale?.value === 'zh' ? 'zh-CN' : 'en-US', { year: 'numeric', month: 'long' }))
const monthlyReward = computed(() => (status.value?.stats.records || []).reduce((sum, record) => sum + (record.reward_amount || 0), 0))
const bonusButtonDisabled = computed(() => loading.value || submitting.value || bonusSubmitting.value || !status.value?.bonus_enabled || !status.value?.bonus_available || !!status.value?.risk)
const bonusResultRecord = computed(() => {
  const record = status.value?.today_record
  return record && record.bonus_status !== 'none' ? record : null
})
const calendarDays = computed<CalendarDay[]>(() => {
  const monthDate = parseMonth(currentMonth.value)
  const firstOfMonth = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1)
  const startOffset = (firstOfMonth.getDay() + 6) % 7
  const gridStart = new Date(firstOfMonth)
  gridStart.setDate(firstOfMonth.getDate() - startOffset)
  const today = formatDate(new Date())
  const recordMap = new Map((status.value?.stats.records || []).map(record => [record.checkin_date, record]))
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(gridStart)
    date.setDate(gridStart.getDate() + index)
    const formatted = formatDate(date)
    return { key: `${formatted}-${index}`, date, inCurrentMonth: date.getMonth() === monthDate.getMonth(), isToday: formatted === today, record: recordMap.get(formatted) }
  })
})
const captchaProps = computed(() => ({
  turnstileEnabled: settings.value?.turnstile_enabled ?? false,
  turnstileSiteKey: settings.value?.turnstile_site_key ?? '',
  tencentEnabled: settings.value?.tencent_captcha_enabled ?? false,
  tencentAppId: settings.value?.tencent_captcha_app_id ?? '',
  tencentRegion: settings.value?.tencent_captcha_region,
  aliyunEnabled: settings.value?.aliyun_captcha_enabled ?? false,
  aliyunSceneId: settings.value?.aliyun_captcha_scene_id ?? '',
  aliyunPrefix: settings.value?.aliyun_captcha_prefix ?? '',
  aliyunRegion: settings.value?.aliyun_captcha_region
}))

watch(currentMonth, () => { void fetchStatus() })
onMounted(async () => {
  await Promise.all([fetchStatus(), getPublicSettings().then(value => { settings.value = value }).catch(error => app.showError(extractApiErrorMessage(error, t('common.error'))))])
})
onBeforeUnmount(() => {
  disposed = true
  if (bonusAnimationTimer !== undefined) window.clearTimeout(bonusAnimationTimer)
})

async function fetchStatus() {
  const requestID = ++loadRequest
  loading.value = true
  try {
    const result = await checkinAPI.status(currentMonth.value)
    if (!disposed && requestID === loadRequest) {
      status.value = result
      if (!selectedDate.value || !selectedDate.value.startsWith(currentMonth.value)) selectedDate.value = formatDate(new Date())
    }
  } catch (error) {
    if (!disposed && requestID === loadRequest) app.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    if (!disposed && requestID === loadRequest) loading.value = false
  }
}

function handleCheckinClick() { void claim(false) }
function handleLuckyBonus() { void claim(true) }

async function claim(bonus: boolean) {
  if (bonus ? bonusButtonDisabled.value : loading.value || submitting.value || !status.value?.enabled || status.value.stats.checked_in_today || !!status.value?.risk) return
  pendingAction.value = bonus
  const actionCaptchaEnabled = settings.value?.tencent_captcha_enabled || settings.value?.aliyun_captcha_enabled
  if (actionCaptchaEnabled) {
    submitting.value = true
    try {
      const result = await captcha.value?.verifyAction()
      if (!result) {
        pendingAction.value = null
        submitting.value = false
        return
      }
      onVerify(result.token, result.randstr)
      await submitClaim(bonus)
    } finally {
      if (!disposed && pendingAction.value === bonus) pendingAction.value = null
    }
    return
  }
  if (settings.value?.turnstile_enabled && settings.value.turnstile_site_key) {
    // Turnstile is rendered inline; the verify event will finish this pending action.
    submitting.value = true
    return
  }
  await submitClaim(bonus)
}

function onVerify(token: string, randstr?: string) {
  proof.value = settings.value?.tencent_captcha_enabled
    ? { tencent_ticket: token, tencent_randstr: randstr }
    : { turnstile_token: token }
  if (pendingAction.value !== null && settings.value?.turnstile_enabled && !bonusSubmitting.value) void submitClaim(pendingAction.value)
}

async function submitClaim(bonus: boolean) {
  if (bonus) bonusSubmitting.value = true
  else submitting.value = true
  try {
    const result = await checkinAPI.play(bonus, { ...proof.value, website: website.value })
    if (result.checkin_date) applyResult(result)
    if (bonus) {
      if (bonusAnimationTimer !== undefined) window.clearTimeout(bonusAnimationTimer)
      bonusAnimationTimer = window.setTimeout(() => {
        bonusAnimationTimer = undefined
        if (!disposed) bonusSubmitting.value = false
      }, 900)
    }
    if (result.bonus_status === 'lose') app.showWarning(t('wallet.bonusLose'))
    else app.showSuccess(bonus ? t('wallet.bonusWin') : t('wallet.checkinSuccess', { reward: formatReward(result.reward_amount) }))
    await Promise.all([fetchStatus(), auth.refreshUser()])
  } catch (error) {
    if (!disposed) {
      bonusSubmitting.value = false
      app.showError(extractApiErrorMessage(error, t('common.error')))
    }
  } finally {
    if (!disposed) {
      pendingAction.value = null
      proof.value = {}
      captcha.value?.reset()
      if (!bonus) bonusSubmitting.value = false
      submitting.value = false
    }
  }
}

function applyResult(result: CheckinRecord) {
  if (!status.value) return
  const previous = status.value.today_record
  status.value.today_record = result
  status.value.bonus_available = false
  status.value.stats.checked_in_today = true
  status.value.stats.total_reward += result.reward_amount - (previous?.reward_amount || 0)
  if (!previous) status.value.stats.total_checkins += 1
  const records = status.value.stats.records.filter(record => record.checkin_date !== result.checkin_date)
  if (result.checkin_date.startsWith(currentMonth.value)) records.unshift(result)
  status.value.stats.records = records
  selectedDate.value = result.checkin_date
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
function formatMonth(date: Date): string { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}` }
function formatDate(date: Date): string { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` }
function formatReward(value: number): string {
  const amount = formatWalletAmount(value)
  return amount.startsWith('-') ? `-$${amount.slice(1)}` : `$${amount}`
}
function formatRewardCompact(value: number): string { return formatWalletAmount(value) }
function formatSignedReward(value: number): string { return `${formatWalletAmount(value).startsWith('-') ? '-' : '+'}${formatReward(Math.abs(value))}` }
function handleCaptchaExpire() { pendingAction.value = null; proof.value = {}; submitting.value = false }
</script>

<style scoped>
.lucky-bonus-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  min-height: 2.5rem;
  border: 1px solid rgb(251 191 36 / 0.45);
  background: linear-gradient(135deg, rgb(255 247 205 / 1), rgb(255 226 153 / 1));
  color: rgb(120 53 15 / 1);
  border-radius: 9999px;
  padding: 0.55rem 0.95rem;
  font-size: 0.8125rem;
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0.01em;
  box-shadow: 0 6px 16px rgb(245 158 11 / 0.1);
  transition: transform 0.2s ease, box-shadow 0.2s ease, opacity 0.2s ease;
}

.lucky-bonus-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 12px 24px rgb(245 158 11 / 0.16);
}

.lucky-bonus-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.lucky-bonus-btn__shine {
  position: absolute;
  inset: 0;
  background: linear-gradient(120deg, transparent, rgb(255 255 255 / 0.6), transparent);
  transform: translateX(-100%);
  animation: lucky-bonus-shine 2.8s ease-in-out infinite;
}

.bonus-flip-card {
  position: relative;
  display: flex;
  width: 3.5rem;
  height: 4.5rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid rgb(251 191 36 / 0.55);
  border-radius: 0.9rem;
  background: linear-gradient(145deg, rgb(254 243 199 / 1), rgb(252 211 77 / 1));
  color: rgb(161 98 7 / 1);
  box-shadow: 0 8px 18px rgb(180 83 9 / 0.12);
  font-size: 1.25rem;
  font-weight: 800;
}

.bonus-flip-card-active {
  animation: bonus-flip 0.65s linear infinite;
}

.bonus-flip-card-win {
  border-color: rgb(110 231 183 / 1);
  background: linear-gradient(145deg, rgb(236 253 245 / 1), rgb(167 243 208 / 1));
  color: rgb(4 120 87 / 1);
}

.bonus-flip-card-lose {
  border-color: rgb(253 186 116 / 1);
  background: linear-gradient(145deg, rgb(255 247 237 / 1), rgb(254 215 170 / 1));
  color: rgb(194 65 12 / 1);
}

.lucky-bonus-panel {
  border-radius: 1.5rem;
  padding: 1.1rem 1.2rem;
  border: 1px solid rgb(229 231 235 / 1);
}

.lucky-bonus-panel-idle {
  background: linear-gradient(135deg, rgb(255 251 235 / 1), rgb(249 250 251 / 1));
}

.lucky-bonus-panel-win {
  border-color: rgb(110 231 183 / 1);
  background: radial-gradient(circle at top left, rgb(209 250 229 / 1), rgb(236 253 245 / 1) 42%, rgb(255 255 255 / 1));
}

.lucky-bonus-panel-lose {
  border-color: rgb(253 186 116 / 1);
  background: radial-gradient(circle at top left, rgb(255 237 213 / 1), rgb(255 247 237 / 1) 42%, rgb(255 255 255 / 1));
}

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

:global(.dark) .lucky-bonus-btn {
  border-color: rgb(245 158 11 / 0.35);
  background: linear-gradient(135deg, rgb(120 53 15 / 0.9), rgb(146 64 14 / 0.8));
  color: rgb(254 243 199 / 1);
}

:global(.dark) .lucky-bonus-panel-idle {
  background: linear-gradient(135deg, rgb(38 38 38 / 1), rgb(24 24 27 / 1));
  border-color: rgb(63 63 70 / 1);
}

:global(.dark) .lucky-bonus-panel-win {
  background: radial-gradient(circle at top left, rgb(6 78 59 / 0.55), rgb(17 24 39 / 0.92) 52%);
  border-color: rgb(5 150 105 / 0.4);
}

:global(.dark) .lucky-bonus-panel-lose {
  background: radial-gradient(circle at top left, rgb(154 52 18 / 0.38), rgb(17 24 39 / 0.92) 52%);
  border-color: rgb(234 88 12 / 0.35);
}

@keyframes lucky-bonus-shine {
  0% {
    transform: translateX(-130%);
  }
  30%,
  100% {
    transform: translateX(130%);
  }
}

@keyframes bonus-flip {
  0% { transform: rotateY(0deg) rotateZ(-4deg); }
  50% { transform: rotateY(180deg) rotateZ(4deg); }
  100% { transform: rotateY(360deg) rotateZ(-4deg); }
}

@media (prefers-reduced-motion: reduce) {
  .lucky-bonus-btn__shine,
  .bonus-flip-card-active {
    animation: none;
  }
}
</style>
