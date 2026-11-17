<template>
  <component :is="props.embedded ? 'div' : AppLayout">
    <div class="space-y-6">
      <!-- 顶部快捷导航与筹码福利余额栏 -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <RouterLink to="/game-center" class="btn btn-secondary text-sm">
            {{ t('sizeBet.backToGameCenter') }}
          </RouterLink>
          <RouterLink to="/game/size-bet/stats" class="btn btn-secondary text-sm">
            📊 走势与统计
          </RouterLink>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <!-- 筹码积分 -->
          <div class="flex items-center gap-2 rounded-full bg-amber-50 px-4 py-1.5 text-sm font-semibold text-amber-800 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-200 dark:ring-amber-500/20">
            <span>🪙 筹码:</span>
            <span class="text-base font-bold text-amber-600 dark:text-amber-400">{{ formatAmount(pointsBalance ?? 0) }}</span>
            <button
              type="button"
              class="ml-1 rounded-full bg-amber-500 px-2 py-0.5 text-xs text-white hover:bg-amber-600 active:scale-95"
              title="使用福利余额补充筹码"
              @click="showQuickExchange = true"
            >
              +兑入
            </button>
          </div>

          <!-- 可用福利余额 -->
          <div class="rounded-full bg-emerald-50 px-4 py-1.5 text-sm font-semibold text-emerald-800 ring-1 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-200 dark:ring-emerald-500/20">
            <span>🎁 福利:</span>
            <span class="text-base font-bold text-emerald-600 dark:text-emerald-400">¥{{ totalWelfareBalance.toFixed(2) }}</span>
          </div>
        </div>
      </div>

      <!-- 快速福利划转弹窗 (无需离开游戏即可补充筹码) -->
      <div v-if="showQuickExchange" class="card overflow-hidden border-2 border-amber-400/50 bg-gradient-to-br from-amber-50/90 via-white to-orange-50/90 p-5 shadow-lg dark:border-amber-500/30 dark:from-slate-900 dark:via-slate-900 dark:to-amber-950/30">
        <div class="flex items-center justify-between border-b border-amber-200/60 pb-3 dark:border-white/10">
          <div class="flex items-center gap-2">
            <span class="text-lg font-bold text-slate-900 dark:text-white">🪙 快速补充筹码</span>
            <span class="text-xs text-slate-500 dark:text-slate-400">(仅消耗签到/礼遇等福利余额，不支持付费)</span>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="showQuickExchange = false">✕ 关闭</button>
        </div>
        <div class="mt-4 flex flex-wrap items-center gap-3">
          <div class="flex-1 min-w-[200px]">
            <label class="text-xs font-medium text-slate-600 dark:text-slate-300">划转福利金额 (可用 ¥{{ totalWelfareBalance.toFixed(2) }})</label>
            <div class="mt-1 flex items-center gap-2">
              <input v-model.number="quickExchangeAmount" type="number" min="0.01" step="0.01" class="input flex-1" placeholder="金额(元)" />
              <button type="button" class="btn btn-secondary btn-sm" @click="quickExchangeAmount = Number(totalWelfareBalance.toFixed(2))">全部</button>
            </div>
          </div>
          <div class="flex items-end">
            <button
              type="button"
              class="btn btn-primary h-10 px-6 font-bold bg-amber-500 hover:bg-amber-600 border-none text-slate-950"
              :disabled="quickExchangeLoading || quickExchangeAmount <= 0 || quickExchangeAmount > totalWelfareBalance"
              @click="handleQuickExchange"
            >
              {{ quickExchangeLoading ? '划转中...' : `确认换成 ${Math.floor(quickExchangeAmount * 100)} 筹码` }}
            </button>
          </div>
        </div>
      </div>

      <!-- 英雄大看板: 倒计时、状态灯与期数概览 -->
      <section class="overflow-hidden rounded-[28px] border border-amber-200/70 bg-gradient-to-br from-amber-50 via-white to-rose-50 p-6 shadow-sm shadow-amber-100/60 dark:border-amber-500/20 dark:from-slate-900 dark:via-slate-900 dark:to-amber-950/30 dark:shadow-none">
        <div class="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
          <div class="space-y-5">
            <div class="flex items-center gap-2">
              <span
                class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium uppercase tracking-[0.2em]"
                :class="statusBadgeClass"
              >
                <span class="h-2 w-2 rounded-full" :class="statusDotClass"></span>
                {{ statusBadgeLabel }}
              </span>
              <span v-if="!currentView?.enabled" class="text-xs font-bold text-rose-500 bg-rose-50 px-2 py-0.5 rounded-full dark:bg-rose-950/40">
                🛑 游戏当前处于维护中
              </span>
            </div>

            <div>
              <h1 class="text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white flex items-center gap-2">
                <span>🎲</span>
                <span>{{ t('sizeBet.title') }}</span>
              </h1>
              <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.heroSubtitle') }}</p>
            </div>

            <!-- 倒计时展示区 (带动态脉冲警戒光环) -->
            <div class="flex flex-wrap items-end gap-6">
              <div class="relative">
                <p class="text-xs font-medium uppercase tracking-wider text-slate-500 dark:text-slate-400">{{ t('sizeBet.countdownLabel') }}</p>
                <div class="mt-1 flex items-baseline gap-2">
                  <span
                    class="text-6xl font-black tabular-nums transition-colors duration-300"
                    :class="countdownNumberClass"
                  >
                    {{ roundCountdownDisplay }}
                  </span>
                  <span class="text-sm font-semibold text-slate-500">秒</span>
                </div>

                <!-- 紧急倒计时警戒波纹 -->
                <div
                  v-if="isUrgentCountdown"
                  class="absolute -inset-2 rounded-2xl bg-rose-500/10 pointer-events-none animate-ping opacity-60"
                ></div>
              </div>

              <!-- 阶段提示文字 -->
              <div class="pb-1">
                <span class="inline-block rounded-xl px-3 py-1 text-sm font-semibold" :class="stageHintBadgeClass">
                  {{ countdownStageHint }}
                </span>
              </div>
            </div>

            <!-- 选型与投注进度条 -->
            <div class="space-y-2 pt-2">
              <div class="flex items-center justify-between gap-4 text-xs font-medium text-slate-500 dark:text-slate-400">
                <span>{{ t('sizeBet.player.currentSelection') }}</span>
                <span class="text-right font-semibold text-slate-800 dark:text-slate-200">{{ selectionSummary }}</span>
              </div>
              <div class="h-3 overflow-hidden rounded-full bg-white/70 ring-1 ring-slate-200/70 dark:bg-white/10 dark:ring-white/10">
                <div
                  class="h-full rounded-full transition-all duration-1000"
                  :class="progressBarClass"
                  :style="{ width: `${progressWidth}%` }"
                ></div>
              </div>
            </div>
          </div>

          <!-- 右侧卡片矩阵: 当前期号、上期揭秘、安全哈希 -->
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
            <div class="rounded-2xl bg-white/85 p-4 ring-1 ring-slate-200/70 backdrop-blur dark:bg-white/10 dark:ring-white/10 flex items-center justify-between">
              <div>
                <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.dealer.title') }}</p>
                <p class="mt-1 text-2xl font-black text-slate-900 dark:text-white">第 {{ currentRound?.round_no ?? '--' }} 期</p>
              </div>
              <div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-amber-500/10 text-xl font-bold text-amber-600">
                #{{ (currentRound?.round_no ?? 0) % 100 }}
              </div>
            </div>

            <!-- 上期开奖结果 (带数字跳动定格效果) -->
            <div class="rounded-2xl bg-white/85 p-4 ring-1 ring-slate-200/70 backdrop-blur dark:bg-white/10 dark:ring-white/10">
              <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.previousRound.title') }}</p>
              <div class="mt-2 flex items-center gap-3">
                <div
                  v-if="previousRound?.result_number != null"
                  class="flex h-12 w-12 items-center justify-center rounded-2xl text-2xl font-black text-white shadow-md animate-bounce"
                  :class="previousRoundBallClass"
                >
                  {{ previousRound.result_number }}
                </div>
                <div>
                  <p class="text-sm font-bold text-slate-900 dark:text-white">{{ previousRoundSummary }}</p>
                  <p v-if="previousRound?.server_seed" class="text-xs font-mono text-slate-400 truncate max-w-[200px]" :title="previousRound.server_seed">
                    公开种子: {{ previousRound.server_seed }}
                  </p>
                </div>
              </div>
            </div>

            <!-- 公平哈希种子 -->
            <div class="rounded-2xl border border-sky-200/70 bg-gradient-to-br from-sky-50 via-white to-indigo-50 p-4 text-slate-900 shadow-sm shadow-sky-100/70 dark:border-sky-400/20 dark:bg-gradient-to-br dark:from-slate-900 dark:via-slate-900 dark:to-sky-950/40 dark:text-white dark:shadow-none">
              <div class="flex items-center justify-between">
                <p class="text-xs uppercase tracking-[0.22em] text-sky-700/75 dark:text-sky-200/80">{{ t('sizeBet.seedTitle') }}</p>
                <span class="text-xs text-sky-600 font-mono">SHA256 公平保真</span>
              </div>
              <p class="mt-2 break-all text-xs font-mono text-slate-700 dark:text-sky-50">{{ currentRound?.server_seed_hash ?? previousRound?.server_seed_hash ?? '--' }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- 加载与异常状态 -->
      <div v-if="loadState === 'loading'" class="flex justify-center py-16"><LoadingSpinner /></div>
      <section v-else-if="loadState === 'error'" class="card px-6 py-12">
        <EmptyState :title="t('sizeBet.loadError.title')" :description="loadErrorMessage || t('sizeBet.loadError.description')" :action-text="t('common.retry')" @action="loadPage" />
      </section>

      <!-- 游戏维护中提示 (开关关闭时沉浸式提示) -->
      <section v-else-if="!currentView?.enabled" class="card overflow-hidden border-2 border-rose-400/40 bg-gradient-to-br from-rose-50/50 to-white px-6 py-14 text-center dark:from-slate-900 dark:to-rose-950/20">
        <div class="mx-auto flex h-20 w-20 items-center justify-center rounded-3xl bg-rose-100 text-4xl shadow-inner dark:bg-rose-950/50">
          🛑
        </div>
        <h2 class="mt-4 text-2xl font-bold text-slate-900 dark:text-white">比大小游戏正在维护 / 暂时关闭</h2>
        <p class="mx-auto mt-2 max-w-lg text-sm text-slate-500 dark:text-slate-400">
          管理员已临时关闭游戏投注。系统将每 15 秒自动轮询，管理员重新开启后本页面将自动刷新恢复，您可稍后再来体验！
        </p>
        <div class="mt-6 flex justify-center gap-3">
          <RouterLink to="/game-center" class="btn btn-secondary">返回游戏大厅</RouterLink>
          <button type="button" class="btn btn-primary" @click="loadPage">手动刷新检查</button>
        </div>
      </section>

      <!-- 游戏正常主内容 -->
      <div v-else class="space-y-6">
        <!-- 结算弹窗 / 通知条 (中奖/未中奖流光展示) -->
        <section v-if="resultNotice" class="card overflow-hidden border-2 transition-all duration-500" :class="resultCardBorderClass(resultNotice)">
          <div class="flex items-center justify-between border-b px-6 py-4" :class="resultHeaderClass(resultNotice)">
            <div class="flex items-center gap-3">
              <span class="text-2xl">{{ resultNotice.status === 'won' ? '🎉' : '🎲' }}</span>
              <div>
                <h2 class="text-lg font-bold" :class="resultTitleColor(resultNotice)">{{ resultTitleText(resultNotice) }}</h2>
                <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('sizeBet.resultModal.roundLabel', { round: resultNotice.round_no }) }}</p>
              </div>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="resultNotice = null">✕ 知道了</button>
          </div>

          <div class="p-6">
            <div class="rounded-2xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4" :class="resultBannerClass(resultNotice)">
              <div>
                <p class="text-2xl font-black" :class="resultAmountClass(resultNotice)">{{ resultSummary(resultNotice) }}</p>
                <p class="mt-1 text-sm font-medium text-slate-700 dark:text-slate-200">{{ resultMessage(resultNotice) }}</p>
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  {{ t('sizeBet.resultModal.selection', { direction: directionLabel(resultNotice.direction), stake: formatAmount(resultNotice.stake_amount) }) }}
                  · {{ resultDetailLabel(resultNotice) }}
                </p>
              </div>
              <div class="flex items-center gap-2">
                <button type="button" class="btn btn-primary font-bold" @click="resultNotice = null">继续竞猜下一期</button>
              </div>
            </div>
          </div>
        </section>

        <div class="grid gap-6 xl:grid-cols-[1.08fr_0.92fr]">
          <!-- 左侧: 庄家期数信息、往期走势列表、规则说明 -->
          <section class="card overflow-hidden">
            <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
              <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.dealer.title') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.dealer.duration', { seconds: rules?.round_duration_seconds ?? 60, close: rules?.bet_close_offset_seconds ?? 10 }) }}</p>
            </div>
            <div class="space-y-5 px-6 py-6">
              <!-- 赔率与概率看板 -->
              <div class="grid grid-cols-3 gap-3">
                <div class="rounded-2xl border border-blue-200/80 bg-blue-50/60 p-3.5 text-center dark:border-blue-500/20 dark:bg-blue-950/20">
                  <p class="text-xs font-semibold text-blue-700 dark:text-blue-300">小 (1 - 5)</p>
                  <p class="mt-1 text-xl font-black text-slate-900 dark:text-white">{{ currentRound?.odds_small ?? 2.0 }}x</p>
                  <p class="mt-0.5 text-xs text-slate-500">胜率 {{ currentRound?.prob_small ?? 45 }}%</p>
                </div>
                <div class="rounded-2xl border border-purple-200/80 bg-purple-50/60 p-3.5 text-center dark:border-purple-500/20 dark:bg-purple-950/20">
                  <p class="text-xs font-semibold text-purple-700 dark:text-purple-300">中 (6 单点)</p>
                  <p class="mt-1 text-xl font-black text-slate-900 dark:text-white">{{ currentRound?.odds_mid ?? 10.0 }}x</p>
                  <p class="mt-0.5 text-xs text-slate-500">胜率 {{ currentRound?.prob_mid ?? 10 }}%</p>
                </div>
                <div class="rounded-2xl border border-rose-200/80 bg-rose-50/60 p-3.5 text-center dark:border-rose-500/20 dark:bg-rose-950/20">
                  <p class="text-xs font-semibold text-rose-700 dark:text-rose-300">大 (7 - 11)</p>
                  <p class="mt-1 text-xl font-black text-slate-900 dark:text-white">{{ currentRound?.odds_big ?? 2.0 }}x</p>
                  <p class="mt-0.5 text-xs text-slate-500">胜率 {{ currentRound?.prob_big ?? 45 }}%</p>
                </div>
              </div>

              <!-- 往期走势列表 -->
              <div class="rounded-2xl bg-slate-50 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
                <div class="flex items-center justify-between gap-3">
                  <h3 class="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-1.5">
                    <span>📜</span>
                    <span>近期开奖走势</span>
                  </h3>
                  <div class="flex items-center gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="roundsLoading || roundsPage <= 1" @click="changeRoundsPage(roundsPage - 1)">上一页</button>
                    <span class="text-xs text-slate-500">{{ roundsPage }} / {{ roundsPages }}</span>
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="roundsLoading || roundsPage >= roundsPages" @click="changeRoundsPage(roundsPage + 1)">下一页</button>
                  </div>
                </div>

                <div v-if="roundsLoading" class="mt-3 text-sm text-slate-500 py-3 text-center">{{ t('common.loading') }}</div>
                <div v-else-if="!recentRoundsView.items.length" class="mt-3 text-sm text-slate-500 py-3 text-center">{{ t('sizeBet.rounds.empty') }}</div>
                <div v-else class="mt-3 space-y-2">
                  <div
                    v-for="item in recentRoundsView.items"
                    :key="item.id"
                    class="flex items-center justify-between rounded-xl bg-white/90 px-3.5 py-2.5 text-sm ring-1 ring-slate-200/70 dark:bg-white/5 dark:ring-white/10"
                  >
                    <div class="flex items-center gap-3">
                      <span class="font-bold text-slate-900 dark:text-white">第 {{ item.round_no }} 期</span>
                      <span class="text-xs text-slate-400">{{ formatRoundTime(item.settles_at) }}</span>
                    </div>

                    <div class="flex items-center gap-2">
                      <template v-if="item.result_number != null && item.result_direction">
                        <span class="flex h-7 w-7 items-center justify-center rounded-lg text-xs font-black text-white shadow-sm" :class="roundNumberPillClass(item.result_direction)">
                          {{ item.result_number }}
                        </span>
                        <span class="rounded-md px-2 py-0.5 text-xs font-bold" :class="directionBadgeClass(item.result_direction)">
                          {{ directionLabel(item.result_direction) }}
                        </span>
                      </template>
                      <span v-else class="text-xs text-amber-600 dark:text-amber-400 font-medium">开奖摇号中...</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 玩法规则折叠或展开 -->
              <div class="rounded-2xl bg-white p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
                <h3 class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.rules.title') }}</h3>
                <div class="markdown-body prose prose-sm mt-3 max-w-none text-xs text-slate-600 dark:prose-invert dark:text-slate-300" v-html="rulesHtml"></div>
              </div>
            </div>
          </section>

          <!-- 右侧: 投注操作台 (3D 赌场拟物筹码 + 下注方向选择) -->
          <section class="card overflow-hidden">
            <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <h2 class="text-xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <span>🎯</span>
                    <span>竞猜投注区</span>
                  </h2>
                  <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ statusHint }}</p>
                </div>
                <div class="rounded-full px-3 py-1 text-xs font-bold uppercase tracking-wider" :class="isBettingOpen ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/20 dark:text-emerald-300' : 'bg-rose-100 text-rose-800 dark:bg-rose-500/20 dark:text-rose-300'">
                  {{ phaseLabel }}
                </div>
              </div>
            </div>

            <div class="space-y-6 px-6 py-6">
              <!-- 已下注信息回显 -->
              <div v-if="currentBet" class="rounded-2xl border-2 border-emerald-500/30 bg-emerald-50/70 p-4 dark:bg-emerald-950/20">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold uppercase tracking-wider text-emerald-800 dark:text-emerald-300">✅ 您已成功下注本期</span>
                  <span class="rounded-full bg-emerald-200/60 px-2.5 py-0.5 text-xs font-bold text-emerald-900">等待结算</span>
                </div>
                <div class="mt-2 flex items-baseline justify-between">
                  <p class="text-lg font-black text-slate-900 dark:text-white">
                    下注方向: <span class="text-amber-600">{{ directionLabel(currentBet.direction) }}</span>
                  </p>
                  <p class="text-lg font-black text-emerald-600 dark:text-emerald-400">
                    {{ currentBet.stake_amount }} 筹码
                  </p>
                </div>
              </div>

              <!-- 步骤1: 选择竞猜方向 -->
              <div class="space-y-3">
                <label class="text-sm font-bold text-slate-800 dark:text-slate-200">
                  步骤 1: 选择竞猜方向
                </label>
                <div class="grid grid-cols-3 gap-3">
                  <button
                    v-for="option in directionOptions"
                    :key="option.value"
                    type="button"
                    class="group relative overflow-hidden rounded-2xl border-2 p-4 text-center transition-all duration-200 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
                    :class="directionButtonClass(option.value)"
                    :disabled="controlsLocked"
                    @click="selectedDirection = option.value"
                  >
                    <div class="text-2xl font-black">{{ option.icon }}</div>
                    <p class="mt-2 text-lg font-bold">{{ option.label }}</p>
                    <p class="text-xs font-semibold text-slate-500 dark:text-slate-400 mt-0.5">{{ option.range }}</p>
                    <div class="mt-2 inline-block rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-bold text-slate-800 group-hover:bg-amber-100 group-hover:text-amber-900 dark:bg-white/10 dark:text-slate-200">
                      赔率 {{ option.odd }}x
                    </div>
                  </button>
                </div>
              </div>

              <!-- 步骤2: 选择下注筹码 (3D 立体拟物赌场筹码) -->
              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <label class="text-sm font-bold text-slate-800 dark:text-slate-200">
                    步骤 2: 选择下注筹码
                  </label>
                  <span class="text-xs text-slate-400">
                    持有: <strong class="text-amber-600">{{ pointsBalance ?? 0 }}</strong> 筹码
                  </span>
                </div>

                <!-- 拟物立体筹码列表 -->
                <div class="grid grid-cols-4 sm:grid-cols-4 gap-3">
                  <button
                    v-for="stake in allowedStakes"
                    :key="stake"
                    type="button"
                    class="chip-button relative flex flex-col items-center justify-center rounded-2xl p-3 font-black transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-40"
                    :class="chipClass(stake)"
                    :disabled="controlsLocked"
                    @click="selectedStake = stake; customStake = null"
                  >
                    <!-- 筹码内同心环与虚线槽拟物纹理 -->
                    <div class="chip-inner flex flex-col items-center justify-center">
                      <span class="text-xs opacity-75">🪙</span>
                      <span class="text-base font-extrabold tracking-tight">{{ stake }}</span>
                    </div>
                  </button>
                </div>

                <!-- 自定义下注金额输入 -->
                <div class="rounded-2xl border border-dashed border-slate-300 bg-slate-50/70 p-4 dark:border-white/10 dark:bg-white/5">
                  <label class="text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">自定义下注筹码</label>
                  <div class="mt-2 flex items-center gap-3">
                    <input
                      v-model.number="customStake"
                      :disabled="controlsLocked"
                      type="number"
                      inputmode="numeric"
                      class="input w-36 font-bold"
                      :min="customStakeMin"
                      :max="customStakeMax"
                      :placeholder="String(customStakeMin)"
                      @focus="selectedStake = null"
                    />
                    <p class="text-xs text-slate-500 dark:text-slate-400">范围: {{ customStakeMin }} ~ {{ customStakeMax }}</p>
                  </div>
                </div>
              </div>

              <!-- 下注确认大按钮 -->
              <div>
                <button
                  type="button"
                  class="btn btn-primary h-14 w-full justify-center text-lg font-black shadow-lg transition-transform active:scale-[0.98]"
                  :class="submitButtonClass"
                  :disabled="submitDisabled"
                  @click="submitBet"
                >
                  <span v-if="submitting">🚀 正在下注...</span>
                  <span v-else-if="currentBet">✅ 本期已成功下注，等待开奖</span>
                  <span v-else-if="!isBettingOpen">⏳ 封盘开奖中 / 稍候开放</span>
                  <span v-else>
                    🎲 确认下注 ({{ selectedDirection ? directionLabel(selectedDirection) : '未选方向' }} · {{ effectiveStake ?? 0 }} 筹码)
                  </span>
                </button>
              </div>
            </div>
          </section>
        </div>

        <!-- 我的竞猜历史记录列表 -->
        <section class="card overflow-hidden">
          <div class="flex items-center justify-between gap-4 border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <div>
              <h2 class="text-xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <span>📋</span>
                <span>我的投注记录</span>
              </h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.history.subtitle') }}</p>
            </div>
            <button v-if="recentHistory.length > visibleHistory.length" type="button" class="btn btn-secondary btn-sm" @click="showAllHistory = !showAllHistory">
              {{ t(showAllHistory ? 'sizeBet.history.toggleLess' : 'sizeBet.history.toggleMore') }}
            </button>
          </div>

          <div class="space-y-3 px-6 py-6">
            <div v-if="historyRefreshError" class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100 flex items-center justify-between">
              <p>{{ historyRefreshError }}</p>
              <button type="button" class="btn btn-secondary btn-sm" @click="retryHistory">{{ t('sizeBet.history.refreshRetry') }}</button>
            </div>

            <p v-if="!recentHistory.length" class="text-sm text-slate-500 dark:text-slate-400 py-6 text-center">暂无投注记录，快去下注第一局吧！</p>
            <div
              v-for="item in visibleHistory"
              :key="item.bet_id"
              class="rounded-2xl bg-slate-50/90 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-sm font-bold text-slate-900 dark:text-white">第 {{ item.round_no }} 期</p>
                  <p class="mt-1 text-xs font-semibold" :class="historyStatusColor(item.status)">{{ historyStatusLabel(item.status) }}</p>
                </div>
                <p class="text-base font-black" :class="historyAmountClass(item)">{{ historyAmountLabel(item) }}</p>
              </div>

              <div class="mt-3 grid gap-2 text-sm text-slate-600 dark:text-slate-300 md:grid-cols-2">
                <p>下注方向: <strong class="text-slate-900 dark:text-white">{{ directionLabel(item.direction) }}</strong> (投注 {{ item.stake_amount }} 筹码)</p>
                <p>开奖情况: {{ historyResultLabel(item) }}</p>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </component>
