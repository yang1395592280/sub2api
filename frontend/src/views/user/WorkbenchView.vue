<template>
  <AppLayout>
    <div class="flex min-h-[calc(100vh-7rem)] flex-col gap-4 px-4 py-4 lg:px-6">
      <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
        <section class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <div>
              <h1 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('workbench.conversations') }}</h1>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('workbench.conversationsHint') }}</p>
            </div>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              data-testid="workbench-new-conversation"
              :disabled="creatingConversation"
              @click="handleCreateConversation"
            >
              {{ t('workbench.newConversation') }}
            </button>
          </div>

          <div class="flex-1 overflow-y-auto p-2">
            <div v-if="loadingConversations" class="flex items-center justify-center p-6 text-sm text-gray-500 dark:text-gray-400">
              {{ t('workbench.loading') }}
            </div>

            <button
              v-for="conversation in conversations"
              :key="conversation.id"
              type="button"
              class="mb-2 flex w-full items-start gap-3 rounded-lg border px-3 py-3 text-left transition"
              :class="activeConversationId === conversation.id
                ? 'border-primary-300 bg-primary-50 dark:border-primary-500/60 dark:bg-primary-500/10'
                : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-dark-500 dark:hover:bg-dark-700/80'"
              @click="selectConversation(conversation.id)"
            >
              <div
                class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg"
                :class="conversation.mode === 'image'
                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
                  : 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300'"
              >
                <SparklesIcon class="h-4 w-4" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex items-center justify-between gap-2">
                  <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ conversation.title || t('workbench.untitled') }}</p>
                  <span class="rounded-full px-2 py-0.5 text-[11px] font-medium uppercase"
                    :class="conversation.mode === 'image'
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
                      : 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300'"
                  >
                    {{ conversation.mode }}
                  </span>
                </div>
                <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ conversation.last_message_preview || t('workbench.emptyConversation') }}
                </p>
                <div class="mt-2 flex items-center justify-between gap-2 text-[11px] text-gray-400 dark:text-gray-500">
                  <span>{{ conversation.message_count || 0 }} {{ t('workbench.messagesCount') }}</span>
                  <span>{{ formatDateTime(conversation.updated_at) }}</span>
                </div>
              </div>
            </button>

            <div v-if="!loadingConversations && conversations.length === 0" class="flex h-full items-center justify-center p-6 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('workbench.emptyState') }}
            </div>
          </div>
        </section>

        <section class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ activeConversationTitle }}</h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('workbench.workspaceHint') }}</p>
              </div>
              <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-800">
                <button
                  type="button"
                  class="rounded-md px-3 py-1.5 text-sm font-medium transition"
                  :class="currentMode === 'chat'
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                    : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
                  data-testid="workbench-mode-chat"
                  @click="setMode('chat')"
                >
                  {{ t('workbench.modeChat') }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-3 py-1.5 text-sm font-medium transition"
                  :class="currentMode === 'image'
                    ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                    : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
                  data-testid="workbench-mode-image"
                  @click="setMode('image')"
                >
                  {{ t('workbench.modeImage') }}
                </button>
              </div>
            </div>
          </div>

          <div class="flex-1 space-y-4 overflow-y-auto bg-gray-50/80 px-4 py-4 dark:bg-dark-900/40">
            <div v-if="loadingMessages" class="flex items-center justify-center rounded-2xl border border-dashed border-gray-300 bg-white/70 p-6 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-400">
              {{ t('workbench.loading') }}
            </div>

            <article
              v-for="message in messages"
              :key="message.id"
              class="max-w-3xl rounded-2xl border px-4 py-3 shadow-sm"
              :class="message.role === 'user'
                ? 'ml-auto border-primary-200 bg-primary-50 dark:border-primary-500/30 dark:bg-primary-500/10'
                : 'mr-auto border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
            >
              <div class="mb-2 flex items-center justify-between gap-3 text-xs">
                <span class="font-medium text-gray-700 dark:text-gray-200">{{ message.role === 'user' ? t('workbench.you') : t('workbench.assistant') }}</span>
                <span class="text-gray-400 dark:text-gray-500">{{ message.status || 'success' }}</span>
              </div>
              <p class="whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-100">{{ message.content }}</p>

              <div v-if="message.image_outputs?.length" class="mt-3 grid gap-3 sm:grid-cols-2">
                <div
                  v-for="(output, index) in message.image_outputs"
                  :key="`${message.id}-${index}`"
                  class="overflow-hidden rounded-xl border border-gray-200 bg-gray-100 dark:border-dark-600 dark:bg-dark-700"
                >
                  <img
                    :src="imageURL(output)"
                    :alt="`generated-${index}`"
                    class="h-full w-full object-cover"
                  />
                </div>
              </div>

              <p v-if="message.error_message" class="mt-2 text-xs text-red-500">{{ message.error_message }}</p>
            </article>

            <div v-if="!loadingMessages && messages.length === 0" class="flex h-full min-h-[320px] items-center justify-center rounded-2xl border border-dashed border-gray-300 bg-white/70 p-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-400">
              {{ t('workbench.emptyMessages') }}
            </div>
          </div>

          <div class="border-t border-gray-100 bg-white px-4 py-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="rounded-2xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
              <textarea
                v-model="prompt"
                data-testid="workbench-input"
                rows="4"
                class="w-full resize-none border-0 bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-white"
                :placeholder="currentMode === 'image' ? t('workbench.imagePlaceholder') : t('workbench.chatPlaceholder')"
                @keydown.enter.exact.prevent="handleSend"
              />
              <div class="mt-3 flex items-center justify-between gap-3">
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ currentMode === 'image' ? t('workbench.imageHint') : t('workbench.chatHint') }}
                </p>
                <button
                  type="button"
                  class="btn btn-primary"
                  data-testid="workbench-send"
                  :disabled="sending || !prompt.trim() || !selectedApiKeyId || !selectedModel"
                  @click="handleSend"
                >
                  {{ sending ? t('workbench.sending') : t('workbench.send') }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <aside class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('workbench.settings') }}</h2>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('workbench.settingsHint') }}</p>
          </div>

          <div class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
            <div class="space-y-2">
              <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.apiKey') }}</label>
              <select v-model.number="selectedApiKeyId" class="input" data-testid="workbench-api-key-select">
                <option :value="0">{{ t('workbench.selectApiKey') }}</option>
                <option v-for="keyItem in activeKeys" :key="keyItem.id" :value="keyItem.id">{{ keyItem.name }}</option>
              </select>
            </div>

            <div class="space-y-2">
              <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.model') }}</label>
              <select v-model="selectedModel" class="input" data-testid="workbench-model-select">
                <option value="">{{ t('workbench.selectModel') }}</option>
                <option v-for="model in availableModels" :key="model.name" :value="model.name">{{ model.name }}</option>
              </select>
            </div>

            <div class="space-y-2">
              <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.endpoint') }}</label>
              <div class="rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200">
                {{ currentEndpoint }}
              </div>
            </div>

            <template v-if="currentMode === 'image'">
              <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-1">
                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageSize') }}</label>
                  <select v-model="imageOptions.size" class="input" data-testid="workbench-image-size-select">
                    <option value="1K">1K</option>
                    <option value="2K">2K</option>
                    <option value="4K">4K</option>
                  </select>
                </div>

                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageQuality') }}</label>
                  <select v-model="imageOptions.quality" class="input">
                    <option value="auto">auto</option>
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                  </select>
                </div>

                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageBackground') }}</label>
                  <select v-model="imageOptions.background" class="input">
                    <option value="auto">auto</option>
                    <option value="transparent">transparent</option>
                    <option value="opaque">opaque</option>
                  </select>
                </div>

                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageFormat') }}</label>
                  <select v-model="imageOptions.output_format" class="input">
                    <option value="png">png</option>
                    <option value="jpeg">jpeg</option>
                    <option value="webp">webp</option>
                  </select>
                </div>

                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageCompression') }}</label>
                  <input v-model.number="imageOptions.output_compression" type="number" min="0" max="100" class="input" />
                </div>

                <div class="space-y-2">
                  <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('workbench.imageCount') }}</label>
                  <input v-model.number="imageOptions.n" type="number" min="1" max="4" class="input" />
                </div>
              </div>
            </template>

            <template v-else>
              <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">
                <p class="font-medium text-gray-900 dark:text-white">{{ t('workbench.chatPanelTitle') }}</p>
                <p class="mt-2 leading-6">{{ t('workbench.chatPanelBody') }}</p>
              </div>
            </template>

            <div class="rounded-2xl border border-dashed border-gray-300 p-4 text-xs leading-6 text-gray-500 dark:border-dark-600 dark:text-gray-400">
              {{ t('workbench.routeOnlyHint') }}
            </div>
          </div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { keysAPI } from '@/api/keys'
