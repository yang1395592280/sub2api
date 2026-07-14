import { computed, onScopeDispose, reactive, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type {
  OpenAIAutoSchedulerEvent,
  OpenAIAutoSchedulerGroup,
  OpenAIAutoSchedulerListResponse,
  OpenAIAutoSchedulerSettings,
  OpenAISchedulerHealthParams,
  OpenAISchedulerHealthRow,
  OpenAISchedulerOverview,
  OpenAISchedulerWindow,
} from '@/api/admin/openaiAutoScheduler'

export type OpenAISchedulerDashboardTab = 'overview' | 'health' | 'events' | 'settings'

const emptyPage = <T>(): OpenAIAutoSchedulerListResponse<T> => ({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
})

function isCanceled(error: unknown): boolean {
  const candidate = error as { name?: string; code?: string }
  return candidate?.name === 'AbortError' || candidate?.code === 'ERR_CANCELED'
}

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  const candidate = error as { message?: string }
  return candidate?.message || 'Request failed'
}

export function useOpenAISchedulerDashboard(pollIntervalMs = 15_000) {
  const activeTab = ref<OpenAISchedulerDashboardTab>('overview')
  const selectedGroupId = ref<number | null>(null)
  const window = ref<OpenAISchedulerWindow>('6h')
  const settings = ref<OpenAIAutoSchedulerSettings | null>(null)
  const groups = ref<OpenAIAutoSchedulerGroup[]>([])
  const overview = ref<OpenAISchedulerOverview | null>(null)
  const healthPage = ref<OpenAIAutoSchedulerListResponse<OpenAISchedulerHealthRow>>(emptyPage())
  const eventsPage = ref<OpenAIAutoSchedulerListResponse<OpenAIAutoSchedulerEvent>>(emptyPage())
  const drawerEvents = ref<OpenAIAutoSchedulerEvent[]>([])
  const initialized = ref(false)

  const loading = reactive({
    initialize: false,
    overview: false,
    health: false,
    events: false,
    settings: false,
    action: false,
  })
  const errors = reactive<Partial<Record<OpenAISchedulerDashboardTab | 'initialize', string>>>({})
  const healthFilters = reactive<OpenAISchedulerHealthParams>({
    page: 1,
    page_size: 20,
    sort: 'predicted_ttft_ms',
    order: 'desc',
  })
  const eventsPagination = reactive({ page: 1, page_size: 20 })

  const selectedGroup = computed(
    () => groups.value.find((group) => group.id === selectedGroupId.value) || null
  )

  let overviewController: AbortController | null = null
  let healthController: AbortController | null = null
  let eventsController: AbortController | null = null
  let drawerEventsController: AbortController | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let disposed = false

  function groupIDParam(): number | undefined {
    return selectedGroupId.value || undefined
  }

  async function loadOverview(): Promise<void> {
    overviewController?.abort()
    const controller = new AbortController()
    overviewController = controller
    loading.overview = true
    delete errors.overview
    try {
      const result = await adminAPI.openaiAutoScheduler.getOverview(
        { group_id: groupIDParam(), window: window.value },
        { signal: controller.signal }
      )
      if (!controller.signal.aborted && overviewController === controller) overview.value = result
    } catch (error: unknown) {
      if (!isCanceled(error) && overviewController === controller) errors.overview = errorMessage(error)
    } finally {
      if (overviewController === controller) {
        overviewController = null
        loading.overview = false
      }
    }
  }

  async function loadHealth(): Promise<void> {
    healthController?.abort()
    const controller = new AbortController()
    healthController = controller
    loading.health = true
    delete errors.health
    try {
      const result = await adminAPI.openaiAutoScheduler.listHealth(
        { ...healthFilters, group_id: groupIDParam() },
        { signal: controller.signal }
      )
      if (!controller.signal.aborted && healthController === controller) healthPage.value = result
    } catch (error: unknown) {
      if (!isCanceled(error) && healthController === controller) errors.health = errorMessage(error)
    } finally {
      if (healthController === controller) {
        healthController = null
        loading.health = false
      }
    }
  }

  async function loadEvents(): Promise<void> {
    eventsController?.abort()
    const controller = new AbortController()
    eventsController = controller
    loading.events = true
    delete errors.events
    try {
      const result = await adminAPI.openaiAutoScheduler.listEvents(
        {
          group_id: groupIDParam(),
          page: eventsPagination.page,
          page_size: eventsPagination.page_size,
        },
        { signal: controller.signal }
      )
      if (!controller.signal.aborted && eventsController === controller) eventsPage.value = result
    } catch (error: unknown) {
      if (!isCanceled(error) && eventsController === controller) errors.events = errorMessage(error)
    } finally {
      if (eventsController === controller) {
        eventsController = null
        loading.events = false
      }
    }
  }

  async function loadAccountEvents(row: OpenAISchedulerHealthRow): Promise<void> {
    drawerEventsController?.abort()
    const controller = new AbortController()
    drawerEventsController = controller
    drawerEvents.value = []
    try {
      const result = await adminAPI.openaiAutoScheduler.listEvents(
        {
          account_id: row.account_id,
          group_id: row.group_id,
          model: row.model_family,
          page: 1,
          page_size: 20,
        },
        { signal: controller.signal }
      )
      if (!controller.signal.aborted && drawerEventsController === controller) {
        drawerEvents.value = result.items || []
      }
    } catch (error: unknown) {
      if (!isCanceled(error) && drawerEventsController === controller) errors.events = errorMessage(error)
    } finally {
      if (drawerEventsController === controller) drawerEventsController = null
    }
  }

  async function refreshActiveTab(): Promise<void> {
    if (activeTab.value === 'overview') return loadOverview()
    if (activeTab.value === 'health') return loadHealth()
    if (activeTab.value === 'events') return loadEvents()
  }

  function poll(): void {
    if (disposed || (typeof document !== 'undefined' && document.hidden)) return
    void refreshActiveTab()
  }

  function startPolling(): void {
    if (pollTimer || pollIntervalMs <= 0) return
    pollTimer = setInterval(poll, pollIntervalMs)
  }

  async function initialize(): Promise<void> {
    loading.initialize = true
    delete errors.initialize
    try {
      const [nextSettings, nextGroups] = await Promise.all([
        adminAPI.openaiAutoScheduler.getSettings(),
        adminAPI.openaiAutoScheduler.listGroups(),
      ])
      if (disposed) return
      settings.value = nextSettings
      groups.value = nextGroups
      const currentStillExists = nextGroups.some((group) => group.id === selectedGroupId.value)
      if (!currentStillExists) {
        selectedGroupId.value =
          nextGroups.find((group) => group.enabled)?.id ?? nextGroups[0]?.id ?? null
      }
      initialized.value = true
      await refreshActiveTab()
      startPolling()
    } catch (error: unknown) {
      errors.initialize = errorMessage(error)
    } finally {
      loading.initialize = false
    }
  }

  async function selectGroup(groupId: number | null): Promise<void> {
    if (selectedGroupId.value === groupId) return
    selectedGroupId.value = groupId
    healthFilters.page = 1
    eventsPagination.page = 1
    await refreshActiveTab()
  }

  async function selectTab(tab: OpenAISchedulerDashboardTab): Promise<void> {
    if (activeTab.value === tab) return
    activeTab.value = tab
    await refreshActiveTab()
  }

  async function selectWindow(nextWindow: OpenAISchedulerWindow): Promise<void> {
    if (window.value === nextWindow) return
    window.value = nextWindow
    if (activeTab.value === 'overview') await loadOverview()
  }

  async function showGroupHealth(groupId: number): Promise<void> {
    selectedGroupId.value = groupId
    healthFilters.page = 1
    activeTab.value = 'health'
    await loadHealth()
  }

  async function applyHealthFilters(next: Partial<OpenAISchedulerHealthParams>): Promise<void> {
    Object.assign(healthFilters, next, { page: next.page ?? 1 })
    await loadHealth()
  }

  async function setHealthPage(page: number, pageSize = healthFilters.page_size || 20): Promise<void> {
    healthFilters.page = page
    healthFilters.page_size = pageSize
    await loadHealth()
  }

  async function setEventsPage(page: number, pageSize = eventsPagination.page_size): Promise<void> {
    eventsPagination.page = page
    eventsPagination.page_size = pageSize
    await loadEvents()
  }

  async function saveSettings(payload: OpenAIAutoSchedulerSettings): Promise<OpenAIAutoSchedulerSettings> {
    loading.settings = true
    delete errors.settings
    try {
      const updated = await adminAPI.openaiAutoScheduler.updateSettings(payload)
      settings.value = updated
      return updated
    } catch (error: unknown) {
      errors.settings = errorMessage(error)
      throw error
    } finally {
      loading.settings = false
    }
  }

  async function setGlobalEnabled(enabled: boolean): Promise<void> {
    if (!settings.value) return
    await saveSettings({ ...settings.value, enabled })
  }

  async function setGroupEnabled(groupId: number, enabled: boolean): Promise<void> {
    loading.action = true
    try {
      const updated = await adminAPI.openaiAutoScheduler.updateGroup(groupId, { enabled })
      const index = groups.value.findIndex((group) => group.id === groupId)
      if (index >= 0) groups.value[index] = updated
    } finally {
      loading.action = false
    }
  }

  async function probeAccount(row: OpenAISchedulerHealthRow): Promise<void> {
    loading.action = true
    try {
      await adminAPI.openaiAutoScheduler.probeScore(row.account_id, {
        group_id: row.group_id,
        model: row.model_family,
      })
      await loadHealth()
    } finally {
      loading.action = false
    }
  }

  async function resetAccount(row: OpenAISchedulerHealthRow): Promise<void> {
    loading.action = true
    try {
      await adminAPI.openaiAutoScheduler.resetScore(row.account_id, {
        group_id: row.group_id,
        model: row.model_family,
      })
      await loadHealth()
    } finally {
      loading.action = false
    }
  }

  function dispose(): void {
    disposed = true
    overviewController?.abort()
    healthController?.abort()
    eventsController?.abort()
    drawerEventsController?.abort()
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = null
  }

  onScopeDispose(dispose)

  return {
    activeTab,
    selectedGroupId,
    selectedGroup,
    window,
    settings,
    groups,
    overview,
    healthPage,
    eventsPage,
    drawerEvents,
    healthFilters,
    eventsPagination,
    initialized,
    loading,
    errors,
    initialize,
    selectGroup,
    selectTab,
    selectWindow,
    showGroupHealth,
    loadOverview,
    loadHealth,
    loadEvents,
    loadAccountEvents,
    refreshActiveTab,
    applyHealthFilters,
    setHealthPage,
    setEventsPage,
    saveSettings,
    setGlobalEnabled,
    setGroupEnabled,
    probeAccount,
    resetAccount,
    dispose,
  }
}
