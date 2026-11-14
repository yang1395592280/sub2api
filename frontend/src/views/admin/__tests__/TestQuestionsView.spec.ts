import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import TestQuestionsView from '../TestQuestionsView.vue'
const { list, save, notify } = vi.hoisted(() => ({ list: vi.fn(), save: vi.fn(), notify: vi.fn() }))
vi.mock('@/api/admin/pelican', () => ({ listTestQuestions: list, saveTestQuestion: save }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: notify }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
const route = reactive({ query: { kind: 'intelligence' } })
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace: ({ query }: { query: typeof route.query }) => { route.query = query } }) }))
const question = { id: 7, test_kind: 'intelligence', name: 'Original', prompt: 'Return a number', expected_answer: '42', enabled: true, updated_at: '2026-11-14T00:00:00Z' }
const create = () => mount(TestQuestionsView, { global: { stubs: { AppLayout: { template: '<div><slot/></div>' }, Icon: true, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' } } } })
beforeEach(() => { vi.clearAllMocks(); route.query.kind = 'intelligence'; list.mockResolvedValue([{ ...question }]); save.mockResolvedValue(question) })
describe('question library maintenance', () => {
  it('edits and disables a question and switches between independent libraries', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.get('article button').trigger('click')
    await wrapper.get('#test-question-name').setValue('Updated')
    await wrapper.get('#test-question-answer').setValue('7')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(save).toHaveBeenCalledWith(7, expect.objectContaining({ name: 'Updated', expected_answer: '7', test_kind: 'intelligence' }))
    await wrapper.findAll('article button')[1].trigger('click'); await flushPromises()
    expect(save).toHaveBeenLastCalledWith(7, expect.objectContaining({ enabled: false }))
    const pelicanTab = wrapper.findAll('button').find(button => button.text() === 'pelican.library.pelicanBank')!
    await pelicanTab.trigger('click'); await flushPromises()
    expect(list).toHaveBeenLastCalledWith('pelican')
    wrapper.unmount()
  })
  it('keeps failed edits open and exposes a retry after list failure', async () => {
    list.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create(); await flushPromises()
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    await wrapper.get('header button').trigger('click')
    await wrapper.get('#test-question-name').setValue('New')
    await wrapper.get('#test-question-prompt').setValue('New prompt')
    await wrapper.get('#test-question-answer').setValue('42')
    save.mockRejectedValueOnce(new Error('save failed'))
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
    expect(notify).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