import {
  workbenchAPI,
  type WorkbenchConversation,
  type WorkbenchImageOutput,
  type WorkbenchMessage,
  type WorkbenchModel,
  type WorkbenchMode,
  type WorkbenchSendResponse,
  type WorkbenchSendRequest,
  type WorkbenchSendResult,
} from '@/api/workbench'
import { useAppStore } from '@/stores/app'

interface ApiKeyListItem {
  id: number
  name: string
  status?: string
}

const SparklesIcon = defineComponent({
  name: 'SparklesIcon',
  setup() {
    return () => h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5', 'aria-hidden': 'true' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.283-3.283L2.06 11.81l2.845-.813a4.5 4.5 0 0 0 3.283-3.283L9 4.872l.813 2.845a4.5 4.5 0 0 0 3.283 3.283l2.845.813-2.845.813a4.5 4.5 0 0 0-3.283 3.283ZM18.258 8.715 18 9.75l-.258-1.035a2.25 2.25 0 0 0-1.64-1.64L15.068 6.75l1.035-.258a2.25 2.25 0 0 0 1.64-1.64L18 3.818l.258 1.034a2.25 2.25 0 0 0 1.64 1.641l1.034.258-1.034.258a2.25 2.25 0 0 0-1.641 1.64ZM16.5 20.25 16 21.75l-.5-1.5a2.25 2.25 0 0 0-1.5-1.5l-1.5-.5 1.5-.5a2.25 2.25 0 0 0 1.5-1.5l.5-1.5.5 1.5a2.25 2.25 0 0 0 1.5 1.5l1.5.5-1.5.5a2.25 2.25 0 0 0-1.5 1.5Z'
        })
      ]
    )
  }
})

