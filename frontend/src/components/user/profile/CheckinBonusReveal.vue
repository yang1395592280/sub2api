<template>
  <BaseDialog
    :show="phase !== null"
    :title="t('wallet.bonusResultTitle')"
    width="narrow"
    :z-index="70"
    :show-close-button="revealed"
    :close-on-escape="revealed"
    @close="emit('close')"
  >
    <div
      class="bonus-stage"
      :class="`bonus-stage--${phase}`"
      data-testid="bonus-reveal"
      :data-phase="phase"
      aria-live="polite"
      :aria-busy="!revealed"
    >
      <!-- Ambient & Outcome Effects -->
      <div class="bonus-ambient" aria-hidden="true">
        <div class="bonus-halo"></div>
        <div v-if="phase === 'win'" class="bonus-rays"></div>
        <div v-if="revealed" class="bonus-shockwave"></div>
        <div v-if="phase === 'win'" class="bonus-shockwave bonus-shockwave--secondary"></div>
        <div v-if="phase === 'lose'" class="bonus-lose-flash"></div>
      </div>

      <!-- Win Celebration Particles: radial sparks + falling confetti -->
      <div v-if="phase === 'win'" class="bonus-burst" data-testid="bonus-win-effects" aria-hidden="true">
        <i
          v-for="n in 16"
          :key="`spark-${n}`"
          class="bonus-spark"
          :style="sparkStyle(n)"
        ></i>
      </div>
      <div v-if="phase === 'win'" class="bonus-confetti" aria-hidden="true">
        <i
          v-for="n in 26"
          :key="`confetti-${n}`"
          :style="confettiStyle(n)"
        ></i>
      </div>

      <!-- Lose Falling Shards -->
      <div v-if="phase === 'lose'" class="bonus-shards" data-testid="bonus-lose-effects" aria-hidden="true">
        <i
          v-for="n in 14"
          :key="`shard-${n}`"
          :style="shardStyle(n)"
        ></i>
      </div>

      <!-- Top Outcome Pill -->
      <div class="bonus-pill-wrap">
        <span
          class="bonus-status-pill"
          :class="`bonus-status-pill--${phase}`"
          data-testid="bonus-status-badge"
        >
          <span v-if="isDrawing" class="bonus-pulse-dot" aria-hidden="true"></span>
          <Icon v-else :name="phase === 'win' ? 'trophy' : 'xCircle'" size="sm" />
          <span>{{ statusBadgeText }}</span>
        </span>
      </div>

      <!-- 3D Card Flip Stage -->
      <div class="bonus-card-stage" aria-hidden="true">
        <div class="bonus-ghost bonus-ghost--left" :class="{ 'bonus-ghost--active': isDrawing }"></div>
        <div class="bonus-ghost bonus-ghost--right" :class="{ 'bonus-ghost--active': isDrawing }"></div>

        <div class="bonus-card-motion" :class="`bonus-card-motion--${phase}`">
          <div class="bonus-card" :class="cardFlipClass" data-testid="bonus-flip-card">
            <!-- Front Face (Mystery Back before flip) -->
            <div class="bonus-face bonus-face--front">
              <div class="bonus-card-frame"></div>
              <div class="bonus-card-sheen"></div>
              <div class="bonus-front-emblem">
                <Icon name="sparkles" size="xl" />
              </div>
              <span class="bonus-front-mark">?</span>
              <span class="bonus-front-caption">{{ t('wallet.bonusResultTitle') }}</span>
            </div>

            <!-- Back Face (Revealed Result after 180deg flip) -->
            <div class="bonus-face bonus-face--back">
              <div class="bonus-card-frame"></div>
              <div v-if="phase === 'win'" class="bonus-card-sheen bonus-card-sheen--win"></div>
              <svg
                v-if="phase === 'lose'"
                class="bonus-card-crack"
                viewBox="0 0 160 210"
                fill="none"
              >
                <path
                  d="M24 0 L48 44 L34 76 L64 114"
                  stroke="currentColor"
                  stroke-width="2.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M138 210 L114 166 L128 134 L102 106"
                  stroke="currentColor"
                  stroke-width="2.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M48 44 L70 52 M114 166 L92 156"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linecap="round"
                />
              </svg>

              <div class="bonus-result-emblem">
                <Icon :name="!revealed ? 'sparkles' : phase === 'win' ? 'trophy' : 'xCircle'" size="xl" />
              </div>
              <strong class="bonus-result-multiplier">
                {{ !revealed ? '?' : phase === 'win' ? '×2' : '−0.5×' }}
              </strong>
              <span v-if="revealed" class="bonus-result-stamp">
                {{ t(phase === 'win' ? 'wallet.win' : 'wallet.lose') }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Outcome Headline & Reward Breakdown -->
      <div class="bonus-copy" :class="{ 'bonus-copy--revealed': revealed }">
        <h3 class="bonus-headline">
          {{ t(!revealed ? 'wallet.bonusPending' : phase === 'win' ? 'wallet.bonusWin' : 'wallet.bonusLose') }}
        </h3>

        <p v-if="isDrawing" class="bonus-hint">
          {{ t('wallet.bonusDrawingHint') }}
        </p>

        <template v-if="revealed && record">
          <div class="bonus-delta-hero" data-testid="bonus-delta-hero">
            <span class="bonus-delta-label">{{ t('wallet.bonusDelta') }}</span>
            <strong class="bonus-delta-value">{{ deltaLabel }}</strong>
          </div>

          <div class="bonus-breakdown">
            <div class="bonus-breakdown-item">
              <span class="bonus-breakdown-label">{{ t('wallet.baseReward') }}</span>
              <strong class="bonus-breakdown-value">{{ baseRewardLabel }}</strong>
            </div>
            <Icon name="arrowRight" size="sm" class="bonus-breakdown-arrow" />
            <div class="bonus-breakdown-item bonus-breakdown-item--final">
              <span class="bonus-breakdown-label">{{ t('wallet.reward') }}</span>
              <strong class="bonus-breakdown-value">{{ rewardLabel }}</strong>
            </div>
          </div>
        </template>

        <p v-if="revealed" class="bonus-hint">
          {{ t(phase === 'win' ? 'wallet.bonusWinHint' : 'wallet.bonusLoseHint') }}
        </p>

        <button
          v-if="revealed"
          type="button"
          class="bonus-close-btn"
          :class="phase === 'win' ? 'bonus-close-btn--win' : 'bonus-close-btn--lose'"
          @click="emit('close')"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { CheckinRecord } from '@/api/checkin'
import { formatWalletAmount } from '@/utils/walletDisplay'

const props = defineProps<{ phase: 'drawing' | 'win' | 'lose' | null; record: CheckinRecord | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const isDrawing = computed(() => props.phase === 'drawing')
const revealed = computed(() => props.phase === 'win' || props.phase === 'lose')

const cardFlipClass = computed(() => ({
  'bonus-card--revealed': revealed.value,
  'bonus-card--flip-win': props.phase === 'win',
  'bonus-card--flip-lose': props.phase === 'lose'
}))

const statusBadgeText = computed(() => {
  if (props.phase === 'win') return t('wallet.win')
  if (props.phase === 'lose') return t('wallet.lose')
  return t('wallet.bonusPending')
})

function formatMoney(value?: number | null): string {
  const amount = formatWalletAmount(value)
  return amount.startsWith('-') ? `-$${amount.slice(1)}` : `$${amount}`
}

function formatSignedMoney(value?: number | null): string {
  const amount = formatWalletAmount(value)
  return amount.startsWith('-') ? `-$${amount.slice(1)}` : `+$${amount}`
}

const baseRewardLabel = computed(() => formatMoney(props.record?.base_reward_amount))
const rewardLabel = computed(() => formatMoney(props.record?.reward_amount))
const deltaLabel = computed(() => formatSignedMoney(props.record?.bonus_delta_amount))

const CONFETTI_COLORS = ['#10b981', '#f59e0b', '#34d399', '#fbbf24', '#38bdf8', '#f97316']

function sparkStyle(n: number) {
  const angle = (n - 1) * (360 / 16)
  const dist = n % 2 === 0 ? 118 : 92
  return {
    '--angle': `${angle}deg`,
    '--dist': `${dist}px`,
    '--delay': `${0.28 + (n % 4) * 0.03}s`,
    '--color': CONFETTI_COLORS[n % CONFETTI_COLORS.length]
  }
}

function confettiStyle(n: number) {
  const left = 4 + ((n * 37) % 92)
  const drift = ((n % 7) - 3) * 14
  return {
    '--x': `${left}%`,
    '--drift': `${drift}px`,
    '--delay': `${0.26 + (n % 9) * 0.14}s`,
    '--dur': `${2.1 + (n % 5) * 0.35}s`,
    '--color': CONFETTI_COLORS[n % CONFETTI_COLORS.length]
  }
}

function shardStyle(n: number) {
  const left = 8 + ((n * 43) % 84)
  return {
    '--x': `${left}%`,
    '--delay': `${0.3 + (n % 6) * 0.12}s`,
    '--dur': `${1.7 + (n % 4) * 0.3}s`,
    '--rot': `${(n % 2 === 0 ? 1 : -1) * (80 + n * 12)}deg`
  }
}
</script>

<style scoped>
.bonus-stage {
  position: relative;
  overflow: hidden;
  isolation: isolate;
  padding: 1.5rem 1.25rem 1.5rem;
  border-radius: 1.5rem;
  border: 1px solid rgb(251 191 36 / 0.38);
  text-align: center;
  color: #92400e;
  background: radial-gradient(ellipse at 50% 24%, #fef3c7 0%, #fffbeb 58%, #ffffff 100%);
  transition: background 0.45s ease, border-color 0.45s ease, color 0.35s ease;
}

.bonus-stage--win {
  color: #065f46;
  border-color: rgb(16 185 129 / 0.55);
  background: radial-gradient(ellipse at 50% 22%, #a7f3d0 0%, #ecfdf5 52%, #ffffff 100%);
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.65), 0 0 36px rgb(16 185 129 / 0.18);
}

.bonus-stage--lose {
  color: #9f1239;
  border-color: rgb(244 63 94 / 0.55);
  background: radial-gradient(ellipse at 50% 24%, #fecdd3 0%, #fff1f2 55%, #f8fafc 100%);
  box-shadow: inset 0 0 42px rgb(190 18 60 / 0.14);
}

/* Ambient & Burst Layers */
.bonus-ambient {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.bonus-halo {
  position: absolute;
  width: 17rem;
  height: 17rem;
  left: calc(50% - 8.5rem);
  top: 2.25rem;
  border-radius: 50%;
  border: 2px dashed currentColor;
  opacity: 0.22;
  animation: halo-spin 9s linear infinite;
}

.bonus-stage--win .bonus-halo {
  border-style: solid;
  border-color: #10b981;
  opacity: 0.28;
  box-shadow: 0 0 0 24px rgb(16 185 129 / 0.08), 0 0 0 50px rgb(251 191 36 / 0.07);
  animation: halo-pulse 1.8s ease-in-out infinite;
}

.bonus-stage--lose .bonus-halo {
  border-style: solid;
  border-color: #e11d48;
  opacity: 0.22;
  animation: halo-sink 2.4s ease-out infinite;
}

.bonus-rays {
  position: absolute;
  width: 28rem;
  height: 28rem;
  left: calc(50% - 14rem);
  top: -3.25rem;
  background: repeating-conic-gradient(
    from 0deg,
    rgb(251 191 36 / 0.22) 0deg 12deg,
    transparent 12deg 30deg
  );
  mask-image: radial-gradient(circle, black 12%, transparent 66%);
  -webkit-mask-image: radial-gradient(circle, black 12%, transparent 66%);
  animation: rays-enter 0.45s 0.28s ease-out both, halo-spin 18s linear infinite;
}

.bonus-shockwave {
  position: absolute;
  width: 11rem;
  height: 11rem;
  left: calc(50% - 5.5rem);
  top: 5.5rem;
  border-radius: 50%;
  border: 3px solid currentColor;
  opacity: 0;
  animation: shockwave-burst 0.85s 0.3s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.bonus-shockwave--secondary {
  border-color: #f59e0b;
  animation-delay: 0.42s;
}

.bonus-lose-flash {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 35%, rgb(244 63 94 / 0.32), rgb(159 18 57 / 0.16) 65%, transparent 100%);
  opacity: 0;
  animation: lose-flash 0.65s 0.28s ease-out both;
}

/* Win Particles */
.bonus-burst {
  position: absolute;
  left: 50%;
  top: 11rem;
  pointer-events: none;
  z-index: 2;
}

.bonus-spark {
  position: absolute;
  width: 9px;
  height: 9px;
  margin: -4.5px;
  border-radius: 9999px;
  background: var(--color);
  box-shadow: 0 0 10px var(--color);
  opacity: 0;
  animation: spark-pop 0.85s var(--delay) cubic-bezier(0.16, 1, 0.3, 1) both;
}

.bonus-confetti {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 2;
}

.bonus-confetti i {
  position: absolute;
  top: -16px;
  left: var(--x);
  width: 8px;
  height: 14px;
  border-radius: 2px;
  background: var(--color);
  opacity: 0;
  animation: confetti-fall var(--dur) var(--delay) ease-in infinite;
}

.bonus-confetti i:nth-child(3n) {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

/* Lose Shards */
.bonus-shards {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 2;
}

.bonus-shards i {
  position: absolute;
  top: 30%;
  left: var(--x);
  width: 8px;
  height: 10px;
  background: linear-gradient(135deg, #f43f5e, #881337);
  clip-path: polygon(50% 0%, 100% 60%, 35% 100%, 0% 40%);
  opacity: 0;
  animation: shard-drop var(--dur) var(--delay) ease-in infinite;
}

.bonus-shards i:nth-child(2n) {
  background: linear-gradient(135deg, #94a3b8, #475569);
  width: 6px;
  height: 8px;
}

/* Top Outcome Badge */
.bonus-pill-wrap {
  position: relative;
  z-index: 3;
  margin-bottom: 1.1rem;
  display: flex;
  justify-content: center;
}

.bonus-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.35rem 0.9rem;
  border-radius: 9999px;
  font-size: 0.8125rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  box-shadow: 0 6px 16px rgb(0 0 0 / 0.08);
}

.bonus-status-pill--drawing {
  background: rgb(254 243 199 / 0.95);
  color: #92400e;
  border: 1px solid rgb(245 158 11 / 0.45);
}

.bonus-status-pill--win {
  background: linear-gradient(135deg, #059669, #10b981);
  color: #ffffff;
  border: 1px solid rgb(167 243 208 / 0.65);
  box-shadow: 0 8px 20px rgb(16 185 129 / 0.35);
  animation: pill-pop 0.45s 0.3s cubic-bezier(0.34, 1.56, 0.64, 1) both;
}

.bonus-status-pill--lose {
  background: linear-gradient(135deg, #be123c, #e11d48);
  color: #ffffff;
  border: 1px solid rgb(254 205 211 / 0.55);
  box-shadow: 0 8px 20px rgb(225 29 72 / 0.32);
  animation: pill-drop 0.42s 0.3s ease-out both;
}

.bonus-pulse-dot {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  background: #f59e0b;
  animation: pulse-dot 0.9s ease-in-out infinite;
}

/* 3D Card Stage */
.bonus-card-stage {
  position: relative;
  z-index: 3;
  perspective: 1100px;
  width: 9.75rem;
  height: 13rem;
  margin: 0 auto;
}

.bonus-ghost {
  position: absolute;
  inset: 0.35rem;
  border-radius: 1.2rem;
  border: 2px solid rgb(245 158 11 / 0.38);
  background: linear-gradient(145deg, rgb(254 243 199 / 0.85), rgb(252 211 77 / 0.65));
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.bonus-ghost--left.bonus-ghost--active {
  opacity: 0.72;
  animation: ghost-shuffle-left 1.1s ease-in-out infinite;
}

.bonus-ghost--right.bonus-ghost--active {
  opacity: 0.72;
  animation: ghost-shuffle-right 1.1s ease-in-out infinite;
}

.bonus-card-motion {
  position: relative;
  width: 100%;
  height: 100%;
  transform-style: preserve-3d;
}

.bonus-card-motion--drawing {
  animation: bonus-drawing-float 1.1s ease-in-out infinite;
}

.bonus-card-motion--win {
  animation: card-motion-win 0.95s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

.bonus-card-motion--lose {
  animation: card-motion-lose 0.95s ease-out forwards;
}

.bonus-card {
  position: relative;
  width: 100%;
  height: 100%;
  transform-style: preserve-3d;
}

.bonus-card--revealed {
  transform: rotateY(180deg);
}

.bonus-card--flip-win {
  animation: card-flip-win 0.72s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

.bonus-card--flip-lose {
  animation: card-flip-lose 0.62s cubic-bezier(0.32, 0.72, 0, 1) forwards;
}

.bonus-face {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.55rem;
  overflow: hidden;
  border-radius: 1.35rem;
  border: 2.5px solid #f59e0b;
  background: linear-gradient(155deg, #fffbeb 0%, #fde68a 48%, #f59e0b 100%);
  box-shadow: 0 20px 42px rgb(180 83 9 / 0.24), inset 0 0 0 6px rgb(255 255 255 / 0.42);
  backface-visibility: hidden;
  -webkit-backface-visibility: hidden;
}

.bonus-face--front {
  transform: translateZ(1px);
  color: #78350f;
}

.bonus-face--back {
  transform: rotateY(180deg) translateZ(1px);
}

.bonus-card-frame {
  position: absolute;
  inset: 10px;
  border-radius: 0.85rem;
  border: 1.5px solid rgb(255 255 255 / 0.55);
  pointer-events: none;
}

.bonus-card-sheen {
  position: absolute;
  inset: -40%;
  background: linear-gradient(
    115deg,
    transparent 35%,
    rgb(255 255 255 / 0.65) 50%,
    transparent 65%
  );
  transform: translateX(-120%);
  animation: card-sheen 1.25s ease-in-out infinite;
  pointer-events: none;
}

.bonus-card-sheen--win {
  animation: card-sheen 1.6s 0.45s ease-in-out 2;
}

.bonus-front-emblem {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.5);
  color: #b45309;
}

.bonus-front-mark {
  font-size: 3.5rem;
  font-weight: 900;
  line-height: 1;
  text-shadow: 0 2px 10px rgb(255 255 255 / 0.5);
}

.bonus-front-caption {
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  opacity: 0.78;
}

/* Back Face: Win vs Lose */
.bonus-stage--win .bonus-face--back {
  color: #064e3b;
  border-color: #10b981;
  background: linear-gradient(155deg, #ecfdf5 0%, #6ee7b7 52%, #10b981 100%);
  box-shadow: 0 22px 48px rgb(16 185 129 / 0.42), inset 0 0 0 6px rgb(255 255 255 / 0.55);
}

.bonus-stage--lose .bonus-face--back {
  color: #881337;
  border-color: #e11d48;
  background: linear-gradient(155deg, #fff1f2 0%, #fda4af 52%, #e11d48 100%);
  box-shadow: 0 22px 48px rgb(225 29 72 / 0.34), inset 0 0 0 6px rgb(255 255 255 / 0.38);
}

.bonus-card-crack {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  color: #4c0519;
  opacity: 0.65;
  pointer-events: none;
}

.bonus-result-emblem {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 9999px;
  background: rgb(255 255 255 / 0.72);
  box-shadow: 0 6px 16px rgb(0 0 0 / 0.1);
}

.bonus-stage--win .bonus-result-emblem {
  color: #059669;
}

.bonus-stage--lose .bonus-result-emblem {
  color: #be123c;
}

.bonus-result-multiplier {
  position: relative;
  font-size: 2.75rem;
  font-weight: 900;
  line-height: 1;
  letter-spacing: -0.02em;
  text-shadow: 0 2px 10px rgb(255 255 255 / 0.45);
}

.bonus-result-stamp {
  position: relative;
  padding: 0.2rem 0.65rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  background: rgb(255 255 255 / 0.85);
}

.bonus-stage--win .bonus-result-stamp {
  color: #047857;
}

.bonus-stage--lose .bonus-result-stamp {
  color: #be123c;
}

/* Copy & Numbers */
.bonus-copy {
  position: relative;
  z-index: 3;
  margin-top: 1.35rem;
}

.bonus-copy--revealed {
  animation: copy-reveal 0.4s 0.32s ease-out both;
}

.bonus-headline {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1.3;
}

.bonus-delta-hero {
  margin: 0.9rem auto 0;
  display: inline-flex;
  align-items: baseline;
  gap: 0.55rem;
  padding: 0.45rem 1.15rem;
  border-radius: 1rem;
  background: rgb(255 255 255 / 0.78);
  border: 1px solid rgb(0 0 0 / 0.06);
  box-shadow: 0 6px 16px rgb(0 0 0 / 0.05);
}

.bonus-stage--win .bonus-delta-hero {
  border-color: rgb(16 185 129 / 0.35);
}

.bonus-stage--lose .bonus-delta-hero {
  border-color: rgb(225 29 72 / 0.35);
}

.bonus-delta-label {
  font-size: 0.75rem;
  font-weight: 600;
  opacity: 0.75;
}

.bonus-delta-value {
  font-size: 1.75rem;
  font-weight: 900;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.bonus-stage--win .bonus-delta-value {
  color: #059669;
}

.bonus-stage--lose .bonus-delta-value {
  color: #e11d48;
}

.bonus-breakdown {
  margin-top: 0.75rem;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 0.5rem;
}

.bonus-breakdown-item {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  padding: 0.55rem 0.65rem;
  border-radius: 0.9rem;
  background: rgb(255 255 255 / 0.72);
  border: 1px solid rgb(0 0 0 / 0.05);
}

.bonus-breakdown-label {
  font-size: 0.6875rem;
  opacity: 0.72;
}

.bonus-breakdown-value {
  font-size: 0.9375rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.bonus-breakdown-arrow {
  opacity: 0.5;
}

.bonus-hint {
  margin-top: 0.75rem;
  font-size: 0.75rem;
  line-height: 1.5;
  opacity: 0.78;
}

.bonus-close-btn {
  margin-top: 1.15rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 8.5rem;
  padding: 0.6rem 1.5rem;
  border-radius: 9999px;
  font-size: 0.875rem;
  font-weight: 700;
  color: #ffffff;
  transition: transform 0.18s ease, box-shadow 0.18s ease, filter 0.18s ease;
}

.bonus-close-btn:hover {
  transform: translateY(-1px);
  filter: brightness(1.04);
}

.bonus-close-btn--win {
  background: linear-gradient(135deg, #059669, #10b981);
  box-shadow: 0 10px 22px rgb(16 185 129 / 0.34);
}

.bonus-close-btn--lose {
  background: linear-gradient(135deg, #be123c, #e11d48);
  box-shadow: 0 10px 22px rgb(225 29 72 / 0.28);
}

/* Dark Mode Adaptation */
:global(.dark) .bonus-stage {
  color: #fde68a;
  border-color: rgb(245 158 11 / 0.35);
  background: radial-gradient(ellipse at 50% 24%, rgb(120 53 15 / 0.55) 0%, rgb(24 24 27 / 0.96) 72%);
}

:global(.dark) .bonus-stage--win {
  color: #a7f3d0;
  border-color: rgb(16 185 129 / 0.55);
  background: radial-gradient(ellipse at 50% 22%, rgb(6 95 70 / 0.7) 0%, rgb(15 23 42 / 0.98) 70%);
  box-shadow: 0 0 36px rgb(16 185 129 / 0.24);
}

:global(.dark) .bonus-stage--lose {
  color: #fecdd3;
  border-color: rgb(244 63 94 / 0.55);
  background: radial-gradient(ellipse at 50% 24%, rgb(136 19 55 / 0.68) 0%, rgb(15 23 42 / 0.98) 70%);
  box-shadow: inset 0 0 42px rgb(225 29 72 / 0.2);
}

:global(.dark) .bonus-status-pill--drawing {
  background: rgb(120 53 15 / 0.75);
  color: #fef3c7;
}

:global(.dark) .bonus-delta-hero,
:global(.dark) .bonus-breakdown-item {
  background: rgb(15 23 42 / 0.68);
  border-color: rgb(255 255 255 / 0.1);
}

:global(.dark) .bonus-stage--win .bonus-delta-value {
  color: #34d399;
}

:global(.dark) .bonus-stage--lose .bonus-delta-value {
  color: #fb7185;
}

/* Keyframes */
@keyframes bonus-drawing-float {
  0%, 100% {
    transform: translateY(0) rotateY(-18deg) rotateZ(-3deg) scale(1);
  }
  50% {
    transform: translateY(-10px) rotateY(18deg) rotateZ(3deg) scale(1.03);
  }
}

@keyframes ghost-shuffle-left {
  0%, 100% {
    transform: translateX(-22px) rotateZ(-10deg) scale(0.94);
  }
  50% {
    transform: translateX(-34px) translateY(-6px) rotateZ(-14deg) scale(0.96);
  }
}

@keyframes ghost-shuffle-right {
  0%, 100% {
    transform: translateX(22px) rotateZ(10deg) scale(0.94);
  }
  50% {
    transform: translateX(34px) translateY(-6px) rotateZ(14deg) scale(0.96);
  }
}

@keyframes card-flip-win {
  0% {
    transform: rotateY(0deg) scale(1);
  }
  32% {
    transform: rotateY(90deg) scale(1.12);
  }
  74% {
    transform: rotateY(194deg) scale(1.07);
  }
  100% {
    transform: rotateY(180deg) scale(1);
  }
}

@keyframes card-flip-lose {
  0% {
    transform: rotateY(0deg) scale(1);
  }
  38% {
    transform: rotateY(90deg) scale(1.06);
  }
  100% {
    transform: rotateY(180deg) scale(0.97);
  }
}

@keyframes card-motion-win {
  0% {
    transform: translateY(0) scale(1);
  }
  30% {
    transform: translateY(-16px) scale(1.05);
  }
  62% {
    transform: translateY(3px) scale(0.98);
  }
  80% {
    transform: translateY(-5px) scale(1.02);
  }
  100% {
    transform: translateY(0) scale(1);
  }
}

@keyframes card-motion-lose {
  0% {
    transform: translateY(0) translateX(0);
  }
  28% {
    transform: translateY(-10px) translateX(0);
  }
  48% {
    transform: translateY(4px) translateX(-11px) rotateZ(-3.5deg);
  }
  58% {
    transform: translateY(4px) translateX(10px) rotateZ(3deg);
  }
  68% {
    transform: translateY(4px) translateX(-8px) rotateZ(-2deg);
  }
  78% {
    transform: translateY(4px) translateX(6px) rotateZ(1.5deg);
  }
  88% {
    transform: translateY(4px) translateX(-3px) rotateZ(-0.5deg);
  }
  100% {
    transform: translateY(4px) translateX(0) rotateZ(0deg);
  }
}

@keyframes card-sheen {
  0% {
    transform: translateX(-130%);
  }
  55%, 100% {
    transform: translateX(130%);
  }
}

@keyframes halo-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes halo-pulse {
  0%, 100% {
    transform: scale(0.98);
    opacity: 0.24;
  }
  50% {
    transform: scale(1.1);
    opacity: 0.36;
  }
}

@keyframes halo-sink {
  0% {
    transform: scale(0.72);
    opacity: 0.34;
  }
  100% {
    transform: scale(1.38);
    opacity: 0;
  }
}

@keyframes rays-enter {
  from {
    opacity: 0;
    transform: scale(0.6);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes shockwave-burst {
  0% {
    transform: scale(0.45);
    opacity: 0.78;
  }
  100% {
    transform: scale(2.35);
    opacity: 0;
  }
}

@keyframes lose-flash {
  0% {
    opacity: 0;
  }
  25% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
}

@keyframes spark-pop {
  0% {
    transform: rotate(var(--angle)) translateX(10px) scale(0.4);
    opacity: 1;
  }
  70% {
    opacity: 1;
  }
  100% {
    transform: rotate(var(--angle)) translateX(var(--dist)) scale(0.1);
    opacity: 0;
  }
}

@keyframes confetti-fall {
  0% {
    transform: translate3d(0, 0, 0) rotate(0deg);
    opacity: 0;
  }
  10% {
    opacity: 1;
  }
  90% {
    opacity: 0.9;
  }
  100% {
    transform: translate3d(var(--drift), 460px, 0) rotate(540deg);
    opacity: 0;
  }
}

@keyframes shard-drop {
  0% {
    transform: translateY(-10px) rotate(0deg) scale(0.9);
    opacity: 0;
  }
  15% {
    opacity: 0.75;
  }
  100% {
    transform: translateY(270px) rotate(var(--rot)) scale(0.5);
    opacity: 0;
  }
}

@keyframes pulse-dot {
  0%, 100% {
    transform: scale(0.85);
    opacity: 0.6;
  }
  50% {
    transform: scale(1.25);
    opacity: 1;
  }
}

@keyframes pill-pop {
  0% {
    transform: scale(0.55);
    opacity: 0;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes pill-drop {
  0% {
    transform: translateY(-10px) scale(0.9);
    opacity: 0;
  }
  100% {
    transform: translateY(0) scale(1);
    opacity: 1;
  }
}

@keyframes copy-reveal {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .bonus-card,
  .bonus-card-motion,
  .bonus-ghost,
  .bonus-halo,
  .bonus-rays,
  .bonus-shockwave,
  .bonus-lose-flash,
  .bonus-spark,
  .bonus-confetti i,
  .bonus-shards i,
  .bonus-card-sheen,
  .bonus-status-pill,
  .bonus-copy,
  .bonus-pulse-dot {
    animation: none !important;
    transition: none !important;
  }

  .bonus-confetti,
  .bonus-burst,
  .bonus-shards,
  .bonus-shockwave,
  .bonus-lose-flash,
  .bonus-ghost {
    display: none;
  }

  .bonus-card--revealed {
    transform: rotateY(180deg);
  }
}
</style>
