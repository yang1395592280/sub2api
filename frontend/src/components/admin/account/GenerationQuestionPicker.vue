<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
      <label class="font-medium">{{ t('pelican.library.choose') }}</label>
      <router-link :to="{ path: '/admin/test-questions', query: { kind } }" class="text-xs text-primary-600 hover:underline">{{ t('pelican.library.manage') }}</router-link>
    </div>
    <div class="flex items-center justify-between gap-2 text-xs text-gray-500">
      <span>{{ t('pelican.library.selected', { count: modelValue.length, max: 20 }) }}</span>
      <button type="button" class="text-primary-600 hover:underline" :disabled="disabled" @click="emit('update:modelValue', [])">{{ t('pelican.library.clear') }}</button>
    </div>
    <p v-if="unavailable" role="alert" class="text-xs text-amber-700 dark:text-amber-300">{{ t('pelican.library.unavailable') }}</p>
    <div v-if="visibleQuestions.length" class="max-h-80 space-y-2 overflow-y-auto pr-1">
      <article v-for="question in visibleQuestions" :key="question.id" class="rounded-xl border p-3 transition-colors" :class="modelValue.includes(question.id) ? 'border-primary-400 bg-primary-50/50 dark:bg-primary-950/20' : 'border-gray-200 dark:border-dark-600'">
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-primary-600" :checked="modelValue.includes(question.id)" :disabled="disabled || (!modelValue.includes(question.id) && (!question.enabled || modelValue.length >= 20))" :data-question-id="question.id" @change="toggle(question.id, ($event.target as HTMLInputElement).checked)" />
          <span class="min-w-0"><span class="block break-words text-sm font-medium">{{ question.name }}<span v-if="!question.enabled" class="ml-2 text-xs text-amber-600">{{ t('pelican.library.disabled') }}</span></span><span class="mt-1 block text-xs text-gray-500">{{ kind === 'intelligence' ? t('pelican.library.expected', { answer: question.expected_answer }) : t('pelican.library.htmlOutput') }}</span></span>
        </label>
        <details class="mt-2 pl-7 text-xs text-gray-500"><summary class="cursor-pointer text-primary-600">{{ t('pelican.library.preview') }}</summary><p class="mt-2 whitespace-pre-wrap break-words leading-5">{{ question.prompt }}</p></details>
      </article>
    </div>
    <p v-else class="rounded-lg border border-dashed p-5 text-center text-sm text-gray-500">{{ t('pelican.library.empty') }}</p>
    <button v-if="missingIds.length" type="button" class="text-xs text-primary-600 hover:underline" :disabled="disabled" @click="emit('update:modelValue', modelValue.filter(id => !missingIds.includes(id)))">{{ t('pelican.library.removeMissing') }}</button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GenerationTestKind } from '@/api/admin/accounts'
import type { GenerationTestQuestion } from '@/api/admin/pelican'

const props = defineProps<{ modelValue: number[]; questions: GenerationTestQuestion[]; kind: GenerationTestKind; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [ids: number[]] }>()
const { t } = useI18n()
const visibleQuestions = computed(() => props.questions.filter(question => question.enabled || props.modelValue.includes(question.id)))
const missingIds = computed(() => props.modelValue.filter(id => !props.questions.some(question => question.id === id)))
const unavailable = computed(() => props.modelValue.some(id => !props.questions.some(question => question.id === id && question.enabled)))
function toggle(id: number, checked: boolean) {
  emit('update:modelValue', checked ? [...new Set([...props.modelValue, id])] : props.modelValue.filter(value => value !== id))
}
</script>
