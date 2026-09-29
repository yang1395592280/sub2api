import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PelicanInlinePreview from '../PelicanInlinePreview.vue'

const { getPelicanTest } = vi.hoisted(() => ({ getPelicanTest: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getPelicanTest } } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('PelicanInlinePreview', () => {
  it('loads only near the viewport and unmounts the sandboxed animation when hidden', async () => {
    let onIntersection: IntersectionObserverCallback = () => {}
    const disconnect = vi.fn()
    class FakeIntersectionObserver {
      constructor(callback: IntersectionObserverCallback) { onIntersection = callback }
      observe() {}
      disconnect = disconnect
    }
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
    getPelicanTest.mockResolvedValue({
      id: 42,
      html: '<!doctype html><html><script>window.x=1</script><head></head><body></body></html>'
    })
    const wrapper = mount(PelicanInlinePreview, { props: { testId: 42 } })
    expect(getPelicanTest).not.toHaveBeenCalled()

    onIntersection([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    await flushPromises()
    expect(getPelicanTest).toHaveBeenCalledWith(42)
    const frame = wrapper.get('iframe')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('srcdoc')!.indexOf('Content-Security-Policy')).toBeLessThan(frame.attributes('srcdoc')!.indexOf('<script>'))
    expect(frame.classes()).toContain('pointer-events-none')
    expect(frame.attributes('style')).toContain('width: 1280px')
    expect(frame.attributes('style')).toContain('height: 720px')
    expect(frame.attributes('style')).toContain('scale(0.1859375)')

    onIntersection([{ isIntersecting: false } as IntersectionObserverEntry], {} as IntersectionObserver)
    await nextTick()
    expect(wrapper.find('iframe').exists()).toBe(false)
    wrapper.unmount()
    expect(disconnect).toHaveBeenCalled()
    vi.unstubAllGlobals()
  })
})