</template>

<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { gameCenterAPI } from '@/api/gameCenter'
import { sizeBetAPI } from '@/api/sizeBet'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { SizeBetCurrentRoundView, SizeBetDirection, SizeBetHistoryItem, SizeBetPhase, SizeBetRoundsView, SizeBetRulesView, SizeBetStatus } from '@/types/sizeBet'

const props = withDefaults(defineProps<{
  embedded?: boolean
}>(), {
  embedded: false,
})

type LoadState = 'loading' | 'ready' | 'error'
type HistoryRefreshMode = 'background' | 'manual'
const TICK_MS = 1000
const HISTORY_PAGE_SIZE = 10
const VISIBLE_HISTORY_COUNT = 5
const MAINTENANCE_POLL_MS = 15000
const ROUND_SYNC_MS = 3000
marked.setOptions({ breaks: true, gfm: true })
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loadState = ref<LoadState>('loading')
const loadErrorMessage = ref('')
const submitting = ref(false)
const currentView = ref<SizeBetCurrentRoundView | null>(null)
const rules = ref<SizeBetRulesView | null>(null)
const selectedDirection = ref<SizeBetDirection | null>(null)
const selectedStake = ref<number | null>(null)
const customStake = ref<number | null>(null)
const showAllHistory = ref(false)
const syncedServerMs = ref<number | null>(null)
const syncedClientMs = ref<number | null>(null)
const clientNowMs = ref(Date.now())
const lastRoundId = ref<number | null>(null)
const lastAutoSyncAt = ref(0)
const lastSeenSettledBetId = ref<number | null>(null)
const historyRefreshError = ref('')
const recentHistory = ref<SizeBetHistoryItem[]>([])
const resultNotice = ref<SizeBetHistoryItem | null>(null)
const roundsPage = ref(1)
const roundsLoading = ref(false)
const recentRoundsView = ref<SizeBetRoundsView>({ items: [], total: 0, page: 1, page_size: 5, pages: 1 })
const pointsBalance = ref<number | null>(authStore.user?.points ?? null)

