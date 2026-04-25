<template>
  <AppLayout>
    <div class="space-y-4">
      <section class="rounded-3xl border border-slate-200/80 bg-white px-5 py-5 shadow-sm dark:border-white/10 dark:bg-white/5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.25em] text-slate-500 dark:text-slate-300">GAME SHELL</p>
            <h1 class="mt-2 text-2xl font-semibold text-slate-900 dark:text-white">{{ title }}</h1>
          </div>
          <div class="flex flex-wrap gap-2">
            <RouterLink to="/game-center" class="btn btn-secondary">
              {{ t('gameCenter.shell.back') }}
            </RouterLink>
            <a v-if="resolvedGamePath" :href="resolvedGamePath" target="_blank" rel="noreferrer" class="btn btn-primary">
              {{ t('gameCenter.shell.openRaw') }}
            </a>
          </div>
        </div>
      </section>

      <section v-if="resolvedGamePath" class="overflow-hidden rounded-3xl border border-slate-200 bg-white p-3 shadow-sm dark:border-white/10 dark:bg-white/5">
        <iframe :src="resolvedGamePath" class="h-[calc(100vh-16rem)] min-h-[560px] w-full rounded-2xl border border-slate-200 dark:border-white/10" />
      </section>

      <section v-else class="card px-6 py-12">
        <EmptyState
          :title="t('gameCenter.shell.unsupportedTitle')"
          :description="t('gameCenter.shell.unsupportedDescription')"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/common/EmptyState.vue'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const route = useRoute()

const GAME_ROUTE_MAP: Record<string, string> = {
  size_bet: '/game/size-bet',
}

const gameKey = computed(() => String(route.params.gameKey || ''))
const resolvedGamePath = computed(() => GAME_ROUTE_MAP[gameKey.value] ?? '')
const title = computed(() => `${t('gameCenter.shell.titlePrefix')} ${gameKey.value || '--'}`)
</script>
