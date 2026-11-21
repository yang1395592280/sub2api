import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CheckinBonusReveal from '../CheckinBonusReveal.vue'
import type { CheckinRecord } from '@/api/checkin'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const winRecord: CheckinRecord = {
  checkin_date: '2026-11-21',
  base_reward_amount: 1.72,
  reward_amount: 3.44,
  bonus_status: 'win',
  bonus_delta_amount: 1.72
}

const loseRecord: CheckinRecord = {
  checkin_date: '2026-11-21',
  base_reward_amount: 1.72,
  reward_amount: -0.86,
  bonus_status: 'lose',
  bonus_delta_amount: -2.58
}

const mountReveal = (phase: 'drawing' | 'win' | 'lose' | null, record: CheckinRecord | null = null) =>
  mount(CheckinBonusReveal, {
    props: { phase, record },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title', 'showCloseButton', 'closeOnEscape'],
          emits: ['close'],
          template: '<div v-if="show" data-testid="dialog"><slot /></div>'
        }
      }
    }
  })

describe('CheckinBonusReveal', () => {
  it('shows shuffling mystery card while drawing and hides result controls', () => {
    const w = mountReveal('drawing', null)
    const stage = w.get('[data-testid="bonus-reveal"]')
    expect(stage.attributes('data-phase')).toBe('drawing')
    expect(stage.classes()).toContain('bonus-stage--drawing')
    expect(w.get('[data-testid="bonus-status-badge"]').text()).toContain('wallet.bonusPending')
    expect(w.findAll('.bonus-ghost--active')).toHaveLength(2)
    expect(w.find('[data-testid="bonus-win-effects"]').exists()).toBe(false)
    expect(w.find('[data-testid="bonus-lose-effects"]').exists()).toBe(false)
    expect(w.find('[data-testid="bonus-delta-hero"]').exists()).toBe(false)
    expect(w.find('.bonus-close-btn').exists()).toBe(false)
    w.unmount()
  })

  it('renders unmistakable win flip, celebration effects, and positive delta breakdown', async () => {
    const w = mountReveal('win', winRecord)
    const stage = w.get('[data-testid="bonus-reveal"]')
    expect(stage.classes()).toContain('bonus-stage--win')
    expect(w.get('[data-testid="bonus-flip-card"]').classes()).toContain('bonus-card--flip-win')
    expect(w.get('[data-testid="bonus-status-badge"]').text()).toContain('wallet.win')
    expect(w.find('[data-testid="bonus-win-effects"]').exists()).toBe(true)
    expect(w.find('[data-testid="bonus-lose-effects"]').exists()).toBe(false)
    expect(w.text()).toContain('×2')
    expect(w.get('[data-testid="bonus-delta-hero"]').text()).toContain('+$1.72')
    expect(w.text()).toContain('$1.72')
    expect(w.text()).toContain('$3.44')

    await w.get('.bonus-close-btn').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount()
  })

  it('renders unmistakable lose flip, failure effects, and negative delta breakdown', async () => {
    const w = mountReveal('lose', loseRecord)
    const stage = w.get('[data-testid="bonus-reveal"]')
    expect(stage.classes()).toContain('bonus-stage--lose')
    expect(w.get('[data-testid="bonus-flip-card"]').classes()).toContain('bonus-card--flip-lose')
    expect(w.get('[data-testid="bonus-status-badge"]').text()).toContain('wallet.lose')
    expect(w.find('[data-testid="bonus-lose-effects"]').exists()).toBe(true)
    expect(w.find('[data-testid="bonus-win-effects"]').exists()).toBe(false)
    expect(w.find('.bonus-card-crack').exists()).toBe(true)
    expect(w.text()).toContain('−0.5×')
    expect(w.get('[data-testid="bonus-delta-hero"]').text()).toContain('-$2.58')
    expect(w.text()).toContain('$1.72')
    expect(w.text()).toContain('-$0.86')

    await w.get('.bonus-close-btn').trigger('click')
    expect(w.emitted('close')).toHaveLength(1)
    w.unmount()
  })
})