// 快速兑换相关
const showQuickExchange = ref(false)
const quickExchangeAmount = ref(1)
const quickExchangeLoading = ref(false)

let tickTimer: number | null = null
let syncInFlight = false
let historySyncInFlight = false

const currentRound = computed(() => currentView.value?.round ?? null)
const currentBet = computed(() => currentView.value?.my_bet ?? null)
const previousRound = computed(() => currentView.value?.previous_round ?? null)
const allowedStakes = computed(() => currentRound.value?.allowed_stakes ?? rules.value?.allowed_stakes ?? [10, 50, 100, 500])
const customStakeMin = computed(() => rules.value?.custom_stake_min ?? 1)
const customStakeMax = computed(() => rules.value?.custom_stake_max ?? 9999)
const visibleHistory = computed(() => showAllHistory.value ? recentHistory.value : recentHistory.value.slice(0, VISIBLE_HISTORY_COUNT))
const roundsPages = computed(() => Math.max(1, recentRoundsView.value.pages || 1))
const estimatedServerNowMs = computed(() => syncedServerMs.value == null || syncedClientMs.value == null ? clientNowMs.value : syncedServerMs.value + (clientNowMs.value - syncedClientMs.value))
const closeCountdownSeconds = computed(() => secondsUntil(currentRound.value?.bet_closes_at, currentRound.value?.bet_countdown_seconds ?? 0))
const roundCountdownSeconds = computed(() => secondsUntil(currentRound.value?.settles_at, currentRound.value?.countdown_seconds ?? 0))

