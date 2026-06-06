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

      <div
        v-if="status !== 'idle'"
        class="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-700 dark:border-blue-900/40 dark:bg-blue-900/20 dark:text-blue-300"
      >
        {{ t('admin.accounts.bulkTestProgress', { current: progressCurrent, total: accounts.length }) }}
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
      <div class="flex flex-wrap justify-between gap-3">
        <div class="flex flex-wrap gap-2">
          <button
            v-if="successEmails.length > 0"
            @click="downloadEmails(successEmails, 'success-emails.txt')"
            data-testid="download-success-emails"
            class="rounded-lg bg-emerald-100 px-4 py-2 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-200 dark:bg-emerald-900/30 dark:text-emerald-300 dark:hover:bg-emerald-900/50"
          >
            {{ t('admin.accounts.bulkDownloadSuccessEmails') }}
          </button>
          <button
            v-if="failedEmails.length > 0"
            @click="downloadEmails(failedEmails, 'failed-emails.txt')"
            data-testid="download-failed-emails"
            class="rounded-lg bg-rose-100 px-4 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-200 dark:bg-rose-900/30 dark:text-rose-300 dark:hover:bg-rose-900/50"
          >
            {{ t('admin.accounts.bulkDownloadFailedEmails') }}
          </button>
        </div>
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
const progressCurrent = ref(0)
const successEmails = ref<string[]>([])
const failedEmails = ref<string[]>([])

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
  progressCurrent.value = 0
  successEmails.value = []
  failedEmails.value = []
}

const handleClose = () => {
  emit('close')
}

const extractEmail = (account: Account): string => {
  const value = account.extra && typeof account.extra.email_address === 'string'
    ? account.extra.email_address.trim()
    : ''
  if (value) return value
  return typeof account.name === 'string' && account.name.includes('@')
    ? account.name.trim()
    : ''
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

const formatElapsedSeconds = (startedAt: number, endedAt: number) => {
  const seconds = Math.max(0, (endedAt - startedAt) / 1000)
  return seconds.toFixed(2)
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

  for (const [index, account] of props.accounts.entries()) {
    progressCurrent.value = index + 1
    const startedAt = Date.now()
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
          prompt: '',
          mode: 'default'
        })
      })

      const body = await response.text()
      const result = parseSSEOutput(body)
      const elapsed = formatElapsedSeconds(startedAt, Date.now())

      if (!response.ok || result.error) {
        failedCount += 1
        const email = extractEmail(account)
        if (email) failedEmails.value.push(email)
        addLine(`ERROR: ${result.error || `HTTP ${response.status}`}`, 'text-red-400')
        addLine(`Elapsed ${elapsed}s`, 'text-gray-500')
      } else {
        successCount += 1
        const email = extractEmail(account)
        if (email) successEmails.value.push(email)
        if (result.model) {
          addLine(t('admin.accounts.usingModel', { model: result.model }), 'text-green-400')
        }
        addLine(result.responseText || t('admin.accounts.testCompleted'), 'text-green-300')
        addLine(`Elapsed ${elapsed}s`, 'text-gray-500')
      }
    } catch (error: unknown) {
      failedCount += 1
      const email = extractEmail(account)
      if (email) failedEmails.value.push(email)
      const message = error instanceof Error ? error.message : 'Unknown error'
      addLine(`ERROR: ${message}`, 'text-red-400')
      addLine(`Elapsed ${formatElapsedSeconds(startedAt, Date.now())}s`, 'text-gray-500')
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

const downloadEmails = (emails: string[], filename: string) => {
  if (emails.length === 0) return
  const blob = new Blob([emails.join(',')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
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
