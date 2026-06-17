import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const copyToClipboard = vi.fn().mockResolvedValue(true)

const messages: Record<string, string> = {
  'keys.endpoints.title': 'API 端点',
  'keys.endpoints.default': '默认',
  'keys.endpoints.copied': '已复制',
  'keys.endpoints.copiedHint': '已复制到剪贴板',
  'keys.endpoints.clickToCopy': '点击可复制此端点',
  'keys.endpoints.speedTest': '测速',
  'keys.endpoints.copyAll': '一键复制',
  'keys.endpoints.copyAllCopied': '全部端点已复制',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

import EndpointPopover from '../EndpointPopover.vue'

describe('EndpointPopover', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('location', {
      origin: 'https://current.example.com',
    })
  })

  it('将说明提示渲染到 URL 上方而不是旧的 title 图标上', () => {
    const wrapper = mount(EndpointPopover, {
      props: {
        apiBaseUrl: 'https://default.example.com/v1',
        customEndpoints: [
          {
            name: '备用线路',
            endpoint: 'https://backup.example.com/v1',
            description: '自定义说明',
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('自定义说明')
    expect(wrapper.text()).toContain('点击可复制此端点')
    expect(wrapper.find('[role="button"]').attributes('title')).toBeUndefined()
    expect(wrapper.find('[title="自定义说明"]').exists()).toBe(false)
  })

  it('点击 URL 后会复制并切换为已复制提示', async () => {
    const wrapper = mount(EndpointPopover, {
      props: {
        apiBaseUrl: 'https://default.example.com/v1',
        customEndpoints: [],
      },
    })

    await wrapper.find('[role="button"]').trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenCalledWith('https://default.example.com/v1', '已复制')
    expect(wrapper.text()).toContain('已复制到剪贴板')
    expect(wrapper.find('button[aria-label="已复制到剪贴板"]').exists()).toBe(true)
  })

  it('点击一键复制按钮后会复制全部端点', async () => {
    const wrapper = mount(EndpointPopover, {
      props: {
        apiBaseUrl: 'https://default.example.com/v1',
        customEndpoints: [
          {
            name: '备用线路',
            endpoint: 'https://backup.example.com/v1',
            description: '自定义说明',
          },
        ],
      },
    })

    await wrapper.find('[data-testid="copy-all-endpoints"]').trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenCalledWith(
      'API 端点: https://default.example.com/v1\n备用线路: https://backup.example.com/v1',
      '全部端点已复制',
    )
    expect(wrapper.text()).toContain('全部端点已复制')
  })

  it('未配置公开端点时使用当前站点地址作为默认端点', async () => {
    const wrapper = mount(EndpointPopover, {
      props: {
        apiBaseUrl: '',
        customEndpoints: [],
      },
    })

    expect(wrapper.text()).toContain('一键复制')
    expect(wrapper.text()).toContain('https://current.example.com')

    await wrapper.find('[data-testid="copy-all-endpoints"]').trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenCalledWith(
      'API 端点: https://current.example.com',
      '全部端点已复制',
    )
  })
})
