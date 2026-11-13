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
    <div class="bonus-stage" :class="`bonus-stage--${phase}`" data-testid="bonus-reveal" :data-phase="phase" aria-live="polite" :aria-busy="!revealed">
      <div class="bonus-halo" aria-hidden="true"></div>
      <div v-if="phase === 'win'" class="bonus-confetti" aria-hidden="true">
        <i v-for="n in 24" :key="n" :style="{ '--i': n }"></i>
      </div>
      <div class="bonus-perspective" aria-hidden="true">
        <div class="bonus-card" :class="{ 'bonus-card--revealed': revealed }">
          <div class="bonus-face bonus-face--front"><Icon name="sparkles" size="xl" /><span>?</span></div>
          <div class="bonus-face bonus-face--back"><Icon :name="!revealed ? 'sparkles' : phase === 'win' ? 'gift' : 'moon'" size="xl" /><strong>{{ !revealed ? '?' : phase === 'win' ? '×2' : '−½' }}</strong></div>
        </div>
      </div>
      <h3 class="relative mt-7 text-xl font-bold">{{ t(!revealed ? 'wallet.bonusPending' : phase === 'win' ? 'wallet.bonusWin' : 'wallet.bonusLose') }}</h3>
      <p v-if="revealed && record" class="relative mt-3 text-sm opacity-80">{{ t('wallet.reward') }} <strong class="ml-2 text-2xl">{{ rewardLabel }}</strong></p>
      <p v-if="revealed" class="relative mt-2 text-xs opacity-70">{{ t(phase === 'win' ? 'wallet.bonusWinHint' : 'wallet.bonusLoseHint') }}</p>
      <button v-if="revealed" type="button" class="btn btn-primary relative mt-6" @click="emit('close')">{{ t('common.close') }}</button>
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
const revealed = computed(() => props.phase === 'win' || props.phase === 'lose')
const rewardLabel = computed(() => {
  const amount = formatWalletAmount(props.record?.reward_amount)
  return amount.startsWith('-') ? `-$${amount.slice(1)}` : `$${amount}`
})
</script>

<style scoped>
.bonus-stage { position: relative; overflow: hidden; isolation: isolate; padding: 2.5rem 1rem 1.5rem; border-radius: 1.5rem; text-align: center; color: #a16207; background: radial-gradient(ellipse at 50% 30%, #fef3c7, #fffbeb 65%); }
.bonus-stage--win { color: #047857; background: radial-gradient(ellipse at 50% 30%, #a7f3d0, #ecfdf5 65%); }
.bonus-stage--lose { color: #6d28d9; background: radial-gradient(ellipse at 50% 30%, #c4b5fd, #f5f3ff 65%); }
.bonus-halo { position: absolute; width: 17rem; height: 17rem; left: calc(50% - 8.5rem); top: 0; border: 2px solid currentColor; border-radius: 50%; opacity: .15; box-shadow: 0 0 0 25px #ffffff45, 0 0 0 55px #ffffff30; animation: halo-pulse 1.5s ease-out infinite; }
.bonus-stage--lose .bonus-halo { animation: halo-ripple 2s ease-out infinite; }
.bonus-perspective { perspective: 900px; position: relative; width: 9rem; height: 12rem; margin: auto; }
.bonus-card { position: relative; width: 100%; height: 100%; transform-style: preserve-3d; transition: transform 1s cubic-bezier(.2,.75,.25,1); }
.bonus-stage--drawing .bonus-card { animation: card-shuffle .8s ease-in-out infinite; }
.bonus-card--revealed { transform: rotateY(180deg); }
.bonus-face { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 1rem; border-radius: 1.25rem; border: 2px solid #fbbf24; background: linear-gradient(145deg, #fef3c7, #fbbf24); box-shadow: 0 18px 45px #b453092a, inset 0 0 0 7px #ffffff50; backface-visibility: hidden; }
.bonus-face--front span { font-size: 3.5rem; font-weight: 800; }
.bonus-face--back { transform: rotateY(180deg); }
.bonus-face strong { font-size: 3.5rem; line-height: 1; }
.bonus-stage--win .bonus-face--back { border-color: #34d399; background: linear-gradient(145deg, #ecfdf5, #6ee7b7); box-shadow: 0 18px 50px #10b98155, inset 0 0 0 7px #ffffff70; }
.bonus-stage--lose .bonus-face--back { border-color: #a78bfa; background: linear-gradient(145deg, #f5f3ff, #c4b5fd); box-shadow: 0 18px 50px #8b5cf640, inset 0 0 0 7px #ffffff70; }
.bonus-confetti { position: absolute; inset: 0; pointer-events: none; }
.bonus-confetti i { position: absolute; top: -12px; left: calc(var(--i) * 4.1%); width: 7px; height: 12px; background: #fbbf24; animation: confetti-fall 2.7s calc(var(--i) * -0.12s) ease-in infinite; }
.bonus-confetti i:nth-child(3n) { background: #34d399; border-radius: 50%; }
.bonus-confetti i:nth-child(3n + 1) { background: #a78bfa; }
@keyframes card-shuffle { 0%, 100% { transform: rotateY(-25deg) rotateZ(-7deg) translateY(0); } 50% { transform: rotateY(155deg) rotateZ(7deg) translateY(-12px); } }
@keyframes halo-pulse { 50% { transform: scale(1.12); opacity: .25; } }
@keyframes halo-ripple { from { transform: scale(.6); opacity: .3; } to { transform: scale(1.4); opacity: 0; } }
@keyframes confetti-fall { to { transform: translateY(460px) rotate(540deg); } }
@media (prefers-reduced-motion: reduce) { .bonus-card, .bonus-halo, .bonus-confetti i { animation: none !important; transition: none; } .bonus-confetti { display: none; } }
</style>
