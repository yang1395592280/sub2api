<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-8">
      <router-link to="/admin/accounts" class="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-primary-600"><Icon name="chevronLeft" size="sm" />{{ t('pelican.back') }}</router-link>
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><p class="mb-2 text-sm text-primary-600">{{ accountName || `#${accountId}` }}</p><h1 class="text-2xl font-semibold">{{ t('pelican.intelligence.historyTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('pelican.intelligence.historyDescription') }}</p></div>
        <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />{{ t('common.refresh') }}</button>
      </header>
      <div class="flex flex-wrap justify-between gap-3 text-sm"><span>{{ t('pelican.library.batches', { batches: total.toLocaleString(), records: totalRecords.toLocaleString() }) }}</span></div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-5 text-sm text-red-600">{{ error }} <button class="underline" @click="load">{{ t('common.retry') }}</button></p>
      <div v-else-if="loading && !items.length" class="rounded-xl bg-gray-50 p-12 text-center text-gray-500" aria-busy="true">{{ t('common.loading') }}</div>
      <div v-else-if="!items.length" class="rounded-xl border border-dashed p-12 text-center text-gray-500">{{ t('pelican.intelligence.empty') }}</div>
      <section v-else :aria-label="t('pelican.intelligence.history')" class="space-y-5" :aria-busy="loading">
        <article v-for="batch in batches" :key="batch.id" class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700" data-test="history-batch">
          <header class="flex flex-wrap items-center justify-between gap-3 bg-gray-50 px-5 py-4 text-sm dark:bg-dark-800">
            <div><strong>{{ t('pelican.library.batch') }} · {{ batch.id.slice(0, 8) }}</strong><time :datetime="batch.items[0].created_at" class="ml-4 text-gray-500">{{ formatDateTime(batch.items[0].created_at) }}</time></div>
            <span class="font-medium text-emerald-600">{{ t('pelican.library.batchSummary', summarizeGenerationTests(batch.items, 'intelligence')) }}</span>
          </header>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="test in batch.items" :key="test.id" class="flex flex-wrap items-center justify-between gap-4 p-5">
              <div class="min-w-0 flex-1"><h2 class="break-words font-medium">{{ test.question_name || `#${test.id}` }}</h2><p class="mt-1 text-xs text-gray-500">{{ test.model_id }} · {{ test.source_group_id ? t('pelican.scheduled') : t('pelican.manual') }} {{ test.source_group_name }}</p><p class="mt-2 text-xs text-gray-500">{{ t('pelican.library.expected', { answer: test.expected_answer || '20' }) }} · {{ test.total_tokens?.toLocaleString() ?? '—' }} Tokens · {{ test.latency_ms }} ms</p><div class="mt-1"><TestEstimatedCost :test="test" /></div></div>
              <div class="text-center"><button :data-test="`intelligence-result-${test.id}`" class="min-w-16 rounded-lg border px-4 py-2 font-mono text-lg font-semibold" :class="resultClass(test)" @click="selectedId = test.id">{{ resultLabel(test) }}</button><p class="mt-1 text-xs text-gray-500">{{ t(`pelican.intelligence.status.${test.status}`) }}</p></div>
            </div>
          </div>
        </article>
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
import TestEstimatedCost from '@/components/admin/account/TestEstimatedCost.vue'
import { formatDateTime } from '@/utils/format'
import { groupGenerationBatches, summarizeGenerationTests } from '@/utils/generationTests'
import { extractApiErrorMessage } from '@/utils/apiError'

const route = useRoute(), { t } = useI18n()
const accountId = computed(() => Number(route.params.id))
const accountName = ref(''), items = ref<PelicanTest[]>([]), total = ref(0), page = ref(1), pageSize = 20
const totalRecords = ref(0)
const batches = computed(() => groupGenerationBatches(items.value))
const selectedId = ref<number | null>(null), loading = ref(false), error = ref('')
let version = 0, lastLoadAt = 0
const active = (test: PelicanTest) => ['queued', 'running'].includes(test.status)
const resultLabel = (test: PelicanTest) => test.answer || (active(test) ? '…' : '×')
const resultClass = (test: PelicanTest) => test.status === 'correct'
  ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
  : active(test) ? 'border-amber-200 bg-amber-50 text-amber-600' : 'border-red-300 bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
async function load() {
  const current = ++version
  if (!Number.isSafeInteger(accountId.value) || accountId.value <= 0) { error.value = t('pelican.historyFailed'); return }
  loading.value = true; error.value = ''
  try {
    const result = await adminAPI.accounts.listPelicanHistory(accountId.value, page.value, pageSize, 'intelligence')
    if (current !== version) return
    items.value = result.items; total.value = result.total; totalRecords.value = result.total_records ?? result.total
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