const totalWelfareBalance = computed(() => {
  const u = authStore.user
  if (!u) return 0
  if (u.welfare_balance != null && u.welfare_balance > 0) return u.welfare_balance
  return Number(((u.checkin_balance ?? 0) + (u.liyu_balance ?? 0) + (u.gift_balance ?? 0)).toFixed(4))
})

const phase = computed<SizeBetPhase>(() => {
  if (!currentView.value?.enabled) return 'maintenance'
  if (currentView.value?.phase === 'preparing') return 'preparing'
  if (!currentRound.value) return currentView.value?.phase ?? 'maintenance'
  return closeCountdownSeconds.value > 0 ? 'betting' : 'closed'
})

const isUrgentCountdown = computed(() => phase.value === 'betting' && closeCountdownSeconds.value > 0 && closeCountdownSeconds.value <= 10)

const statusBadgeLabel = computed(() => {
  if (loadState.value === 'loading') return t('common.loading')
  if (loadState.value === 'error') return t('sizeBet.loadError.badge')
  if (!currentView.value?.enabled) return '维护中'
  if (phase.value === 'betting') return '投注中'
  if (phase.value === 'closed') return '封盘中'
  return '准备中'
})

const statusBadgeClass = computed(() => {
  if (!currentView.value?.enabled) return 'bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-300'
  if (phase.value === 'betting') return 'bg-emerald-100 text-emerald-800 ring-1 ring-emerald-300 dark:bg-emerald-950/40 dark:text-emerald-300'
  if (phase.value === 'closed') return 'bg-amber-100 text-amber-800 ring-1 ring-amber-300 dark:bg-amber-950/40 dark:text-amber-300'
  return 'bg-slate-100 text-slate-700 dark:bg-white/10 dark:text-slate-300'
})

