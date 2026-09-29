<template>
  <div ref="container" class="relative h-36 w-60 overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-600" :aria-label="t('admin.accounts.pelican.previewColumn')">
    <iframe
      v-if="visible && html"
      :srcdoc="buildPelicanPreviewSource(html, previewNonce)"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      loading="lazy"
      class="pointer-events-none absolute left-1/2 top-1/2 origin-top-left border-0"
      :style="{
        width: `${PELICAN_PREVIEW_WIDTH}px`,
        height: `${PELICAN_PREVIEW_HEIGHT}px`,
        transform: `scale(${inlineScale}) translate(-50%, -50%)`
      }"
      :title="t('admin.accounts.pelican.previewColumn')"
    />
    <div v-else-if="error" class="flex h-full items-center justify-center p-2 text-center text-xs text-red-600">{{ error }}</div>
    <div v-else class="flex h-full items-center justify-center text-xs text-gray-400">{{ t('admin.accounts.pelican.previewLoading') }}</div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { buildPelicanPreviewSource, getPelicanPreviewNonce, PELICAN_PREVIEW_HEIGHT, PELICAN_PREVIEW_WIDTH } from '@/utils/pelicanPreview'

const props = defineProps<{ testId: number }>()
const { t } = useI18n()
const container = ref<HTMLElement | null>(null)
const visible = ref(false)
const html = ref('')
const error = ref('')
const loading = ref(false)
const inlineScale = Math.min(238 / PELICAN_PREVIEW_WIDTH, 142 / PELICAN_PREVIEW_HEIGHT)
const previewNonce = getPelicanPreviewNonce()
let observer: IntersectionObserver | null = null
let requestVersion = 0

watch(() => props.testId, () => {
  requestVersion += 1
  html.value = ''
  error.value = ''
  loading.value = false
  if (visible.value) void loadPreview()
})
watch(visible, visibleNow => {
  if (visibleNow) void loadPreview()
})

async function loadPreview() {
  if (loading.value || html.value || error.value) return
  const version = ++requestVersion
  const testId = props.testId
  loading.value = true
  try {
    const test = await adminAPI.accounts.getPelicanTest(testId)
    if (version !== requestVersion) return
    html.value = test.html
    if (!test.html) error.value = t('admin.accounts.pelican.previewUnavailable')
  } catch {
    if (version === requestVersion) error.value = t('admin.accounts.pelican.loadResultFailed')
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

onMounted(() => {
  if (!container.value) return
  if (typeof IntersectionObserver === 'undefined') {
    visible.value = true
    return
  }
  observer = new IntersectionObserver(entries => {
    visible.value = entries.some(entry => entry.isIntersecting)
  }, { rootMargin: '100px' })
  observer.observe(container.value)
})
onUnmounted(() => {
  requestVersion += 1
  observer?.disconnect()
})
</script>
