import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import WalletBalanceCard from '../WalletBalanceCard.vue'
import wallet from '@/i18n/locales/zh/wallet'
import type { User } from '@/types'

describe('wallet net welfare rules', () => {
  it.each([{ paid_units: 2, welfare_units: 1 }, { paid_units: 3, welfare_units: 1 }, { paid_units: 2, welfare_units: 0 }])('shows the server ratio %j and debt separately', policy => {
    const user = { balance: 9, welfare_balance: -2, checkin_balance: -5, liyu_balance: 3, gift_balance: 0, available_balance: 9, wallet_usage_policy: policy } as User
    const wrapper = mount(WalletBalanceCard, { props: { user }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { wallet } } })] } })
    expect(wrapper.text()).toContain(`充值 : 福利 = ${policy.paid_units} : ${policy.welfare_units}`)
    expect(wrapper.text()).toContain('福利合计为零或负数时，消费全扣充值')
    expect(wrapper.text()).toContain('$-2.00'); expect(wrapper.text()).toContain('$9.00')
  })
})
