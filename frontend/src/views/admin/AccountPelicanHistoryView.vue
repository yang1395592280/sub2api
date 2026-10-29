<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-8">
      <header class="space-y-5">
        <router-link to="/admin/accounts" class="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-primary-600"><Icon name="chevronLeft" size="sm" />{{ t('pelican.back') }}</router-link>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p class="mb-2 text-xs font-medium uppercase tracking-widest text-primary-600 dark:text-primary-400">{{ accountName || t('pelican.account') }}</p>
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">{{ t('pelican.historyTitle') }}</h1>
            <p class="mt-2 text-sm text-gray-500">{{ t('pelican.historyDescription') }}</p>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />{{ t('common.refresh') }}</button>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 pb-4 text-sm dark:border-dark-700">
          <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('pelican.total', { count: total.toLocaleString() }) }}</span>
          <span class="text-xs text-gray-400">{{ t('pelican.generationHint') }}</span>
        </div>
      </header>
      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 p-5 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/20" role="alert">
        {{ error }}<button class="ml-3 underline" @click="load">{{ t('common.retry') }}</button>
      </div>
      <div v-else-if="loading && !items.length" class="grid gap-6 lg:grid-cols-2" aria-busy="true">
        <div v-for="i in 4" :key="i" class="aspect-video animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800" />
      </div>
      <section v-else-if="!items.length" class="rounded-2xl border border-dashed border-gray-300 px-6 py-20 text-center dark:border-dark-600">
        <Icon name="play" class="mx-auto mb-4 h-8 w-8 text-gray-300" />
        <h2 class="font-medium">{{ t('pelican.empty') }}</h2>
        <p class="mt-2 text-sm text-gray-500">{{ t('pelican.emptyHint') }}</p>
      </section>
      <section v-else class="grid gap-6 lg:grid-cols-2" :aria-label="t('pelican.history')" :aria-busy="loading">
        <article v-for="test in items" :key="test.id" class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-3.5 dark:border-dark-700">
            <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(test.status)">
              <span class="h-1.5 w-1.5 rounded-full bg-current" />{{ t(`pelican.cardStatus.${test.status}`) }}
            </span>
            <time :datetime="test.created_at" class="text-xs text-gray-400">{{ formatDateTime(test.created_at) }}</time>
          </div>
          <button type="button" class="group block w-full text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500" :aria-label="t('pelican.details')" @click="selectedId = test.id">
            <PelicanInlinePreview v-if="test.status === 'previewable'" :test-id="test.id" responsive class="!rounded-none !border-0" />
            <div v-else class="flex aspect-video flex-col items-center justify-center gap-3 bg-gray-50 px-8 text-center dark:bg-dark-900/50">
              <Icon :name="isActive(test) ? 'clock' : 'eyeOff'" class="h-10 w-10 text-gray-300 dark:text-dark-500" />
              <p class="text-sm text-gray-500">{{ isActive(test) ? t('pelican.inProgress') : t('pelican.noPreview') }}</p>
              <p v-if="test.error_message" class="line-clamp-2 max-w-md text-xs text-red-500">{{ test.error_message }}</p>
            </div>
            <div class="space-y-4 p-5">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0"><h2 class="truncate font-semibold text-gray-900 dark:text-white" :title="test.model_id">{{ test.model_id }}</h2><p class="mt-1 truncate text-xs text-gray-500">{{ accountName || `#${test.account_id}` }}</p></div>
                <Icon name="eye" class="text-gray-400 transition-colors group-hover:text-primary-500" size="sm" />
              </div>
              <p class="flex items-center gap-2 text-xs text-gray-500"><span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-700">{{ test.source_group_id ? t('pelican.scheduled') : t('pelican.manual') }}</span><span class="truncate">{{ test.source_group_name }}</span></p>
              <dl class="grid grid-cols-3 gap-4 border-t border-gray-100 pt-4 dark:border-dark-700">
                <div><dt class="text-xs text-gray-400">{{ t('pelican.latency') }}</dt><dd class="mt-1 font-mono text-sm text-gray-700 dark:text-gray-200">{{ isActive(test) ? '—' : `${(test.latency_ms / 1000).toFixed(1)} s` }}</dd></div>
                <div><dt class="text-xs text-gray-400">{{ t('pelican.inputTokens') }}</dt><dd class="mt-1 font-mono text-sm text-gray-700 dark:text-gray-200">{{ test.input_tokens?.toLocaleString() ?? '—' }}</dd></div>
                <div><dt class="text-xs text-gray-400">{{ t('pelican.outputTokens') }}</dt><dd class="mt-1 font-mono text-sm text-gray-700 dark:text-gray-200">{{ test.output_tokens?.toLocaleString() ?? '—' }}</dd></div>
              </dl>
            </div>
          </button>
        </article>
      </section>
      <Pagination v-if="total > 0 && !error" :total="total" :page="page" :page-size="pageSize" :show-page-size-selector="false" @update:page="changePage" />
      <PelicanResultDialog :show="selectedId !== null" :test-id="selectedId" :account-name="accountName" @close="selectedId = null" />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useIntervalFn } from '@vueuse/core'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import PelicanInlinePreview from '@/components/admin/account/PelicanInlinePreview.vue'
import PelicanResultDialog from '@/components/admin/account/PelicanResultDialog.vue'
import { adminAPI } from '@/api/admin'
import type { PelicanTest } from '@/api/admin/accounts'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const route = useRoute(), { t } = useI18n()
const accountName = ref(''), items = ref<PelicanTest[]>([])
const page = ref(1), pageSize = 12, total = ref(0)
const loading = ref(false), error = ref(''), selectedId = ref<number | null>(null)
let version = 0, lastLoadAt = 0
const isActive = (test: PelicanTest) => test.status === 'queued' || test.status === 'running'
function statusClass(status: string) {
  if (status === 'previewable') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  if (status === 'failed' || status === 'unpreviewable') return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
}
async function load() {
  const current = ++version
  const id = Number(route.params.id)
  if (!Number.isSafeInteger(id) || id <= 0) { error.value = t('pelican.historyFailed'); return }
  loading.value = true; error.value = ''
  try {
    const result = await adminAPI.accounts.listPelicanHistory(id, page.value, pageSize)
    if (current !== version) return
    items.value = result.items; total.value = result.total
  } catch (cause) {
    if (current === version) error.value = extractApiErrorMessage(cause, t('pelican.historyFailed'))
  } finally { if (current === version) { loading.value = false; lastLoadAt = Date.now() } }
}
function changePage(value: number) { page.value = value; items.value = []; void load() }
watch(() => route.params.id, async id => {
  selectedId.value = null; page.value = 1; items.value = []; accountName.value = ''
  void load()
  try {
    const account = await adminAPI.accounts.getById(Number(id))
    if (String(route.params.id) === String(id)) accountName.value = account.name
  } catch { if (String(route.params.id) === String(id)) accountName.value = t('pelican.deletedAccount') }
}, { immediate: true })
useIntervalFn(() => {
  if (!loading.value && !error.value && !selectedId.value && (items.value.some(isActive) || Date.now() - lastLoadAt >= 30000)) void load()
}, 5000)
onUnmounted(() => { version++ })
</script>