const statusDotClass = computed(() => {
  if (!currentView.value?.enabled) return 'bg-rose-500'
  if (phase.value === 'betting') return 'bg-emerald-500 animate-pulse'
  if (phase.value === 'closed') return 'bg-amber-500 animate-ping'
  return 'bg-slate-400'
})

const countdownNumberClass = computed(() => {
  if (!currentView.value?.enabled) return 'text-slate-400'
  if (phase.value === 'closed') return 'text-rose-500 animate-pulse'
  if (isUrgentCountdown.value) return 'text-amber-500 animate-bounce'
  return 'text-slate-900 dark:text-white'
})

const stageHintBadgeClass = computed(() => {
  if (!currentView.value?.enabled) return 'bg-slate-100 text-slate-500 dark:bg-white/10 dark:text-slate-400'
  if (phase.value === 'betting') {
    return isUrgentCountdown.value
      ? 'bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-300'
      : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'
  }
  return 'bg-amber-100 text-amber-800 dark:bg-amber-950/40 dark:text-amber-300'
})

const phaseLabel = computed(() => {
  if (!currentView.value?.enabled) return '维护状态'
  return t(`sizeBet.phase.${phase.value}`)
})

const roundCountdownDisplay = computed(() => {
  if (!currentView.value?.enabled) return '--'
  if (phase.value === 'preparing') return '--'
  return currentRound.value ? roundCountdownSeconds.value : '--'
})

