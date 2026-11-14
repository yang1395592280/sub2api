import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import GenerationQuestionPicker from '../GenerationQuestionPicker.vue'
import type { GenerationTestQuestion } from '@/api/admin/pelican'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('GenerationQuestionPicker', () => {
  it('allows removing disabled selections and prevents selecting more than 20 items', async () => {
    const questions = Array.from({ length: 22 }, (_, i) => ({ id: i + 1, name: `Q${i + 1}`, enabled: i > 0, prompt: '<script>evil()</script>' })) as GenerationTestQuestion[]
    const wrapper = mount(GenerationQuestionPicker, { props: { kind: 'pelican', questions, modelValue: Array.from({ length: 20 }, (_, i) => i + 1) }, global: { stubs: { RouterLink: true } } })
    expect(wrapper.get('[data-question-id="21"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-question-id="1"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('script').exists()).toBe(false)
    await wrapper.get('[data-question-id="1"]').setValue(false)
    expect(wrapper.emitted('update:modelValue')?.[0][0]).toEqual(Array.from({ length: 19 }, (_, i) => i + 2))
    wrapper.unmount()
  })
})
