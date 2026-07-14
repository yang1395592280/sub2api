import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerGroupList from '../SchedulerGroupList.vue'
import { createSchedulerTestI18n } from './testI18n'

const groups = [
  { id: 33, name: 'Codex', status: 'active', enabled: true },
  { id: 82, name: 'Control', status: 'active', enabled: false },
]

describe('SchedulerGroupList', () => {
  it('emits selection and group switch commands', async () => {
    const wrapper = mount(SchedulerGroupList, { props: { groups, modelValue: 33 }, global: { plugins: [createSchedulerTestI18n()] } })

    await wrapper.get('[data-testid="scheduler-group-82"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[82]])

    await wrapper.get('[data-testid="scheduler-group-toggle-82"]').trigger('click')
    expect(wrapper.emitted('toggle')).toEqual([[82, true]])
  })
})
