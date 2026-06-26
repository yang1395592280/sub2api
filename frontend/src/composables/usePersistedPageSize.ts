import { getConfiguredTableDefaultPageSize, normalizeTablePageSize } from '@/utils/tablePreferences'

const STORAGE_KEY = 'table-page-size'
const STORAGE_SOURCE_KEY = 'table-page-size-source'

export function getPersistedPageSize(fallback = getConfiguredTableDefaultPageSize()): number {
  return normalizeTablePageSize(getConfiguredTableDefaultPageSize() || fallback)
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(size))
    window.localStorage.setItem(STORAGE_SOURCE_KEY, 'user')
  } catch (error) {
    console.warn('Failed to persist page size:', error)
  }
}
