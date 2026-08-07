<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="search" class="input w-full md:w-80" :placeholder="t('admin.affiliates.ticketCampaign.searchPlaceholder')" @keyup.enter="reload" />
          <select v-model="status" class="input w-full sm:w-40" @change="reload">
            <option value="">{{ t('admin.affiliates.ticketCampaign.allStatuses') }}</option>
            <option value="granted">{{ t('admin.affiliates.ticketCampaign.granted') }}</option>
            <option value="blocked">{{ t('admin.affiliates.ticketCampaign.blocked') }}</option>
            <option value="frozen">{{ t('admin.affiliates.ticketCampaign.frozen') }}</option>
            <option value="skipped">{{ t('admin.affiliates.ticketCampaign.skipped') }}</option>
          </select>
          <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.event') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.inviter') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.invitee') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.amount') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.tickets') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.status') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.risk') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.ticketCampaign.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="8" class="px-3 py-10 text-center text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="events.length === 0">
                <td colspan="8" class="px-3 py-10 text-center text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.ticketCampaign.empty') }}</td>
              </tr>
              <tr v-for="event in events" :key="event.id" class="border-b border-gray-100 dark:border-dark-800">
                <td class="px-3 py-3">{{ event.event_type === 'invite_register' ? t('admin.affiliates.ticketCampaign.register') : t('admin.affiliates.ticketCampaign.recharge') }}</td>
                <td class="px-3 py-3 text-gray-900 dark:text-white">{{ event.inviter_email || `#${event.inviter_id}` }}</td>
                <td class="px-3 py-3 text-gray-900 dark:text-white">{{ event.invitee_email || `#${event.invitee_id}` }}</td>
                <td class="px-3 py-3">{{ event.amount > 0 ? `¥${event.amount.toFixed(2)}` : '-' }}</td>
                <td class="px-3 py-3 font-semibold text-emerald-600 dark:text-emerald-400">{{ event.ticket_count }}</td>
                <td class="px-3 py-3"><span :class="statusClass(event.status)">{{ t(`admin.affiliates.ticketCampaign.${event.status}`) }}</span></td>
                <td class="max-w-md whitespace-normal break-words px-3 py-3 leading-5 text-amber-700 dark:text-amber-300" :title="formatRiskReason(event.risk_reason)">{{ formatRiskReason(event.risk_reason) }}</td>
                <td class="px-3 py-3 text-gray-600 dark:text-dark-300">{{ formatDateTime(event.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template #pagination>
        <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="changePage" @update:pageSize="changePageSize" />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { affiliatesAPI, type AffiliateTicketCampaignEvent } from '@/api/admin/affiliates'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const status = ref('')
const events = ref<AffiliateTicketCampaignEvent[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const riskReasonKeys: Record<string, string> = {
  'trusted registration IP unavailable': 'trustedRegistrationIpUnavailable',
  'inviter and invitee share the same network and device': 'sameNetworkAndDevice',
  'identical registration IP with different device session': 'sameNetworkDifferentDevice',
  'inviter does not meet campaign eligibility': 'ineligibleInviter',
  'inviter or invitee is not active': 'inactiveAccount',
  'invitee has already completed an earlier recharge': 'previousRecharge',
  'invite relationship is under risk control': 'relationshipUnderRiskControl',
  'frozen after same-network and same-device invitation risk': 'frozenAfterRisk',
  'repeated same-network and same-device invitation risk': 'repeatedRisk',
}

function formatRiskReason(reason?: string): string {
  if (!reason) return '-'
  const key = riskReasonKeys[reason]
  return key ? t(`admin.affiliates.ticketCampaign.riskReasons.${key}`) : reason
}

function statusClass(value: string): string {
  if (value === 'granted') return 'text-emerald-600 dark:text-emerald-400'
  if (value === 'blocked') return 'text-red-600 dark:text-red-400'
  return 'text-amber-600 dark:text-amber-400'
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const result = await affiliatesAPI.listTicketCampaignEvents({ page: pagination.page, page_size: pagination.page_size, search: search.value, status: status.value })
    events.value = result.items
    pagination.total = result.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.affiliates.errors.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reload(): void {
  pagination.page = 1
  void load()
}

function changePage(page: number): void {
  pagination.page = page
  void load()
}

function changePageSize(size: number): void {
  pagination.page_size = size
  pagination.page = 1
  void load()
}

onMounted(() => void load())
</script>
