import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CustomFeaturesSettings from '../CustomFeaturesSettings.vue'
const { get, patch, loadStatus } = vi.hoisted(() => ({ get: vi.fn(), patch: vi.fn(), loadStatus: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, patch } }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ fetch: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: vi.fn() }) }))
vi.mock('@/stores/zenxiangLiyu', () => ({ useZenxiangLiyuStore: () => ({ loadStatus }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('central feature patch', () => {
  it('saves only the changed flag without sending the surrounding settings document', async () => {
    get.mockResolvedValue({ data: { flags: { zenxiang_liyu_enabled: true }, menus: [] } })
    patch.mockResolvedValue({ data: { flags: { zenxiang_liyu_enabled: false }, menus: [] } })
    const wrapper = mount(CustomFeaturesSettings); await flushPromises()
    await wrapper.findAll('input')[0]!.setValue(false); await wrapper.get('button').trigger('click'); await flushPromises()
    expect(patch).toHaveBeenCalledWith('/admin/settings/custom-features', { flags: { zenxiang_liyu_enabled: false }, menus: {} })
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(loadStatus).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
