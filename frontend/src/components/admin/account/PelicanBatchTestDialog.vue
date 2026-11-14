<template>
  <BaseDialog :show="show" :title="t(intelligence ? 'pelican.intelligence.title' : 'admin.accounts.pelican.title')" width="wide" :show-close-button="!submitting" :close-on-escape="!submitting" @close="!submitting && emit('close')">
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
      <p v-if="loadingQuestions" class="text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="selectionError" role="alert" class="text-red-600">{{ selectionError }} <button class="underline" @click="loadQuestions(modelLoadVersion)">{{ t('common.retry') }}</button></p>
      <template v-else>
        <p v-if="questionIds.length" class="rounded-lg bg-primary-50 px-3 py-2 text-xs text-primary-700 dark:bg-primary-950/30 dark:text-primary-300">{{ t(remembered ? 'pelican.library.remembered' : 'pelican.library.newSelection') }}</p>
        <GenerationQuestionPicker v-model="questionIds" :questions="questions" :kind="kind" :disabled="submitting" />
      </template>
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
      <div class="space-y-2">
        <label class="block font-medium text-gray-700 dark:text-gray-200" for="pelican-custom-model">
          {{ t('admin.accounts.pelican.customModel') }}
        </label>
        <input
          id="pelican-custom-model"
          v-model="customModelId"
          type="text"
          maxlength="255"
          :disabled="submitting"
          :placeholder="t('admin.accounts.pelican.customModelPlaceholder')"
          aria-describedby="pelican-custom-model-hint"
          class="input w-full"
          @keydown.enter.prevent="submit"
        />
        <p id="pelican-custom-model-hint" class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.pelican.customModelHint') }}
        </p>
      </div>
      <p v-if="modelListUnavailable" class="text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.pelican.modelListUnavailable') }}
      </p>
      <p v-else-if="!loadingModels && !customModelId.trim() && !models.some(model => model.id === 'gpt-6-astra')" class="text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.pelican.defaultUnavailable') }}
      </p>
      <div class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-900 dark:bg-primary-950/20"><p class="font-semibold text-primary-700 dark:text-primary-300" aria-live="polite">{{ t('pelican.library.plan', { accounts: accountIds.length, questions: questionIds.length, count: plannedCount }) }}</p><p class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('pelican.library.requestCost') }}</p></div>
      <p class="text-xs" :class="plannedCount > 1000 || questionIds.length > 20 ? 'text-red-600' : 'text-gray-500'">{{ t('pelican.library.limits') }}</p>
      <p class="text-gray-500 dark:text-gray-400">{{ t('pelican.library.timeoutHint') }}</p>
      <p v-if="accountIds.length > 100" class="text-red-600">{{ t('admin.accounts.pelican.tooMany') }}</p>
      <p v-if="error" class="text-red-600" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <button class="btn btn-secondary" :disabled="submitting" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button data-test="start-pelican-test" class="btn btn-primary" :disabled="!canSubmit" @click="submit">
        {{ submitting ? t('common.loading') : t('pelican.library.start', { count: plannedCount }) }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type { ClaudeModel } from '@/types'
import type { PelicanTest } from '@/api/admin/accounts'
import { getTestPreference, listTestQuestions, type GenerationTestQuestion } from '@/api/admin/pelican'
import GenerationQuestionPicker from './GenerationQuestionPicker.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean; accountIds: number[]; accountNames: Record<number, string>; intelligence?: boolean }>()
const emit = defineEmits<{ close: []; submitted: [tests: PelicanTest[], plannedCount: number] }>()
const { t } = useI18n()
const kind = computed(() => props.intelligence ? 'intelligence' as const : 'pelican' as const)
const questions = ref<GenerationTestQuestion[]>([]), questionIds = ref<number[]>([])
const loadingQuestions = ref(false), selectionError = ref(''), remembered = ref(false)
const plannedCount = computed(() => props.accountIds.length * questionIds.value.length)
const modelId = ref('')
const customModelId = ref('')
const effectiveModelId = computed(() => customModelId.value.trim() || modelId.value)
const models = ref<ClaudeModel[]>([])
const loadingModels = ref(false)
const modelListUnavailable = ref(false)
const submitting = ref(false)
const error = ref('')
const canSubmit = computed(() => !submitting.value &&
  (!loadingModels.value || !!customModelId.value.trim()) &&
  props.accountIds.length > 0 && props.accountIds.length <= 100 &&
  effectiveModelId.value.length > 0 && effectiveModelId.value.length <= 255 &&
  !loadingQuestions.value && !selectionError.value && questionIds.value.length > 0 && questionIds.value.length <= 20 && plannedCount.value <= 1000 &&
  questionIds.value.every(id => questions.value.some(question => question.id === id && question.enabled)))
let modelLoadVersion = 0

watch(() => [props.show, props.intelligence] as const, async ([show]) => {
  const version = ++modelLoadVersion
  if (!show || props.accountIds.length === 0) return
  void loadQuestions(version)
  modelId.value = ''
  customModelId.value = ''
  models.value = []
  modelListUnavailable.value = false
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
  } catch {
    if (version !== modelLoadVersion) return
    models.value = []
    modelListUnavailable.value = true
  } finally {
    if (version === modelLoadVersion) loadingModels.value = false
  }
}, { immediate: true })

async function loadQuestions(version: number) {
  loadingQuestions.value = true; selectionError.value = ''; questionIds.value = []; questions.value = []
  try {
    const [rows, preference] = await Promise.all([listTestQuestions(kind.value), getTestPreference(kind.value)])
    if (version !== modelLoadVersion) return
    questions.value = rows
    remembered.value = preference.question_ids.length > 0
    questionIds.value = remembered.value ? [...preference.question_ids] : rows.filter(question => question.enabled).slice(0, 1).map(question => question.id)
  } catch (cause) {
    if (version === modelLoadVersion) selectionError.value = extractApiErrorMessage(cause, t('pelican.library.loadFailed'))
  } finally { if (version === modelLoadVersion) loadingQuestions.value = false }
}

async function submit() {
  if (!canSubmit.value) return
  submitting.value = true
  error.value = ''
  const version = modelLoadVersion
  const count = plannedCount.value
  try {
    const tests = await adminAPI.accounts.startPelicanTests([...props.accountIds], effectiveModelId.value, [...questionIds.value], kind.value)
    if (version !== modelLoadVersion) return
    if (!tests.length) { error.value = t('pelican.library.emptyCreated'); return }
    emit('submitted', tests, count)
  } catch (cause) {
    if (version === modelLoadVersion) error.value = extractApiErrorMessage(cause, t('admin.accounts.pelican.startFailed'))
  } finally {
    submitting.value = false
  }
}
onUnmounted(() => { modelLoadVersion++ })
</script>
