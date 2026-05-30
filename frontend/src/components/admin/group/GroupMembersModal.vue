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
        class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-4 text-sm text-blue-700 dark:border-blue-900/40 dark:bg-blue-900/10 dark:text-blue-300"
      >
        {{ t('admin.groups.publicGroupNoFixedMembers') }}
      </div>

      <div
        v-else-if="members.items.length === 0"
        class="py-8 text-center text-sm text-gray-400 dark:text-gray-500"
      >
        {{ t('admin.groups.noMembers') }}
      </div>

      <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
        <div class="max-h-[420px] overflow-y-auto">
          <table class="w-full text-sm">
            <thead class="sticky top-0 z-[1]">
              <tr class="border-b border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700">
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">ID</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.groups.columns.userName') }}
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.groups.columns.userEmail') }}
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.groups.columns.userNotes') }}
                </th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('admin.groups.columns.userStatus') }}
                </th>
                <th
                  v-if="canRemoveMembers"
                  class="w-16 px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400"
                >
                  {{ t('common.actions') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr
                v-for="member in members.items"
                :key="member.id"
                class="hover:bg-gray-50 dark:hover:bg-dark-700/50"
              >
                <td class="whitespace-nowrap px-3 py-2 text-gray-400 dark:text-gray-500">{{ member.id }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-900 dark:text-white">{{ member.username || '-' }}</td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{{ member.email }}</td>
                <td class="max-w-[220px] truncate px-3 py-2 text-gray-500 dark:text-gray-400" :title="member.notes">
                  {{ member.notes || '-' }}
                </td>
                <td class="whitespace-nowrap px-3 py-2">
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
                </td>
                <td v-if="canRemoveMembers" class="whitespace-nowrap px-3 py-2 text-right">
                  <button
                    type="button"
                    :data-testid="`remove-member-${member.id}`"
                    class="rounded px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                    :disabled="removingUserId === member.id"
                    @click="handleRemove(member.id)"
                  >
                    {{ removingUserId === member.id ? t('common.loading') : t('admin.groups.removeMember') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupMembersResponse } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
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
const members = reactive<GroupMembersResponse>({
  group_id: 0,
  has_fixed_members: false,
  items: [],
  total: 0
})

const canRemoveMembers = computed(() => {
  return !!props.group && props.group.is_exclusive && props.group.subscription_type === 'standard'
})

function resetMembers() {
  members.group_id = 0
  members.has_fixed_members = false
  members.items = []
  members.total = 0
}

async function loadMembers(groupId: number) {
  loading.value = true
  try {
    const data = await adminAPI.groups.getGroupMembers(groupId)
    members.group_id = data.group_id
    members.has_fixed_members = data.has_fixed_members
    members.items = data.items
    members.total = data.total
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
