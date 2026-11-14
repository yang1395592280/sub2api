<template>
  <AppLayout>
    <main class="mx-auto max-w-5xl space-y-6 pb-8">
      <header class="flex flex-wrap items-start justify-between gap-4"><div><h1 class="text-2xl font-semibold">{{ t('pelican.library.title') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('pelican.library.description') }}</p></div><button class="btn btn-primary" :disabled="loading || saving || !!error" @click="edit()"><Icon name="plus" size="sm" />{{ t('pelican.library.create') }}</button></header>
      <div class="flex flex-wrap items-center gap-2"><button v-for="item in kinds" :key="item" type="button" class="btn" :class="kind === item ? 'btn-primary' : 'btn-secondary'" :aria-pressed="kind === item" @click="changeKind(item)">{{ t(item === 'intelligence' ? 'pelican.library.intelligenceBank' : 'pelican.library.pelicanBank') }}</button></div>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }} <button class="underline" @click="load">{{ t('common.retry') }}</button></p>
      <div v-else-if="loading" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</div>
      <section v-else class="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white px-5 dark:divide-dark-700 dark:border-dark-700 dark:bg-dark-800">
        <article v-for="question in questions" :key="question.id" class="flex flex-wrap items-center justify-between gap-4 py-5">
          <div class="min-w-0 flex-1"><h2 class="break-words text-sm font-semibold">{{ question.name }}</h2><p class="mt-1 text-xs text-gray-500">{{ kind === 'intelligence' ? t('pelican.library.expected', { answer: question.expected_answer }) : t('pelican.library.htmlOutput') }} · {{ formatDateTime(question.updated_at) }}</p><p class="mt-2 line-clamp-2 whitespace-pre-wrap break-words text-xs text-gray-500">{{ question.prompt }}</p></div>
          <div class="flex items-center gap-3 text-xs"><span :class="question.enabled ? 'text-primary-600' : 'text-gray-400'">{{ t(question.enabled ? 'pelican.library.enabled' : 'pelican.library.disabled') }}</span><button class="text-primary-600 hover:underline" :disabled="saving" @click="edit(question)">{{ t('common.edit') }}</button><button class="text-primary-600 hover:underline" :disabled="saving" @click="toggle(question)">{{ t(question.enabled ? 'pelican.library.disable' : 'pelican.library.enable') }}</button></div>
        </article>
        <p v-if="!questions.length" class="p-10 text-center text-sm text-gray-500">{{ t('pelican.library.empty') }}</p>
      </section>
      <p class="text-xs leading-5 text-gray-500">{{ t('pelican.library.snapshotHint') }}</p>
      <BaseDialog :show="editorOpen" :title="t(editingId == null ? 'pelican.library.create' : 'pelican.library.edit')" width="wide" @close="closeEditor">
        <form id="test-question-form" class="space-y-4" @submit.prevent="save">
          <div><label for="test-question-name" class="input-label">{{ t('pelican.library.name') }}</label><input id="test-question-name" v-model="form.name" class="input w-full" maxlength="120" required :disabled="saving" /></div>
          <div><label for="test-question-prompt" class="input-label">{{ t('pelican.library.content') }}</label><textarea id="test-question-prompt" v-model="form.prompt" class="input w-full" rows="8" maxlength="4000" required :disabled="saving" /></div>
          <div v-if="kind === 'intelligence'"><label for="test-question-answer" class="input-label">{{ t('pelican.library.answer') }}</label><input id="test-question-answer" v-model="form.expected_answer" class="input w-full" inputmode="numeric" pattern="[0-9]{1,12}" maxlength="12" required :disabled="saving" /><p class="mt-2 text-xs text-gray-500">{{ t('pelican.library.answerHint') }}</p></div>
          <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-primary-600" :disabled="saving" />{{ t('pelican.library.enabled') }}</label>
          <p v-if="saveError" role="alert" class="text-sm text-red-600">{{ saveError }}</p>
        </form>
        <template #footer><button class="btn btn-secondary" :disabled="saving" @click="closeEditor">{{ t('common.cancel') }}</button><button class="btn btn-primary" form="test-question-form" :disabled="saving || !form.name.trim() || !form.prompt.trim()">{{ saving ? t('common.loading') : t('common.save') }}</button></template>
      </BaseDialog>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { listTestQuestions, saveTestQuestion, type GenerationTestQuestion, type GenerationTestQuestionInput } from '@/api/admin/pelican'
import type { GenerationTestKind } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n(), route = useRoute(), router = useRouter()
const kinds: GenerationTestKind[] = ['intelligence', 'pelican']
const kind = ref<GenerationTestKind>(route.query.kind === 'pelican' ? 'pelican' : 'intelligence')
const questions = ref<GenerationTestQuestion[]>([]), loading = ref(false), saving = ref(false)
const error = ref(''), saveError = ref(''), editorOpen = ref(false), editingId = ref<number | null>(null)
const form = ref<GenerationTestQuestionInput>({ test_kind: kind.value, name: '', prompt: '', expected_answer: '', enabled: true })
let version = 0
async function load() {
  const current = ++version
  loading.value = true; error.value = ''; questions.value = []
  try { const rows = await listTestQuestions(kind.value); if (current === version) questions.value = rows }
  catch (cause) { if (current === version) error.value = extractApiErrorMessage(cause, t('pelican.library.loadFailed')) }
  finally { if (current === version) loading.value = false }
}
function changeKind(value: GenerationTestKind) {
  if (saving.value) return
  void router.replace({ query: { ...route.query, kind: value } })
}
function edit(question?: GenerationTestQuestion) {
  editingId.value = question?.id ?? null
  form.value = { test_kind: kind.value, name: question?.name ?? '', prompt: question?.prompt ?? '', expected_answer: question?.expected_answer ?? '', enabled: question?.enabled ?? true }
  saveError.value = ''; editorOpen.value = true
}
function closeEditor() { if (!saving.value) editorOpen.value = false }
async function save() {
  if (saving.value) return
  saving.value = true; saveError.value = ''
  try { await saveTestQuestion(editingId.value, { ...form.value }); editorOpen.value = false; useAppStore().showSuccess(t('pelican.library.saved')); await load() }
  catch (cause) { saveError.value = extractApiErrorMessage(cause, t('pelican.library.saveFailed')) }
  finally { saving.value = false }
}
async function toggle(question: GenerationTestQuestion) {
  saving.value = true; error.value = ''
  try { await saveTestQuestion(question.id, { test_kind: question.test_kind, name: question.name, prompt: question.prompt, expected_answer: question.expected_answer, enabled: !question.enabled }); await load() }
  catch (cause) { error.value = extractApiErrorMessage(cause, t('pelican.library.saveFailed')) }
  finally { saving.value = false }
}
watch(() => route.query.kind, value => { kind.value = value === 'pelican' ? 'pelican' : 'intelligence'; editorOpen.value = false; void load() }, { immediate: true })
onUnmounted(() => { version++ })
</script>
