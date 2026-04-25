<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="rounded-3xl border border-sky-200 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.14),_transparent_55%),linear-gradient(135deg,_rgba(240,249,255,0.96),_rgba(248,250,252,0.92))] p-6 shadow-sm dark:border-sky-900/40 dark:bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.12),_transparent_50%),linear-gradient(135deg,_rgba(15,23,42,0.92),_rgba(15,23,42,0.82))]">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="max-w-3xl space-y-2">
            <p class="text-xs font-semibold uppercase tracking-[0.3em] text-sky-600 dark:text-sky-300">Windsurf</p>
            <h1 class="text-2xl font-semibold text-slate-900 dark:text-white">{{ t('windsurfAccounts.title') }}</h1>
            <p class="text-sm leading-6 text-slate-600 dark:text-slate-300">{{ t('windsurfAccounts.description') }}</p>
          </div>
          <button class="btn btn-primary shrink-0" @click="openCreateDialog">
            <Icon name="plus" size="md" class="mr-1" />
            {{ t('windsurfAccounts.create') }}
          </button>
        </div>
        <div class="mt-4 rounded-2xl border border-white/70 bg-white/80 px-4 py-3 text-sm text-slate-600 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/5 dark:text-slate-300">
          {{ t('windsurfAccounts.autoDisableHint') }}
        </div>
        <div class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/90 px-4 py-4 text-sm text-amber-950 shadow-sm dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-100">
          <div class="flex items-start gap-3">
            <Icon name="link" size="md" class="mt-0.5 shrink-0 text-amber-600 dark:text-amber-300" />
            <div class="space-y-2">
              <p class="font-semibold">{{ t('windsurfAccounts.noticeTitle') }}</p>
              <p class="leading-6">
                {{ t('windsurfAccounts.noticeIntro') }}
                <a
                  href="https://windsurf.com/"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="font-medium underline decoration-amber-400 underline-offset-2 hover:text-amber-700 dark:hover:text-amber-200"
                >
                  https://windsurf.com/
                </a>
              </p>
              <ul class="space-y-1.5 pl-5 text-sm leading-6 text-amber-900 marker:text-amber-500 dark:text-amber-100">
                <li>{{ t('windsurfAccounts.noticeItems.register') }}</li>
                <li>{{ t('windsurfAccounts.noticeItems.subscription') }}</li>
                <li>{{ t('windsurfAccounts.noticeItems.payment') }}</li>
                <li>{{ t('windsurfAccounts.noticeItems.multiplier') }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-wrap items-center gap-3">
            <div class="flex-1 sm:max-w-72">
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('windsurfAccounts.searchPlaceholder')"
                class="input"
                @input="handleSearch"
              />
            </div>
            <div class="flex flex-1 justify-end">
              <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadAccounts">
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              </button>
            </div>
          </div>
        </template>

        <template #table>
          <DataTable
            :columns="columns"
            :data="accounts"
            :loading="loading"
            :server-side-sort="true"
            default-sort-key="maintained_at"
            default-sort-order="desc"
            @sort="handleSort"
          >
            <template #cell-account="{ row }">
              <div class="flex items-center gap-2">
                <div class="flex flex-col">
                  <span class="font-medium text-slate-900 dark:text-white">{{ row.account }}</span>
                  <span class="text-xs text-slate-500 dark:text-slate-400">#{{ row.id }}</span>
                </div>
                <button
                  class="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-dark-700 dark:hover:text-slate-200"
                  :title="t('windsurfAccounts.accountCopied')"
                  @click="copyAccount(row)"
                >
                  <Icon name="copy" size="sm" />
                </button>
              </div>
            </template>

            <template #cell-password="{ row }">
              <div class="flex items-center gap-2">
                <code class="rounded-lg bg-slate-100 px-2.5 py-1 text-xs text-slate-700 dark:bg-dark-700 dark:text-slate-200">
                  {{ revealedPasswords[row.id] ?? row.password_masked }}
                </code>
                <button
                  v-if="canRevealPassword(row)"
                  class="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-dark-700 dark:hover:text-slate-200"
                  :title="revealedPasswords[row.id] ? t('windsurfAccounts.hide') : t('windsurfAccounts.reveal')"
                  @click="togglePassword(row)"
                >
                  <Icon :name="revealedPasswords[row.id] ? 'eyeOff' : 'eye'" size="sm" />
                </button>
                <button
                  v-if="canRevealPassword(row)"
                  class="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-dark-700 dark:hover:text-slate-200"
                  :title="t('windsurfAccounts.passwordCopied')"
                  @click="copyPassword(row)"
                >
                  <Icon name="clipboard" size="sm" />
                </button>
              </div>
            </template>

            <template #cell-status="{ row }">
              <span
                :class="[
                  'inline-flex items-center rounded-full px-3 py-1 text-xs font-medium',
                  row.enabled
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
                ]"
              >
                {{ row.enabled ? t('windsurfAccounts.enabled') : t('windsurfAccounts.disabled') }}
              </span>
            </template>

            <template #cell-maintained_by="{ row }">
              <div class="flex flex-col">
                <span class="font-medium text-slate-900 dark:text-white">{{ row.maintained_by_name || '-' }}</span>
                <span class="text-xs text-slate-500 dark:text-slate-400">{{ row.maintained_by_email || '-' }}</span>
              </div>
            </template>

            <template #cell-maintained_at="{ value }">
              <span class="text-sm text-slate-500 dark:text-slate-400">{{ formatDateTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <div class="flex items-center gap-1">
                <button
                  v-if="canEditAccount(row)"
                  class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-700 dark:hover:text-slate-200"
                  :title="t('common.edit')"
                  @click="openEditDialog(row)"
                >
                  <Icon name="edit" size="sm" />
                </button>
                <button
                  v-if="authStore.isAdmin"
                  class="rounded-lg p-1.5 text-rose-500 transition-colors hover:bg-rose-50 hover:text-rose-700 dark:hover:bg-rose-950/40 dark:hover:text-rose-300"
                  :title="t('common.delete')"
                  :disabled="deletingId === row.id"
                  @click="openDeleteDialog(row)"
                >
                  <Icon name="trash" size="sm" />
                </button>
                <button
                  v-if="authStore.isAdmin"
                  class="rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
                  :class="row.enabled
                    ? 'bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/30 dark:text-amber-300'
                    : 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200 dark:bg-emerald-900/30 dark:text-emerald-300'"
                  :disabled="statusUpdatingId === row.id"
                  @click="toggleStatus(row)"
                >
                  {{ row.enabled ? t('windsurfAccounts.disable') : t('windsurfAccounts.enable') }}
                </button>
              </div>
            </template>

            <template #empty>
              <div class="flex flex-col items-center py-10 text-center">
                <Icon name="inbox" size="xl" class="mb-4 text-slate-400 dark:text-slate-500" />
                <p class="text-lg font-medium text-slate-900 dark:text-white">{{ t('windsurfAccounts.noData') }}</p>
              </div>
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </div>

    <BaseDialog :show="showCreateDialog" :title="t('windsurfAccounts.create')" width="normal" @close="closeCreateDialog">
      <form id="windsurf-create-form" class="space-y-4" @submit.prevent="submitCreate">
        <div>
          <label class="input-label">{{ t('windsurfAccounts.account') }}</label>
          <input v-model.trim="createForm.account" type="text" class="input" autocomplete="username" />
        </div>
        <div>
          <label class="input-label">{{ t('windsurfAccounts.password') }}</label>
          <input v-model="createForm.password" type="password" class="input" autocomplete="new-password" />
          <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ t('windsurfAccounts.passwordHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeCreateDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="windsurf-create-form" class="btn btn-primary" :disabled="saving">
            {{ t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showEditDialog" :title="editDialogTitle" width="normal" @close="closeEditDialog">
      <form id="windsurf-edit-form" class="space-y-4" @submit.prevent="submitEdit">
        <div>
          <label class="input-label">{{ t('windsurfAccounts.account') }}</label>
          <input
            v-model.trim="editForm.account"
            type="text"
            class="input"
            autocomplete="username"
            :readonly="!authStore.isAdmin"
            :disabled="!authStore.isAdmin"
          />
          <p v-if="!authStore.isAdmin" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
            {{ t('windsurfAccounts.accountReadonlyHint') }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('windsurfAccounts.password') }}</label>
          <input v-model="editForm.password" type="password" class="input" autocomplete="new-password" />
          <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
            {{ authStore.isAdmin ? t('windsurfAccounts.passwordOptionalHint') : t('windsurfAccounts.passwordRequiredHint') }}
          </p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="windsurf-edit-form" class="btn btn-primary" :disabled="saving">
            {{ t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="Boolean(deletingAccount)"
      :title="t('windsurfAccounts.deleteTitle')"
      :message="t('windsurfAccounts.deleteConfirm', { account: deletingAccount?.account ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="closeDeleteDialog"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { windsurfAccountsAPI, type WindsurfAccountItem } from '@/api/windsurfAccounts'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const saving = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const statusUpdatingId = ref<number | null>(null)
const deletingId = ref<number | null>(null)
const searchQuery = ref('')
const searchTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const editingId = ref<number | null>(null)
const deletingAccount = ref<WindsurfAccountItem | null>(null)

const accounts = ref<WindsurfAccountItem[]>([])
const revealedPasswords = reactive<Record<number, string>>({})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortState = reactive({
  sort_by: 'maintained_at',
  sort_order: 'desc' as 'asc' | 'desc',
})

const createForm = reactive({
  account: '',
  password: '',
})

const editForm = reactive({
  account: '',
  password: '',
})

const columns = computed<Column[]>(() => [
  { key: 'account', label: t('windsurfAccounts.columns.account'), sortable: true },
  { key: 'password', label: t('windsurfAccounts.columns.password') },
  { key: 'status', label: t('windsurfAccounts.columns.status'), sortable: true },
  { key: 'maintained_by', label: t('windsurfAccounts.columns.maintainedBy') },
  { key: 'maintained_at', label: t('windsurfAccounts.columns.maintainedAt'), sortable: true },
  { key: 'actions', label: t('windsurfAccounts.columns.actions') },
])
const editDialogTitle = computed(() => (authStore.isAdmin ? t('windsurfAccounts.edit') : t('windsurfAccounts.editPassword')))

function canRevealPassword(row: WindsurfAccountItem) {
  return authStore.isAdmin || row.maintained_by_id === authStore.user?.id
}

function canEditAccount(row: WindsurfAccountItem) {
  return authStore.isAdmin || row.maintained_by_id === authStore.user?.id
}

async function loadAccounts() {
  loading.value = true
  try {
    const response = await windsurfAccountsAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      search: searchQuery.value || undefined,
      sort_by: sortState.sort_by,
      sort_order: sortState.sort_order,
    })
    accounts.value = response.items
    pagination.total = response.total
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.loadFailed')))
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  if (searchTimer.value) clearTimeout(searchTimer.value)
  searchTimer.value = setTimeout(() => {
    pagination.page = 1
    void loadAccounts()
  }, 250)
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadAccounts()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadAccounts()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadAccounts()
}

function openCreateDialog() {
  createForm.account = ''
  createForm.password = ''
  showCreateDialog.value = true
}

function closeCreateDialog() {
  showCreateDialog.value = false
}

function openEditDialog(row: WindsurfAccountItem) {
  if (!canEditAccount(row)) return
  editingId.value = row.id
  editForm.account = row.account
  editForm.password = ''
  showEditDialog.value = true
}

function closeEditDialog() {
  editingId.value = null
  editForm.account = ''
  editForm.password = ''
  showEditDialog.value = false
}

function openDeleteDialog(row: WindsurfAccountItem) {
  deletingAccount.value = row
}

function closeDeleteDialog() {
  deletingAccount.value = null
}

async function submitCreate() {
  saving.value = true
  try {
    await windsurfAccountsAPI.create({
      account: createForm.account,
      password: createForm.password,
    })
    appStore.showSuccess(t('windsurfAccounts.createSuccess'))
    closeCreateDialog()
    await loadAccounts()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function submitEdit() {
  if (!editingId.value) return
  saving.value = true
  try {
    await windsurfAccountsAPI.update(editingId.value, {
      account: editForm.account,
      password: editForm.password || undefined,
    })
    appStore.showSuccess(t('windsurfAccounts.updateSuccess'))
    closeEditDialog()
    await loadAccounts()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row: WindsurfAccountItem) {
  statusUpdatingId.value = row.id
  try {
    await windsurfAccountsAPI.updateStatus(row.id, !row.enabled)
    appStore.showSuccess(t('windsurfAccounts.statusUpdateSuccess'))
    await loadAccounts()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.statusUpdateFailed')))
  } finally {
    statusUpdatingId.value = null
  }
}

async function confirmDelete() {
  if (!deletingAccount.value) return

  deletingId.value = deletingAccount.value.id
  try {
    await windsurfAccountsAPI.delete(deletingAccount.value.id)
    delete revealedPasswords[deletingAccount.value.id]
    appStore.showSuccess(t('windsurfAccounts.deleteSuccess'))
    closeDeleteDialog()
    await loadAccounts()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.deleteFailed')))
  } finally {
    deletingId.value = null
  }
}

async function ensurePassword(row: WindsurfAccountItem) {
  if (revealedPasswords[row.id]) return revealedPasswords[row.id]
  const password = await windsurfAccountsAPI.revealPassword(row.id)
  revealedPasswords[row.id] = password
  return password
}

async function togglePassword(row: WindsurfAccountItem) {
  if (!canRevealPassword(row)) return
  if (revealedPasswords[row.id]) {
    delete revealedPasswords[row.id]
    return
  }
  try {
    await ensurePassword(row)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.passwordLoadFailed')))
  }
}

async function copyPassword(row: WindsurfAccountItem) {
  if (!canRevealPassword(row)) return
  try {
    const password = await ensurePassword(row)
    await copyToClipboard(password, t('windsurfAccounts.passwordCopied'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('windsurfAccounts.passwordLoadFailed')))
  }
}

async function copyAccount(row: WindsurfAccountItem) {
  await copyToClipboard(row.account, t('windsurfAccounts.accountCopied'))
}

onMounted(() => {
  void loadAccounts()
})
</script>
