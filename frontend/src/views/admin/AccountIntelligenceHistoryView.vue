<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-8">
      <router-link to="/admin/accounts" class="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-primary-600"><Icon name="chevronLeft" size="sm" />{{ t('pelican.back') }}</router-link>
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><p class="mb-2 text-sm text-primary-600">{{ accountName || `#${accountId}` }}</p><h1 class="text-2xl font-semibold">{{ t('pelican.intelligence.historyTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('pelican.intelligence.historyDescription') }}</p></div>
        <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />{{ t('common.refresh') }}</button>
      </header>
      <div class="flex flex-wrap justify-between gap-3 text-sm"><span>{{ t('pelican.total', { count: total.toLocaleString() }) }}</span><span class="text-gray-500">{{ t('pelican.intelligence.expected') }}</span></div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-5 text-sm text-red-600">{{ error }} <button class="underline" @click="load">{{ t('common.retry') }}</button></p>
      <div v-else-if="loading && !items.length" class="rounded-xl bg-gray-50 p-12 text-center text-gray-500" aria-busy="true">{{ t('common.loading') }}</div>
      <div v-else-if="!items.length" class="rounded-xl border border-dashed p-12 text-center text-gray-500">{{ t('pelican.intelligence.empty') }}</div>
      <section v-else :aria-label="t('pelican.intelligence.history')" class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700" :aria-busy="loading">
        <table class="w-full border-collapse text-center text-sm">
          <thead class="bg-gray-50 text-gray-500 dark:bg-dark-800"><tr><th class="min-w-36 p-4 text-left">{{ t('pelican.account') }}</th><th v-for="test in items" :key="test.id" class="min-w-28 px-3 py-4 font-medium"><time :datetime="test.created_at" class="whitespace-pre-line">{{ timelineTime(test.created_at) }}</time></th></tr></thead>
          <tbody><tr class="bg-sky-50/60 dark:bg-dark-900/30"><th class="max-w-48 break-all px-4 py-8 text-left font-medium">{{ accountName || `#${accountId}` }}</th><td v-for="test in items" :key="test.id" class="px-3 py-8"><button :data-test="`intelligence-result-${test.id}`" class="min-w-14 rounded-lg border px-3 py-2 font-mono text-lg font-semibold" :class="resultClass(test)" :title="`${test.model_id} · ${t(`pelican.intelligence.status.${test.status}`)}`" @click="selectedId = test.id">{{ resultLabel(test) }}</button></td></tr></tbody>
        </table>
      </section>
      <Pagination v-if="total > 0 && !error" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false" @update:page="changePage" />
      <PelicanResultDialog :intelligence="true" :show="selectedId !== null" :test-id="selectedId" :account-name="accountName" @close="selectedId = null" />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useIntervalFn } from '@vueuse/core'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import PelicanResultDialog from '@/components/admin/account/PelicanResultDialog.vue'
import { adminAPI } from '@/api/admin'
import type { PelicanTest } from '@/api/admin/accounts'
import { extractApiErrorMessage } from '@/utils/apiError'

const route = useRoute(), { t } = useI18n()
const accountId = computed(() => Number(route.params.id))
const accountName = ref(''), items = ref<PelicanTest[]>([]), total = ref(0), page = ref(1), pageSize = 20
const selectedId = ref<number | null>(null), loading = ref(false), error = ref('')
let version = 0, lastLoadAt = 0
const active = (test: PelicanTest) => ['queued', 'running'].includes(test.status)
const resultLabel = (test: PelicanTest) => test.answer || (active(test) ? '…' : '×')
const resultClass = (test: PelicanTest) => test.status === 'correct'
  ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
  : active(test) ? 'border-amber-200 bg-amber-50 text-amber-600' : 'border-red-300 bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
const timelineTime = (value: string) => new Intl.DateTimeFormat('en-GB', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(value)).replace(', ', '\n')
async function load() {
  const current = ++version
  if (!Number.isSafeInteger(accountId.value) || accountId.value <= 0) { error.value = t('pelican.historyFailed'); return }
  loading.value = true; error.value = ''
  try {
    const result = await adminAPI.accounts.listPelicanHistory(accountId.value, page.value, pageSize, 'intelligence')
    if (current !== version) return
    items.value = result.items; total.value = result.total
  } catch (cause) { if (current === version) error.value = extractApiErrorMessage(cause, t('pelican.historyFailed')) }
  finally { if (current === version) { loading.value = false; lastLoadAt = Date.now() } }
}
function changePage(value: number) { page.value = value; items.value = []; void load() }
watch(accountId, async id => {
  selectedId.value = null; page.value = 1; items.value = []; accountName.value = ''; void load()
  try { const account = await adminAPI.accounts.getById(id); if (accountId.value === id) accountName.value = account.name }
  catch { if (accountId.value === id) accountName.value = t('pelican.deletedAccount') }
}, { immediate: true })
useIntervalFn(() => { if (!loading.value && !error.value && selectedId.value === null && (items.value.some(active) || Date.now() - lastLoadAt >= 30000)) void load() }, 5000)
onUnmounted(() => { version++ })
</script>
