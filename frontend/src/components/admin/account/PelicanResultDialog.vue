<template>
  <BaseDialog :show="show" :title="dialogTitle" width="extra-wide" @close="emit('close')">
    <div v-if="loading" class="p-8 text-center">{{ t('common.loading') }}</div>
    <p v-else-if="error" class="p-4 text-sm text-red-600" role="alert">{{ error }}</p>
    <div v-else-if="test" class="space-y-3">
      <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-gray-300">
        <span>{{ test.model_id }}</span>
        <span>{{ test.latency_ms }} ms</span>
        <span v-if="test.total_tokens != null">{{ t('admin.accounts.pelican.tokensTotal', { count: test.total_tokens.toLocaleString() }) }}</span>
        <span v-else>{{ t('admin.accounts.pelican.tokensUnavailable') }}</span>
        <span>{{ new Date(test.created_at).toLocaleString() }}</span>
      </div>
      <p v-if="test.input_tokens != null || test.output_tokens != null" class="text-xs text-gray-500 dark:text-gray-300">
        {{ t('admin.accounts.pelican.tokensBreakdown', { input: test.input_tokens?.toLocaleString() ?? '—', output: test.output_tokens?.toLocaleString() ?? '—' }) }}
      </p>
      <p class="whitespace-pre-wrap break-words text-xs text-gray-600 dark:text-gray-300">{{ test.prompt }}</p>
      <p v-if="test.error_message" class="text-sm text-red-600">{{ test.error_message }}</p>
      <p v-if="test.html" class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.pelican.sandboxHint') }}</p>
      <div class="flex gap-2">
        <button class="btn btn-secondary btn-sm" :disabled="!test.html" @click="tab = 'preview'">{{ t('admin.accounts.pelican.effect') }}</button>
        <button class="btn btn-secondary btn-sm" @click="tab = 'source'">{{ t('admin.accounts.pelican.source') }}</button>
        <button v-if="test.html" class="btn btn-secondary btn-sm" @click="reloadPreview">{{ t('admin.accounts.pelican.reload') }}</button>
        <button v-if="test.html" class="btn btn-secondary btn-sm" @click="downloadHTML">{{ t('admin.accounts.pelican.download') }}</button>
      </div>
      <template v-if="tab === 'preview' && test.html">
        <div class="flex flex-wrap items-center gap-3 text-xs text-gray-600 dark:text-gray-300">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="fitToWindow" @click="fitToWindow = true">{{ t('admin.accounts.pelican.fitWindow') }}</button>
          <label class="flex items-center gap-2">
            <span>{{ t('admin.accounts.pelican.zoom') }}</span>
            <input type="range" min="5" max="150" step="1" :value="Math.round(displayScale * 100)" class="w-32 accent-primary-600" @input="setZoom" />
            <span class="w-10 text-right tabular-nums">{{ Math.round(displayScale * 100) }}%</span>
          </label>
        </div>
        <div ref="previewViewport" data-testid="pelican-preview-viewport" class="h-[55vh] w-full overflow-auto rounded-lg border border-gray-200 bg-white dark:border-dark-600">
          <div class="relative" :style="{ width: `${canvasWidth}px`, height: `${canvasHeight}px` }">
            <iframe
              :key="previewKey"
              :srcdoc="safeSource"
              sandbox="allow-scripts"
              referrerpolicy="no-referrer"
              class="absolute origin-top-left border-0"
              :style="{
                width: `${PELICAN_PREVIEW_WIDTH}px`,
                height: `${PELICAN_PREVIEW_HEIGHT}px`,
                left: `${(canvasWidth - scaledWidth) / 2}px`,
                top: `${(canvasHeight - scaledHeight) / 2}px`,
                transform: `scale(${displayScale})`
              }"
              :title="t('admin.accounts.pelican.preview')"
            />
          </div>
        </div>
      </template>
      <pre v-else class="max-h-[55vh] overflow-auto whitespace-pre-wrap break-all rounded-lg bg-gray-100 p-4 text-xs dark:bg-dark-800">{{ test.response_text || t('admin.accounts.pelican.emptyResult') }}</pre>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { PelicanTest } from '@/api/admin/accounts'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildPelicanPreviewSource, getPelicanPreviewNonce, PELICAN_PREVIEW_HEIGHT, PELICAN_PREVIEW_WIDTH } from '@/utils/pelicanPreview'

const props = defineProps<{ show: boolean; testId: number | null; accountName: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const test = ref<PelicanTest | null>(null)
const loading = ref(false)
const error = ref('')
const tab = ref<'preview' | 'source'>('preview')
const previewKey = ref(0)
const previewViewport = ref<HTMLElement | null>(null)
const viewportWidth = ref(0)
const viewportHeight = ref(0)
const fitToWindow = ref(true)
const manualScale = ref(1)
const previewNonce = getPelicanPreviewNonce()
const fitScale = computed(() => viewportWidth.value && viewportHeight.value
  ? Math.min(viewportWidth.value / PELICAN_PREVIEW_WIDTH, viewportHeight.value / PELICAN_PREVIEW_HEIGHT, 1)
  : 1)
const displayScale = computed(() => fitToWindow.value ? fitScale.value : manualScale.value)
const scaledWidth = computed(() => PELICAN_PREVIEW_WIDTH * displayScale.value)
const scaledHeight = computed(() => PELICAN_PREVIEW_HEIGHT * displayScale.value)
const canvasWidth = computed(() => Math.max(viewportWidth.value, scaledWidth.value))
const canvasHeight = computed(() => Math.max(viewportHeight.value, scaledHeight.value))
const dialogTitle = computed(() => [
  t('admin.accounts.pelican.preview'),
  props.accountName,
  test.value ? t(`admin.accounts.pelican.status.${test.value.status}`) : '',
  test.value?.model_id
].filter(Boolean).join(' · '))

const safeSource = computed(() => test.value?.html ? buildPelicanPreviewSource(test.value.html, previewNonce) : '')

watch(previewViewport, (element, _previous, onCleanup) => {
  if (!element) return
  const updateSize = () => {
    viewportWidth.value = element.clientWidth
    viewportHeight.value = element.clientHeight
  }
  updateSize()
  if (typeof ResizeObserver !== 'undefined') {
    const observer = new ResizeObserver(updateSize)
    observer.observe(element)
    onCleanup(() => observer.disconnect())
  } else {
    window.addEventListener('resize', updateSize)
    onCleanup(() => window.removeEventListener('resize', updateSize))
  }
}, { flush: 'post' })

watch(() => [props.show, props.testId] as const, async ([show, id]) => {
  if (!show || !id) {
    test.value = null
    return
  }
  loading.value = true
  error.value = ''
  try {
    test.value = await adminAPI.accounts.getPelicanTest(id)
    tab.value = test.value.html ? 'preview' : 'source'
    fitToWindow.value = true
  } catch (cause) {
    test.value = null
    error.value = extractApiErrorMessage(cause, t('admin.accounts.pelican.loadResultFailed'))
  } finally {
    loading.value = false
  }
}, { immediate: true })

function reloadPreview() { previewKey.value += 1 }

function setZoom(event: Event) {
  manualScale.value = Number((event.target as HTMLInputElement).value) / 100
  fitToWindow.value = false
}

function downloadHTML() {
  if (!test.value?.html) return
  const url = URL.createObjectURL(new Blob([test.value.html], { type: 'text/html;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `pelican-${test.value.account_id}-${test.value.id}.html`
  link.click()
  URL.revokeObjectURL(url)
}
</script>
