<template>
  <section v-if="status?.enabled || status?.risk" class="card space-y-4 p-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="font-semibold">{{ t('wallet.checkinTitle') }}</h2>
      <button class="btn btn-secondary" :disabled="busy" @click="load">{{ t('wallet.refresh') }}</button>
    </div>
    <div v-if="status.risk" class="space-y-2 rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
      <p class="font-semibold">{{ t('wallet.suspended') }}</p>
      <p>{{ status.risk.reason }}</p>
      <p>{{ t('wallet.clearedAmount') }}: ${{ money(status.risk.cleared_amount) }} · {{ t('wallet.caseID') }}: {{ status.risk.case_id }}</p>
      <p>{{ t('wallet.reviewHint') }}</p>
    </div>
    <template v-else>
      <p class="text-sm text-gray-500">{{ t('wallet.checkinHint') }} {{ t('wallet.rewardRange') }}: ${{ money(status.min_reward) }} – ${{ money(status.max_reward) }}</p>
      <div class="flex flex-wrap gap-3">
        <button class="btn btn-primary" :disabled="busy || !settings || status.stats.checked_in_today" @click="claim(false)">{{ t(status.stats.checked_in_today ? 'wallet.checked' : 'wallet.checkinAction') }}</button>
        <button v-if="status.bonus_enabled" class="btn btn-secondary" :disabled="busy || !settings || !status.bonus_available" @click="claim(true)">{{ t('wallet.bonus') }}</button>
      </div>
      <input v-model="website" name="website" tabindex="-1" autocomplete="off" aria-hidden="true" class="hidden" />
      <CaptchaChallenge v-if="settings" ref="captcha" v-bind="captchaProps" @verify="onVerify" @expire="proof = {}" @error="proof = {}" />
      <p v-if="status.bonus_enabled" class="text-xs text-gray-500">{{ t('wallet.bonusRule') }} {{ t('wallet.successRate') }}: {{ status.bonus_success_rate }}%</p>
      <p class="text-xs text-gray-500">{{ t('wallet.activityRule') }}</p>
      <p class="text-sm">{{ t('wallet.totalCheckins') }}: {{ status.stats.total_checkins }} · {{ t('wallet.totalReward') }}: ${{ money(status.stats.total_reward) }}</p>
    </template>
    <label class="flex items-center gap-3 text-sm">{{ t('wallet.month') }}<input v-model="month" class="input w-auto" type="month" @change="load" /></label>
    <div class="overflow-x-auto">
      <table class="w-full text-left text-sm"><thead><tr class="border-b dark:border-dark-600"><th class="p-2">{{ t('wallet.date') }}</th><th class="p-2">{{ t('wallet.baseReward') }}</th><th class="p-2">{{ t('wallet.status') }}</th><th class="p-2">{{ t('wallet.reward') }}</th></tr></thead>
        <tbody><tr v-for="r in status.stats.records" :key="r.checkin_date" class="border-b dark:border-dark-600"><td class="p-2">{{ r.checkin_date }}</td><td class="p-2">${{ money(r.base_reward_amount) }}</td><td class="p-2">{{ t(`wallet.${r.bonus_status}`) }}</td><td class="p-2">${{ money(r.reward_amount) }}</td></tr><tr v-if="!status.stats.records.length"><td colspan="4" class="p-4 text-gray-500">{{ t('wallet.noRecords') }}</td></tr></tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import { getPublicSettings } from '@/api/auth'
import { checkinAPI, type CheckinProof, type CheckinStatus } from '@/api/checkin'
import type { PublicSettings } from '@/types'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const status = ref<CheckinStatus | null>(null)
const settings = ref<PublicSettings | null>(null)
const captcha = ref<InstanceType<typeof CaptchaChallenge> | null>(null)
const proof = ref<CheckinProof>({})
const website = ref('')
const busy = ref(false)
const month = ref(new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit' }).format(new Date()))
const money = (n: number) => n.toFixed(8).replace(/(\.\d*?[1-9])0+$|\.0+$/, '$1')
const captchaProps = computed(() => ({
  turnstileEnabled: settings.value?.turnstile_enabled ?? false,
  turnstileSiteKey: settings.value?.turnstile_site_key ?? '',
  tencentEnabled: settings.value?.tencent_captcha_enabled ?? false,
  tencentAppId: settings.value?.tencent_captcha_app_id ?? '',
  tencentRegion: settings.value?.tencent_captcha_region,
  aliyunEnabled: settings.value?.aliyun_captcha_enabled ?? false,
  aliyunSceneId: settings.value?.aliyun_captcha_scene_id ?? '',
  aliyunPrefix: settings.value?.aliyun_captcha_prefix ?? '',
  aliyunRegion: settings.value?.aliyun_captcha_region,
}))
function onVerify(token: string, randstr?: string) {
  proof.value = settings.value?.tencent_captcha_enabled
    ? { tencent_ticket: token, tencent_randstr: randstr }
    : { turnstile_token: token }
}
async function load() {
  try { status.value = await checkinAPI.status(month.value) }
  catch (e) { app.showError(extractApiErrorMessage(e, t('common.error'))) }
}
async function claim(bonus: boolean) {
  busy.value = true
  try {
    const result = await captcha.value?.verifyAction()
    if (!result && (settings.value?.tencent_captcha_enabled || settings.value?.aliyun_captcha_enabled)) return
    if (result) onVerify(result.token, result.randstr)
    await checkinAPI.play(bonus, { ...proof.value, website: website.value })
    await Promise.all([load(), auth.refreshUser()])
    app.showSuccess(t('common.success'))
  } catch (e) { app.showError(extractApiErrorMessage(e, t('common.error'))) }
  finally { busy.value = false; proof.value = {}; captcha.value?.reset() }
}
onMounted(async () => {
  await load()
  try { settings.value = await getPublicSettings() }
  catch (e) { app.showError(extractApiErrorMessage(e, t('common.error'))) }
})
</script>
