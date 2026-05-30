<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkActions.testConnection')"
    width="normal"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="rounded-xl border border-gray-200 bg-gradient-to-r from-gray-50 to-gray-100 p-3 dark:border-dark-500 dark:from-dark-700 dark:to-dark-600">
        <div class="flex items-center justify-between gap-3">
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">
              {{ t('admin.accounts.bulkActions.selected', { count: accounts.length }) }}
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.bulkTestHint') }}
            </div>
          </div>
          <span class="rounded-full bg-primary-100 px-2.5 py-1 text-xs font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
            {{ accounts.length }}
          </span>
        </div>
      </div>

      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.selectTestModel') }}
        </label>
        <Select
          v-model="selectedModelId"
          :options="availableModels"
          :disabled="loadingModels || status === 'connecting'"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? t('common.loading') + '...' : t('admin.accounts.selectTestModel')"
        />
      </div>

      <div class="group relative">
        <div
          ref="terminalRef"
          class="max-h-[360px] min-h-[160px] overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
        >
          <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
            <Icon name="play" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.readyToTest') }}</span>
          </div>
          <div v-else-if="status === 'connecting'" class="flex items-center gap-2 text-yellow-400">
            <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <span>{{ t('admin.accounts.connectingToApi') }}</span>
          </div>

          <div v-for="(line, index) in outputLines" :key="index" :class="line.class">
            {{ line.text }}
          </div>

          <div
            v-if="status === 'success'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
          >
            <Icon name="check" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.testCompleted') }}</span>
          </div>
          <div
            v-else-if="status === 'error'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-red-400"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
            <span>{{ errorMessage }}</span>
          </div>
        </div>

        <button
          v-if="outputLines.length > 0"
          @click="copyOutput"
          class="absolute right-2 top-2 rounded-lg bg-gray-800/80 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white group-hover:opacity-100"
          :title="t('admin.accounts.copyOutput')"
        >
          <Icon name="link" size="sm" :stroke-width="2" />
        </button>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          @click="handleClose"
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
        >
          {{ t('common.close') }}
        </button>
        <button
          @click="startBatchTest"
          :disabled="status === 'connecting' || !selectedModelId || accounts.length === 0"
          :class="[
            'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all',
            status === 'connecting' || !selectedModelId || accounts.length === 0
              ? 'cursor-not-allowed bg-primary-400 text-white'
              : 'bg-primary-500 text-white hover:bg-primary-600'
          ]"
        >
          <Icon
            v-if="status === 'connecting'"
            name="refresh"
            size="sm"
            class="animate-spin"
            :stroke-width="2"
          />
          <Icon v-else name="play" size="sm" :stroke-width="2" />
          <span>
            {{
              status === 'connecting'
                ? t('admin.accounts.testing')
                : t('admin.accounts.startTest')
            }}
          </span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { Account, ClaudeModel } from '@/types'

interface OutputLine {
  text: string
  class: string
}

const props = defineProps<{
  show: boolean
  accounts: Account[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const outputLines = ref<OutputLine[]>([])
const errorMessage = ref('')
const availableModels = ref<ClaudeModel[]>([])
const selectedModelId = ref('')
const loadingModels = ref(false)

const addLine = (text: string, className: string = 'text-gray-300') => {
  outputLines.value.push({ text, class: className })
  scrollToBottom()
}

const scrollToBottom = async () => {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

const resetState = () => {
  status.value = 'idle'
  outputLines.value = []
  errorMessage.value = ''
}

const handleClose = () => {
  emit('close')
}

const parseSSEOutput = (body: string) => {
  const lines = body.split('\n')
  let responseText = ''
  let error = ''
  let model = ''

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed.startsWith('data: ')) continue
    const payload = trimmed.slice(6).trim()
    if (!payload) continue
    try {
      const event = JSON.parse(payload) as {
        type: string
        text?: string
        error?: string
        model?: string
      }
      if (event.type === 'test_start' && event.model) {
        model = event.model
      } else if (event.type === 'content' && event.text) {
        responseText += event.text
      } else if (event.type === 'error' && event.error) {
        error = event.error
      }
    } catch {
      // ignore malformed line
    }
  }

  return { responseText, error, model }
}

const loadAvailableModels = async () => {
  if (props.accounts.length === 0) return

  loadingModels.value = true
  selectedModelId.value = ''
  try {
    const models = await adminAPI.accounts.getAvailableModels(props.accounts[0].id)
    availableModels.value = models
    if (models.length > 0) {
      selectedModelId.value = models[0].id
    }
  } catch (error) {
    console.error('Failed to load available models for batch test:', error)
    availableModels.value = []
  } finally {
    loadingModels.value = false
  }
}

const startBatchTest = async () => {
  if (!selectedModelId.value || props.accounts.length === 0) return

  resetState()
  status.value = 'connecting'

  let successCount = 0
  let failedCount = 0

  for (const account of props.accounts) {
    addLine('', 'text-gray-300')
    addLine(`=== ${account.name} (#${account.id}) ===`, 'text-cyan-400')
    addLine(t('admin.accounts.testAccountTypeLabel', { type: account.type }), 'text-gray-400')

    try {
      const response = await fetch(`/api/v1/admin/accounts/${account.id}/test`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${localStorage.getItem('auth_token')}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          model_id: selectedModelId.value,
          prompt: ''
        })
      })

      const body = await response.text()
      const result = parseSSEOutput(body)

      if (!response.ok || result.error) {
        failedCount += 1
        addLine(`ERROR: ${result.error || `HTTP ${response.status}`}`, 'text-red-400')
      } else {
        successCount += 1
        if (result.model) {
          addLine(t('admin.accounts.usingModel', { model: result.model }), 'text-green-400')
        }
        addLine(result.responseText || t('admin.accounts.testCompleted'), 'text-green-300')
      }
    } catch (error: unknown) {
      failedCount += 1
      const message = error instanceof Error ? error.message : 'Unknown error'
      addLine(`ERROR: ${message}`, 'text-red-400')
    }
  }

  addLine('', 'text-gray-300')
  addLine(
    t('admin.accounts.bulkTestSummary', { success: successCount, failed: failedCount }),
    failedCount > 0 ? 'text-yellow-400' : 'text-green-400'
  )

  status.value = failedCount > 0 ? 'error' : 'success'
  errorMessage.value = failedCount > 0 ? t('admin.accounts.bulkTestHasFailures') : ''
}

const copyOutput = () => {
  const text = outputLines.value.map((line) => line.text).join('\n')
  copyToClipboard(text, t('admin.accounts.outputCopied'))
}

watch(
  () => props.show,
  async (show) => {
    if (show) {
      resetState()
      await loadAvailableModels()
    }
  }
)
</script>
