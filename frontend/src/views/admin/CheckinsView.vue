<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between"><h1 class="text-2xl font-bold">{{ t('wallet.adminTitle') }}</h1><button class="btn btn-secondary" :disabled="busy" @click="load">{{ t('wallet.refresh') }}</button></div>
      <form v-if="settings" class="card space-y-4 p-5" @submit.prevent="save">
        <label class="flex items-center gap-2"><input v-model="settings.enabled" type="checkbox" />{{ t('wallet.enabled') }}</label>
        <div class="grid gap-4 sm:grid-cols-2"><label>{{ t('wallet.minReward') }}<input v-model.number="settings.min_reward" class="input mt-1" type="number" min="0" max="10000" step="0.00000001" required /></label><label>{{ t('wallet.maxReward') }}<input v-model.number="settings.max_reward" class="input mt-1" type="number" min="0" max="10000" step="0.00000001" required /></label></div>
        <label class="flex items-center gap-2"><input v-model="settings.bonus_enabled" type="checkbox" />{{ t('wallet.bonusEnabled') }}</label>
        <label class="block">{{ t('wallet.successRate') }}<input v-model.number="settings.bonus_success_rate" class="input mt-1" type="number" min="0" max="100" step="0.01" required /></label>
        <label class="block">{{ t('wallet.distribution') }}<textarea v-model="distribution" class="input mt-1 font-mono" rows="3" placeholder='[{"start_percent":0,"end_percent":100,"weight":1}]' /></label>
        <p class="text-xs text-gray-500">{{ t('wallet.bonusRule') }} {{ t('wallet.moneyUnit') }}</p>
        <button class="btn btn-primary" :disabled="busy">{{ t('common.save') }}</button>
      </form>
      <section class="card space-y-4 p-5">
        <h2 class="font-semibold">{{ t('wallet.riskCases') }}</h2><p class="text-sm text-gray-500">{{ t('wallet.observationRule') }}</p>
        <form class="grid gap-3 sm:grid-cols-2" @submit.prevent="confirmRisk">
          <label>{{ t('wallet.userID') }}<input v-model.number="riskUser" class="input mt-1" type="number" min="1" required /></label>
          <label>{{ t('wallet.reason') }}<input v-model="reason" class="input mt-1" maxlength="2000" required /></label>
          <label class="sm:col-span-2">{{ t('wallet.evidence') }}<textarea v-model="evidence" class="input mt-1" maxlength="8000" required /></label>
          <button class="btn btn-primary sm:col-span-2" :disabled="busy">{{ t('wallet.confirmRisk') }}</button>
        </form>
        <div class="overflow-x-auto"><table class="w-full text-left text-sm"><thead><tr><th class="p-2">ID / {{ t('wallet.userID') }}</th><th class="p-2">{{ t('wallet.status') }}</th><th class="p-2">{{ t('wallet.reason') }} / {{ t('wallet.evidence') }}</th><th class="p-2">{{ t('wallet.observeUntil') }}</th><th class="p-2">{{ t('wallet.clearedAmount') }}</th><th /></tr></thead><tbody>
          <tr v-for="r in risks" :key="r.id" class="border-t dark:border-dark-600"><td class="p-2">{{ r.id }} / {{ r.user_id }}</td><td class="p-2">{{ t(`wallet.${r.status}`) }}</td><td class="max-w-md whitespace-pre-wrap break-words p-2">{{ r.reason }}<p class="text-xs text-gray-500">{{ r.evidence }}</p></td><td class="p-2">{{ new Date(r.observe_until).toLocaleString() }}</td><td class="p-2">${{ r.cleared_amount.toFixed(8) }}</td><td class="p-2"><button v-if="r.status === 'observing'" class="btn btn-secondary" :disabled="busy" @click="resolve(r.id, 'revoke')">{{ t('wallet.revoke') }}</button><button v-if="r.status === 'cleared'" class="btn btn-secondary" :disabled="busy" @click="resolve(r.id, 'restore')">{{ t('wallet.restore') }}</button></td></tr>
        </tbody></table></div>
        <div class="flex items-center gap-3"><button class="btn btn-secondary" :disabled="riskPage <= 1 || busy" @click="riskPage--; loadRisks()">←</button><span>{{ riskPage }} / {{ Math.max(1, Math.ceil(riskTotal / 20)) }}</span><button class="btn btn-secondary" :disabled="riskPage * 20 >= riskTotal || busy" @click="riskPage++; loadRisks()">→</button></div>
      </section>
      <section class="card space-y-4 p-5"><h2 class="font-semibold">{{ t('wallet.records') }}</h2><div class="overflow-x-auto"><table class="w-full text-left text-sm"><thead><tr><th class="p-2">{{ t('wallet.userID') }}</th><th class="p-2">{{ t('wallet.date') }}</th><th class="p-2">{{ t('wallet.baseReward') }}</th><th class="p-2">{{ t('wallet.status') }}</th><th class="p-2">{{ t('wallet.reward') }}</th></tr></thead><tbody><tr v-for="r in records" :key="r.id" class="border-t dark:border-dark-600"><td class="p-2">{{ r.user_id }} {{ r.user_email }}</td><td class="p-2">{{ r.checkin_date }}</td><td class="p-2">${{ r.base_reward_amount.toFixed(8) }}</td><td class="p-2">{{ t(`wallet.${r.bonus_status}`) }}</td><td class="p-2">${{ r.reward_amount.toFixed(8) }}</td></tr></tbody></table></div>
        <div class="flex items-center gap-3"><button class="btn btn-secondary" :disabled="recordPage <= 1 || busy" @click="recordPage--; loadRecords()">←</button><span>{{ recordPage }} / {{ Math.max(1, Math.ceil(recordTotal / 20)) }}</span><button class="btn btn-secondary" :disabled="recordPage * 20 >= recordTotal || busy" @click="recordPage++; loadRecords()">→</button></div>
      </section>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { checkinAPI, type CheckinRecord, type CheckinRisk, type CheckinSettings } from '@/api/checkin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const app = useAppStore()
