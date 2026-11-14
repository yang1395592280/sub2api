<template>
  <BaseDialog :show="show" :title="t(intelligence ? 'pelican.intelligence.groupTitle' : 'pelican.groupTitle')" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="!saving && emit('close')">
    <div v-if="loading" class="py-12 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
    <div v-else-if="loadError" class="space-y-3 py-6 text-center">
      <p role="alert" class="text-sm text-red-600">{{ loadError }}</p>
      <button class="btn btn-secondary" @click="load">{{ t('common.retry') }}</button>
    </div>
    <form v-else-if="config" id="group-pelican-form" class="space-y-6" @submit.prevent="save">
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
        <p class="font-semibold text-gray-900 dark:text-white">{{ group?.name }}</p>
        <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t(intelligence ? 'pelican.intelligence.scheduleHint' : 'pelican.scheduleHint', { minutes: config.interval_minutes }) }}</p>
      </div>
      <label class="flex cursor-pointer items-center justify-between gap-4">
        <span><span class="block font-medium">{{ t(intelligence ? 'pelican.intelligence.enabled' : 'pelican.enabled') }}</span><span class="mt-1 block text-xs text-gray-500">{{ t('pelican.disabledHint') }}</span></span>
        <input v-model="config.enabled" type="checkbox" :disabled="saving" class="h-5 w-5 accent-primary-500" data-test="pelican-group-enabled" />
      </label>
      <div class="space-y-2">
        <label for="group-test-interval" class="input-label">{{ t('pelican.library.interval') }}</label>
        <input id="group-test-interval" v-model.number="config.interval_minutes" type="number" min="5" max="1440" step="1" required class="input w-full" :disabled="saving" />
        <div class="flex flex-wrap gap-2"><button v-for="minutes in [15, 30, 60, 120]" :key="minutes" type="button" class="btn btn-sm" :class="config.interval_minutes === minutes ? 'btn-primary' : 'btn-secondary'" :disabled="saving" @click="config.interval_minutes = minutes">{{ t('pelican.library.intervalPreset', { count: minutes }) }}</button></div>
        <p class="text-xs text-gray-500">{{ t('pelican.library.intervalHint') }}</p>
      </div>
      <div class="space-y-2">
        <label for="group-pelican-model" class="input-label">{{ t('pelican.model') }}</label>
        <input id="group-pelican-model" v-model.trim="config.model_id" list="group-pelican-models" class="input w-full" :required="config.enabled" maxlength="255" :disabled="saving" :placeholder="t('admin.accounts.pelican.customModelPlaceholder')" aria-describedby="group-pelican-model-hint" />
        <datalist id="group-pelican-models"><option v-for="model in models" :key="model" :value="model" /></datalist>
        <p id="group-pelican-model-hint" class="text-xs text-gray-500">{{ t('pelican.modelHint') }}</p>
        <p v-if="modelError" class="text-xs text-amber-600">{{ t('pelican.modelUnavailable') }}</p>
      </div>
      <p v-if="questionError" role="alert" class="text-sm text-red-600">{{ questionError }} <button type="button" class="underline" @click="load">{{ t('common.retry') }}</button></p>
      <GenerationQuestionPicker v-else v-model="config.question_ids" :questions="questions" :kind="intelligence ? 'intelligence' : 'pelican'" :disabled="saving" />
      <p class="rounded-lg bg-primary-50 p-3 text-xs text-primary-700 dark:bg-primary-950/30 dark:text-primary-300">{{ t('pelican.library.perRound') }}</p>
      <dl class="grid grid-cols-2 gap-4 border-t border-gray-100 pt-4 text-xs dark:border-dark-700">
        <div><dt class="text-gray-500">{{ t('pelican.lastRun') }}</dt><dd class="mt-1.5">{{ config.last_run_at ? formatDateTime(config.last_run_at) : t('pelican.notRun') }}</dd></div>
        <div><dt class="text-gray-500">{{ t('pelican.nextRun') }}</dt><dd class="mt-1.5">{{ !config.enabled ? t('pelican.off') : config.next_run_at ? formatDateTime(config.next_run_at) : t('pelican.pending') }}</dd></div>
      </dl>
      <p class="text-xs leading-5 text-amber-700 dark:text-amber-400">{{ t('pelican.costHint') }}</p>
      <p v-if="saveError" role="alert" class="text-sm text-red-600">{{ saveError }}</p>
    </form>
    <template #footer>
      <button class="btn btn-secondary" :disabled="saving" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" form="group-pelican-form" :disabled="!canSave" data-test="save-group-pelican">{{ saving ? t('common.loading') : t('common.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { getGroupPelicanConfig, saveGroupPelicanConfig, listTestQuestions, type GroupPelicanConfig, type GenerationTestQuestion } from '@/api/admin/pelican'
