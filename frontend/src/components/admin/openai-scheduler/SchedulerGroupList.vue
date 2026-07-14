<template>
  <nav :aria-label="t('admin.openaiAutoScheduler.groups.title')" class="min-w-0">
    <div class="mb-2 flex items-center justify-between gap-3 px-1">
      <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.groups.title') }}</h2>
      <span class="text-xs text-gray-500 dark:text-dark-400">{{ groups.length }}</span>
    </div>
    <div class="flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-x-visible">
      <button
        v-for="group in groups"
        :key="group.id"
        type="button"
        :data-testid="`scheduler-group-${group.id}`"
        class="flex min-w-[210px] items-center justify-between gap-3 rounded-md border px-3 py-2.5 text-left transition lg:min-w-0"
        :class="group.id === modelValue
          ? 'border-primary-300 bg-primary-50 text-primary-900 dark:border-primary-700 dark:bg-primary-500/10 dark:text-primary-100'
          : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200'"
        @click="emit('update:modelValue', group.id)"
      >
        <span class="min-w-0">
          <span class="block truncate text-sm font-medium">{{ group.name }}</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
            {{ group.enabled ? t('admin.openaiAutoScheduler.groups.participating') : t('admin.openaiAutoScheduler.groups.excluded') }}
          </span>
        </span>
        <Toggle
          :data-testid="`scheduler-group-toggle-${group.id}`"
          :modelValue="group.enabled"
          @click.stop
          @update:modelValue="emit('toggle', group.id, $event)"
        />
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import Toggle from '@/components/common/Toggle.vue'
import type { OpenAIAutoSchedulerGroup } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  groups: OpenAIAutoSchedulerGroup[]
  modelValue: number | null
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: number): void
  (event: 'toggle', groupId: number, enabled: boolean): void
}>()
</script>