const settings = ref<CheckinSettings | null>(null)
const distribution = ref('')
const records = ref<Array<CheckinRecord & { user_email: string }>>([])
const risks = ref<CheckinRisk[]>([])
const busy = ref(false), recordPage = ref(1), riskPage = ref(1), recordTotal = ref(0), riskTotal = ref(0)
const riskUser = ref<number>(), reason = ref(''), evidence = ref('')
async function run(action: () => Promise<void>) {
  busy.value = true
  try { await action() } catch (e) { app.showError(extractApiErrorMessage(e, t('common.error'))) }
  finally { busy.value = false }
}
async function loadRecords() { await run(async () => { const data = await checkinAPI.records(recordPage.value); records.value = data.items; recordTotal.value = data.total }) }
async function loadRisks() { await run(async () => { const data = await checkinAPI.risks(riskPage.value); risks.value = data.items; riskTotal.value = data.total }) }
async function load() {
  await run(async () => { settings.value = await checkinAPI.settings(); distribution.value = settings.value.distribution?.length ? JSON.stringify(settings.value.distribution, null, 2) : '' })
  await loadRecords(); await loadRisks()
}
async function save() {
  await run(async () => {
    if (!settings.value) return
    const buckets: unknown = distribution.value.trim() ? JSON.parse(distribution.value) : null
    if (buckets !== null && !Array.isArray(buckets)) throw new Error(t('wallet.distribution'))
    settings.value = await checkinAPI.saveSettings({ ...settings.value, distribution: buckets as CheckinSettings['distribution'] })
    app.showSuccess(t('common.success'))
  })
}
async function confirmRisk() {
  await run(async () => {
    if (!riskUser.value) return
    await checkinAPI.confirmRisk(riskUser.value, reason.value, evidence.value)
    riskUser.value = undefined; reason.value = ''; evidence.value = ''
    app.showSuccess(t('common.success'))
  })
  await loadRisks()
}
async function resolve(id: number, action: 'revoke' | 'restore') {
  if (action === 'restore' && !window.confirm(t('wallet.restoreConfirm'))) return
  await run(async () => { await checkinAPI.resolveRisk(id, action); app.showSuccess(t('common.success')) })
  await loadRisks()
}
onMounted(load)
</script>
