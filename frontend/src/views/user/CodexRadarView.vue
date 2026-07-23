<template>
  <AppLayout>
    <section class="codex-radar-page" :aria-label="t('codexRadar.title')">
      <div
        v-if="loading"
        class="codex-radar-loading"
        role="status"
        aria-live="polite"
      >
        <span class="codex-radar-spinner" aria-hidden="true"></span>
        <span>{{ t('codexRadar.loading') }}</span>
      </div>
      <iframe
        src="/api/v1/codex-radar/embed"
        class="codex-radar-frame"
        :title="t('codexRadar.title')"
        sandbox="allow-forms allow-scripts"
        referrerpolicy="no-referrer"
        @load="loading = false"
      ></iframe>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const loading = ref(true)
</script>

<style scoped>
.codex-radar-page {
  position: relative;
  width: 100%;
  height: calc(100vh - 64px - 4rem);
  min-height: 32rem;
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 8px;
  background: #0d1420;
}

.codex-radar-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
  background: #0d1420;
}

.codex-radar-loading {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  color: #cbd5e1;
  background: #0d1420;
  font-size: 0.875rem;
}

.codex-radar-spinner {
  width: 1.25rem;
  height: 1.25rem;
  border: 2px solid rgb(100 116 139 / 45%);
  border-top-color: #93c5fd;
  border-radius: 9999px;
  animation: codex-radar-spin 0.8s linear infinite;
}

:global(.dark) .codex-radar-page {
  border-color: rgb(51 65 85);
}

@media (max-width: 767px) {
  .codex-radar-page {
    height: calc(100dvh - 64px - 2rem);
    min-height: 28rem;
  }
}

@keyframes codex-radar-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
