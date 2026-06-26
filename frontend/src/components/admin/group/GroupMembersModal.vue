<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.membersTitle', { name: group?.name || '-' })"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="group" class="space-y-4">
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.membersCount', { count: members.total }) }}
        </span>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div
        v-else-if="!members.has_fixed_members"
        data-testid="group-members-empty-state"
        class="rounded-lg border border-dashed border-gray-300 bg-white px-6 py-10 text-center dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400">
          <span class="text-lg">—</span>
        </div>
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.groups.publicGroupNoFixedMembers') }}
        </div>
      </div>

      <div
        v-else-if="members.items.length === 0"
        class="py-8 text-center text-sm text-gray-400 dark:text-gray-500"
      >
        {{ t('admin.groups.noMembers') }}
      </div>

      <div v-else class="max-h-[520px] space-y-3 overflow-y-auto pr-1">
        <div
          v-for="member in members.items"
          :key="member.id"
          data-testid="group-member-panel"
          class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-mono text-xs text-gray-400 dark:text-gray-500">#{{ member.id }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ member.username || '-' }}</span>
                <span
                  :class="[
                    'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                    member.status === 'active'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                  ]"
                >
                  {{ member.status }}
                </span>
              </div>
              <div class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                <span class="break-all">{{ member.email }}</span>
                <span class="break-all">{{ member.notes || '-' }}</span>
              </div>
            </div>

            <button
              v-if="canRemoveMembers"
              type="button"
              :data-testid="`remove-member-${member.id}`"
              class="self-start rounded px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
              :disabled="removingUserId === member.id"
              @click="handleRemove(member.id)"
            >
              {{ removingUserId === member.id ? t('common.loading') : t('admin.groups.removeMember') }}
            </button>
          </div>

          <div
            v-if="showUsageComparison"
            :data-testid="`member-usage-comparison-${member.id}`"
            class="mt-3 rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-700/50"
          >
            <div v-if="usageLoading" class="space-y-2">
              <div class="h-8 animate-pulse rounded bg-gray-100 dark:bg-dark-600"></div>
              <div class="h-8 animate-pulse rounded bg-gray-100 dark:bg-dark-600"></div>
            </div>
            <div v-else-if="usageError" class="text-sm text-amber-600 dark:text-amber-400">
              {{ usageError }}
            </div>
            <div v-else class="space-y-2">
              <div
                :data-testid="`member-usage-today-${member.id}`"
                class="grid gap-2 rounded bg-white p-2 text-xs dark:bg-dark-800 md:grid-cols-[88px_repeat(4,minmax(0,1fr))]"
              >
                <div class="flex flex-col">
                  <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.groups.memberUsageToday') }}</span>
                  <span class="text-gray-400">{{ usageDates.today }}</span>
                </div>
                <span class="rounded bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatCompactNumber(getUsageForUser(member.id).today.requests) }} req</span>
                <span class="rounded bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatTokens(getUsageForUser(member.id).today.tokens) }} token</span>
                <span class="rounded bg-emerald-50 px-2 py-1 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">A ${{ formatMoney(getUsageForUser(member.id).today.cost) }}</span>
                <span class="rounded bg-sky-50 px-2 py-1 font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">U ${{ formatMoney(getUsageForUser(member.id).today.user_cost) }}</span>
              </div>

              <div
                :data-testid="`member-usage-yesterday-${member.id}`"
                class="grid gap-2 rounded bg-white p-2 text-xs dark:bg-dark-800 md:grid-cols-[88px_repeat(4,minmax(0,1fr))]"
              >
                <div class="flex flex-col">
                  <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.groups.memberUsageYesterday') }}</span>
                  <span class="text-gray-400">{{ usageDates.yesterday }}</span>
                </div>
                <span class="rounded bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatCompactNumber(getUsageForUser(member.id).yesterday.requests) }} req</span>
                <span class="rounded bg-gray-100 px-2 py-1 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatTokens(getUsageForUser(member.id).yesterday.tokens) }} token</span>
                <span class="rounded bg-emerald-50 px-2 py-1 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">A ${{ formatMoney(getUsageForUser(member.id).yesterday.cost) }}</span>
                <span class="rounded bg-sky-50 px-2 py-1 font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">U ${{ formatMoney(getUsageForUser(member.id).yesterday.user_cost) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupMemberUsageComparison, GroupMembersResponse } from '@/api/admin/groups'
import type { AdminGroup, WindowStats } from '@/types'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'

interface Props {
  show: boolean
  group: AdminGroup | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const removingUserId = ref<number | null>(null)
const usageLoading = ref(false)
const usageError = ref<string | null>(null)
const usageDates = reactive({ today: '', yesterday: '' })
const usageByUserId = ref<Record<string, GroupMemberUsageComparison>>({})
const members = reactive<GroupMembersResponse>({
  group_id: 0,
  has_fixed_members: false,
  items: [],
  total: 0
})

const emptyWindowStats = (): WindowStats => ({
  requests: 0,
  tokens: 0,
  cost: 0,
  standard_cost: 0,
  user_cost: 0
})

const canRemoveMembers = computed(() => {
  return !!props.group && props.group.is_exclusive && props.group.subscription_type === 'standard'
})

const showUsageComparison = computed(() => {
  return !!props.group?.is_exclusive && members.has_fixed_members
})

function resetMembers() {
  members.group_id = 0
  members.has_fixed_members = false
  members.items = []
  members.total = 0
  resetUsage()
}

function resetUsage() {
  usageLoading.value = false
  usageError.value = null
  usageDates.today = ''
  usageDates.yesterday = ''
  usageByUserId.value = {}
}

async function loadUsageComparison(groupId: number) {
  if (!props.group?.is_exclusive || !members.has_fixed_members || members.items.length === 0) {
    resetUsage()
    return
  }

  usageLoading.value = true
  usageError.value = null
  try {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
    const userIds = members.items.map(member => member.id)
    const data = await adminAPI.groups.getGroupMemberUsageComparison(groupId, userIds, timezone)
    usageDates.today = data.today
    usageDates.yesterday = data.yesterday
    usageByUserId.value = data.stats || {}
  } catch (error) {
    usageByUserId.value = {}
    usageError.value = t('admin.groups.memberUsageLoadFailed')
    console.error('Error loading group member usage comparison:', error)
  } finally {
    usageLoading.value = false
  }
}

function getUsageForUser(userId: number): GroupMemberUsageComparison {
  return usageByUserId.value[String(userId)] || {
    today: emptyWindowStats(),
    yesterday: emptyWindowStats()
  }
}

function formatCompactNumber(value: number): string {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return String(value)
}

function formatTokens(tokens: number): string {
  if (tokens >= 1000000000) return `${(tokens / 1000000000).toFixed(2)}B`
  if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(2)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
  return String(tokens)
}

function formatMoney(value: number | undefined): string {
  const amount = Number(value || 0)
  return amount.toFixed(2)
}

async function loadMembers(groupId: number) {
  loading.value = true
  try {
    const data = await adminAPI.groups.getGroupMembers(groupId)
    members.group_id = data.group_id
    members.has_fixed_members = data.has_fixed_members
    members.items = data.items
    members.total = data.total
    await loadUsageComparison(groupId)
  } catch (error: any) {
    resetMembers()
    appStore.showError(error?.response?.data?.detail || t('admin.groups.failedToLoadMembers'))
  } finally {
    loading.value = false
  }
}

async function handleRemove(userId: number) {
  if (!props.group || !canRemoveMembers.value) return
  const confirmed = window.confirm(t('admin.groups.removeMemberConfirm'))
  if (!confirmed) return

  removingUserId.value = userId
  try {
    await adminAPI.groups.removeGroupMember(props.group.id, userId)
    appStore.showSuccess(t('admin.groups.memberRemoved'))
    await loadMembers(props.group.id)
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || t('admin.groups.failedToRemoveMember'))
  } finally {
    removingUserId.value = null
  }
}

watch(
  () => [props.show, props.group?.id] as const,
  ([show, groupId]) => {
    if (!show || !groupId) {
      if (!show) resetMembers()
      return
    }
    void loadMembers(groupId)
  },
  { immediate: true }
)
</script>
