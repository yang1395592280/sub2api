<template>
  <div class="card overflow-hidden">
    <div class="border-b border-gray-100 bg-gradient-to-r from-amber-50 via-orange-50 to-rose-50 px-6 py-5 dark:border-dark-700 dark:from-amber-500/10 dark:via-orange-500/10 dark:to-rose-500/10">
      <div class="flex items-start justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-amber-700 dark:text-amber-300">游戏中心</p>
          <h2 class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">积分玩法总览</h2>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
            每日签到、转盘和大小单双游戏都基于积分结算，不涉及余额兑换。
          </p>
        </div>
        <div class="rounded-2xl bg-white/80 p-3 text-amber-600 shadow-sm ring-1 ring-amber-100 dark:bg-dark-900/70 dark:text-amber-300 dark:ring-amber-500/20">
          <Icon name="sparkles" size="lg" />
        </div>
      </div>
    </div>

    <div class="space-y-5 p-6">
      <div class="grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-amber-50 p-4 dark:bg-amber-500/10">
          <p class="text-xs font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">当前积分</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatCompactNumber(overview.points) }}
          </p>
        </div>
        <div class="rounded-2xl bg-emerald-50 p-4 dark:bg-emerald-500/10">
          <p class="text-xs font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">签到累计</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatCompactNumber(overview.checkin?.stats.total_reward_points ?? 0) }}
          </p>
        </div>
      </div>

      <div
        v-if="overview.checkin"
        class="rounded-2xl border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/70"
      >
        <div class="flex items-center justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">今日签到</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              奖励范围 {{ overview.checkin.min_reward_points }} - {{ overview.checkin.max_reward_points }} 积分
            </p>
          </div>
          <span
            :class="overview.checkin.stats.checked_in_today
              ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
              : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'"
            class="inline-flex rounded-full px-3 py-1 text-xs font-medium"
          >
            {{ overview.checkin.stats.checked_in_today ? '今日已签到' : '待签到' }}
          </span>
        </div>
      </div>

      <div>
        <div class="mb-3 flex items-center justify-between gap-3">
          <p class="text-sm font-medium text-gray-900 dark:text-white">可用玩法</p>
          <RouterLink
            to="/game-center"
            class="inline-flex items-center gap-1 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200"
          >
            查看全部
            <Icon name="arrowRight" size="sm" />
          </RouterLink>
        </div>
        <div class="space-y-2">
          <RouterLink
            v-for="game in previewGames"
            :key="game.game_key"
            :to="resolveGamePath(game.game_key)"
            class="flex items-center justify-between rounded-2xl border border-gray-100 px-4 py-3 transition hover:border-primary-200 hover:bg-primary-50/60 dark:border-dark-700 dark:hover:border-primary-500/30 dark:hover:bg-primary-500/10"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ game.name }}</p>
              <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ game.subtitle }}</p>
            </div>
            <Icon name="chevronRight" size="sm" class="text-gray-400 dark:text-dark-500" />
          </RouterLink>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import { formatCompactNumber } from '@/utils/format'
import type { GameCenterOverview } from '@/types'

interface Props {
  overview: GameCenterOverview
}

const props = defineProps<Props>()

const previewGames = props.overview.catalogs.filter((item) => item.enabled).slice(0, 3)

function resolveGamePath(gameKey: string): string {
  if (gameKey === 'lucky-wheel') return '/game/lucky-wheel'
  if (gameKey === 'size-bet') return '/game/size-bet'
  return '/game-center'
}
</script>
