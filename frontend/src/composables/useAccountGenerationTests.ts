import { computed, onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GenerationTestKind, PelicanTest } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { isGenerationActive, summarizeGenerationTests } from '@/utils/generationTests'

type AccountLabel = { id: number; name: string }
type Batch = { batchId: string; ids: number[]; recordIds: number[] }

export function useAccountGenerationTests(kind: GenerationTestKind, accounts: Ref<AccountLabel[]>, selectedIds: Ref<number[]>) {
  const { t } = useI18n(), app = useAppStore(), auth = useAuthStore()
  const showBatch = ref(false), targetIds = ref<number[]>([]), names = ref<Record<number, string>>({})
  const records = ref<Record<number, PelicanTest[]>>({}), batchRecords = ref<PelicanTest[]>([])
  const resultAccountId = ref<number | null>(null), resultAccountName = ref('')
  const resultRecords = computed(() => resultAccountId.value === null ? [] : records.value[resultAccountId.value] ?? [])
  const tests = computed(() => Object.fromEntries(Object.entries(records.value).filter(([, rows]) => rows.length).map(([id, rows]) => [id, rows[0]])))
  const summaries = computed(() => Object.fromEntries(Object.entries(records.value).map(([id, rows]) => [id, summarizeGenerationTests(rows, kind)])))
  const batch = ref<Batch | null>(null)
  const storageKey = `account-${kind}-active-batch-${auth.user?.id ?? 'unknown'}`
  const storeBatch = () => {
    try {
      if (batch.value && progress.value.done < progress.value.total) localStorage.setItem(storageKey, JSON.stringify(batch.value))
      else localStorage.removeItem(storageKey)
    } catch { /* Results remain usable when browser storage is unavailable. */ }
  }
  const progress = computed(() => ({ ...summarizeGenerationTests(batchRecords.value, kind), total: batch.value?.recordIds.length ?? 0 }))
  const openBatch = () => {
    targetIds.value = [...selectedIds.value]
    names.value = Object.fromEntries(accounts.value.map(account => [account.id, account.name]))
    showBatch.value = targetIds.value.length > 0
  }
  const retryAccount = (account: AccountLabel) => {
    targetIds.value = [account.id]; names.value = { [account.id]: account.name }; showBatch.value = true
  }
  const openResult = (account: AccountLabel) => { resultAccountId.value = account.id; resultAccountName.value = account.name }
  let lastRefresh = 0, refreshing = false, disposed = false
  async function refresh() {
    if (refreshing) return
    const ids = [...new Set([...accounts.value.map(account => account.id), ...(batch.value?.ids ?? [])])]
    if (!ids.length) return
    refreshing = true
    try {
      for (let offset = 0; offset < ids.length; offset += 100) {
        const chunk = ids.slice(offset, offset + 100)
        const latest = await adminAPI.accounts.listLatestPelicanTests(chunk, kind)
        if (disposed) return
        for (const id of chunk) {
          const rows = latest.filter(row => row.account_id === id).sort((a, b) => b.id - a.id)
          if (rows.length && (!records.value[id]?.length || rows[0].id >= records.value[id][0].id)) records.value[id] = rows
        }
      }
      // Fetch the submitted batch separately: newer manual/scheduled batches must
      // never replace records used to calculate this batch's progress.
      const current = batch.value
      if (current && progress.value.done < progress.value.total) {
        const rows = await adminAPI.accounts.listLatestPelicanTests(current.ids, kind, current.batchId)
        if (disposed || batch.value?.batchId !== current.batchId) return
        batchRecords.value = rows.filter(row => current.recordIds.includes(row.id))
      }
      storeBatch()
    } catch (error) { console.error(`Failed to load ${kind} test results:`, error) }
    finally { lastRefresh = Date.now(); refreshing = false }
  }
  function submitted(rows: PelicanTest[], plannedCount = targetIds.value.length) {
    showBatch.value = false
    for (const id of new Set(rows.map(row => row.account_id))) records.value[id] = rows.filter(row => row.account_id === id).sort((a, b) => b.id - a.id)
    if (rows.length) {
      batch.value = { batchId: rows[0].batch_id, ids: [...new Set(rows.map(row => row.account_id))], recordIds: rows.map(row => row.id) }
      batchRecords.value = rows
      storeBatch()
    }
    if (rows.length < plannedCount) app.showWarning(t('pelican.library.skipped', { created: rows.length, count: plannedCount - rows.length }))
    void refresh()
  }
  watch(() => accounts.value.map(account => account.id).join(','), () => { void refresh() })
  useIntervalFn(() => {
    if (Date.now() - lastRefresh >= 30000 || Object.values(records.value).some(rows => rows.some(isGenerationActive)) || (batch.value && progress.value.done < progress.value.total)) void refresh()
  }, 3000)
  onMounted(() => {
    try {
      const saved: unknown = JSON.parse(localStorage.getItem(storageKey) || 'null')
      const validIDs = (value: unknown, limit: number): value is number[] => Array.isArray(value) && value.length > 0 && value.length <= limit && new Set(value).size === value.length && value.every(id => Number.isSafeInteger(id) && id > 0)
      if (saved && typeof saved === 'object' && 'batchId' in saved && typeof saved.batchId === 'string' && saved.batchId.length <= 64 && 'ids' in saved && validIDs(saved.ids, 100) && 'recordIds' in saved && validIDs(saved.recordIds, 1000)) batch.value = saved as Batch
    } catch { try { localStorage.removeItem(storageKey) } catch { /* Storage can be disabled. */ } }
    void refresh()
  })
  onUnmounted(() => { disposed = true })
  return { showBatch, targetIds, names, tests, records, summaries, resultAccountId, resultRecords, resultAccountName, batch, progress, openBatch, retryAccount, openResult, submitted }
}
