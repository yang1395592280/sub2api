import { computed, onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GenerationTestKind, PelicanTest } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'

type AccountLabel = { id: number; name: string }
type Batch = { batchId: string; ids: number[] }

export function useAccountGenerationTests(kind: GenerationTestKind, accounts: Ref<AccountLabel[]>, selectedIds: Ref<number[]>) {
  const { t } = useI18n(), app = useAppStore()
  const showBatch = ref(false), targetIds = ref<number[]>([]), names = ref<Record<number, string>>({})
  const tests = ref<Record<number, PelicanTest>>({}), resultId = ref<number | null>(null), resultAccountName = ref('')
  const batch = ref<Batch | null>(null), storageKey = `account-${kind}-active-batch`
  const active = (test?: PelicanTest) => !!test && ['queued', 'running'].includes(test.status)
  const success = (test: PelicanTest) => test.status === (kind === 'intelligence' ? 'correct' : 'previewable')
  const progress = computed(() => {
    const rows = batch.value?.ids.map(id => tests.value[id]).filter(test => test?.batch_id === batch.value?.batchId) ?? []
    return { done: rows.filter(test => !active(test)).length, success: rows.filter(success).length, failed: rows.filter(test => !active(test) && !success(test)).length }
  })
  const openBatch = () => {
    targetIds.value = [...selectedIds.value]
    names.value = Object.fromEntries(accounts.value.map(account => [account.id, account.name]))
    showBatch.value = targetIds.value.length > 0
  }
  const retryAccount = (account: AccountLabel) => {
    targetIds.value = [account.id]; names.value = { [account.id]: account.name }; showBatch.value = true
  }
  const openResult = (account: AccountLabel) => {
    resultId.value = tests.value[account.id]?.id ?? null; resultAccountName.value = account.name
  }
  let lastRefresh = 0, refreshing = false, disposed = false
  async function refresh() {
    if (refreshing) return
    const ids = [...new Set([...accounts.value.map(account => account.id), ...(batch.value?.ids ?? [])])]
    if (!ids.length) return
    refreshing = true
    try {
      for (let offset = 0; offset < ids.length; offset += 100) {
        const latest = await adminAPI.accounts.listLatestPelicanTests(ids.slice(offset, offset + 100), kind)
        if (disposed) return
        for (const test of latest) {
          if (!tests.value[test.account_id] || tests.value[test.account_id].id <= test.id) tests.value[test.account_id] = test
        }
      }
      if (batch.value && progress.value.done === batch.value.ids.length) localStorage.removeItem(storageKey)
    } catch (error) { console.error(`Failed to load ${kind} test results:`, error) }
    finally { lastRefresh = Date.now(); refreshing = false }
  }
  function submitted(rows: PelicanTest[]) {
    showBatch.value = false
    rows.forEach(test => { tests.value[test.account_id] = test })
    if (rows.length) {
      batch.value = { batchId: rows[0].batch_id, ids: rows.map(test => test.account_id) }
      localStorage.setItem(storageKey, JSON.stringify(batch.value))
    }
    if (rows.length < targetIds.value.length) app.showWarning(t('admin.accounts.pelican.skippedActive', { count: targetIds.value.length - rows.length }))
    void refresh()
  }
  watch(() => accounts.value.map(account => account.id).join(','), () => { void refresh() })
  useIntervalFn(() => {
    if (Date.now() - lastRefresh >= 30000 || accounts.value.some(account => active(tests.value[account.id])) || (batch.value && progress.value.done < batch.value.ids.length)) void refresh()
  }, 3000)
  onMounted(() => {
    try {
      const saved: unknown = JSON.parse(localStorage.getItem(storageKey) || 'null')
      if (saved && typeof saved === 'object' && 'batchId' in saved && typeof saved.batchId === 'string' && 'ids' in saved && Array.isArray(saved.ids) && saved.ids.length <= 100 && saved.ids.every(id => Number.isSafeInteger(id) && id > 0)) batch.value = saved as Batch
    } catch { localStorage.removeItem(storageKey) }
    void refresh()
  })
  onUnmounted(() => { disposed = true })
  return { showBatch, targetIds, names, tests, resultId, resultAccountName, batch, progress, openBatch, retryAccount, openResult, submitted }
}
