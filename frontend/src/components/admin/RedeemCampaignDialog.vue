<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[60] flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-black/50" @click="close" />
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="redeem-campaign-title"
        class="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800"
        data-test="campaign-dialog"
        @keydown.esc="close"
      >
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 id="redeem-campaign-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.redeem.campaigns.title') }}</h2>
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.close') }}</button>
        </div>
        <p class="mb-5 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.redeem.campaigns.hint') }}</p>
        <div class="grid gap-6 md:grid-cols-2">
          <div>
            <form class="mb-3 flex gap-2" @submit.prevent="load(1)">
              <input v-model="search" maxlength="100" class="input min-w-0" :placeholder="t('admin.redeem.campaigns.search')" :aria-label="t('admin.redeem.campaigns.search')" />
              <button class="btn btn-secondary shrink-0" :disabled="loading">{{ t('common.search') }}</button>
            </form>
            <p v-if="listError" role="alert" class="mb-3 text-sm text-red-500">{{ listError }}</p>
            <p v-if="loading" class="py-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
            <div v-else class="max-h-72 space-y-2 overflow-y-auto">
              <div v-for="campaign in campaigns" :key="campaign.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600" data-test="campaign-item">
                <div class="break-words font-medium text-gray-900 dark:text-white">{{ campaign.name }}</div>
                <div class="mt-1 text-sm text-gray-500 dark:text-dark-400">#{{ campaign.id }} · {{ t('admin.redeem.campaigns.limitSummary', { count: campaign.per_user_limit }) }}</div>
                <div class="mt-2 flex justify-end gap-2">
                  <button type="button" class="btn btn-secondary btn-sm" :disabled="saving" data-test="campaign-edit" @click="edit(campaign)">{{ t('common.edit') }}</button>
                  <button v-if="selectable" type="button" class="btn btn-primary btn-sm" :disabled="saving" data-test="campaign-select" @click="select(campaign)">{{ t('admin.redeem.campaigns.use') }}</button>
                </div>
              </div>
              <p v-if="campaigns.length === 0" class="py-6 text-center text-sm text-gray-500">{{ t('admin.redeem.campaigns.empty') }}</p>
            </div>
            <div class="mt-3 flex items-center justify-between gap-2 text-sm">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || page <= 1" @click="load(page - 1)">{{ t('pagination.previous') }}</button>
              <span class="text-gray-500">{{ page }} / {{ Math.max(1, pages) }}</span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || page >= pages" @click="load(page + 1)">{{ t('pagination.next') }}</button>
            </div>
          </div>
          <form class="space-y-4" data-test="campaign-form" @submit.prevent="save">
            <div class="flex items-center justify-between gap-2">
              <h3 class="font-medium text-gray-900 dark:text-white">{{ editingId ? t('admin.redeem.campaigns.edit') : t('admin.redeem.campaigns.create') }}</h3>
              <button v-if="editingId" type="button" class="text-sm text-primary-600" :disabled="saving" @click="resetForm">{{ t('admin.redeem.campaigns.create') }}</button>
            </div>
            <div>
              <label for="campaign-name" class="input-label">{{ t('admin.redeem.campaigns.name') }}</label>
              <input id="campaign-name" v-model="form.name" required maxlength="100" class="input" :disabled="saving" data-test="campaign-name" />
            </div>
            <div>
              <label for="campaign-limit" class="input-label">{{ t('admin.redeem.campaigns.perUserLimit') }}</label>
              <input id="campaign-limit" v-model.number="form.per_user_limit" type="number" required min="1" max="1000" step="1" class="input" :disabled="saving" data-test="campaign-limit" />
              <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ t('admin.redeem.campaigns.limitHint') }}</p>
            </div>
            <p v-if="formError" role="alert" class="text-sm text-red-500">{{ formError }}</p>
            <button type="submit" class="btn btn-primary w-full" :disabled="saving" data-test="campaign-save">{{ saving ? t('common.saving') : selectable ? t('admin.redeem.campaigns.saveAndUse') : t('common.save') }}</button>
          </form>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { RedeemCampaign } from '@/types'

const props = defineProps<{ show: boolean; selectable?: boolean; initialCampaignId?: number | null }>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'select', campaign: RedeemCampaign): void
  (event: 'saved', campaign: RedeemCampaign): void
}>()
const { t } = useI18n()
const appStore = useAppStore()
const campaigns = ref<RedeemCampaign[]>([])
const search = ref('')
const page = ref(1)
const pages = ref(1)
const loading = ref(false)
const saving = ref(false)
const listError = ref('')
const formError = ref('')
const editingId = ref<number | null>(null)
const form = reactive({ name: '', per_user_limit: 1 })
let requestSequence = 0
let dialogSequence = 0
let pendingWrite: { signature: string; key: string } | null = null

function close() {
  if (!saving.value) emit('close')
}

function resetForm() {
  editingId.value = null
  form.name = ''
  form.per_user_limit = 1
  formError.value = ''
}

function edit(campaign: RedeemCampaign) {
  editingId.value = campaign.id
  form.name = campaign.name
  form.per_user_limit = campaign.per_user_limit
  formError.value = ''
}

function select(campaign: RedeemCampaign) {
  emit('select', campaign)
  emit('close')
}

function errorMessage(error: unknown) {
  return extractI18nErrorMessage(error, t, 'admin.redeem.campaigns.errors', t('admin.redeem.campaigns.failed'))
}

async function load(targetPage = 1) {
  const sequence = ++requestSequence
  loading.value = true
  listError.value = ''
  try {
    const result = await adminAPI.redeem.listCampaigns(targetPage, search.value.trim())
    if (sequence !== requestSequence) return
    campaigns.value = result.items
    page.value = result.page
    pages.value = result.pages
  } catch (error) {
    if (sequence === requestSequence) listError.value = errorMessage(error)
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function save() {
  if (saving.value) return
  const input = { name: form.name.trim(), per_user_limit: form.per_user_limit }
  if (!input.name || [...input.name].length > 100 || !Number.isInteger(input.per_user_limit) || input.per_user_limit < 1 || input.per_user_limit > 1000) {
    formError.value = t('admin.redeem.campaigns.errors.REDEEM_CAMPAIGN_INVALID')
    return
  }
  saving.value = true
  formError.value = ''
  try {
    const signature = JSON.stringify({ id: editingId.value, ...input })
    if (pendingWrite?.signature !== signature) pendingWrite = { signature, key: crypto.randomUUID() }
    const campaign = editingId.value
      ? await adminAPI.redeem.updateCampaign(editingId.value, input, pendingWrite.key)
      : await adminAPI.redeem.createCampaign(input, pendingWrite.key)
    pendingWrite = null
    emit('saved', campaign)
    appStore.showSuccess(t('admin.redeem.campaigns.saved'))
    edit(campaign)
    if (props.selectable) {
      select(campaign)
    } else {
      await load(page.value)
    }
  } catch (error) {
    formError.value = errorMessage(error)
  } finally {
    saving.value = false
  }
}

watch(() => props.show, async (show) => {
  const opening = ++dialogSequence
  if (!show) {
    requestSequence++
    return
  }
  search.value = ''
  resetForm()
  void load(1)
  if (props.initialCampaignId) {
    try {
      const campaign = await adminAPI.redeem.getCampaign(props.initialCampaignId)
      if (props.show && opening === dialogSequence) edit(campaign)
    } catch (error) {
      if (props.show && opening === dialogSequence) formError.value = errorMessage(error)
    }
  }
}, { immediate: true })
</script>
