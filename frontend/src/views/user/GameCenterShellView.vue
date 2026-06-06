<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-amber-100 via-orange-50 to-rose-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-amber-500/10 dark:via-orange-500/5 dark:to-rose-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">Game Center</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              积分玩法入口
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              游戏中心统一展示签到、Lucky Wheel 与大小单双玩法，所有奖励和结算都只使用积分。
            </p>
          </div>
          <div class="flex flex-wrap gap-3">
            <RouterLink to="/game-center" class="btn btn-secondary">总览</RouterLink>
            <RouterLink to="/game/lucky-wheel" class="btn btn-secondary">Lucky Wheel</RouterLink>
            <RouterLink to="/game/size-bet" class="btn btn-secondary">大小单双</RouterLink>
            <RouterLink to="/game/size-bet/stats" class="btn btn-secondary">统计看板</RouterLink>
          </div>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <RouterLink
          v-for="item in navigationCards"
          :key="item.to"
          :to="item.to"
          class="rounded-3xl border border-gray-100 bg-white p-5 transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-500/30"
        >
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-base font-semibold text-gray-900 dark:text-white">{{ item.title }}</p>
              <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ item.description }}</p>
            </div>
            <Icon name="chevronRight" size="md" class="text-gray-400 dark:text-dark-500" />
          </div>
        </RouterLink>
      </section>

      <RouterView />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'

const navigationCards = [
  {
    to: '/game-center',
    title: '游戏中心总览',
    description: '查看当前积分、签到状态、积分流水和可用玩法。',
  },
  {
    to: '/game/lucky-wheel',
    title: 'Lucky Wheel',
    description: '按日参与转盘抽奖，查看奖池、排行榜和最近开奖记录。',
  },
  {
    to: '/game/size-bet',
    title: '大小单双',
    description: '查看当前局、下注规则、我的下注与当期榜单表现。',
  },
]
</script>
