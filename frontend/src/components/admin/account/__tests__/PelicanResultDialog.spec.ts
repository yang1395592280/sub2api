import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PelicanResultDialog from '../PelicanResultDialog.vue'

const { getPelicanTest } = vi.hoisted(() => ({ getPelicanTest: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getPelicanTest } } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  template: '<div v-if="show"><slot /></div>'
})

describe('PelicanResultDialog', () => {
  it('runs returned HTML only in a sandboxed iframe with network blocked', async () => {
    getPelicanTest.mockResolvedValue({
      id: 9, account_id: 2, model_id: 'gpt-6-astra', prompt: 'pelican',
      status: 'previewable', response_text: '<!doctype html><html><script>window.x=1</script><head></head><body></body></html>',
      html: '<!doctype html><html><script>window.x=1</script><head></head><body></body></html>',
      error_message: '', latency_ms: 100, created_at: new Date().toISOString()
    })
    const wrapper = mount(PelicanResultDialog, {
      props: { show: true, testId: 9, accountName: 'A' },
      global: { stubs: { BaseDialog: BaseDialogStub } }
    })
    await flushPromises()
    const frame = wrapper.get('iframe')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('srcdoc')).toContain("connect-src 'none'")
    expect(frame.attributes('srcdoc')).toContain("default-src 'none'")
    expect(frame.attributes('srcdoc')!.indexOf('Content-Security-Policy')).toBeLessThan(frame.attributes('srcdoc')!.indexOf('<script>'))
    expect(wrapper.find('script').exists()).toBe(false)
  })
})