const { t } = useI18n()
const appStore = useAppStore()

const conversations = ref<WorkbenchConversation[]>([])
const messages = ref<WorkbenchMessage[]>([])
const activeConversationId = ref<number | null>(null)
const loadingConversations = ref(false)
const loadingMessages = ref(false)
const creatingConversation = ref(false)
const sending = ref(false)
const prompt = ref('')
const currentMode = ref<WorkbenchMode>('chat')
const keys = ref<ApiKeyListItem[]>([])
const models = ref<WorkbenchModel[]>([])
const selectedApiKeyId = ref<number>(0)
const selectedModel = ref('')
const modelsLoadedForApiKeyId = ref<number | null>(null)
const modelSelectionReady = ref(false)

const imageOptions = ref({
  size: '1K',
  quality: 'auto',
  background: 'auto',
  output_format: 'png',
  output_compression: 100,
  n: 1,
})

const activeKeys = computed(() => keys.value.filter((item) => item.status === 'active' || !item.status))

const availableModels = computed<WorkbenchModel[]>(() => models.value)

const currentEndpoint = computed(() => currentMode.value === 'image' ? 'images_generations' : 'chat_completions')

const activeConversationTitle = computed(() => {
  const current = conversations.value.find((item) => item.id === activeConversationId.value)
  return current?.title || t('workbench.title')
})

function formatDateTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function imageURL(output: WorkbenchImageOutput): string {
  if (output.url) return output.url
  if (output.b64_json) {
    const mimeType = output.mime_type || 'image/png'
    return `data:${mimeType};base64,${output.b64_json}`
  }
  return ''
}

function sanitizeErrorSummary(message?: string | null): string {
  if (!message) return ''

  return message
    .replace(/\bsk-[A-Za-z0-9_-]+\b/g, '[redacted]')
    .replace(/\bBearer\s+[A-Za-z0-9._-]+\b/gi, 'Bearer [redacted]')
}

function sanitizeWorkbenchMessage(message: WorkbenchMessage): WorkbenchMessage {
  return message.error_message
    ? { ...message, error_message: sanitizeErrorSummary(message.error_message) }
    : message
}

function sanitizeWorkbenchMessages(items: WorkbenchMessage[]): WorkbenchMessage[] {
  return items.map((item) => sanitizeWorkbenchMessage(item))
}

function isSendResult(payload: WorkbenchSendResponse): payload is WorkbenchSendResult {
  return 'user_message' in payload && 'assistant_message' in payload && 'conversation' in payload
}

function normalizeSendResponse(payload: WorkbenchSendResponse): { result: WorkbenchSendResult | null; errorSummary: string } {
  if (isSendResult(payload)) {
    return { result: payload, errorSummary: '' }
  }

  return {
    result: payload.result ?? null,
    errorSummary: sanitizeErrorSummary(payload.error?.message),
  }
}

async function loadKeys(): Promise<void> {
  const response = await keysAPI.list(1, 100)
  keys.value = response.items as ApiKeyListItem[]
  if (!selectedApiKeyId.value && activeKeys.value.length > 0) {
    selectedApiKeyId.value = activeKeys.value[0].id
  }
}

async function loadModels(): Promise<void> {
  if (!selectedApiKeyId.value) {
    models.value = []
    selectedModel.value = ''
    modelsLoadedForApiKeyId.value = null
    return
  }
  if (modelsLoadedForApiKeyId.value === selectedApiKeyId.value) {
    return
  }
  models.value = await workbenchAPI.listModels(selectedApiKeyId.value)
  modelsLoadedForApiKeyId.value = selectedApiKeyId.value
  if (!availableModels.value.some((model) => model.name === selectedModel.value)) {
    selectedModel.value = ''
  }
  if (!selectedModel.value && availableModels.value.length > 0) {
    selectedModel.value = availableModels.value[0].name
  }
}