const countdownStageHint = computed(() => {
  if (!currentView.value?.enabled) return '游戏暂时维护，无法下注'
  if (phase.value === 'betting') return `距离封盘还有 ${closeCountdownSeconds.value} 秒`
  if (phase.value === 'closed') return '已封盘，正在紧张开奖中...'
  return '准备中...'
})

const isBettingOpen = computed(() => loadState.value === 'ready' && currentView.value?.enabled === true && !!currentRound.value && closeCountdownSeconds.value > 0)
const controlsLocked = computed(() => submitting.value || !isBettingOpen.value || !!currentBet.value)

const effectiveStake = computed(() => {
  if (customStake.value != null && customStake.value > 0) return customStake.value
  return selectedStake.value
})

const submitDisabled = computed(() => {
  return controlsLocked.value || !selectedDirection.value || effectiveStake.value == null || effectiveStake.value < customStakeMin.value || effectiveStake.value > customStakeMax.value
})

const progressWidth = computed(() => {
  const duration = rules.value?.round_duration_seconds ?? 60
  if (duration <= 0 || !currentRound.value) return 0
  return Math.max(0, Math.min(100, (roundCountdownSeconds.value / duration) * 100))
})

const progressBarClass = computed(() => {
  if (!currentView.value?.enabled) return 'bg-slate-300'
  if (isUrgentCountdown.value) return 'bg-gradient-to-r from-amber-500 to-rose-500'
  return 'bg-gradient-to-r from-emerald-400 via-teal-400 to-cyan-500'
})

const previousRoundSummary = computed(() => {
  if (!previousRound.value) return '暂无往期记录'
  if (previousRound.value.result_number != null && previousRound.value.result_direction) {
    return `第 ${previousRound.value.round_no} 期 开奖 [ ${previousRound.value.result_number} ] - ${directionLabel(previousRound.value.result_direction)}`
  }
  return `第 ${previousRound.value.round_no} 期 正在摇号中`
})

const previousRoundBallClass = computed(() => {
  const dir = previousRound.value?.result_direction
  if (dir === 'small') return 'bg-gradient-to-br from-blue-500 to-blue-700'
  if (dir === 'mid') return 'bg-gradient-to-br from-purple-500 to-purple-700'
  if (dir === 'big') return 'bg-gradient-to-br from-rose-500 to-rose-700'
  return 'bg-slate-600'
})

const selectionSummary = computed(() => {
  if (!selectedDirection.value && effectiveStake.value == null) return '尚未选择下注项'
  const dir = selectedDirection.value ? directionLabel(selectedDirection.value) : '未选方向'
  const stake = effectiveStake.value ? `${effectiveStake.value} 筹码` : '未定额度'
  return `${dir} · ${stake}`
})

const statusHint = computed(() => {
  if (!currentView.value?.enabled) return '管理员暂时关闭了比大小竞猜'
  if (currentBet.value) return '本期已下注完成，等待开奖揭晓'
  if (phase.value === 'betting') return '请在倒计时结束前选择方向和筹码下注'
  return '当前已封盘停止下注，即将开出最新结果'
})

const directionOptions = computed(() => [
  { value: 'small' as SizeBetDirection, icon: '🔵', label: '小 (Small)', range: '开奖 1 ~ 5', odd: currentRound.value?.odds_small ?? 2.0 },
  { value: 'mid' as SizeBetDirection, icon: '🟣', label: '中 (Mid)', range: '开奖 单点 6', odd: currentRound.value?.odds_mid ?? 10.0 },
  { value: 'big' as SizeBetDirection, icon: '🔴', label: '大 (Big)', range: '开奖 7 ~ 11', odd: currentRound.value?.odds_big ?? 2.0 },
])

const rulesHtml = computed(() => {
  const content = rules.value?.rules_markdown || '本游戏每60秒一期。开出 1-5 为小，6 为中，7-11 为大。公平开奖，仅限娱乐。'
  return DOMPurify.sanitize(marked.parse(content) as string)
})

const submitButtonClass = computed(() => {
  if (currentBet.value) return 'bg-emerald-600 border-none hover:bg-emerald-700'
  if (!isBettingOpen.value) return 'bg-slate-400 border-none opacity-80 cursor-not-allowed'
  return 'bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-600 hover:to-orange-600 border-none text-slate-950'
})

function directionButtonClass(dir: SizeBetDirection): string {
  const isSelected = selectedDirection.value === dir
  if (isSelected) {
    if (dir === 'small') return 'border-blue-500 bg-blue-500/15 text-blue-900 shadow-md ring-2 ring-blue-400 dark:border-blue-400 dark:text-blue-200'
    if (dir === 'mid') return 'border-purple-500 bg-purple-500/15 text-purple-900 shadow-md ring-2 ring-purple-400 dark:border-purple-400 dark:text-purple-200'
    return 'border-rose-500 bg-rose-500/15 text-rose-900 shadow-md ring-2 ring-rose-400 dark:border-rose-400 dark:text-rose-200'
  }
  return 'border-slate-200 bg-white text-slate-700 hover:border-amber-300 dark:border-white/10 dark:bg-white/5 dark:text-slate-300'
}

function chipClass(stake: number): string {
  const isSelected = selectedStake.value === stake
  let baseColor = 'border-slate-200 bg-slate-50 text-slate-700 dark:border-white/10 dark:bg-white/5 dark:text-slate-200'
  if (stake <= 10) baseColor = 'border-blue-200 bg-gradient-to-br from-blue-50 to-white text-blue-800 dark:border-blue-500/30 dark:from-slate-800 dark:to-blue-950/30 dark:text-blue-200'
  else if (stake <= 50) baseColor = 'border-rose-200 bg-gradient-to-br from-rose-50 to-white text-rose-800 dark:border-rose-500/30 dark:from-slate-800 dark:to-rose-950/30 dark:text-rose-200'
  else if (stake <= 100) baseColor = 'border-emerald-200 bg-gradient-to-br from-emerald-50 to-white text-emerald-800 dark:border-emerald-500/30 dark:from-slate-800 dark:to-emerald-950/30 dark:text-emerald-200'
  else baseColor = 'border-amber-300 bg-gradient-to-br from-amber-50 to-amber-100 text-amber-900 dark:border-amber-500/40 dark:from-slate-800 dark:to-amber-950/30 dark:text-amber-200'

  if (isSelected) {
    return `${baseColor} ring-4 ring-amber-400/80 -translate-y-1 shadow-lg font-black scale-105`
  }
  return `${baseColor} hover:-translate-y-0.5 hover:shadow`
}

