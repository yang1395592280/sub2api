import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CustomFeaturesSettings from '../CustomFeaturesSettings.vue'
const { get, patch, loadStatus } = vi.hoisted(() => ({ get: vi.fn(), patch: vi.fn(), loadStatus: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, patch } }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ fetch: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: vi.fn() }) }))
vi.mock('@/stores/zenxiangLiyu', () => ({ useZenxiangLiyuStore: () => ({ loadStatus }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('central feature patch', () => {
  beforeEach(() => vi.clearAllMocks())

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

  it('lists only confirmed custom switches and rejects the restored monitor controls', async () => {
    const flags = { zenxiang_liyu_enabled: true, redeem_code_purchase_enabled: false, join_group_enabled: true, channel_monitor_enabled: true }
    get.mockResolvedValue({ data: { flags, menus: [] } })
    patch.mockResolvedValue({ data: { flags: { ...flags, available_channels_enabled: false }, menus: [] } })
    const wrapper = mount(CustomFeaturesSettings)
    await flushPromises()
    const labels = wrapper.findAll('label')
    for (const key of ['zenxiang_liyu_enabled', 'redeem_code_purchase_enabled', 'join_group_enabled']) {
      expect(labels.map(label => label.text())).toContain(`customFeatures.flags.${key}`)
    }
    for (const key of ['channel_monitor_enabled', 'compact_home_enabled', 'openai_codex_ticket_enabled']) {
      expect(labels.map(label => label.text())).not.toContain(`customFeatures.flags.${key}`)
    }
    await wrapper.get('button:not([disabled])').trigger('click')
    expect(wrapper.emitted('configureCommunity')).toHaveLength(1)
    await labels.find(label => label.text() === 'customFeatures.flags.join_group_enabled')!.get('input').setValue(false)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(patch).toHaveBeenCalledWith('/admin/settings/custom-features', { flags: { join_group_enabled: false }, menus: {} })
    wrapper.unmount()
  })
})