import GenerationQuestionPicker from '@/components/admin/account/GenerationQuestionPicker.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminGroup } from '@/types'

const props = defineProps<{ show: boolean; group: Pick<AdminGroup, 'id' | 'name'> | null; intelligence?: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const config = ref<GroupPelicanConfig | null>(null)
const models = ref<string[]>([])
const questions = ref<GenerationTestQuestion[]>([]), questionError = ref('')
const loading = ref(false), saving = ref(false), modelError = ref(false)
const loadError = ref(''), saveError = ref('')
let version = 0
const canSave = computed(() => !loading.value && !!config.value && !saving.value && !loadError.value &&
  Number.isInteger(config.value.interval_minutes) && config.value.interval_minutes >= 5 && config.value.interval_minutes <= 1440 &&
  (!config.value.enabled || (!!config.value.model_id.trim() && !questionError.value && config.value.question_ids.length > 0 && config.value.question_ids.length <= 20 &&
    config.value.question_ids.every(id => questions.value.some(question => question.id === id && question.enabled)))))
async function load() {
  const current = ++version
  if (!props.show || !props.group) return
  loading.value = true; config.value = null; loadError.value = ''; saveError.value = ''; models.value = []; modelError.value = false; questions.value = []; questionError.value = ''
  const groupId = props.group.id
  const results = await Promise.allSettled([getGroupPelicanConfig(groupId, props.intelligence ? 'intelligence' : 'pelican'), adminAPI.groups.getModelAllowlistCandidates(groupId, 'openai'), listTestQuestions(props.intelligence ? 'intelligence' : 'pelican')])
  if (current !== version) return
  if (results[0].status === 'fulfilled') config.value = { ...results[0].value, question_ids: results[0].value.question_ids ?? [] }
  else loadError.value = extractApiErrorMessage(results[0].reason, t('pelican.loadFailed'))
  if (results[1].status === 'fulfilled') models.value = results[1].value.filter(model => !/^(gpt-image-|grok-imagine-|grok-video|gemini-.*-image)/i.test(model))
  else modelError.value = true
  if (results[2].status === 'fulfilled') questions.value = results[2].value
  else questionError.value = extractApiErrorMessage(results[2].reason, t('pelican.library.loadFailed'))
  loading.value = false
}
async function save() {
  if (!canSave.value || !config.value || !props.group) return
  saving.value = true; saveError.value = ''
  const current = version
  try {
    await saveGroupPelicanConfig(props.group.id, { enabled: config.value.enabled, model_id: config.value.model_id, prompt: config.value.prompt, interval_minutes: config.value.interval_minutes, question_ids: [...config.value.question_ids] }, props.intelligence ? 'intelligence' : 'pelican')
    if (current !== version) return
    useAppStore().showSuccess(t(props.intelligence ? 'pelican.intelligence.saved' : 'pelican.saved'))
    emit('saved'); emit('close')
  } catch (error) { if (current === version) saveError.value = extractApiErrorMessage(error, t('pelican.saveFailed')) }
  finally { saving.value = false }
}
watch(() => [props.show, props.group?.id, props.intelligence], () => { void load() }, { immediate: true })
onUnmounted(() => { version++ })
</script>
