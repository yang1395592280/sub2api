<template>
  <AppLayout>
    <div class="flex min-h-[calc(100vh-7rem)] flex-col gap-4 px-4 py-4 lg:px-6">
      <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
        <!-- 左侧：会话列表 -->
        <section class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-100 bg-gradient-to-r from-primary-50/60 to-transparent px-4 py-3 dark:border-dark-700 dark:from-primary-500/5">
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
            <div class="mb-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
              {{ t('workbench.retentionNotice') }}
            </div>

            <div v-if="loadingConversations" class="flex items-center justify-center p-6 text-sm text-gray-500 dark:text-gray-400">
              {{ t('workbench.loading') }}
            </div>

            <div
              v-for="conversation in conversations"
              :key="conversation.id"
              class="group mb-2 flex w-full items-start gap-2 rounded-xl border px-3 py-3 transition-all duration-200"
              :class="activeConversationId === conversation.id
                ? 'border-primary-300 bg-primary-50 shadow-sm dark:border-primary-500/60 dark:bg-primary-500/10'
                : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 hover:shadow-sm dark:border-dark-700 dark:bg-dark-800 dark:hover:border-dark-500 dark:hover:bg-dark-700/80'"
            >
              <button
                type="button"
                class="flex min-w-0 flex-1 items-start gap-3 text-left"
                @click="selectConversation(conversation.id)"
              >
                <div
                  class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg transition-transform group-hover:scale-105"
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
              <button
                type="button"
                class="mt-1 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-gray-400 transition hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-500 dark:hover:bg-red-500/10 dark:hover:text-red-300"
                :aria-label="t('workbench.deleteConversation')"
                :title="t('workbench.deleteConversation')"
                :disabled="isDeletingConversation(conversation.id)"
                :data-testid="`workbench-delete-conversation-${conversation.id}`"
                @click.stop="handleDeleteConversation(conversation.id)"
              >
                <TrashIcon class="h-4 w-4" />
              </button>
            </div>

            <div v-if="!loadingConversations && conversations.length === 0" class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
              <div class="flex h-14 w-14 items-center justify-center rounded-full bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500">
                <SparklesIcon class="h-6 w-6" />
              </div>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('workbench.emptyState') }}</p>
            </div>
          </div>
        </section>

        <!-- 中间：聊天区域 -->
        <section class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="border-b border-gray-100 bg-gradient-to-r from-primary-50/40 to-transparent px-4 py-3 dark:border-dark-700 dark:from-primary-500/5">
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

          <!-- 消息列表 -->
          <div ref="messageContainerRef" class="flex-1 space-y-4 overflow-y-auto bg-gradient-to-b from-gray-50/50 to-gray-100/50 px-4 py-4 dark:from-dark-900/40 dark:to-dark-900/60">
            <div v-if="loadingMessages" class="flex items-center justify-center rounded-2xl border border-dashed border-gray-300 bg-white/70 p-6 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800/70 dark:text-gray-400">
              {{ t('workbench.loading') }}
            </div>

            <TransitionGroup v-else name="msg" tag="div" class="space-y-4">
              <article
                v-for="message in messages"
                :key="message.id"
                class="flex max-w-3xl gap-3"
                :class="message.role === 'user' ? 'ml-auto flex-row-reverse' : 'mr-auto'"
              >
                <!-- 头像 -->
                <div
                  class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white"
                  :class="message.role === 'user'
                    ? 'bg-primary-500'
                    : 'bg-gradient-to-br from-emerald-400 to-blue-500'"
                >
                  {{ message.role === 'user' ? t('workbench.you').charAt(0) : 'AI' }}
                </div>
                <!-- 气泡 -->
                <div
                  class="rounded-2xl border px-4 py-3 shadow-sm transition-all"
                  :class="[
                    message.role === 'user'
                      ? 'border-primary-200 bg-primary-50 dark:border-primary-500/30 dark:bg-primary-500/10'
                      : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800',
                    message.status === 'pending' ? 'opacity-70' : '',
                    message.status === 'error' ? 'border-red-200 bg-red-50 dark:border-red-500/30 dark:bg-red-500/10' : ''
                  ]"
                >
                  <div class="mb-1.5 flex items-center justify-between gap-3 text-xs">
                    <span class="font-medium text-gray-700 dark:text-gray-200">{{ message.role === 'user' ? t('workbench.you') : t('workbench.assistant') }}</span>
                    <span
                      class="rounded-full px-2 py-0.5 text-[10px] font-medium"
                      :class="message.status === 'pending'
                        ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300'
                        : message.status === 'error'
                          ? 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'"
                    >
                      {{ message.status || 'success' }}
                    </span>
                  </div>
                  <p class="whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-100">{{ message.content }}</p>

                  <div v-if="message.image_outputs?.length" class="mt-3 grid gap-3 sm:grid-cols-2">
                    <div
                      v-for="(output, index) in message.image_outputs"
                      :key="`${message.id}-${index}`"
                      class="group/img relative overflow-hidden rounded-xl border border-gray-200 bg-gray-100 shadow-sm dark:border-dark-600 dark:bg-dark-700"
                    >
                      <img
                        :src="imageURL(output)"
                        :alt="`generated-${index}`"
                        class="w-full cursor-zoom-in object-cover transition-transform duration-300 group-hover/img:scale-105"
                        @click="openLightbox(imageURL(output))"
                      />
                      <!-- 悬浮操作栏 -->
                      <div class="absolute inset-x-0 bottom-0 flex items-center justify-end gap-2 bg-gradient-to-t from-black/60 to-transparent p-2 opacity-0 transition-opacity duration-200 group-hover/img:opacity-100">
                        <button
                          type="button"
                          class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/90 text-gray-700 shadow-sm transition hover:bg-white hover:text-gray-900 dark:bg-dark-800/90 dark:text-gray-300 dark:hover:bg-dark-800"
                          @click="openLightbox(imageURL(output))"
                        >
                          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v6M7 10h6" />
                          </svg>
                        </button>
                        <button
                          type="button"
                          class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/90 text-gray-700 shadow-sm transition hover:bg-white hover:text-gray-900 dark:bg-dark-800/90 dark:text-gray-300 dark:hover:bg-dark-800"
                          @click="downloadImage(imageURL(output), `${message.id}-${index}`)"
                        >
                          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  </div>

                  <p v-if="message.error_message" class="mt-2 text-xs text-red-500">{{ message.error_message }}</p>
                </div>
              </article>
            </TransitionGroup>

            <!-- 思考中占位气泡 -->
            <div v-if="sending" class="mr-auto flex max-w-3xl gap-3">
              <div class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-emerald-400 to-blue-500 text-xs font-semibold text-white">
                AI
              </div>
              <div class="rounded-2xl border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
                <div class="flex items-center gap-2">
                  <span class="flex gap-1">
                    <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400 [animation-delay:-0.3s] dark:bg-gray-500"></span>
                    <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400 [animation-delay:-0.15s] dark:bg-gray-500"></span>
                    <span class="h-2 w-2 animate-bounce rounded-full bg-gray-400 dark:bg-gray-500"></span>
                  </span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('workbench.thinking') }}</span>
                </div>
              </div>
            </div>

            <div v-if="!loadingMessages && messages.length === 0 && !sending" class="flex h-full min-h-[320px] flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-gray-300 bg-white/70 p-8 text-center dark:border-dark-600 dark:bg-dark-800/70">
              <div class="flex h-14 w-14 items-center justify-center rounded-full bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500">
                <SparklesIcon class="h-6 w-6" />
              </div>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('workbench.emptyMessages') }}</p>
            </div>
          </div>

          <!-- 输入框 -->
          <div class="border-t border-gray-100 bg-white px-4 py-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="rounded-2xl border border-gray-200 bg-gray-50 p-3 transition-all duration-200 focus-within:border-primary-400 focus-within:ring-2 focus-within:ring-primary-100 dark:border-dark-600 dark:bg-dark-800 dark:focus-within:border-primary-500 dark:focus-within:ring-primary-500/20">
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
                  class="btn btn-primary inline-flex items-center gap-2"
                  data-testid="workbench-send"
                  :disabled="sending || !prompt.trim() || !selectedApiKeyId || !selectedModel"
                  @click="handleSend"
                >
                  <svg v-if="sending" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 0 1 4 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  <span>{{ sending ? t('workbench.sending') : t('workbench.send') }}</span>
                </button>
              </div>
            </div>
          </div>
        </section>

        <!-- 右侧：设置面板 -->
        <aside class="card flex min-h-[720px] flex-col overflow-hidden">
          <div class="border-b border-gray-100 bg-gradient-to-r from-primary-50/40 to-transparent px-4 py-3 dark:border-dark-700 dark:from-primary-500/5">
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
                    <option value="1024x1024">1K</option>
                    <option value="1536x1024">2K</option>
                    <option value="3840x2160">4K</option>
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

            <a
              href="/image-api-docs"
              target="_blank"
              rel="noopener noreferrer"
              data-testid="workbench-image-api-docs-link"
              class="flex items-center justify-between gap-3 rounded-xl border border-primary-200 bg-primary-50 px-4 py-3 text-sm font-medium text-primary-700 transition hover:border-primary-300 hover:bg-primary-100 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200 dark:hover:border-primary-400/50 dark:hover:bg-primary-500/20"
            >
              <span>{{ t('workbench.imageApiDocs') }}</span>
              <span aria-hidden="true">→</span>
            </a>
          </div>
        </aside>
      </div>
    </div>
    <!-- 图片大图查看 -->
    <Teleport to="body">
      <Transition name="lightbox">
        <div
          v-if="lightboxImage"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
          @click="closeLightbox"
        >
          <div class="relative max-h-full max-w-full">
            <img
              :src="lightboxImage"
              alt="preview"
              class="max-h-[90vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
              @click.stop
            />
            <button
              type="button"
              class="absolute -right-3 -top-3 flex h-9 w-9 items-center justify-center rounded-full bg-white text-gray-700 shadow-lg transition hover:bg-gray-100"
              @click="closeLightbox"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            <button
              type="button"
              class="absolute -bottom-3 -right-3 flex h-9 w-9 items-center justify-center rounded-full bg-white text-gray-700 shadow-lg transition hover:bg-gray-100"
              @click.stop="downloadImage(lightboxImage, 'full')"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
            </button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const TrashIcon = defineComponent({
  name: 'TrashIcon',
  setup() {
    return () => h(
      'svg',
      { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5', 'aria-hidden': 'true' },
      [
        h('path', {
          'stroke-linecap': 'round',
          'stroke-linejoin': 'round',
          d: 'M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673A2.25 2.25 0 0 1 15.916 21H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0'
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
const deletingConversationIds = ref<Set<number>>(new Set())
const sending = ref(false)
const prompt = ref('')
const currentMode = ref<WorkbenchMode>('chat')
const keys = ref<ApiKeyListItem[]>([])
const models = ref<WorkbenchModel[]>([])
const selectedApiKeyId = ref<number>(0)
const selectedModel = ref('')
const modelsLoadedForApiKeyId = ref<number | null>(null)
const modelSelectionReady = ref(false)
const messageContainerRef = ref<HTMLElement | null>(null)
const lightboxImage = ref<string>('')
let optimisticIdCounter = -1
let pendingImagePollTimer: ReturnType<typeof setInterval> | null = null
const defaultImageModel = 'gpt-image-2'
const pendingImagePollIntervalMs = 2000

const imageOptions = ref({
  size: '1024x1024',
  quality: 'auto',
  background: 'auto',
  output_format: 'png',
  output_compression: 100,
  n: 1,
})

const activeKeys = computed(() => keys.value.filter((item) => item.status === 'active' || !item.status))

function isImageModel(model: string): boolean {
  return model.trim().toLowerCase().startsWith('gpt-image-')
}

const availableModels = computed<WorkbenchModel[]>(() => {
  if (currentMode.value !== 'image') {
    return models.value
  }

  const imageModels = models.value.filter((model) => isImageModel(model.name))
  if (imageModels.some((model) => model.name === defaultImageModel)) {
    return imageModels
  }
  return [...imageModels, { name: defaultImageModel }]
})

const currentEndpoint = computed(() => currentMode.value === 'image' ? 'images_generations' : 'chat_completions')

const activeConversationTitle = computed(() => {
  const current = conversations.value.find((item) => item.id === activeConversationId.value)
  return current?.title || t('workbench.title')
})

function openLightbox(url: string): void {
  lightboxImage.value = url
}

function closeLightbox(): void {
  lightboxImage.value = ''
}

async function downloadImage(url: string, name: string): Promise<void> {
  try {
    const response = await fetch(url)
    const blob = await response.blob()
    const blobUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = `workbench-${name}.png`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(blobUrl)
  } catch {
    const link = document.createElement('a')
    link.href = url
    link.download = `workbench-${name}.png`
    link.target = '_blank'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }
}

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const container = messageContainerRef.value
  if (container) {
    container.scrollTop = container.scrollHeight
  }
}

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

function isDeletingConversation(conversationId: number): boolean {
  return deletingConversationIds.value.has(conversationId)
}

function setConversationDeleting(conversationId: number, deleting: boolean): void {
  const next = new Set(deletingConversationIds.value)
  if (deleting) {
    next.add(conversationId)
  } else {
    next.delete(conversationId)
  }
  deletingConversationIds.value = next
}

function hasPendingImageMessage(items: WorkbenchMessage[] = messages.value): boolean {
  return items.some((message) =>
    message.mode === 'image' &&
    message.role === 'assistant' &&
    message.status === 'pending'
  )
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

function isWorkbenchSendTimeoutError(error: unknown): boolean {
  const candidate = error as { code?: unknown; message?: unknown }
  const code = typeof candidate?.code === 'string' ? candidate.code : ''
  const message = typeof candidate?.message === 'string' ? candidate.message.toLowerCase() : ''
  return code === 'ECONNABORTED' || message.includes('timeout')
}

function ensureSelectedModel(): void {
  if (!availableModels.value.some((model) => model.name === selectedModel.value)) {
    selectedModel.value = ''
  }
  if (!selectedModel.value && availableModels.value.length > 0) {
    selectedModel.value = availableModels.value[0].name
  }
}

async function refreshMessagesAfterTimedOutSend(conversationId: number, inputText: string): Promise<void> {
  const refreshed = sanitizeWorkbenchMessages(await workbenchAPI.listMessages(conversationId))
  const hasPersistedUserMessage = refreshed.some((message) =>
    message.role === 'user' && message.content === inputText && message.id > 0
  )
  if (hasPersistedUserMessage) {
    messages.value = refreshed
    syncPendingImagePolling()
  }
}

function stopPendingImagePolling(): void {
  if (pendingImagePollTimer) {
    clearInterval(pendingImagePollTimer)
    pendingImagePollTimer = null
  }
}

function syncPendingImagePolling(): void {
  if (!hasPendingImageMessage()) {
    stopPendingImagePolling()
    return
  }
  if (pendingImagePollTimer || !activeConversationId.value) {
    return
  }
  const conversationId = activeConversationId.value
  pendingImagePollTimer = setInterval(() => {
    refreshPendingImageMessages(conversationId)
  }, pendingImagePollIntervalMs)
}

async function refreshPendingImageMessages(conversationId: number): Promise<void> {
  if (activeConversationId.value !== conversationId) {
    stopPendingImagePolling()
    return
  }
  try {
    const refreshed = sanitizeWorkbenchMessages(await workbenchAPI.listMessages(conversationId))
    if (activeConversationId.value !== conversationId) {
      return
    }
    messages.value = refreshed
    if (!hasPendingImageMessage(refreshed)) {
      stopPendingImagePolling()
    }
    await scrollToBottom()
  } catch (error: unknown) {
    console.error(error)
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
  ensureSelectedModel()
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
    syncPendingImagePolling()
    await scrollToBottom()
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
  ensureSelectedModel()
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

async function handleDeleteConversation(conversationId: number): Promise<void> {
  if (isDeletingConversation(conversationId)) return

  const deleteIndex = conversations.value.findIndex((item) => item.id === conversationId)
  const wasActive = activeConversationId.value === conversationId
  setConversationDeleting(conversationId, true)

  try {
    await workbenchAPI.deleteConversation(conversationId)
    const remaining = conversations.value.filter((item) => item.id !== conversationId)
    conversations.value = remaining

    if (wasActive) {
      stopPendingImagePolling()
      const nextConversation = remaining[deleteIndex] ?? remaining[deleteIndex - 1] ?? remaining[0] ?? null
      if (nextConversation) {
        activeConversationId.value = nextConversation.id
        currentMode.value = nextConversation.mode
        await loadMessages(nextConversation.id)
      } else {
        activeConversationId.value = null
        messages.value = []
      }
    }

    appStore.showSuccess(t('workbench.deleteConversationSuccess'))
  } catch (error: unknown) {
    console.error(error)
    appStore.showError(t('workbench.deleteConversationFailed'))
  } finally {
    setConversationDeleting(conversationId, false)
  }
}

async function ensureConversation(): Promise<number | null> {
  if (activeConversationId.value) return activeConversationId.value
  await handleCreateConversation()
  return activeConversationId.value
}

async function handleSend(): Promise<void> {
  if (sending.value) return
  if (!prompt.value.trim()) return
  if (!selectedApiKeyId.value) {
    appStore.showError(t('workbench.apiKeyRequired'))
    return
  }
  if (!selectedModel.value) {
    appStore.showError(t('workbench.modelRequired'))
    return
  }

  const inputText = prompt.value.trim()

  // 乐观上屏：立即将用户消息显示到聊天区域
  const optimisticId = optimisticIdCounter--
  const optimisticMessage = {
    id: optimisticId,
    conversation_id: activeConversationId.value || 0,
    user_id: 0,
    mode: currentMode.value,
    role: 'user' as const,
    content: inputText,
    endpoint: currentEndpoint.value,
    model: selectedModel.value,
    request_options: {},
    response_metadata: {},
    image_outputs: [],
    status: 'pending' as const,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  } as WorkbenchMessage
  messages.value = [...messages.value, optimisticMessage]
  prompt.value = ''
  await scrollToBottom()

  const conversationId = await ensureConversation()
  if (!conversationId) {
    messages.value = messages.value.map((m) =>
      m.id === optimisticId ? { ...m, status: 'error' as const } : m
    )
    return
  }

  const payload: WorkbenchSendRequest = {
    mode: currentMode.value,
    api_key_id: selectedApiKeyId.value,
    endpoint: currentEndpoint.value,
    model: selectedModel.value,
    input: inputText,
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

      // 用真实消息替换乐观消息，并追加助手回复
      messages.value = messages.value.map((m) =>
        m.id === optimisticId ? sanitizedUserMessage : m
      )
      messages.value = [...messages.value, assistantMessage as WorkbenchMessage]
      upsertConversation(result.conversation as WorkbenchConversation)
      activeConversationId.value = result.conversation.id
      syncPendingImagePolling()
    } else {
      messages.value = messages.value.map((m) =>
        m.id === optimisticId ? { ...m, status: 'error' as const } : m
      )
    }

    if (errorSummary) {
      appStore.showError(errorSummary)
    }

    if (!result) {
      appStore.showError(t('workbench.sendFailed'))
    }
  } catch (error: unknown) {
    if (isWorkbenchSendTimeoutError(error)) {
      await refreshMessagesAfterTimedOutSend(conversationId, inputText)
    } else {
      console.error(error)
      messages.value = messages.value.map((m) =>
        m.id === optimisticId ? { ...m, status: 'error' as const } : m
      )
      appStore.showError(t('workbench.sendFailed'))
    }
  } finally {
    sending.value = false
    await scrollToBottom()
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

onBeforeUnmount(() => {
  stopPendingImagePolling()
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

watch(() => messages.value.length, () => {
  scrollToBottom()
})
</script>

<style scoped>
.msg-enter-active {
  transition: all 0.3s ease;
}
.msg-enter-from {
  opacity: 0;
  transform: translateY(12px);
}
.lightbox-enter-active,
.lightbox-leave-active {
  transition: opacity 0.2s ease;
}
.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}
</style>
