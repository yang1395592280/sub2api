<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
        <div v-for="item in statCards" :key="item.key" class="card p-4">
          <p class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">{{ item.label }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ item.value }}</p>
        </div>
      </div>

      <div class="card p-4">
        <div class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(180px,1fr)_140px_140px_140px_auto_auto]">
          <input
            v-model.trim="filters.search"
            class="input"
            type="search"
            :placeholder="t('admin.workbench.searchPlaceholder')"
            @keyup.enter="reload"
          />
          <select v-model="filters.mode" class="input">
            <option value="">{{ t('admin.workbench.allModes') }}</option>
            <option value="chat">chat</option>
            <option value="image">image</option>
          </select>
          <select v-model="filters.status" class="input">
            <option value="">{{ t('admin.workbench.allStatuses') }}</option>
            <option value="pending">pending</option>
            <option value="success">success</option>
            <option value="error">error</option>
          </select>
          <label class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm text-gray-700 dark:border-dark-600 dark:text-gray-300">
            <input v-model="filters.hasImages" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            {{ t('admin.workbench.hasImages') }}
          </label>
          <button class="btn btn-secondary" type="button" @click="reload">
            {{ t('common.search') }}
          </button>
          <button class="btn btn-secondary" type="button" @click="resetFilters">
            {{ t('common.reset') }}
          </button>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 p-4 dark:border-dark-700">
          <div class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.workbench.selectedCount', { count: selectedIds.length }) }}
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-danger"
              :disabled="selectedIds.length === 0 || loading"
              data-testid="admin-workbench-delete-selected"
              @click="deleteSelected"
            >
              {{ t('admin.workbench.deleteSelected') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              data-testid="admin-workbench-cleanup-expired"
              @click="cleanupExpired"
            >
              {{ t('admin.workbench.cleanupExpired') }}
            </button>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="w-10 px-4 py-3 text-left">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :checked="allCurrentSelected"
                    @change="toggleCurrentPage"
                  />
                </th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-gray-500">{{ t('admin.workbench.user') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-gray-500">{{ t('admin.workbench.conversation') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-gray-500">{{ t('admin.workbench.mode') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-gray-500">{{ t('admin.workbench.images') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-gray-500">{{ t('admin.workbench.updatedAt') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase text-gray-500">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="loading">
                <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</td>
              </tr>
              <tr v-for="row in conversations" :key="row.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
                <td class="px-4 py-3">
                  <input
                    v-model="selectedIds"
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600"
                    :value="row.id"
                    :data-testid="`admin-workbench-select-${row.id}`"
                  />
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                  <div class="font-medium text-gray-900 dark:text-white">{{ row.user_email || `#${row.user_id}` }}</div>
                  <div class="text-xs text-gray-500">{{ row.username || `ID ${row.user_id}` }}</div>
                </td>
                <td class="max-w-xs px-4 py-3 text-sm">
                  <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.title || t('workbench.untitled') }}</div>
                  <div class="truncate text-xs text-gray-500">{{ row.last_message_preview || '-' }}</div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ row.mode }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">
                  {{ row.image_count }} / {{ formatBytes(row.image_bytes) }}
                </td>
                <td class="px-4 py-3 text-sm text-gray-500">{{ formatDateTime(row.updated_at) }}</td>
                <td class="px-4 py-3 text-right">
                  <button
                    type="button"
                    class="btn btn-secondary px-3 py-1.5 text-xs"
                    :data-testid="`admin-workbench-open-${row.id}`"
                    @click="openDetail(row.id)"
                  >
                    {{ t('common.view') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!loading && conversations.length === 0">
                <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500">{{ t('common.noData') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="detail" class="card p-4">
        <div class="mb-4 flex items-start justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ detail.conversation.title || t('workbench.untitled') }}</h2>
            <p class="mt-1 text-sm text-gray-500">{{ detail.conversation.user_email || `#${detail.conversation.user_id}` }}</p>
          </div>
          <button type="button" class="btn btn-secondary px-3 py-1.5 text-xs" @click="detail = null">{{ t('common.close') }}</button>
        </div>
        <div class="space-y-3">
          <div
            v-for="message in detail.messages"
            :key="message.id"
            class="rounded-lg border border-gray-200 p-3 dark:border-dark-700"
          >
            <div class="mb-2 flex items-center gap-2 text-xs text-gray-500">
              <span class="font-semibold uppercase">{{ message.role }}</span>
              <span>{{ message.status }}</span>
              <span>{{ message.model }}</span>
            </div>
            <p class="whitespace-pre-wrap text-sm text-gray-800 dark:text-gray-200">{{ message.content || message.error_message || '-' }}</p>
            <div v-if="message.image_outputs?.length" class="mt-3 grid grid-cols-2 gap-3 md:grid-cols-4">
              <img
                v-for="(image, index) in message.image_outputs"
                :key="index"
                class="aspect-square rounded-lg border border-gray-200 object-cover dark:border-dark-700"
                :src="image.url || `data:${image.mime_type || 'image/png'};base64,${image.b64_json}`"
                :alt="`image-${index + 1}`"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminAPI } from '@/api/admin'
import type { AdminWorkbenchConversation, AdminWorkbenchConversationDetail } from '@/api/admin/workbench'
import type { WorkbenchMode } from '@/api/workbench'
import { useAppStore } from '@/stores/app'

type StatusFilter = '' | 'pending' | 'success' | 'error'

const { t } = useI18n()
const appStore = useAppStore()
const retentionDays = 7

const loading = ref(false)
const stats = ref({
  total_conversations: 0,
  total_messages: 0,
  image_messages: 0,
  expired_conversations: 0,
  image_bytes: 0,
  retention_days: retentionDays,
})
const conversations = ref<AdminWorkbenchConversation[]>([])
const detail = ref<AdminWorkbenchConversationDetail | null>(null)
const selectedIds = ref<number[]>([])

const filters = reactive<{
  search: string
  mode: WorkbenchMode | ''
  status: StatusFilter
  hasImages: boolean
}>({
  search: '',
  mode: '',
  status: '',
  hasImages: false,
})

const statCards = computed(() => [
  { key: 'conversations', label: t('admin.workbench.totalConversations'), value: stats.value.total_conversations },
  { key: 'messages', label: t('admin.workbench.totalMessages'), value: stats.value.total_messages },
  { key: 'images', label: t('admin.workbench.imageMessages'), value: stats.value.image_messages },
  { key: 'expired', label: t('admin.workbench.expiredConversations'), value: stats.value.expired_conversations },
  { key: 'bytes', label: t('admin.workbench.imageBytes'), value: formatBytes(stats.value.image_bytes) },
])

const allCurrentSelected = computed(() =>
  conversations.value.length > 0 && conversations.value.every((row) => selectedIds.value.includes(row.id))
)

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatDateTime(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function listParams() {
  return {
    page: 1,
    page_size: 20,
    search: filters.search || undefined,
    mode: filters.mode || undefined,
    status: filters.status || undefined,
    has_images: filters.hasImages ? true : undefined,
  }
}

async function loadStats(): Promise<void> {
  stats.value = await adminAPI.workbench.getStats(retentionDays)
}

async function loadConversations(): Promise<void> {
  const result = await adminAPI.workbench.listConversations(listParams())
  conversations.value = result.items
  selectedIds.value = selectedIds.value.filter((id) => conversations.value.some((row) => row.id === id))
}

async function reload(): Promise<void> {
  loading.value = true
  try {
    await Promise.all([loadStats(), loadConversations()])
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.loadFailed'))
  } finally {
    loading.value = false
  }
}

function resetFilters(): void {
  filters.search = ''
  filters.mode = ''
  filters.status = ''
  filters.hasImages = false
  reload()
}

function toggleCurrentPage(event: Event): void {
  const checked = (event.target as HTMLInputElement).checked
  const ids = conversations.value.map((row) => row.id)
  selectedIds.value = checked
    ? Array.from(new Set([...selectedIds.value, ...ids]))
    : selectedIds.value.filter((id) => !ids.includes(id))
}

async function openDetail(conversationId: number): Promise<void> {
  try {
    detail.value = await adminAPI.workbench.getConversation(conversationId)
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.detailLoadFailed'))
  }
}

async function deleteSelected(): Promise<void> {
  if (selectedIds.value.length === 0) return
  try {
    const result = await adminAPI.workbench.batchDeleteConversations(selectedIds.value)
    selectedIds.value = []
    detail.value = null
    await reload()
    appStore.showSuccess(t('admin.workbench.deleteSelectedSuccess', { count: result.deleted }))
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.deleteSelectedFailed'))
  }
}

async function cleanupExpired(): Promise<void> {
  try {
    const result = await adminAPI.workbench.cleanupExpiredConversations(retentionDays)
    selectedIds.value = []
    detail.value = null
    await reload()
    appStore.showSuccess(t('admin.workbench.cleanupSuccess', { count: result.deleted }))
  } catch (error) {
    console.error(error)
    appStore.showError(t('admin.workbench.cleanupFailed'))
  }
}

onMounted(() => {
  reload()
})
</script>
