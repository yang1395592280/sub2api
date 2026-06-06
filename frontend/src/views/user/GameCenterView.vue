<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-amber-100 via-orange-50 to-rose-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-amber-500/10 dark:via-orange-500/5 dark:to-rose-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-amber-700 dark:text-amber-300">游戏中心</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              积分内容总览
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              这里汇总每日签到、可玩的积分游戏和最近积分流水。整个页面保持纯积分语义，不涉及任何余额兑换入口。
            </p>
          </div>
          <div class="flex flex-wrap gap-3">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadOverview">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              <span class="ml-2">刷新</span>
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!canCheckin || actionLoading"
              @click="handleCheckin"
            >
              {{ actionLoading && actionType === 'checkin' ? '签到中...' : canCheckin ? '立即签到' : '今日已签到' }}
            </button>
            <button
              v-if="canPlayLuckyBonus"
              type="button"
              class="btn btn-secondary"
              :disabled="actionLoading"
              @click="handleLuckyBonus"
            >
              {{ actionLoading && actionType === 'bonus' ? '加奖中...' : '试试幸运加奖' }}
            </button>
          </div>
        </div>
      </section>

      <div v-if="loading && !overview" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="overview">
        <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">当前积分</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ formatCompactNumber(overview.points) }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">游戏内所有结算均以积分为单位。</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">累计签到</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">
              {{ overview.checkin?.stats.total_checkins ?? 0 }}
            </p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              累计获得 {{ formatCompactNumber(overview.checkin?.stats.total_reward_points ?? 0) }} 积分
            </p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">今日签到状态</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ overview.checkin?.stats.checked_in_today ? '已完成' : '待完成' }}
            </p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              奖励范围 {{ overview.checkin?.min_reward_points ?? 0 }} - {{ overview.checkin?.max_reward_points ?? 0 }} 积分
            </p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">可用玩法</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ enabledCatalogs.length }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">按当前分支契约展示已启用的积分玩法。</p>
          </article>
        </section>

        <section
          v-if="overview.checkin"
          class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div class="max-w-2xl">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">每日签到</p>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">
                每天签到可随机获得积分；如果系统开启幸运加奖，签到后还可能追加额外积分。
              </p>
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">今日结果</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ overview.checkin.today_record ? `${overview.checkin.today_record.reward_points} 积分` : '未签到' }}
                </p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">幸运加奖</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ overview.checkin.bonus_enabled ? `${Math.round(overview.checkin.bonus_success_rate)}% 概率` : '未开启' }}
                </p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">本月签到</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ overview.checkin.stats.checkin_count }}
                </p>
              </div>
            </div>
          </div>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5 flex items-center justify-between gap-3">
              <div>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">可玩游戏</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按游戏目录展示目前开放的积分玩法。</p>
              </div>
              <span class="rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">
                {{ enabledCatalogs.length }} 个可用
              </span>
            </div>

            <div v-if="enabledCatalogs.length" class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <RouterLink
                v-for="catalog in enabledCatalogs"
                :key="catalog.game_key"
                :to="resolveGamePath(catalog.game_key)"
                class="group rounded-3xl border border-gray-100 p-5 transition hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-sm dark:border-dark-700 dark:hover:border-primary-500/30"
              >
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ catalog.name }}</p>
                    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ catalog.subtitle }}</p>
                  </div>
                  <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-300">
                    {{ catalog.default_open_mode }}
                  </span>
                </div>
                <p class="mt-4 line-clamp-3 text-sm leading-6 text-gray-600 dark:text-dark-300">
                  {{ catalog.description }}
                </p>
                <div class="mt-4 flex items-center gap-2 text-sm font-medium text-primary-600 dark:text-primary-300">
                  进入玩法
                  <Icon name="arrowRight" size="sm" class="transition group-hover:translate-x-0.5" />
                </div>
              </RouterLink>
            </div>
            <EmptyState
              v-else
              title="暂无可用玩法"
              description="当前没有启用中的积分游戏，请稍后再来查看。"
            />
          </div>

          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">最近积分流水</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">仅展示游戏中心相关积分变动。</p>
            </div>

            <div v-if="overview.recent_ledger.length" class="space-y-3">
              <div
                v-for="item in overview.recent_ledger.slice(0, 8)"
                :key="item.id"
                class="rounded-2xl border border-gray-100 px-4 py-3 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ item.reason }}</p>
                  <span
                    :class="item.delta_points >= 0
                      ? 'text-emerald-600 dark:text-emerald-300'
                      : 'text-rose-600 dark:text-rose-300'"
                    class="text-sm font-semibold"
                  >
                    {{ item.delta_points >= 0 ? '+' : '' }}{{ item.delta_points }}
                  </span>
                </div>
                <div class="mt-2 flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
                  <span>{{ item.entry_type }}</span>
                  <span>{{ formatDateTime(item.created_at) }}</span>
                </div>
              </div>
            </div>
            <EmptyState
              v-else
              title="暂无流水"
              description="还没有产生游戏中心相关的积分记录。"
            />
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { checkinAPI } from '@/api/checkin'
import { gameCenterAPI } from '@/api/gameCenter'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import type { GameCenterOverview } from '@/types'

const appStore = useAppStore()

const overview = ref<GameCenterOverview | null>(null)
const loading = ref(false)
const actionLoading = ref(false)
const actionType = ref<'checkin' | 'bonus' | null>(null)

const enabledCatalogs = computed(() => overview.value?.catalogs.filter((item) => item.enabled) ?? [])
const canCheckin = computed(() => Boolean(overview.value?.checkin?.enabled && !overview.value?.checkin?.stats.checked_in_today))
const canPlayLuckyBonus = computed(() => Boolean(
  overview.value?.checkin?.enabled &&
  overview.value?.checkin?.bonus_enabled &&
  overview.value?.checkin?.bonus_available,
))

async function loadOverview() {
  loading.value = true
  try {
    overview.value = await gameCenterAPI.getOverview({
      page: 1,
      page_size: 8,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '游戏中心加载失败'))
  } finally {
    loading.value = false
  }
}

async function handleCheckin() {
  actionLoading.value = true
  actionType.value = 'checkin'
  try {
    const result = await checkinAPI.checkin({
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
    appStore.showSuccess(`签到成功，获得 ${result.reward_points} 积分`)
    await loadOverview()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '签到失败'))
  } finally {
    actionLoading.value = false
    actionType.value = null
  }
}

async function handleLuckyBonus() {
  actionLoading.value = true
  actionType.value = 'bonus'
  try {
    const result = await checkinAPI.playLuckyBonus({
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
    appStore.showSuccess(`幸运加奖结果：${result.reward_points} 积分`)
    await loadOverview()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '幸运加奖失败'))
  } finally {
    actionLoading.value = false
    actionType.value = null
  }
}

function resolveGamePath(gameKey: string): string {
  if (gameKey === 'lucky-wheel') return '/game/lucky-wheel'
  if (gameKey === 'size-bet') return '/game/size-bet'
  return '/game-center'
}

onMounted(loadOverview)
</script>
