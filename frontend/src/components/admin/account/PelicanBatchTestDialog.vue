<template>
  <BaseDialog :show="show" :title="t('admin.accounts.pelican.title')" width="wide" @close="emit('close')">
    <div class="space-y-4 text-sm">
      <p class="text-gray-600 dark:text-gray-300">
        {{ t('admin.accounts.pelican.selected', { count: accountIds.length }) }}
      </p>
      <details class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
        <summary class="cursor-pointer">{{ t('admin.accounts.pelican.targets') }}</summary>
        <div class="mt-2 max-h-28 overflow-auto break-all text-xs text-gray-500">
          {{ accountIds.map(id => accountNames[id] || `#${id}`).join('、') }}
        </div>
      </details>
      <label class="block font-medium text-gray-700 dark:text-gray-200" for="pelican-prompt">
        {{ t('admin.accounts.pelican.prompt') }}
      </label>
      <textarea
        id="pelican-prompt"
        v-model="prompt"
        maxlength="4000"
        rows="4"
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-gray-900 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
      />
      <label class="block font-medium text-gray-700 dark:text-gray-200">
        {{ t('admin.accounts.pelican.model') }}
      </label>
      <Select
        v-model="modelId"
        :options="models"
        :loading="loadingModels"
        :disabled="loadingModels || submitting"
        value-key="id"
        label-key="display_name"
        searchable
        :placeholder="t('admin.accounts.pelican.chooseModel')"
      />
      <p v-if="!loadingModels && !models.some(model => model.id === 'gpt-6-astra')" class="text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.pelican.defaultUnavailable') }}
      </p>
      <p class="text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.pelican.costWarning', { count: accountIds.length }) }}
      </p>
      <p class="text-gray-500 dark:text-gray-400">{{ t('admin.accounts.pelican.timeoutHint') }}</p>
      <p v-if="accountIds.length > 100" class="text-red-600">{{ t('admin.accounts.pelican.tooMany') }}</p>
      <p v-if="error" class="text-red-600" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <button class="btn btn-secondary" :disabled="submitting" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button data-test="start-pelican-test" class="btn btn-primary" :disabled="submitting || loadingModels || accountIds.length > 100 || !modelId || !prompt.trim()" @click="submit">
        {{ submitting ? t('common.loading') : t('admin.accounts.pelican.start') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type { ClaudeModel } from '@/types'
import type { PelicanTest } from '@/api/admin/accounts'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean; accountIds: number[]; accountNames: Record<number, string> }>()
const emit = defineEmits<{ close: []; submitted: [tests: PelicanTest[]] }>()
const { t } = useI18n()
const defaultPrompt = '创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试'
const prompt = ref(defaultPrompt)
const modelId = ref('')
const models = ref<ClaudeModel[]>([])
const loadingModels = ref(false)
const submitting = ref(false)
const error = ref('')
let modelLoadVersion = 0

watch(() => props.show, async show => {
  const version = ++modelLoadVersion
  if (!show || props.accountIds.length === 0) return
  prompt.value = defaultPrompt
  modelId.value = ''
  models.value = []
  error.value = ''
  loadingModels.value = true
  try {
    for (const accountId of props.accountIds) {
      try {
        const available = await adminAPI.accounts.getAvailableModels(accountId)
        if (version !== modelLoadVersion) return
        models.value = available.filter(model => !/^(gpt-image-|grok-imagine-(image|video)|grok-video|gemini-.*-image)/i.test(model.id))
        if (models.value.length > 0) break
      } catch {
        // A selected account may have been deleted; another target can still
        // provide the model picker for this batch.
      }
    }
    if (version !== modelLoadVersion) return
    if (models.value.length === 0) throw new Error(t('admin.accounts.pelican.loadModelsFailed'))
    if (models.value.some(model => model.id === 'gpt-6-astra')) modelId.value = 'gpt-6-astra'
  } catch (cause) {
    if (version !== modelLoadVersion) return
    models.value = []
    error.value = extractApiErrorMessage(cause, t('admin.accounts.pelican.loadModelsFailed'))
  } finally {
    if (version === modelLoadVersion) loadingModels.value = false
  }
}, { immediate: true })

async function submit() {
  if (submitting.value || !modelId.value || !prompt.value.trim()) return
  submitting.value = true
  error.value = ''
  try {
    const tests = await adminAPI.accounts.startPelicanTests([...props.accountIds], modelId.value, prompt.value.trim())
    emit('submitted', tests)
  } catch (cause) {
    error.value = extractApiErrorMessage(cause, t('admin.accounts.pelican.startFailed'))
  } finally {
    submitting.value = false
  }
}
</script>