async function loadConversations(): Promise<void> {
  loadingConversations.value = true
  try {
    const response = await workbenchAPI.listConversations({ page: 1, page_size: 20 })
    conversations.value = response.items
    if (!activeConversationId.value && conversations.value.length > 0) {
      activeConversationId.value = conversations.value[0].id
      currentMode.value = conversations.value[0].mode
      await loadMessages(activeConversationId.value)
    }
  } finally {
    loadingConversations.value = false
  }
}

async function loadMessages(conversationId: number): Promise<void> {
  loadingMessages.value = true
  try {
    messages.value = sanitizeWorkbenchMessages(await workbenchAPI.listMessages(conversationId))
  } finally {
    loadingMessages.value = false
  }
}

async function selectConversation(conversationId: number): Promise<void> {
  const target = conversations.value.find((item) => item.id === conversationId)
  activeConversationId.value = conversationId
  if (target?.mode) {
    currentMode.value = target.mode
  }
  await loadMessages(conversationId)
}

function setMode(mode: WorkbenchMode): void {
  currentMode.value = mode
}

async function handleCreateConversation(): Promise<void> {
  creatingConversation.value = true
  try {
    const created = await workbenchAPI.createConversation({
      mode: currentMode.value,
      api_key_id: selectedApiKeyId.value || undefined,
      endpoint: currentEndpoint.value,
      model: selectedModel.value || undefined,
      title: t('workbench.newConversation')
    })
    conversations.value = [created, ...conversations.value]
    activeConversationId.value = created.id
    messages.value = []
  } catch (error: unknown) {
    console.error(error)
    appStore.showError(t('workbench.createConversationFailed'))
  } finally {
    creatingConversation.value = false
  }
}

function upsertConversation(nextConversation: WorkbenchConversation): void {
  const index = conversations.value.findIndex((item) => item.id === nextConversation.id)
  if (index >= 0) {
    conversations.value.splice(index, 1, nextConversation)
  } else {
    conversations.value.unshift(nextConversation)
  }
  conversations.value = [...conversations.value]
}

async function ensureConversation(): Promise<number | null> {
  if (activeConversationId.value) return activeConversationId.value
  await handleCreateConversation()
  return activeConversationId.value
}

async function handleSend(): Promise<void> {
  if (!prompt.value.trim()) return
  if (!selectedApiKeyId.value) {
    appStore.showError(t('workbench.apiKeyRequired'))
    return
  }
  if (!selectedModel.value) {
    appStore.showError(t('workbench.modelRequired'))
    return
  }

  const conversationId = await ensureConversation()
  if (!conversationId) return

  const payload: WorkbenchSendRequest = {
    mode: currentMode.value,
    api_key_id: selectedApiKeyId.value,
    endpoint: currentEndpoint.value,
    model: selectedModel.value,
    input: prompt.value.trim(),
    options: currentMode.value === 'image' ? { ...imageOptions.value } : undefined,
  }

  sending.value = true
  try {
    const response = await workbenchAPI.send(conversationId, payload)
    const { result, errorSummary } = normalizeSendResponse(response)

    if (result) {
      const sanitizedUserMessage = sanitizeWorkbenchMessage(result.user_message as WorkbenchMessage)
      const sanitizedAssistantMessage = sanitizeWorkbenchMessage(result.assistant_message as WorkbenchMessage)
      const assistantMessage = errorSummary && !result.assistant_message.error_message
        ? { ...sanitizedAssistantMessage, error_message: errorSummary }
        : sanitizedAssistantMessage

      messages.value = [...messages.value, sanitizedUserMessage, assistantMessage as WorkbenchMessage]
      upsertConversation(result.conversation as WorkbenchConversation)
      activeConversationId.value = result.conversation.id
      prompt.value = ''
    }

    if (errorSummary) {
      appStore.showError(errorSummary)
    }

    if (!result) {
      appStore.showError(t('workbench.sendFailed'))
    }
  } catch (error: unknown) {
    console.error(error)
    appStore.showError(t('workbench.sendFailed'))
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  try {
    await loadKeys()
    await Promise.all([loadModels(), loadConversations()])
    modelSelectionReady.value = true
  } catch (error: unknown) {
    console.error(error)
    appStore.showError(t('workbench.loadFailed'))
  }
})

watch(selectedApiKeyId, async (next, prev) => {
  if (!modelSelectionReady.value) return
  if (next === prev) return
  try {
    await loadModels()
  } catch (error: unknown) {
    console.error(error)
    appStore.showError(t('workbench.loadFailed'))
  }
})
</script>
