<template>
  <img v-if="source" :src="source" :alt="alt" class="h-full w-full object-contain" loading="lazy" decoding="async" />
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
const props = defineProps<{ svg: string; alt: string }>()
const source = ref('')
// SVG is a backend-validated image document. Image context disables scripts;
// it is never inserted into the application's DOM or an executable iframe.
watch(() => props.svg, svg => {
  if (source.value) URL.revokeObjectURL(source.value)
  source.value = svg ? URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' })) : ''
}, { immediate: true })
onBeforeUnmount(() => { if (source.value) URL.revokeObjectURL(source.value) })
</script>