function roundNumberPillClass(dir?: string): string {
  if (dir === 'small') return 'bg-blue-600'
  if (dir === 'mid') return 'bg-purple-600'
  if (dir === 'big') return 'bg-rose-600'
  return 'bg-slate-600'
}

function directionBadgeClass(dir?: string): string {
  if (dir === 'small') return 'bg-blue-100 text-blue-800 dark:bg-blue-500/20 dark:text-blue-300'
  if (dir === 'mid') return 'bg-purple-100 text-purple-800 dark:bg-purple-500/20 dark:text-purple-300'
  if (dir === 'big') return 'bg-rose-100 text-rose-800 dark:bg-rose-500/20 dark:text-rose-300'
  return 'bg-slate-100 text-slate-700 dark:bg-white/10 dark:text-slate-300'
}

function directionLabel(dir?: string | null): string {
  if (dir === 'small') return '小 (1-5)'
  if (dir === 'mid') return '中 (6)'
  if (dir === 'big') return '大 (7-11)'
  return dir || '--'
}

function formatAmount(value: number): string {
  return new Intl.NumberFormat().format(Math.trunc(value))
}

function formatRoundTime(val?: string): string {
  if (!val) return '--:--'
  const d = new Date(val)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

function secondsUntil(isoDate?: string | null, fallbackSeconds = 0): number {
  if (!isoDate) return Math.max(0, fallbackSeconds)
  const targetMs = new Date(isoDate).getTime()
  if (Number.isNaN(targetMs)) return Math.max(0, fallbackSeconds)
  return Math.max(0, Math.ceil((targetMs - estimatedServerNowMs.value) / 1000))
}

function syncClock(serverTimeStr?: string) {
  if (!serverTimeStr) return
  const sMs = new Date(serverTimeStr).getTime()
  if (!Number.isNaN(sMs)) {
    syncedServerMs.value = sMs
    syncedClientMs.value = Date.now()
  }
}

function syncSelection(view: SizeBetCurrentRoundView | null) {
  const stakes = view?.round?.allowed_stakes ?? rules.value?.allowed_stakes ?? []
  if (view?.my_bet) {
    selectedDirection.value = view.my_bet.direction
    selectedStake.value = view.my_bet.stake_amount
    customStake.value = null
  } else if (view?.round?.id !== lastRoundId.value) {
    if (!selectedStake.value || !stakes.includes(selectedStake.value)) {
      selectedStake.value = stakes[0] ?? 10
    }
  }
  lastRoundId.value = view?.round?.id ?? null
}

async function refreshPointsBalance() {
  try {
    const overview = await gameCenterAPI.getOverview()
    pointsBalance.value = overview.points
    if (authStore.user) {
      authStore.user.points = overview.points
    }
  } catch {
    // ignore
  }
}

function maybeAutoSync(intervalMs: number) {
  if (syncInFlight || Date.now() - lastAutoSyncAt.value < intervalMs) return
  void syncCurrent(true).catch(() => undefined)
}

function maybeOpenResultModal(items: SizeBetHistoryItem[]) {
  const latest = items.find((item) => item.status !== 'placed')
  if (!latest || latest.bet_id === lastSeenSettledBetId.value) return
  lastSeenSettledBetId.value = latest.bet_id
  resultNotice.value = latest
}

async function loadRounds(page = roundsPage.value) {
  roundsLoading.value = true
  try {
    const response = await sizeBetAPI.getRounds(page, 5)
    recentRoundsView.value = response
    roundsPage.value = response.page
  } finally {
    roundsLoading.value = false
  }
}

async function loadHistory(mode: HistoryRefreshMode = 'background') {
  if (historySyncInFlight) return
  historySyncInFlight = true
  try {
    const response = await sizeBetAPI.getHistory(1, HISTORY_PAGE_SIZE)
    recentHistory.value = response.items
    historyRefreshError.value = ''
    maybeOpenResultModal(response.items)
  } catch (error: any) {
    if (mode === 'manual') appStore.showError(error?.message || '获取投注历史失败')
  } finally {
    historySyncInFlight = false
  }
}

async function syncCurrent(silent = true) {
  if (syncInFlight) return
  syncInFlight = true
  lastAutoSyncAt.value = Date.now()
  try {
    const nextView = await sizeBetAPI.getCurrent()
    currentView.value = nextView
    syncClock(nextView.server_time)
    syncSelection(nextView)
    loadState.value = 'ready'
    loadErrorMessage.value = ''
    await loadHistory()
    await refreshPointsBalance()
  } catch (error: any) {
    if (!silent) appStore.showError(error?.message || t('common.error'))
  } finally {
    syncInFlight = false
  }
}

async function loadPage() {
  loadState.value = 'loading'
  loadErrorMessage.value = ''
  try {
    const [view, rulesView] = await Promise.all([sizeBetAPI.getCurrent(), sizeBetAPI.getRules()])
    rules.value = rulesView
    currentView.value = view
    syncClock(view.server_time)
    syncSelection(view)
    lastAutoSyncAt.value = Date.now()
    loadState.value = 'ready'
    await loadHistory()
    await loadRounds()
    void refreshPointsBalance()
  } catch (error: any) {
    currentView.value = null
    lastRoundId.value = null
    loadState.value = 'error'
    loadErrorMessage.value = error?.message || t('sizeBet.loadError.description')
  }
}

function changeRoundsPage(page: number) {
  if (page < 1 || page > roundsPages.value || roundsLoading.value) return
  void loadRounds(page)
}

function tick() {
  clientNowMs.value = Date.now()
  if (loadState.value !== 'ready') return
  if (currentView.value?.enabled === false) return maybeAutoSync(MAINTENANCE_POLL_MS)
  if (!currentRound.value || roundCountdownSeconds.value <= 0) maybeAutoSync(ROUND_SYNC_MS)
}

function buildIdempotencyKey() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `size-bet-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function retryHistory() {
  void loadHistory('manual')
}

async function submitBet() {
  if (!selectedDirection.value) return appStore.showError(t('sizeBet.player.selectDirection'))
  if (effectiveStake.value == null || !currentRound.value) return appStore.showError(t('sizeBet.player.selectStake'))
  if (effectiveStake.value < customStakeMin.value || effectiveStake.value > customStakeMax.value) {
    return appStore.showError(`下注筹码需在 ${customStakeMin.value} 至 ${customStakeMax.value} 之间`)
  }
  const currentPts = pointsBalance.value ?? 0
  if (currentPts < effectiveStake.value) {
    showQuickExchange.value = true
    return appStore.showWarning('当前筹码不足，请先划转福利余额兑入筹码！')
  }

  submitting.value = true
  try {
    const bet = await sizeBetAPI.placeBet({
      round_id: currentRound.value.id,
      direction: selectedDirection.value,
      stake_amount: effectiveStake.value,
      idempotency_key: buildIdempotencyKey()
    })
    if (currentView.value) currentView.value.my_bet = bet
    syncSelection(currentView.value)
    await loadHistory()
    void refreshPointsBalance()
    appStore.showSuccess('下注成功！祝您好运连连！')
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    submitting.value = false
  }
}

async function handleQuickExchange() {
  if (quickExchangeAmount.value <= 0 || quickExchangeAmount.value > totalWelfareBalance.value) return
  quickExchangeLoading.value = true
  try {
    await gameCenterAPI.exchangeBalanceToPoints({ amount: Number(quickExchangeAmount.value) })
    appStore.showSuccess(`成功划转 ¥${quickExchangeAmount.value} 为 ${Math.floor(quickExchangeAmount.value * 100)} 筹码！`)
    await authStore.refreshUser?.()
    await refreshPointsBalance()
    showQuickExchange.value = false
  } catch (error: any) {
    appStore.showError(error?.message || '兑入筹码失败')
  } finally {
    quickExchangeLoading.value = false
  }
}

// 结果通知卡片辅助样式
function resultCardBorderClass(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'border-amber-400 bg-amber-50/20 shadow-amber-200/50'
  if (item.status === 'lost') return 'border-slate-300 bg-slate-50/20'
  return 'border-sky-300 bg-sky-50/20'
}

function resultHeaderClass(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'bg-amber-100/60 dark:bg-amber-950/40 border-amber-200'
  if (item.status === 'lost') return 'bg-slate-100 dark:bg-white/5 border-slate-200'
  return 'bg-sky-100/60 dark:bg-sky-950/40 border-sky-200'
}

function resultTitleColor(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'text-amber-800 dark:text-amber-200'
  return 'text-slate-800 dark:text-white'
}

function resultTitleText(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return '恭喜中奖！胜局已定！'
  if (item.status === 'lost') return '遗憾未中，下期再战！'
  return '本局已退款'
}

function resultBannerClass(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'bg-gradient-to-r from-amber-100/80 to-orange-100/80 dark:from-amber-950/40 dark:to-orange-950/40 text-amber-950 dark:text-amber-100'
  if (item.status === 'lost') return 'bg-slate-100 dark:bg-white/5 text-slate-800 dark:text-slate-200'
  return 'bg-sky-50 dark:bg-sky-950/30 text-sky-800 dark:text-sky-200'
}

function resultAmountClass(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'text-amber-600 dark:text-amber-400'
  if (item.status === 'lost') return 'text-slate-500 dark:text-slate-400'
  return 'text-sky-600'
}

function resultSummary(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return `+${formatAmount(item.net_result_amount)} 筹码 胜出！`
  if (item.status === 'lost') return `-${formatAmount(item.stake_amount)} 筹码`
  return '已全额原路退还'
}

function resultMessage(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return `精准猜中！赢得奖金 ${formatAmount(item.payout_amount)} 筹码！`
  if (item.status === 'lost') return '开奖数字与预测方向不同，下期好运在等着您！'
  return '本期异常或平局，已自动退回所有下注筹码。'
}

function resultDetailLabel(item: SizeBetHistoryItem): string {
  if (item.result_number != null && item.result_direction) {
    return `开奖结果: [ ${item.result_number} ] - ${directionLabel(item.result_direction)}`
  }
  return '正在核验开奖数据...'
}

function historyStatusLabel(status: SizeBetStatus): string {
  if (status === 'placed') return '已投注·等待开奖'
  if (status === 'won') return '胜出'
  if (status === 'lost') return '未中'
  return '已退款'
}

function historyStatusColor(status: SizeBetStatus): string {
  if (status === 'won') return 'text-amber-600 dark:text-amber-400'
  if (status === 'placed') return 'text-emerald-600 dark:text-emerald-400'
  if (status === 'lost') return 'text-slate-500'
  return 'text-sky-600'
}

function historyAmountClass(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return 'text-amber-600 dark:text-amber-400'
  if (item.status === 'lost') return 'text-slate-500'
  return 'text-slate-700 dark:text-slate-300'
}

function historyAmountLabel(item: SizeBetHistoryItem): string {
  if (item.status === 'won') return `+${formatAmount(item.net_result_amount)}`
  if (item.status === 'lost') return `-${formatAmount(item.stake_amount)}`
  if (item.status === 'placed') return `${formatAmount(item.stake_amount)} 筹码`
  return '0 (退还)'
}

function historyResultLabel(item: SizeBetHistoryItem): string {
  if (item.result_number != null && item.result_direction) {
    return `开奖: [ ${item.result_number} ] ${directionLabel(item.result_direction)}`
  }
  return '等待期数揭晓'
}

onMounted(() => {
  void loadPage()
  tickTimer = window.setInterval(tick, TICK_MS)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') void syncCurrent(true)
  })
})

onBeforeUnmount(() => {
  if (tickTimer !== null) window.clearInterval(tickTimer)
})
</script>

<style scoped>
.chip-button {
  background-size: 200% 200%;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1);
}
.chip-button:hover {
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -4px rgba(0, 0, 0, 0.1);
}
.chip-inner {
  border: 2px dashed rgba(160, 160, 160, 0.35);
  border-radius: 9999px;
  width: 100%;
  padding: 8px 4px;
}
</style>
